# Contributing to Outless

Thanks for your interest. This document describes the rules we follow.

## What is Outless

Outless is a backend for managing VLESS nodes and tokenized access. Key feature: managing external Xray via gRPC API instead of embedding Xray into the codebase.

Architecture: Clean Architecture / Hexagonal (ports and adapters).

---

## Architectural Principles

### 1. Clean Architecture (Ports & Adapters)

```
cmd/           → entry points, wiring
internal/
  domain/      → entities, errors, domain interfaces (ports)
  app/         → use cases, orchestration
  adapters/    → concrete implementations
    postgres/  
    http/
    grpc/
    xray/      → gRPC API client
```

**Rule:** Adapters depend on ports (interfaces) defined in domain. Never the reverse.

### 2. Interface-Driven Design

Interfaces are defined at the consumer side:

```go
// domain/ports.go — defined by the service
type NodeRepository interface {
    GetByID(ctx context.Context, id int64) (Node, error)
}

// adapters/postgres/node.go — implementation
func (r *NodeRepo) GetByID(ctx context.Context, id int64) (domain.Node, error)
```

### 3. Dependency Injection

All dependencies via constructors. No global vars, no `init()` with side effects:

```go
func NewService(repo NodeRepository, log *slog.Logger) *Service {
    return &Service{repo: repo, log: log}
}
```

### 4. Contexts and Concurrency

- First parameter is always `context.Context`
- Respect cancellation
- Graceful shutdown: listen for signals, drain workers, close in reverse order
- No "naked" goroutines — use `errgroup` or structured concurrency

---

## Code Style

### Go

- Package names: short, lowercase, no underscores (`node`, not `node_repo`)
- Interface names: `-er` suffix for single-method (`Reader`, `Checker`)
- Constructors: `NewT()` or `NewT(dep1, dep2)`
- Acronyms: CAPS (`URL`, `ID`, `UUID`, `HTTP`)
- Exported identifiers must have doc comments

### Logging

- Use `log/slog` only, structured JSON in production
- Context-aware: `logger.With(slog.String("node_id", id))`
- No `fmt.Printf` debugging
- Sensitive data (IPs, tokens) must not appear in logs; log hashes or IDs only

### Error Handling

- Wrap with context: `fmt.Errorf("checking node %s: %w", id, err)`
- Sentinel errors for domain cases:
  ```go
  var ErrNodeNotFound = errors.New("node not found")
  ```
- Never swallow errors
- Timeouts via context, not sleep/retry loops

---

## Commits

Conventional Commits:

```
feat: add bulk node import endpoint
fix: handle empty group list in subscription API
docs: update Xray config examples
refactor: extract VLESS URL builder to domain package
test: add cases for country code resolution
```

---

## PR Process

1. Fork → feature branch → PR
2. CI must pass (tests, lint)
3. Review from maintainer
4. Merge to main — via PR only, no direct pushes

---

## Local Development

```bash
# 1. Infrastructure
docker compose up -d

# 2. Tests
go test ./...

# 3. Run
cp outless.yaml.example outless.yaml
# Edit configuration
go run ./cmd/outless -config outless.yaml
```

---

## Questions?

Open an issue or discussion. Keep it technical and concise.
