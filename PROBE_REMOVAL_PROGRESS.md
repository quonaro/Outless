# Outless Probe Removal & External Xray Migration — Progress Report

**Date:** 2026-04-26  
**Goal:** Remove all probe functionality from backend/frontend, run Outless against external Xray API instead of embedded subprocesses, keep only manual sync for source groups.

---

## Context & Decisions

- User confirmed: **no auto probing** — users manually select nodes, probe is not needed.
- Backend becomes **control plane only** (manage routing/clients via Xray API), **data plane is external Xray**.
- For groups with `source_url`: keep only manual sync (`Frontend -> Backend /sync_group`) as bulk DB import.
- Xray runs as separate container in docker-compose, reachable via `xray:10085` (compose network).
- Backend config now has explicit `xray_api` section with `address` and `timeout`.

---

## Completed Changes

### Backend

1. **HTTP API wiring** — removed probe endpoints and handlers from server registration:
   - `internal/adapters/http/server.go` — removed `ProbeJobs` handler and its `Register()`
   - `internal/adapters/http/node_management_handler.go` — removed `ProbeNode` endpoint, `ProbeJobRepository` dependency, probe DTOs and helper functions
   - `internal/adapters/http/group_management_handler.go` — removed probe unavailable endpoints, DTOs, helper functions, and `ProbeJobRepository` dependency

2. **Runtime / Embedded Xray removal** — stopped embedded hub/probe subprocess management:
   - `cmd/outless/main.go` — removed:
     - `NewEmbeddedHubRuntime(...).Start/Reload/Stop`
     - `NewProbeRuntimePool(...).Start/Stop`
     - monitor/checker worker (`runMonitorWorker`)
     - monitor service and job runner instantiation
     - agent logger (no longer needed)
   - `router.Manager` now receives `nil` runtime (external Xray only)

3. **Realtime WebSocket** — disabled probe actions:
   - `internal/adapters/http/realtime_handler.go` — `probe_unavailable` and `probe_unavailable_state` actions now return error `"probe feature removed"`

4. **Configuration** — added explicit external Xray API settings:
   - `pkg/config/config.go` — added `XrayAPIConfig` struct with `Address` and `Timeout`, added defaults and validation
   - `cmd/outless/main.go` — added `XrayAPIAddress` and `XrayAPITimeout` to runtime config
   - `outless.yaml` — added `xray_api` section, set `address: "xray:10085"` (compose network), `timeout: "5s"`

5. **Docker Compose** — added dedicated Xray container:
   - `docker-compose.yaml` — added `xray` service:
     - image: `ghcr.io/xtls/xray-core:latest`
     - config mount: `./xray/config.json:/etc/xray/config.json:ro`
     - ports: `443:443`
     - outless now depends on `xray`, no longer publishes port 443

### Frontend

1. **Pages / UI** — removed probe actions and forms:
   - `app/pages/nodes.vue` — removed probe mutation, probe node form, localStorage persistence, probe UI buttons and panels
   - `app/components/GroupAccordion.vue` — removed probe mutation, probe handler wiring, probe state functions (now no-ops), removed probe props binding to `GroupAccordionItem`
   - `app/components/GroupAccordionItem.vue` — removed probe panel/form templates, probe state helpers, probe-related props and events

2. **Services** — removed probe API clients:
   - `app/utils/services/node.ts` — removed `probeNode()`, `fetchProbeJobStatus()`, probe DTOs, removed now-unused `unwrapBody` helper

3. **Composables** — simplified to sync-only:
   - `app/composables/nodes/useProbeJobNodePatch.ts` — turned into no-op (compatibility layer)
   - `app/composables/groups/useGroupSync.ts` — removed all probe state/actions, WS handlers, localStorage persistence, kept only sync logic

---

## Remaining Dead Code (Not Yet Removed)

### Backend

- **Files still present but unused:**
  - `internal/adapters/http/probe_job_handler.go` — handler not registered in server
  - `internal/adapters/postgres/gorm_probe_job_repository.go` — repository not used anywhere
  - `internal/app/monitor/*` — monitor service and job runner not started
  - `internal/app/nodeprobe/*` — not used
  - `internal/adapters/xray/probe_runtime.go`, `probe_pool.go`, `probe_engine_pool.go` — embedded probe runtime files
  - `internal/domain/probe_job.go` — probe job domain types
  - `internal/domain/errors.go` — `ErrProbeJobNotFound` sentinel
  - `internal/domain/ports.go` — `ProbeJobRepository` interface

- **Partial cleanup needed in:**
  - `internal/adapters/http/realtime_handler.go` — still contains `runGroupProbeUnavailable` and probe event types (unused after WS action removal)
  - `internal/app/public/service.go` — still contains `ProbeUnavailable` methods (unused after monitor removal)

### Frontend

- **Files still present but unused:**
  - `app/utils/groups/probe-storage.ts` — probe state localStorage utilities
  - `app/utils/groups/probe-utils.ts` — probe status/mode normalizers
  - `app/utils/groups/sync-ws-handler.ts` — contains probe event handlers (still referenced by sync but probe parts unused)
  - `app/utils/services/group.ts` — probe DTOs and `probeUnavailableGroupNodes`, `fetchGroupProbeUnavailableState` (unused)
  - `app/composables/nodes/useProbeJobNodePatch.ts` — currently no-op, can be removed if no callers remain

- **Stub functions to clean:**
  - `app/components/GroupAccordion.vue` — `retryNode`, `probeUnavailable`, `canProbeUnavailable`, `nodeProbeState` are stubs, props removed from bindings but functions still present
  - `app/components/GroupAccordionItem.vue` — still has `probingIds` prop, can be removed entirely

---

## Next Steps for Cleanup

### Backend

1. Delete unused files:
   - `internal/adapters/http/probe_job_handler.go`
   - `internal/adapters/postgres/gorm_probe_job_repository.go`
   - `internal/app/monitor/` directory
   - `internal/app/nodeprobe/` directory
   - `internal/adapters/xray/probe_runtime.go`, `probe_pool.go`, `probe_engine_pool.go`
   - `internal/domain/probe_job.go`
   - Update `internal/domain/errors.go` to remove `ErrProbeJobNotFound`
   - Update `internal/domain/ports.go` to remove `ProbeJobRepository` interface

2. Clean `internal/adapters/http/realtime_handler.go`:
   - Remove `runGroupProbeUnavailable` function
   - Remove probe-related types (`GroupProbeUnavailableState`, etc.)

3. Clean `internal/app/public/service.go`:
   - Remove `ProbeUnavailableGroup` and related methods

4. Update `README.md`:
   - Remove references to embedded Xray, probe, monitor worker
   - Document external Xray requirement and `xray_api` config section

### Frontend

1. Delete unused files:
   - `app/utils/groups/probe-storage.ts`
   - `app/utils/groups/probe-utils.ts`
   - Remove probe event handling from `app/utils/groups/sync-ws-handler.ts` (or simplify to sync-only)
   - Remove probe DTOs and methods from `app/utils/services/group.ts`
   - Delete `app/composables/nodes/useProbeJobNodePatch.ts`

2. Clean component stubs:
   - Remove `retryNode`, `probeUnavailable`, `canProbeUnavailable`, `nodeProbeState` from `GroupAccordion.vue`
   - Remove `probingIds` prop from `GroupAccordionItem.vue`
   - Remove any remaining probe-related template bindings

3. Run `pnpm run typecheck` and `pnpm run lint` to ensure no broken imports

---

## Configuration Notes

- `outless.yaml` now requires `xray_api.address` (validated at startup)
- In docker-compose, backend connects to Xray via service name: `xray:10085`
- For local development outside docker, set `xray_api.address` to `127.0.0.1:10085` (or external Xray host)

---

## Verification Steps

After cleanup:

1. Backend:
   - `go test ./...` should pass (excluding pre-existing unrelated failures)
   - `go build ./cmd/outless` should succeed
   - `docker compose config` should be valid

2. Frontend:
   - `pnpm run typecheck` should pass
   - `pnpm run lint` should pass
   - `pnpm run build` should succeed

3. Integration:
   - Start stack: `docker compose up -d`
   - Verify `outless` logs show successful connection to Xray API
   - Verify manual group sync still works via frontend
   - Verify no probe actions/buttons visible in UI

---

## Important Files Modified

### Backend
- `cmd/outless/main.go`
- `internal/adapters/http/server.go`
- `internal/adapters/http/node_management_handler.go`
- `internal/adapters/http/group_management_handler.go`
- `internal/adapters/http/realtime_handler.go`
- `pkg/config/config.go`
- `outless.yaml`
- `docker-compose.yaml`

### Frontend
- `app/pages/nodes.vue`
- `app/components/GroupAccordion.vue`
- `app/components/GroupAccordionItem.vue`
- `app/utils/services/node.ts`
- `app/composables/nodes/useProbeJobNodePatch.ts`
- `app/composables/groups/useGroupSync.ts`
