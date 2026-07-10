# ⚡ Outless

Outless is a monolithic VLESS node and subscription manager. The backend is written in Go and built on top of [`sing-box`](https://github.com/Sagernet/sing-box): it runs VLESS REALITY inbounds, distributes clients across node groups, collects traffic, and serves subscriptions. The web UI is built with Nuxt 4 + shadcn-vue.

---

## ✨ Features

- **VLESS REALITY inbounds** — create and manage server-side entry points directly from the UI.
- **Nodes & groups** — add private or public VLESS nodes and organize them into groups with random selection limits.
- **Tokens & subscriptions** — issue access tokens with traffic quotas, expiry dates, and group/inbound bindings.
- **Public sources** — import third-party node lists by URL and auto-assign them to a group.
- **Traffic & statistics** — collect and display per-token traffic in real time.
- **Multi-admin** — manage administrators with JWT authentication and optional TOTP.
- **OpenWrt** — ready-made init scripts and a guide for building an `.ipk` package.

---

## 🛠️ Tech Stack

- **Backend**: Go 1.26, embedded `sing-box` runtime, SQLite (GORM), Huma/v2 OpenAPI, chi
- **Frontend**: Nuxt 4, Vue 3, TypeScript, Tailwind CSS, shadcn-vue, TanStack Query, Zod
- **API**: REST + Server-Sent Events (logs, connections, system metrics)
- **Build**: Docker multi-stage, Air for Go hot-reload, pnpm

---

## 🚀 Quick Start

### Requirements

- Go 1.26+
- Node.js 22+ and pnpm 9+
- (optional) Docker

### Clone & Build

```bash
git clone https://github.com/quonaro/outless.git
cd outless

# Build frontend
cd frontend
pnpm install
pnpm generate
cd ..

# Build backend with REALITY server and uTLS support
CGO_ENABLED=0 go build -trimpath -tags "with_reality_server with_utls" \
  -ldflags="-s -w" -o outless ./cmd/outless
```

### Configuration

Edit `config.yaml` in the project root:

```yaml
app:
  http_port: 41220
  external_host: "your-server-ip-or-domain"
  log_level: info
  singbox_log_level: warn

jwt:
  secret: "your-64-char-hex-secret-must-be-changed" # make sure to change this
database: ./outless.db
```

> **Security**: on first start a default user `admin` with password `admin` is created. Change it immediately after login.

### Run

```bash
./outless server run
```

Open in your browser: `http://localhost:41220`

---

## 🖥️ CLI

The app uses the custom `lota` CLI engine. Available commands:

| Command                                                | Description                                                |
| ------------------------------------------------------ | ---------------------------------------------------------- |
| `./outless server run`                                 | Start the HTTP server and sing-box runtime                 |
| `./outless server status`                              | Check whether the server is running on the configured port |
| `./outless admin reset-password <username> <password>` | Reset an administrator password                            |
| `./outless config show`                                | Show current configuration (secrets hidden)                |
| `./outless config show true`                           | Show current configuration including secrets               |
| `./outless config validate`                            | Validate configuration without starting the app            |
| `./outless db backup`                                  | Create a `tar.gz` backup of the database                   |
| `./outless version`                                    | Show version                                               |

You can override the config path via environment variable:

```bash
OUTLESS_CONFIG=/etc/outless/config.yaml ./outless server run
```

---

## ⚙️ Configuration

Main parameters in `config.yaml`:

```yaml
app:
  shutdown_gracetime: 10s # graceful shutdown timeout
  http_port: 41220 # web UI and API port
  external_host: "" # host used in subscription URLs
  singbox_log_level: warn # trace/debug/info/warn/error/fatal/panic
  log_level: info # debug/info/warn/error
  disable_docs: false # disable OpenAPI/Swagger UI
  pprof_enabled: false # enable Go profiling
  pprof_bind: "127.0.0.1:6060"

jwt:
  secret: "" # hex string, must be changed
  expiry: 24h0m0s

database: /var/lib/outless/outless.db
```

- `external_host` is used in subscription URLs when the inbound has no `URLHost` set.
- The database is SQLite. For production, store the file under `/var/lib/outless/`.

---

## 🐳 Docker

```bash
# Build
docker build -t outless:latest .

# Run with config and DB mounted
docker run -d \
  --name outless \
  -p 41220:41220 \
  -v $(pwd)/config.yaml:/config.yaml \
  -v $(pwd)/outless.db:/outless.db \
  outless:latest server run
```

The Dockerfile builds a static frontend, compiles the Go binary, and packs everything into a `scratch` image. By default it runs `server run`.

---

## 📡 OpenWrt

OpenWrt files are located in `deploy/openwrt/`:

- `files/outless.init` — procd init script
- `files/outless.config` — UCI config
- `Makefile` — package Makefile for `.ipk` builds

Quick install using a prebuilt binary:

```bash
scp outless-linux-arm64 root@router:/usr/bin/outless
ssh root@router chmod +x /usr/bin/outless
scp deploy/openwrt/files/outless.init root@router:/etc/init.d/outless
scp deploy/openwrt/files/outless.config root@router:/etc/config/outless
ssh root@router chmod +x /etc/init.d/outless

# Configure
ssh root@router uci set outless.app.external_host='your-public-ip'
ssh root@router uci set outless.jwt.secret='your-64-char-hex-secret'
ssh root@router uci commit outless

# Start
ssh root@router /etc/init.d/outless enable
ssh root@router /etc/init.d/outless start
```

More details: [`deploy/openwrt/README.md`](deploy/openwrt/README.md).

---

## 🏗️ Architecture

```text
cmd/outless/              # entry point, CLI commands, wiring
internal/
  adapter/http/           # HTTP handlers, middleware, SSE
  adapter/repository/     # SQLite repositories (GORM)
  adapter/singbox/        # embedded sing-box runtime adapter
  domain/                 # entities, repositories (ports)
  service/                # business logic: auth, subscription, traffic, cleanup...
  utils/                  # helper utilities
shared/
  config/                 # configuration loading and validation
  crypto/                 # cryptography
  logging/                # logger
  template/               # subscription templates
  vless/                  # VLESS URL parsing/building
frontend/                 # Nuxt 4 SPA
web/                      # embedded static assets (dist -> embed)
```

- **Clean Architecture**: the domain layer does not depend on adapters.
- **Embedded sing-box**: the runtime controls inbounds and routing directly, without an external sing-box config.
- **SSE streams**: logs, connections, and system metrics are pushed to the UI via Server-Sent Events.

---

## 💻 Development

### Backend

For hot reload:

```bash
# Install air if you haven't already
go install github.com/cosmtrek/air@latest
air
```

Without Air:

```bash
CGO_ENABLED=0 go build -trimpath -tags "with_reality_server with_utls" \
  -ldflags="-s -w" -o ./tmp/outless ./cmd/outless
./tmp/outless server run
```

Formatting and linting:

```bash
gofmt -w .
goimports -w .
golangci-lint run ./...
```

### Frontend

```bash
cd frontend
pnpm install
pnpm dev          # http://localhost:41221
pnpm typecheck
pnpm lint:all
pnpm format
```

When generating a static site, the frontend expects the API on the same host (`/api`). In dev mode, it proxies to `http://localhost:41220`.

---

## 🔐 Security

- **JWT secret**: make sure to change the default; `config.Validate()` will refuse to start with `CHANGE_ME_IN_PRODUCTION`.
- **Admin password**: the first run creates `admin/admin` — change the password and enable TOTP.
- **TOTP**: two-factor authentication is supported for admins.
- **REALITY**: inbounds use X25519 key pairs and `short_id`.

---

## 📄 License

[MIT](LICENSE)

---

## 👤 Author

Developed by [quonaro](https://github.com/quonaro).
