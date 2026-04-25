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

## Name Template for Subscription API

The subscription API supports a micro-template system for generating dynamic connection names (remarks) in VLESS URLs. This is configured via the `router.name_template` field in `outless.yaml`.

### Available Variables

All variables are prefixed with `vless.`:

| Variable | Description | Example |
|----------|-------------|---------|
| `vless.name` | Original name from VLESS URL fragment | `Poland Premium` |
| `vless.host` / `vless.ip` | Host IP or domain | `82.22.41.75` |
| `vless.port` | Port number | `443` |
| `vless.sni` | SNI (Server Name Indication) | `www.google.com` |
| `vless.security` | Security type | `reality` |
| `vless.encryption` | Encryption method | `none` |
| `vless.flow` | Flow type | `xtls-rprx-vision` |
| `vless.fp` | Fingerprint | `chrome` |
| `vless.country` | Full country name | `Poland` |
| `vless.country_short` | 2-letter country code | `PL` |
| `vless.country_flag` | Country flag emoji | `🇵🇱` |
| `vless.ping` | Latency in milliseconds | `150` |
| `vless.group` | Group name | `Premium` |
| `vless.user` | Token owner email | `user@example.com` |

### Template Syntax

- `{{var}}` - Simple variable substitution
- `{{var|"default"}}` - Fallback to string literal if variable is empty
- `{{var|other_var}}` - Fallback to another variable if variable is empty

### Examples

```yaml
router:
  name_template: "{{vless.country_flag}} {{vless.country}} | {{vless.group}} | {{vless.ping}}ms"
```

Result: `🇵🇱 Poland | Premium | 150ms`

```yaml
router:
  name_template: "{{vless.name|vless.host}} | {{vless.country_flag}}"
```

Result: If `vless.name` is empty, falls back to `vless.host`: `82.22.41.75 | 🇵🇱`

```yaml
router:
  name_template: "{{vless.country_flag}} {{vless.country_short}} | {{vless.group|\"Default\"}} | {{vless.ping}}ms"
```

Result: `🇵🇱 PL | Premium | 150ms` (or `Default` if group is empty)

### Notes

- Only `vless.country_flag` returns an emoji; all other variables return plain text values
- Empty variables return the original `{{var}}` placeholder unless a fallback is provided
- Template is applied only in the subscription API when generating VLESS URLs

## Repository standards

- Contribution rules: see `CONTRIBUTING.md`
- Project license: see `LICENSE`

## Third-party licensing note

Outless depends on `xray-core`. Keep upstream license obligations in mind when distributing binaries or modified runtime-related code.
