# Outless Backend

Outless is a Go backend for managing node health checks, tokenized access, and Xray-based runtime configuration.

## Why Xray is important here

This project uses the **Xray technology stack** (`xray-core`) as a core transport/runtime component:

- `hub` syncs generated runtime config for Xray edge nodes
- `checker` probes network quality using Xray-compatible paths
- runtime mode supports embedded and external Xray operation

If you work on networking/runtime behavior, assume Xray compatibility is a hard requirement.

## Stack

- Go 1.26.2+
- PostgreSQL
- gRPC
- `xray-core`
- Docker Compose (for local infra)

## Quick start

1. Copy example config:

   ```bash
   cp outless.yaml.example outless.yaml
   ```

2. Start local dependencies:

   ```bash
   docker compose up -d
   ```

3. Run tests:

   ```bash
   go test ./...
   ```

4. Run services:

   ```bash
   go run ./cmd/outless -config outless.yaml
   ```

**Note:** Database migrations are embedded in the binary and applied automatically on startup.

## Async probe jobs (API contract)

Node/group probe actions are asynchronous:

- `POST /v1/nodes/{id}/probe` returns `202 Accepted` with `job_id`
- `POST /v1/groups/{id}/nodes/probe-unavailable` returns `202 Accepted` with `batch_id`
- `GET /v1/probe-jobs/{id}` returns a single job status
- `GET /v1/probe-jobs?status=&group_id=&limit=` lists latest jobs

Execution model:

- API only enqueues probe jobs into `probe_jobs`
- `checker` is the only executor that claims jobs and writes probe results
- Failed jobs are retried automatically (up to 3 attempts)

## Repository standards

- Contribution rules: see `CONTRIBUTING.md`
- Project license: see `LICENSE`

## Third-party licensing note

Outless depends on `xray-core`. Keep upstream license obligations in mind when distributing binaries or modified runtime-related code.
