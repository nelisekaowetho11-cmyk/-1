
#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

# ==========================================
# PRIME NET VPN - Asset Downloader
# ==========================================

# Set magic variables for current file & directory
__dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
__file="${__dir}/$(basename "${BASH_SOURCE[0]}")"
__base="$(basename "${__file}" .sh)"

DATADIR="${__dir}/data"

# Check required dependencies
check_dependencies() {
    command -v jq >/dev/null 2>&1 || {
        echo >&2 "PRIME NET VPN: jq is required but not installed."
        exit 1
    }

    command -v go >/dev/null 2>&1 || {
        echo >&2 "PRIME NET VPN: Go is required but not installed."
        exit 1
    }

    command -v curl >/dev/null 2>&1 || {
        echo >&2 "PRIME NET VPN: curl is required but not installed."
        exit 1
    }
}

# Download data files
download_dat() {
    if [[ ! -d "$DATADIR" ]]; then
        echo "PRIME NET VPN: Creating data directory..."
        mkdir -p "$DATADIR"
    fi

    echo "=========================================="
    echo "       PRIME NET VPN ASSET DOWNLOADER"
    echo "=========================================="

    echo "Downloading geoip.dat..."
    curl -fsSL https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
        -o "$DATADIR/geoip.dat"

    echo "Downloading geosite.dat..."
    curl -fsSL https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat \
        -o "$DATADIR/geosite.dat"

    echo "Downloading geoip-only-cn-private.dat..."
    curl -fsSL https://raw.githubusercontent.com/Loyalsoldier/geoip/release/geoip-only-cn-private.dat \
        -o "$DATADIR/geoip-only-cn-private.dat"

    echo "=========================================="
    echo "PRIME NET VPN: Downloads completed."
    echo "=========================================="
}

# Main execution
ACTION="${1:-download}"

check_dependencies

case "$ACTION" in
    download)
        download_dat
        ;;
    *)
        echo "PRIME NET VPN: Invalid action: $ACTION"
        exit 1
        ;;
esac
