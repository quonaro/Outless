#!/bin/sh
set -e

# Download GeoIP database if not present
GEOIP_DB="/app/tmp/GeoLite2-Country.mmdb"
GEOIP_URL="https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"

if [ ! -f "$GEOIP_DB" ]; then
    echo "Downloading GeoIP database..."
    curl -L -o "$GEOIP_DB" "$GEOIP_URL"
    echo "GeoIP database downloaded"
else
    echo "GeoIP database already exists"
fi

# Start the application
exec /usr/local/bin/outless "$@"
