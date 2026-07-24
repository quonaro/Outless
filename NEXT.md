# NEXT — Scaling Inbound Load Balancing

## Current: Option B (least-loaded-first sorting)

All inbounds are returned in every subscription response, sorted by active
connection count from `TrafficSnapshot`. Clients that pick the first URL
get the least loaded inbound; the rest serve as fallback.

**Limitations at scale:**

- Clients cache subscriptions. Distribution is frozen between refreshes.
- No hard guarantee on per-inbound token count.
- If many users refresh simultaneously, they may all pick the same inbound.

---

## Option A: Per-token primary inbound + fallback

**When:** 30-50+ users.

Each token gets a `PrimaryInboundID`. Subscription returns all inbounds,
but primary first, rest sorted by load.

### Changes needed

- `Token` entity: add `PrimaryInboundID string` field.
- `TokenRepository`: update `IssueToken` / `Update` to accept primary.
- `SubscriptionService.resolveInboundsForToken`: primary first, then
  sort remaining by `TrafficSnapshot`.
- Assignment strategy: round-robin or weighted by current load at
  token creation time.
- DB migration: add column, backfill existing tokens with round-robin.

### Pros

- Guaranteed distribution: N tokens / M inbounds ≈ N/M per inbound.
- Failover preserved — fallback inbounds still in subscription.

### Cons

- No automatic rebalance if one inbound becomes overloaded.
- Requires DB migration.

---

## Option A+: Primary + background rebalancer

**When:** 50-100+ users.

Same as A, plus a background job that periodically checks distribution
and reassigns primary inbounds.

### Changes needed

- New `internal/service/rebalancer.go`:
  - Runs every N minutes (configurable).
  - Counts active tokens per inbound from `TokenRepository.List`.
  - If imbalance > threshold (e.g. 20% above average), migrates tokens
    from overloaded to underloaded inbounds.
  - Updates `PrimaryInboundID` on migrated tokens.
- Health check: if `InboundStatus` returns "disabled", immediately
  reassign affected tokens to active inbounds.
- Config: `app.rebalancer.interval`, `app.rebalancer.threshold`.

### Pros

- Self-healing distribution.
- Handles inbound failures automatically.

### Cons

- More moving parts.
- Token primary changes mid-session — client needs subscription refresh
  to pick up new primary.

---

## Option C: External load balancer (HAProxy / nginx)

**When:** 100+ users, multiple servers.

Put a TCP load balancer in front of multiple Outless instances. Each
instance runs its own sing-box with one inbound. The LB distributes
connections by least-connections algorithm.

### Changes needed

- No app changes — pure infrastructure.
- HAProxy config: `balance leastconn`, backend = Outless instances.
- Each Outless instance: single inbound, same VLESS UUIDs.
- Shared DB: switch SQLite → Postgres (or use single DB server with
  read replicas).

### Pros

- Battle-tested LB algorithms.
- Horizontal scaling — add more instances.
- No application-level rebalancing logic.

### Cons

- Infrastructure complexity.
- Requires Postgres migration (SQLite write locks at scale).
- All instances must share the same token DB.

---

## Migration path

```
B (current) → A → A+ → C
  10 users    30+   50+   100+
```

Each step is backward-compatible. No breaking changes to subscription
format or API.
