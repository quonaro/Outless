# Outless Backend

Outless is a **VLESS node management system** with tokenized access control. It manages groups of exit nodes and provides subscription URLs for end users.

**Key architecture decision:** Outless does not embed Xray. Instead, it controls an external Xray instance via gRPC API. This separates concerns: Outless manages business logic (groups, tokens, subscriptions), Xray handles the network layer.

## What Outless Does

- **Node Groups:** Organize VLESS nodes into groups (e.g., "Premium", "Free")
- **Tokenized Access:** Issue access tokens that grant users specific group subscriptions
- **Dynamic Routing:** Configure Xray in real-time via gRPC API to route users through selected exit nodes
- **Subscription API:** Generate VLESS subscription URLs for clients (V2Ray, Nekoray, etc.)
- **GeoIP Resolution:** Automatic country detection for nodes

## Architecture

```
User → Outless API → Xray gRPC API → External Xray Instance
                          ↓
                    VLESS Outbound (exit node)
```

**Components:**
- Go 1.26.2+ backend (Clean Architecture / Hexagonal)
- PostgreSQL for persistence
- External Xray (via gRPC API, not embedded)
- Docker Compose for deployment

**Default Ports:**
- Backend API: `41220`
- Frontend dev: `41221`
- Xray gRPC API: `10085`
- Xray VLESS: `443`

---

## Quick Start (Docker Compose)

Deploy Outless using pre-built images:

### 1. Create Config Files

```bash
mkdir outless && cd outless

# Download compose file
curl -O https://raw.githubusercontent.com/quonaro/outless/main/backend/docker-compose.yaml

# Create Xray config
cat > xray.config.json << 'EOF'
{
  "api": {
    "tag": "api",
    "services": ["HandlerService", "RoutingService"]
  },
  "inbounds": [{
    "tag": "api",
    "listen": "0.0.0.0",
    "port": 10085,
    "protocol": "dokodemo-door",
    "settings": { "address": "127.0.0.1" }
  }],
  "routing": { "domainStrategy": "AsIs", "rules": [] }
}
EOF

# Create Outless config
cat > outless.yaml << 'EOF'
app:
  http_port: 41220
  logs: { level: info, type: pretty }
auth:
  admin: { login: admin, password: CHANGE_ME }
  jwt: { secret: CHANGE_ME_RANDOM_STRING, expiry: 24h }
database:
  url: "postgres://outless:outless@database:5432/outless?sslmode=disable"
router:
  url_host: "your-domain.com"
  inbound:
    port: 443
    address: ":443"
    sni: "www.google.com"
    public_key: "YOUR_PUBLIC_KEY"
    private_key: "YOUR_PRIVATE_KEY"
    short_id: "YOUR_SHORT_ID"
  api: "xray:10085"
  sync_interval: "30s"
EOF
```

### 2. Generate REALITY Keys

```bash
docker run --rm ghcr.io/xtls/xray-core x25519
```

Update `outless.yaml` with the generated keys.

### 3. Start Services

```bash
docker compose up -d
```

**Services:**
| Service | Port | Image |
|---------|------|-------|
| PostgreSQL | 5432 | postgres:16-alpine |
| Xray | 443, 10085 | ghcr.io/xtls/xray-core |
| Outless Backend | 41220 | quonaro/outless:backend |
| Outless Frontend | 41221 | quonaro/outless:frontend |

### 4. Verify

```bash
# Check Xray API
curl http://localhost:10085

# Check Outless health
curl http://localhost:41220/health

# Login and get token
curl -X POST http://localhost:41220/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"login":"admin","password":"CHANGE_ME"}'
```

**Access the UI:** Open `http://localhost:41221` in your browser.

---

## Configuration Reference

### Xray Config (`xray.config.json`)

| Field | Required | Description |
|-------|----------|-------------|
| `api.services` | Yes | Must include `HandlerService` and `RoutingService` for Outless to work |
| `inbounds[0].port` | Yes | gRPC API port (default: 10085) |
| `inbounds[0].listen` | Yes | Bind address (use `0.0.0.0` for Docker) |

### Outless Config (`outless.yaml`)

#### Application (`app`)

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `shutdown_gracetime` | No | `10s` | Graceful shutdown timeout |
| `http_port` | No | `41220` | HTTP API server port |
| `logs.level` | No | `info` | Log level (debug, info, warn, error) |
| `logs.type` | No | `pretty` | Log format (pretty, json) |

#### Authentication (`auth`)

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `admin.login` | Yes | `admin` | Admin username |
| `admin.password` | Yes | - | Admin password (change in production!) |
| `jwt.secret` | Yes | - | JWT signing secret (random string) |
| `jwt.expiry` | Yes | `24h` | Token lifetime |

#### Router (`router`)

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `url_host` | Yes | `localhost` | Domain for subscription URL generation |
| `inbound.port` | Yes | `443` | Listening port for VLESS connections |
| `inbound.sni` | Yes | - | SNI for REALITY handshake |
| `inbound.public_key` | Yes | - | REALITY public key |
| `inbound.private_key` | Yes | - | REALITY private key (base64) |
| `inbound.short_id` | Yes | - | REALITY short ID |
| `inbound.fingerprint` | No | `chrome` | TLS fingerprint |
| `api` | Yes | - | Xray gRPC API address |
| `sync_interval` | No | `30s` | How often to sync DB state to Xray |
| `name_template` | No | - | Template for VLESS URL remarks |

---

## Name Templates

Templates control how VLESS connection names appear in subscription URLs.

### Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `vless.name` | Original node name | `Poland Premium` |
| `vless.host` | Node host IP/domain | `82.22.41.75` |
| `vless.port` | Port | `443` |
| `vless.sni` | SNI | `www.google.com` |
| `vless.security` | Security type | `reality` |
| `vless.flow` | Flow type | `xtls-rprx-vision` |
| `vless.country` | Full country name | `Poland` |
| `vless.country_short` | 2-letter code | `PL` |
| `vless.country_flag` | Flag emoji | `🇵🇱` |
| `vless.group` | Group name | `Premium` |
| `vless.user` | User email | `user@example.com` |

### Syntax

```yaml
# Simple substitution
name_template: "{{vless.country}} | {{vless.group}}"
# Result: "Poland | Premium"

# Fallback to literal
name_template: "{{vless.group|\"Default\"}}"
# Result: "Default" (if group is empty)

# Fallback to another variable
name_template: "{{vless.name|vless.host}}"
# Result: host IP if name is empty
```

---

## API Endpoints

### Authentication
- `POST /api/auth/login` - Get JWT token

### Nodes
- `GET /api/nodes` - List nodes
- `POST /api/nodes` - Add node (VLESS URL)
- `DELETE /api/nodes/:id` - Remove node

### Groups
- `GET /api/groups` - List groups
- `POST /api/groups` - Create group
- `POST /api/groups/:id/sync` - Sync nodes from source URL

### Tokens
- `GET /api/tokens` - List tokens
- `POST /api/tokens` - Create token
- `DELETE /api/tokens/:id` - Revoke token

### Subscription
- `GET /sub/:token` - Get VLESS subscription (base64)

---

## Repository Standards

- Contributing: see `CONTRIBUTING.md`
- License: see `LICENSE`

## Third-party Licensing

Outless depends on `xray-core`. Keep upstream license obligations in mind when distributing binaries.
