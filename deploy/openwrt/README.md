# OpenWrt packaging for Outless

This directory contains files needed to build an `.ipk` package for OpenWrt.

## Files

- `files/outless.init` — procd init script (`/etc/init.d/outless`)
- `files/outless.config` — UCI config (`/etc/config/outless`)
- `Makefile` — OpenWrt package Makefile

## Quick install (without building ipk)

1. Copy the prebuilt `outless-linux-arm64` binary to the router:

   ```bash
   scp outless-linux-arm64 root@46.181.139.138:/usr/bin/outless
   ssh root@46.181.139.138 chmod +x /usr/bin/outless
   ```

2. Copy init script and config:

   ```bash
   scp files/outless.init root@46.181.139.138:/etc/init.d/outless
   scp files/outless.config root@46.181.139.138:/etc/config/outless
   ssh root@46.181.139.138 chmod +x /etc/init.d/outless
   ```

3. Edit UCI config on the router:

   ```bash
   uci set outless.app.external_host='46.181.139.138'
   uci set outless.jwt.secret='your-64-char-hex-secret'
   uci commit outless
   ```

4. Enable and start:
   ```bash
   /etc/init.d/outless enable
   /etc/init.d/outless start
   ```

## Building `.ipk` with OpenWrt SDK

1. Download the [OpenWrt SDK](https://downloads.openwrt.org/releases/23.05.4/targets/mediatek/mt7622/openwrt-sdk-23.05.4-mediatek-mt7622_gcc-12.3.0_musl.Linux-x86_64.tar.xz) for your target (`mediatek/mt7622`).

2. Extract and enter SDK:

   ```bash
   tar xf openwrt-sdk-*.tar.xz
   cd openwrt-sdk-*
   ```

3. Copy this `deploy/openwrt/` directory into the SDK feeds:

   ```bash
   mkdir -p package/outless
   cp -r /path/to/outless/deploy/openwrt/* package/outless/
   ```

4. Place your prebuilt binary:

   ```bash
   cp /path/to/outless-linux-arm64 package/outless/outless
   ```

5. Update and build:

   ```bash
   ./scripts/feeds update -a
   ./scripts/feeds install -a
   make menuconfig  # select Network -> outless
   make package/outless/compile V=s
   ```

6. The `.ipk` will be in `bin/packages/*/base/`.

### One-liner build script

If you have the SDK extracted at `~/openwrt-sdk`:

```bash
#!/bin/bash
set -euo pipefail

SDK_DIR="${SDK_DIR:-$HOME/openwrt-sdk}"
OUTLESS_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TARGET="mediatek-mt7622"

# Build the binary first
cd "$OUTLESS_DIR"
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
  -trimpath -tags "with_reality_server with_utls" \
  -ldflags="-s -w" -o "$OUTLESS_DIR/outless-linux-arm64" ./cmd/outless

# Copy to SDK
cp "$OUTLESS_DIR/outless-linux-arm64" "$SDK_DIR/package/outless/outless"
cp -r "$OUTLESS_DIR/deploy/openwrt/files" "$SDK_DIR/package/outless/"
cp "$OUTLESS_DIR/deploy/openwrt/Makefile" "$SDK_DIR/package/outless/"

# Build
cd "$SDK_DIR"
make package/outless/compile V=s

# Copy result back
cp "$SDK_DIR/bin/packages/${TARGET}/base/"outless_*.ipk "$OUTLESS_DIR/"
echo "IPK built: $(ls "$OUTLESS_DIR"/outless_*.ipk)"
```

## Firewall

After starting Outless, open the VLESS inbound port in fw4:

```bash
uci add firewall rule
uci set firewall.@rule[-1].name='Allow-Outless-VLESS'
uci set firewall.@rule[-1].src='wan'
uci set firewall.@rule[-1].dest_port='8443'
uci set firewall.@rule[-1].proto='tcp udp'
uci set firewall.@rule[-1].target='ACCEPT'
uci commit firewall
/etc/init.d/firewall reload
```

Replace `8443` with the port configured in the Outless UI for your inbound.

## Notes

- UCI config is parsed at every start into `/etc/outless/config.yaml`.
- The database is stored at `/etc/outless/outless.db` by default.
- Default admin `admin/admin` is created on first start. Change it immediately.
