#!/bin/sh
set -e

# Color codes for log formatting
RESET='\033[0m'
GREEN='\033[32m'
CYAN='\033[36m'
YELLOW='\033[33m'

# Download GeoIP database if not present
GEOIP_DB="/app/tmp/GeoLite2-Country.mmdb"
GEOIP_URL="https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"

if [ ! -f "$GEOIP_DB" ]; then
    printf "${GREEN}[INFO]${RESET} ${CYAN}(entrypoint)${RESET}: GeoIP database not found, downloading...\n"
    curl -L -o "$GEOIP_DB" "$GEOIP_URL"
    printf "${GREEN}[INFO]${RESET} ${CYAN}(entrypoint)${RESET}: GeoIP database downloaded successfully\n"
else
    printf "${GREEN}[INFO]${RESET} ${CYAN}(entrypoint)${RESET}: GeoIP database already exists\n"
fi

# Start the application
printf "${GREEN}[INFO]${RESET} ${CYAN}(entrypoint)${RESET}: Starting Outless application\n"
exec /usr/local/bin/outless "$@"
