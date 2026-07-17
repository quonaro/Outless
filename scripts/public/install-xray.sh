#!/usr/bin/env bash
# install-simple.sh — one-click Xray-core installer for VLESS+REALITY+mldsa65.
# Generates all credentials automatically (UUID, REALITY keys, mldsa65 keys, shortIds).
#
# Usage (run as root):
#   ./install-simple.sh
#   ./install-simple.sh --target yandex.ru:443 --sni yandex.ru --port 44333

set -euo pipefail

# ---------------------------------------------------------------------------
# User-tunable defaults. Empty values are generated automatically.
# ---------------------------------------------------------------------------
PORT=""                                        # random 10000-65535 if empty
REALITY_TARGET="yandex.ru:443"                 # camouflage destination
REALITY_SNI="yandex.ru"                        # SNI / serverName
FINGERPRINT="chrome"
SPIDERX="/"
FLOW="xtls-rprx-vision"
EMAIL=""                                       # generated from UUID if empty

# Leave the following empty to auto-generate.
UUID=""
REALITY_PRIVATE_KEY=""
REALITY_PUBLIC_KEY=""
MLDSA65_SEED=""
MLDSA65_VERIFY=""
SHORT_IDS=()

MAX_TIMEDIFF=0
XVER=0
SHOW=false
MIN_CLIENT_VER=""
MAX_CLIENT_VER=""

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
# When run as a file, place outputs next to the script. When run via
# "curl ... | bash", $0 is not a file path, so fall back to $PWD.
if [[ -f "$0" ]]; then
  SCRIPT_DIR="$(cd -- "$(dirname "$0")" && pwd)"
else
  SCRIPT_DIR="$PWD"
fi
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR_XRAY="/usr/local/etc/xray"
CREDS_FILE="$SCRIPT_DIR/vless-credentials.txt"
LINK_FILE="$SCRIPT_DIR/vless-link.txt"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log_info()  { printf '\e[32m[INFO]\e[0m  %s\n' "$*"; }
log_error() { printf '\e[31m[ERROR]\e[0m %s\n' "$*" >&2; }

die() { log_error "$1"; exit 1; }

url_encode() { jq -nr --arg s "$1" '@uri $s'; }

# ---------------------------------------------------------------------------
# Root privileges check
# ---------------------------------------------------------------------------
ensure_root() {
  [[ $EUID -eq 0 ]] && return 0

  if command -v sudo &>/dev/null && sudo -n true 2>/dev/null; then
    echo "[INFO]  Not root. Restarting with sudo..."

    if [[ -f "$0" ]]; then
      exec sudo bash "$0" "$@"
    else
      # Running via "curl | bash". Re-download to a temp file and re-execute.
      local tmp_script
      tmp_script="$(mktemp /tmp/install-xray.XXXXXX.sh)"
      curl -fsSL "https://raw.githubusercontent.com/quonaro/Outless/main/scripts/public/install-xray.sh" > "$tmp_script"
      chmod +x "$tmp_script"
      exec sudo bash "$tmp_script" "$@"
    fi
  fi

  die "This script must be run as root or with passwordless sudo access."
}

usage() {
  cat <<'USAGE'
Usage: ./install-simple.sh [OPTIONS]

Optional overrides:
  --port NUM                    Server port (random 10000-65535 if omitted)
  --target HOST:PORT            REALITY destination (default: yandex.ru:443)
  --sni SNI                     REALITY SNI / serverName (default: yandex.ru)
  --fingerprint FP              uTLS fingerprint: chrome, firefox, safari, ...
  --spiderx PATH                REALITY spiderX path (default: /)
  --flow FLOW                   VLESS flow (default: xtls-rprx-vision)
  --email EMAIL                 Client email label (default: <uuid-prefix>@outless)

  The following are normally auto-generated, but you may override them:
  --uuid UUID
  --private-key KEY
  --public-key KEY
  --mldsa65-seed SEED
  --mldsa65-verify VERIFY
  --short-ids a,b,c

  -h, --help                    Show this help

Examples:
  ./install-simple.sh
  ./install-simple.sh --port 44333 --fingerprint firefox
USAGE
}

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --port=*)       PORT="${1#*=}"; shift ;;
    --port)         PORT="${2:-}"; shift 2 ;;
    --target=*)     REALITY_TARGET="${1#*=}"; shift ;;
    --target)       REALITY_TARGET="${2:-}"; shift 2 ;;
    --sni=*)        REALITY_SNI="${1#*=}"; shift ;;
    --sni)          REALITY_SNI="${2:-}"; shift 2 ;;
    --fingerprint=*) FINGERPRINT="${1#*=}"; shift ;;
    --fingerprint) FINGERPRINT="${2:-}"; shift 2 ;;
    --spiderx=*)    SPIDERX="${1#*=}"; shift ;;
    --spiderx)      SPIDERX="${2:-}"; shift 2 ;;
    --flow=*)       FLOW="${1#*=}"; shift ;;
    --flow)         FLOW="${2:-}"; shift 2 ;;
    --email=*)      EMAIL="${1#*=}"; shift ;;
    --email)        EMAIL="${2:-}"; shift 2 ;;
    --uuid=*)       UUID="${1#*=}"; shift ;;
    --uuid)         UUID="${2:-}"; shift 2 ;;
    --private-key=*) REALITY_PRIVATE_KEY="${1#*=}"; shift ;;
    --private-key)  REALITY_PRIVATE_KEY="${2:-}"; shift 2 ;;
    --public-key=*) REALITY_PUBLIC_KEY="${1#*=}"; shift ;;
    --public-key)   REALITY_PUBLIC_KEY="${2:-}"; shift 2 ;;
    --mldsa65-seed=*) MLDSA65_SEED="${1#*=}"; shift ;;
    --mldsa65-seed) MLDSA65_SEED="${2:-}"; shift 2 ;;
    --mldsa65-verify=*) MLDSA65_VERIFY="${1#*=}"; shift ;;
    --mldsa65-verify) MLDSA65_VERIFY="${2:-}"; shift 2 ;;
    --short-ids=*)  IFS=',' read -r -a SHORT_IDS <<<"${1#*=}"; shift ;;
    --short-ids)    IFS=',' read -r -a SHORT_IDS <<<"${2:-}"; shift 2 ;;
    -h|--help)      usage; exit 0 ;;
    *)              die "Unknown option: $1 (run with --help)" ;;
  esac
done

# ---------------------------------------------------------------------------
# Validation (only static/user-provided values)
# ---------------------------------------------------------------------------
[[ -n "$REALITY_TARGET" ]] || die "REALITY target is empty"
[[ -n "$REALITY_SNI" ]]    || die "REALITY SNI is empty"
[[ -n "$FINGERPRINT" ]]    || die "uTLS fingerprint is empty"
[[ -z "$PORT" || "$PORT" =~ ^[0-9]+$ ]] || die "Invalid port: $PORT"

# ---------------------------------------------------------------------------
# OS / package manager detection
# ---------------------------------------------------------------------------
PKG_MGR=""
OS=""

detect_os() {
  if [[ ! -f /etc/os-release ]]; then
    die "Cannot detect OS: /etc/os-release not found"
  fi

  # shellcheck source=/dev/null
  . /etc/os-release
  OS="${ID:-unknown}"

  case "$OS" in
    debian|ubuntu|linuxmint|pop|zorin|elementary)
      PKG_MGR="apt"
      ;;
    fedora|rhel|centos|rocky|almalinux|ol|amzn)
      if command -v dnf &>/dev/null; then
        PKG_MGR="dnf"
      else
        PKG_MGR="yum"
      fi
      ;;
    arch|manjaro|endeavouros|garuda)
      PKG_MGR="pacman"
      ;;
    alpine)
      PKG_MGR="apk"
      ;;
    *)
      die "Unsupported OS: $OS"
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Architecture detection
# ---------------------------------------------------------------------------
XRAY_ARCH=""

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "$machine" in
    x86_64|amd64)        XRAY_ARCH="64" ;;
    aarch64|arm64)       XRAY_ARCH="arm64-v8a" ;;
    armv7l|armv7)         XRAY_ARCH="armv7a" ;;
    *)                   die "Unsupported architecture: $machine" ;;
  esac
}

# ---------------------------------------------------------------------------
# Dependency installation
# ---------------------------------------------------------------------------
install_deps() {
  log_info "Detected OS: $OS / package manager: $PKG_MGR / arch: $XRAY_ARCH"
  log_info "Installing curl, jq, coreutils..."

  case "$PKG_MGR" in
    apt)
      export DEBIAN_FRONTEND=noninteractive
      apt-get update -y
      apt-get install -y curl jq coreutils unzip tar
      ;;
    dnf|yum)
      $PKG_MGR install -y curl jq coreutils unzip tar
      ;;
    pacman)
      pacman -Sy --noconfirm curl jq coreutils unzip tar
      ;;
    apk)
      apk add --no-cache curl jq coreutils unzip tar
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Xray-core installation
# ---------------------------------------------------------------------------
install_xray() {
  log_info "Downloading latest Xray-core..."

  local api_url="https://api.github.com/repos/XTLS/Xray-core/releases/latest"
  local download_url
  download_url="$(curl -fsSL "$api_url" \
    | jq -r --arg arch "$XRAY_ARCH" '
      .assets[]
      | select(.name | test("Xray-linux-\($arch)\\.zip$"))
      | .browser_download_url
    ' | head -n1)"

  [[ -n "$download_url" ]] || die "Could not find Xray asset for linux-$XRAY_ARCH"

  rm -rf /tmp/xray-install
  mkdir -p /tmp/xray-install
  curl -fsSL "$download_url" -o /tmp/xray-install/xray.zip
  unzip -q -o /tmp/xray-install/xray.zip -d /tmp/xray-install/

  install -Dm 755 /tmp/xray-install/xray "$INSTALL_DIR/xray"
  install -Dm 644 /tmp/xray-install/geoip.dat "$INSTALL_DIR/geoip.dat" 2>/dev/null || true
  install -Dm 644 /tmp/xray-install/geosite.dat "$INSTALL_DIR/geosite.dat" 2>/dev/null || true

  rm -rf /tmp/xray-install
  log_info "Xray-core installed to $INSTALL_DIR/xray"
}

# ---------------------------------------------------------------------------
# Credential generation
# ---------------------------------------------------------------------------
random_hex() {
  local bytes="$1"
  od -An -tx1 -N"$bytes" /dev/urandom | tr -d ' \n'
}

generate_port() {
  [[ -n "$PORT" ]] && return 0
  PORT=$(shuf -i 10000-65535 -n 1)
  log_info "Generated port: $PORT"
}

generate_uuid() {
  [[ -n "$UUID" ]] && return 0
  UUID="$("$INSTALL_DIR/xray" uuid 2>/dev/null)"
  [[ -n "$UUID" ]] || die "Could not generate UUID"
  log_info "Generated UUID: $UUID"
}

generate_email() {
  [[ -n "$EMAIL" ]] && return 0
  EMAIL="${UUID%%-*}@outless"
}

generate_reality_keys() {
  [[ -n "$REALITY_PRIVATE_KEY" && -n "$REALITY_PUBLIC_KEY" ]] && return 0

  log_info "Generating REALITY X25519 key pair..."

  local output
  output="$("$INSTALL_DIR/xray" x25519 2>&1)" || die "xray x25519 failed: $output"

  REALITY_PRIVATE_KEY="$(echo "$output" | awk -F': ' '/^PrivateKey:/ || /^Private key:/ {print $2}')"
  # Current Xray labels the public key as "Password" (historically "Public key").
  REALITY_PUBLIC_KEY="$(echo "$output" | awk -F': ' '/^Password:/ {print $2}')"

  [[ -n "$REALITY_PRIVATE_KEY" && -n "$REALITY_PUBLIC_KEY" ]] || die "Could not parse x25519 keys from:\n$output"

  log_info "Generated REALITY public key."
}

generate_mldsa65_keys() {
  [[ -n "$MLDSA65_SEED" && -n "$MLDSA65_VERIFY" ]] && return 0

  log_info "Generating mldsa65 key pair..."

  local output
  output="$("$INSTALL_DIR/xray" mldsa65 2>&1)" || die "xray mldsa65 failed: $output"

  MLDSA65_SEED="$(echo "$output" | awk -F': ' '/^Seed:/ {print $2}')"
  MLDSA65_VERIFY="$(echo "$output" | awk -F': ' '/^Verify:/ {print $2}')"

  [[ -n "$MLDSA65_SEED" && -n "$MLDSA65_VERIFY" ]] || die "Could not parse mldsa65 keys from:\n$output"

  log_info "Generated mldsa65 key pair."
}

generate_shortids() {
  [[ ${#SHORT_IDS[@]} -gt 0 ]] && return 0

  log_info "Generating REALITY shortIds..."

  local i bytes
  for i in {1..8}; do
    bytes=$((RANDOM % 8 + 1))   # 1..8 bytes -> 2..16 hex chars
    SHORT_IDS+=("$(random_hex "$bytes")")
  done
}

generate_all() {
  generate_port
  generate_uuid
  generate_email
  generate_reality_keys
  generate_mldsa65_keys
  generate_shortids
}

# ---------------------------------------------------------------------------
# Xray config generation
# ---------------------------------------------------------------------------
generate_xray_config() {
  log_info "Generating Xray config..."

  local short_ids_json
  short_ids_json="$(printf '%s\n' "${SHORT_IDS[@]}" | jq -R . | jq -s .)"

  mkdir -p "$CONFIG_DIR_XRAY"

  jq -n \
    --arg uuid "$UUID" \
    --arg flow "$FLOW" \
    --arg email "$EMAIL" \
    --argjson port "$PORT" \
    --arg target "$REALITY_TARGET" \
    --arg sni "$REALITY_SNI" \
    --arg private_key "$REALITY_PRIVATE_KEY" \
    --arg public_key "$REALITY_PUBLIC_KEY" \
    --argjson short_ids "$short_ids_json" \
    --arg seed "$MLDSA65_SEED" \
    --arg verify "$MLDSA65_VERIFY" \
    --arg fingerprint "$FINGERPRINT" \
    --arg spider_x "$SPIDERX" \
    --argjson max_timediff "$MAX_TIMEDIFF" \
    --argjson xver "$XVER" \
    --argjson show "$SHOW" \
    --arg min_ver "$MIN_CLIENT_VER" \
    --arg max_ver "$MAX_CLIENT_VER" \
    '{
      log: { loglevel: "warning" },
      inbounds: [
        {
          port: $port,
          protocol: "vless",
          settings: {
            clients: [
              { id: $uuid, flow: $flow, email: $email }
            ],
            decryption: "none"
          },
          streamSettings: {
            network: "tcp",
            security: "reality",
            realitySettings: {
              show: $show,
              xver: $xver,
              target: $target,
              serverNames: [$sni],
              privateKey: $private_key,
              publicKey: $public_key,
              minClientVer: $min_ver,
              maxClientVer: $max_ver,
              maxTimediff: $max_timediff,
              shortIds: $short_ids,
              mldsa65Seed: $seed,
              mldsa65Verify: $verify,
              fingerprint: $fingerprint,
              spiderX: $spider_x
            }
          },
          sniffing: {
            enabled: true,
            destOverride: ["http", "tls", "quic"]
          }
        }
      ],
      outbounds: [
        { protocol: "freedom", tag: "direct" }
      ]
    }' > "$CONFIG_DIR_XRAY/config.json"

  log_info "Xray config written to $CONFIG_DIR_XRAY/config.json"
}

# ---------------------------------------------------------------------------
# Systemd service
# ---------------------------------------------------------------------------
create_service() {
  cat > /etc/systemd/system/xray.service <<EOF
[Unit]
Description=Xray Service
Documentation=https://github.com/XTLS/Xray-core
After=network.target nss-lookup.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/xray run -config $CONFIG_DIR_XRAY/config.json
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable xray.service
}

# ---------------------------------------------------------------------------
# Start
# ---------------------------------------------------------------------------
start_service() {
  log_info "Starting xray.service..."
  systemctl restart xray.service
  sleep 1
  systemctl status xray.service --no-pager || true
}

# ---------------------------------------------------------------------------
# Share link + credentials
# ---------------------------------------------------------------------------
build_link() {
  local ip="$1"
  local sid="${SHORT_IDS[0]}"
  local remark params
  remark="$(url_encode "${EMAIL}")"

  params="security=reality&encryption=none&fp=${FINGERPRINT}&pbk=${REALITY_PUBLIC_KEY}&sid=${sid}&spx=$(url_encode "$SPIDERX")&flow=${FLOW}&type=tcp&headerType=none&sni=${REALITY_SNI}&pqv=$(url_encode "$MLDSA65_VERIFY")"

  echo "vless://${UUID}@${ip}:${PORT}?${params}#${remark}"
}

save_credentials() {
  local ip
  ip="$(curl -4 -fsSL https://icanhazip.com 2>/dev/null || curl -4 -fsSL https://api.ipify.org 2>/dev/null || echo "YOUR_SERVER_IP")"

  {
    echo "============================================================"
    echo " Generated VLESS+REALITY credentials"
    echo "============================================================"
    echo "Server IP     : $ip"
    echo "Port          : $PORT"
    echo "UUID          : $UUID"
    echo "Email         : $EMAIL"
    echo "Flow          : $FLOW"
    echo "SNI           : $REALITY_SNI"
    echo "Target        : $REALITY_TARGET"
    echo "Fingerprint   : $FINGERPRINT"
    echo "SpiderX       : $SPIDERX"
    echo "Private key   : $REALITY_PRIVATE_KEY"
    echo "Public key    : $REALITY_PUBLIC_KEY"
    echo "mldsa65Seed   : $MLDSA65_SEED"
    echo "mldsa65Verify : $MLDSA65_VERIFY"
    echo "ShortIds      : $(IFS=','; echo "${SHORT_IDS[*]}")"
    echo "============================================================"
    echo "Share link:"
    echo "$(build_link "$ip")"
    echo "============================================================"
  } > "$CREDS_FILE"
}

print_and_save_link() {
  local ip
  ip="$(curl -4 -fsSL https://icanhazip.com 2>/dev/null || curl -4 -fsSL https://api.ipify.org 2>/dev/null || echo "YOUR_SERVER_IP")"

  local link
  link="$(build_link "$ip")"

  printf '%s\n' "$link" > "$LINK_FILE"
  save_credentials

  cat <<EOF

============================================================
 Share link (also saved to $LINK_FILE):
 $link
------------------------------------------------------------
 Server IP     : $ip
 Port          : $PORT
 UUID          : $UUID
 Email         : $EMAIL
 Flow          : $FLOW
 SNI           : $REALITY_SNI
 Target        : $REALITY_TARGET
 Fingerprint   : $FINGERPRINT
 Public key    : $REALITY_PUBLIC_KEY
 First shortId : ${SHORT_IDS[0]}
 mldsa65Verify : $MLDSA65_VERIFY
------------------------------------------------------------
 Full credentials saved to: $CREDS_FILE
============================================================

EOF
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  ensure_root "$@"

  detect_os
  detect_arch
  install_deps
  install_xray
  generate_all
  generate_xray_config
  create_service
  start_service
  print_and_save_link

  log_info "Done. Xray is running on port $PORT."
}

main "$@"
