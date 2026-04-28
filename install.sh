#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Outless Installation Script ===${NC}"
echo ""

# Default values
DEFAULT_DB_HOST="outless-db"
DEFAULT_DB_PORT="5432"
DEFAULT_DB_NAME="outless"
DEFAULT_DB_USER="outless"
DEFAULT_DB_PASSWORD="$(openssl rand -base64 16 | tr -d '+/=')"

DEFAULT_XRAY_PORT="8443"
DEFAULT_XRAY_API_PORT="10085"

DEFAULT_BACKEND_PORT="41220"
DEFAULT_FRONTEND_PORT="41221"

DEFAULT_REALITY_SNI="www.google.com"
DEFAULT_REALITY_FINGERPRINT="chrome"

# Interactive prompts
read -p "Database host [${DEFAULT_DB_HOST}]: " DB_HOST
DB_HOST=${DB_HOST:-$DEFAULT_DB_HOST}

read -p "Database port [${DEFAULT_DB_PORT}]: " DB_PORT
DB_PORT=${DB_PORT:-$DEFAULT_DB_PORT}

read -p "Database name [${DEFAULT_DB_NAME}]: " DB_NAME
DB_NAME=${DB_NAME:-$DEFAULT_DB_NAME}

read -p "Database user [${DEFAULT_DB_USER}]: " DB_USER
DB_USER=${DB_USER:-$DEFAULT_DB_USER}

read -p "Database password [auto-generated]: " DB_PASSWORD
DB_PASSWORD=${DB_PASSWORD:-$DEFAULT_DB_PASSWORD}

read -p "Xray inbound port [${DEFAULT_XRAY_PORT}]: " XRAY_PORT
XRAY_PORT=${XRAY_PORT:-$DEFAULT_XRAY_PORT}

read -p "Xray API port [${DEFAULT_XRAY_API_PORT}]: " XRAY_API_PORT
XRAY_API_PORT=${XRAY_API_PORT:-$DEFAULT_XRAY_API_PORT}

read -p "Backend port [${DEFAULT_BACKEND_PORT}]: " BACKEND_PORT
BACKEND_PORT=${BACKEND_PORT:-$DEFAULT_BACKEND_PORT}

read -p "Frontend port [${DEFAULT_FRONTEND_PORT}]: " FRONTEND_PORT
FRONTEND_PORT=${FRONTEND_PORT:-$DEFAULT_FRONTEND_PORT}

read -p "REALITY SNI [${DEFAULT_REALITY_SNI}]: " REALITY_SNI
REALITY_SNI=${REALITY_SNI:-$DEFAULT_REALITY_SNI}

read -p "REALITY fingerprint [${DEFAULT_REALITY_FINGERPRINT}]: " REALITY_FINGERPRINT
REALITY_FINGERPRINT=${REALITY_FINGERPRINT:-$DEFAULT_REALITY_FINGERPRINT}

read -p "Public domain for subscription URLs: " PUBLIC_DOMAIN
while [[ -z "$PUBLIC_DOMAIN" ]]; do
    echo -e "${RED}Public domain is required${NC}"
    read -p "Public domain for subscription URLs: " PUBLIC_DOMAIN
done

# Generate REALITY keys
REALITY_PRIVATE_KEY=$(xray x25519 | head -n 1 | awk '{print $3}')
REALITY_PUBLIC_KEY=$(xray x25519 | head -n 1 | awk '{print $4}')
REALITY_SHORT_ID=$(openssl rand -hex 8)

# Generate JWT secret
JWT_SECRET=$(openssl rand -base64 32 | tr -d '+/=')

echo ""
echo -e "${GREEN}Generating configuration files...${NC}"

# Generate .env file
cat > .env <<EOF
# Database
POSTGRES_DB=${DB_NAME}
POSTGRES_USER=${DB_USER}
POSTGRES_PASSWORD=${DB_PASSWORD}

# Xray
XRAY_PORT=${XRAY_PORT}
XRAY_API_PORT=${XRAY_API_PORT}
REALITY_SNI=${REALITY_SNI}
REALITY_PRIVATE_KEY=${REALITY_PRIVATE_KEY}
REALITY_PUBLIC_KEY=${REALITY_PUBLIC_KEY}
REALITY_SHORT_ID=${REALITY_SHORT_ID}
REALITY_FINGERPRINT=${REALITY_FINGERPRINT}

# Backend
BACKEND_PORT=${BACKEND_PORT}
JWT_SECRET=${JWT_SECRET}

# Frontend
FRONTEND_PORT=${FRONTEND_PORT}
NUXT_PUBLIC_API_BASE=https://${PUBLIC_DOMAIN}/api/
EOF
echo -e "${GREEN}✓ .env created${NC}"

# Generate xray.config.json
cat > xray.config.json <<EOF
{
  "api": {
    "tag": "api",
    "services": [
      "HandlerService",
      "LoggerService",
      "StatsService",
      "RoutingService"
    ]
  },
  "inbounds": [
    {
      "tag": "api",
      "listen": "0.0.0.0",
      "port": ${XRAY_API_PORT},
      "protocol": "dokodemo-door",
      "settings": {
        "address": "127.0.0.1"
      }
    }
  ],
  "routing": {
    "domainStrategy": "AsIs",
    "rules": []
  }
}
EOF
echo -e "${GREEN}✓ xray.config.json created${NC}"

# Generate docker-compose.yml
cat > docker-compose.yml <<EOF
services:
  database:
    image: postgres:16-alpine
    container_name: outless-db
    restart: always
    environment:
      POSTGRES_DB: \${POSTGRES_DB}
      POSTGRES_USER: \${POSTGRES_USER}
      POSTGRES_PASSWORD: \${POSTGRES_PASSWORD}
    volumes:
      - ./data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U \${POSTGRES_USER} -d \${POSTGRES_DB}"]
      interval: 5s
      timeout: 5s
      retries: 10
    networks:
      - outless-net

  backend:
    image: quonaro/outless:backend
    container_name: outless-backend
    restart: always
    depends_on:
      database:
        condition: service_healthy
    ports:
      - "\${BACKEND_PORT}:${BACKEND_PORT}"
    environment:
      DATABASE_URL: postgres://\${POSTGRES_USER}:\${POSTGRES_PASSWORD}@database:5432/\${POSTGRES_DB}?sslmode=disable
      JWT_SECRET: \${JWT_SECRET}
      XRAY_API_ADDRESS: xray:\${XRAY_API_PORT}
      REALITY_SNI: \${REALITY_SNI}
      REALITY_PRIVATE_KEY: \${REALITY_PRIVATE_KEY}
      REALITY_PUBLIC_KEY: \${REALITY_PUBLIC_KEY}
      REALITY_SHORT_ID: \${REALITY_SHORT_ID}
      REALITY_FINGERPRINT: \${REALITY_FINGERPRINT}
      REALITY_PORT: \${XRAY_PORT}
      PUBLIC_DOMAIN: \${PUBLIC_DOMAIN}
    volumes:
      - ./data/geoip:/app/tmp
    networks:
      - outless-net

  frontend:
    image: quonaro/outless:frontend
    container_name: outless-frontend
    restart: unless-stopped
    depends_on:
      - backend
    environment:
      NUXT_PUBLIC_API_BASE: https://${PUBLIC_DOMAIN}/api/
    ports:
      - "\${FRONTEND_PORT}:3000"
    networks:
      - outless-net

  xray:
    image: ghcr.io/xtls/xray-core:latest
    container_name: outless-xray
    restart: unless-stopped
    volumes:
      - ./xray.config.json:/etc/xray/config.json:ro
    command: ["run", "-config", "/etc/xray/config.json"]
    ports:
      - "\${XRAY_PORT}:\${XRAY_PORT}"
      - "\${XRAY_API_PORT}:\${XRAY_API_PORT}"
    networks:
      - outless-net

networks:
  outless-net:
    driver: bridge
    name: outless-net
EOF
echo -e "${GREEN}✓ docker-compose.yml created${NC}"

# Generate outless.yaml
cat > outless.yaml <<EOF
# Application settings
app:
  shutdown_gracetime: "10s"
  http_port: ${BACKEND_PORT}
  logs:
    level: "info"
    colored: true
    type: "pretty"
    access: "stdout"
    error: "stderr"

# Authentication
auth:
  admin:
    login: "admin"
    password: "admin"
  jwt:
    secret: "${JWT_SECRET}"
    expiry: "24h"

# Database
database: "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

# GeoIP
geoip:
  db_path: "/app/tmp/GeoLite2-Country.mmdb"
  db_url: "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"
  auto: true
  expiry: "24h"

# Router (Xray edge)
router:
  url_host: "${PUBLIC_DOMAIN}"

  inbound:
    port: ${XRAY_PORT}
    sni: "${REALITY_SNI}"
    public_key: "${REALITY_PUBLIC_KEY}"
    private_key: "${REALITY_PRIVATE_KEY}"
    short_id: "${REALITY_SHORT_ID}"
    fingerprint: "${REALITY_FINGERPRINT}"

  api: "xray:${XRAY_API_PORT}"
  sync_interval: "5s"
  name_template: "{{vless.country_flag}} {{vless.country_short}} | {{vless.host}} | {{vless.group}}"
EOF
echo -e "${GREEN}✓ outless.yaml created${NC}"

# Create data directories
mkdir -p data/geoip
echo -e "${GREEN}✓ data directories created${NC}"

# Set permissions
chmod 600 .env
chmod 644 outless.yaml
chmod 644 xray.config.json
chmod 644 docker-compose.yml
echo -e "${GREEN}✓ permissions set${NC}"

echo ""
echo -e "${GREEN}=== Installation Complete ===${NC}"
echo ""
echo "Generated files:"
echo "  - .env (environment variables)"
echo "  - docker-compose.yml (container orchestration)"
echo "  - xray.config.json (Xray API config)"
echo "  - outless.yaml (application config)"
echo ""
echo "REALITY Keys (save these!):"
echo -e "  Private Key: ${YELLOW}${REALITY_PRIVATE_KEY}${NC}"
echo -e "  Public Key:  ${YELLOW}${REALITY_PUBLIC_KEY}${NC}"
echo -e "  Short ID:     ${YELLOW}${REALITY_SHORT_ID}${NC}"
echo ""
echo "To start the application:"
echo -e "  ${YELLOW}docker-compose up -d${NC}"
echo ""
echo "To stop the application:"
echo -e "  ${YELLOW}docker-compose down${NC}"
echo ""
echo -e "${RED}IMPORTANT: Change the default admin password in outless.yaml!${NC}"
