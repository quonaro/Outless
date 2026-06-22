# Traffic & Quota Collection Plan

## Problem

We need per-token traffic statistics and quota enforcement without:
- Opening an extra TCP port for Clash API
- Parsing sing-box text logs

## Solution: Internal sing-box Traffic Collector

sing-box can track per-connection traffic internally through `trafficontrol.Manager`.
If `experimental.clash_api` is configured with an empty `external_controller`,
the HTTP listener is not started, but the traffic manager is still created and
populated. We can access it directly via `box.Router().ClashServer()` type
assertion.

## Architecture

### 1. sing-box config (`internal/adapter/singbox/config.go`)

Add `Experimental` block to generated `option.Options`:

```go
Experimental: &option.ExperimentalOptions{
    ClashAPI: &option.ClashAPIOptions{
        // ExternalController left empty -> no HTTP listener
    },
},
```

### 2. RuntimeController extension (`internal/adapter/singbox/runtime.go`)

After `box.New()` in `rebuildLocked`, extract `trafficManager`:

```go
if clashSrv, ok := instance.Router().ClashServer().(*clashapi.Server); ok {
    r.trafficManager = clashSrv.TrafficManager()
}
```

Add `trafficManager *trafficontrol.Manager` field to `RuntimeController`.

Expose a getter so the collector service can poll it:

```go
func (r *RuntimeController) TrafficSnapshot() *trafficontrol.Snapshot {
    if r.trafficManager == nil {
        return nil
    }
    return r.trafficManager.Snapshot()
}
```

### 3. Domain model (`internal/domain/entities.go`)

Extend `Token`:

```go
type Token struct {
    // ... existing fields ...
    QuotaBytes   *int64     // nil = unlimited
    QuotaPeriod  string     // "day", "month", "" = unlimited
}
```

New entity `TokenUsage`:

```go
type TokenUsage struct {
    TokenID      string
    PeriodStart  time.Time  // truncated to period boundary
    PeriodType   string     // "day" | "month"
    UploadBytes  int64
    DownloadBytes int64
    UpdatedAt    time.Time
}
```

### 4. Repository port (`internal/domain/ports.go`)

New interface:

```go
type TrafficRepository interface {
    RecordUsage(ctx context.Context, usage TokenUsage) error
    GetUsage(ctx context.Context, tokenID string, periodType string, periodStart time.Time) (TokenUsage, error)
    ListUsageByToken(ctx context.Context, tokenID string, periodType string, limit int) ([]TokenUsage, error)
    ResetAllForPeriod(ctx context.Context, periodType string, periodStart time.Time) error
}
```

### 5. SQLite implementation (`internal/adapter/repository/`)

New file `traffic.go`:

- GORM model `tokenUsageModel` mapping to `token_usage` table
- Composite primary key: `(token_id, period_type, period_start)`
- `RecordUsage` does upsert (INSERT ... ON CONFLICT UPDATE)
- Indexes on `(token_id, period_type, period_start)` for fast lookups

Update `db.go` AutoMigrate to include `&tokenUsageModel{}`.

### 6. TrafficCollector service (`internal/service/traffic.go`)

New background service, analogous to `CleanupService`:

```go
type TrafficCollector struct {
    runtimeCtrl     domain.RuntimeController
    trafficRepo     domain.TrafficRepository
    tokenRepo       domain.TokenRepository
    logger          *slog.Logger
    interval        time.Duration
}
```

Run loop:

1. Sleep `interval` (default 30s)
2. Call `runtimeCtrl.TrafficSnapshot()`
3. Iterate `snapshot.Connections`
4. For each connection:
   - Parse `tokenID` from `metadata.User` (`t-<tokenID>-n-<nodeID>`)
   - Read `metadata.Upload.Load()` and `metadata.Download.Load()`
   - Aggregate by tokenID + current period boundary
5. Upsert into `TrafficRepository`
6. For each token with a quota, check if current period usage exceeds `QuotaBytes`
7. If exceeded, call `tokenRepo.Deactivate(tokenID)` and log

**Quota logic:**
- If `QuotaPeriod == "day"`, truncate `now` to midnight UTC.
- If `QuotaPeriod == "month"`, truncate to first of month UTC.
- Sum `UploadBytes + DownloadBytes` for that period.
- If sum > `QuotaBytes`, deactivate.

**Rolling counters:** We store raw per-period aggregates. The collector
periodically snapshots sing-box state. sing-box counters are per-connection and
reset on restart, but our DB accumulates deltas across snapshots.

### 7. HTTP handlers (`internal/adapter/http/`)

Extend `StatsHandler` or create `TrafficHandler`:

- `GET /v1/tokens/{id}/traffic`
  - Query params: `period=day|month`, `limit=N`
  - Returns array of `{period_start, upload_bytes, download_bytes, total_bytes}`

- `GET /v1/stats/traffic`
  - Returns aggregate totals across all tokens for current period

- `PATCH /v1/tokens/{id}/quota`
  - Body: `{"quota_bytes": 10737418240, "quota_period": "month"}`
  - Update token quota fields

### 8. Token handler update (`internal/adapter/http/token.go`)

- Add `quota_bytes` and `quota_period` to create/update request/response structs
- Pass through to `tokenRepo.Update` (requires extending `Update` signature)

### 9. Frontend (out of scope for this plan)

- New page/component for token traffic charts
- Quota input on token create/edit form
- Dashboard widget showing top consumers

## Files to touch

| File | Action |
|---|---|
| `internal/domain/entities.go` | Add `QuotaBytes`, `QuotaPeriod` to `Token`; add `TokenUsage` entity |
| `internal/domain/ports.go` | Add `TrafficRepository`, extend `RuntimeController` with `TrafficSnapshot` |
| `internal/domain/errors.go` | Add `ErrQuotaExceeded` if needed |
| `internal/adapter/singbox/config.go` | Inject `Experimental.ClashAPI` with empty controller |
| `internal/adapter/singbox/runtime.go` | Extract `trafficManager`; expose `TrafficSnapshot()` |
| `internal/adapter/repository/db.go` | Add `tokenUsageModel` to AutoMigrate |
| `internal/adapter/repository/traffic.go` | New file: GORM model + `TrafficRepository` impl |
| `internal/adapter/repository/token.go` | Extend `Update` to accept quota fields; add `SetQuota` helper |
| `internal/service/traffic.go` | New file: `TrafficCollector` background service |
| `internal/adapter/http/stats.go` | Add traffic aggregate endpoint |
| `internal/adapter/http/traffic.go` | New file: token-specific traffic endpoints |
| `internal/adapter/http/token.go` | Add quota fields to DTOs |
| `internal/adapter/http/server.go` | Register new handlers |
| `cmd/` (main) | Wire `TrafficRepository`, `TrafficCollector` into DI |

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| sing-box internal API changes in future versions | Pin version in `go.mod`; add compile-time interface check; test on upgrade |
| SQLite write amplification from frequent upserts | Batch upserts per collection tick; use WAL mode (already enabled) |
| Token deactivation on quota hit disconnects active users | Acceptable; user sees connection drop and knows limit reached |
| Counter reset on sing-box reload/restart | Store DB deltas; on first snapshot after restart compute diff from zero |
| Memory growth from too many `TokenUsage` rows | Add retention job (keep 90 days daily, 12 months monthly) |

## Open Questions

1. Should quota enforcement be **soft** (throttle speed) or **hard** (deactivate)?
   - Plan implements **hard** (deactivate) as simplest.
2. Do we need real-time WebSocket push for traffic updates?
   - Start with polling. Add WS later if UI demands it.
3. Should admins receive alerts when a token hits its quota?
   - Out of scope for MVP. Can be added later via logging/events.

## Estimated Effort

- Backend (Go): ~350–450 lines across ~8 files
- Tests: ~150 lines
- Frontend: separate task, ~1–2 days
- Total backend time: ~4–6 hours
