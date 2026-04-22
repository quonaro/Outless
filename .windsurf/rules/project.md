---
trigger: manual
---

## 📜 Outless Project Development Rules

### 1. Architecture: Clean Architecture / Hexagonal

- **Layered Structure** (outer → inner):
  - `cmd/` - application entry points (wire up dependencies)
  - `internal/app/` - service layer (business logic, use cases)
  - `internal/ports/` - interfaces for external adapters (driven + driving)
  - `internal/adapters/` - concrete implementations (postgres, grpc, http, xray)
  - `internal/domain/` - core entities, value objects, domain errors
- **Dependency Direction:** Adapters depend on Ports (interfaces defined by domain), never the reverse
- **No Global State:** No package-level vars, no init() side effects, no singletons
- **Constructor Injection:** All dependencies injected via constructors: `NewService(repo Repository, log *slog.Logger)`

### 2. Interface-Driven Design

- Define interfaces at consumer side (accept interfaces, return concrete types)
- Repository interfaces: `NodeRepository`, `TokenRepository`, `ProxyRepository`
- Service interfaces: `HealthChecker`, `ProxyEngine`, `SubscriptionProvider`
- Adapters implement domain-defined interfaces (XrayAdapter implements ProxyEngine)

### 3. Concurrency Patterns

- **Context propagation:** Pass `context.Context` as first param; respect cancellation
- **Worker pools:** For bulk operations (node checking, parsing)
- **Graceful shutdown:** Listen for signals, drain workers, close connections in reverse init order
- **No naked goroutines:** Use `errgroup` or structured concurrency; goroutine must be stoppable
- **Channel ownership:** Only sender closes channel; document buffer sizes

### 4. Error Handling

- Wrap errors with context: `fmt.Errorf("checking node %s: %w", id, err)`
- Sentinel errors for domain cases: `var ErrNodeNotFound = errors.New("node not found")`
- Never swallow errors; log at application boundaries only
- Fail-fast: Timeouts enforced via context, not sleep/retry loops

### 5. Logging & Observability

- Use `log/slog` exclusively; structured JSON in production
- Context-aware logging: `logger.With(slog.String("node_id", id))`
- No printf debugging; use debug level with `slog.Debug()`
- Sensitive data (IPs, tokens) must not appear in logs; log hashes or IDs only

### 6. Technology Stack

- **Go:** 1.26.2+ (use iterators, improved runtime)
- **Database:** PostgreSQL via `pgx/v5/pgxpool` with connection pooling
- **Networking:** `net` package, `io.Copy` for L4 proxy; avoid third-party HTTP clients
- **Xray:** Control exclusively via gRPC API; wrap in adapter implementing domain interface
- **Migrations:** `golang-migrate/migrate` for schema versioning

### 7. Code Style & Naming

- Package names: short, lowercase, no underscores (`node`, `checker`, not `node_checker`)
- Interface names: `-er` suffix for single-method (`Reader`, `Checker`), descriptive for multi-method
- Constructor: `NewT()` or `NewT(dep1, dep2)`; no `New` prefix for factory functions returning interfaces
- Acronyms: all caps (`URL`, `ID`, `UUID`, `HTTP`) even when mixed case
- Exported identifiers must have doc comments

### 8. Testing

- Table-driven tests for all exported functions
- Interface-based mocking: mock implementations in `internal/adapters/mocks/`
- Integration tests for adapters: use testcontainers for PostgreSQL
- Unit tests for domain logic: no I/O, pure functions where possible

### 9. Configuration

- Explicit config struct passed to constructors, not `os.Getenv` scattered in code
- Validate config at startup: fail fast on missing required fields
- Secrets from environment only; never commit to config files

### 10. Package Organization

```
outless/
├── cmd/
│   ├── hub/          # L4 proxy + Xray adapter
│   ├── api/          # Subscription API
│   └── checker/      # Health checker daemon
├── internal/
│   ├── domain/       # entities, errors, domain interfaces (ports)
│   ├── app/          # use cases, orchestration (service layer)
│   └── adapters/     # concrete implementations
│       ├── postgres/
│       ├── http/
│       ├── grpc/
│       └── xray/
├── pkg/              # shared utilities (if any)
└── migrations/

