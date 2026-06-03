#!/usr/bin/env bash
# Common download and caching utilities for sample dataset scripts.
# Source this file from dataset load scripts: source "$(dirname "$0")/download-common.sh"
set -euo pipefail

# Cache directory: resolves to test/testdata/cache/ relative to scripts directory
CACHE_DIR="$(cd "$(dirname "$0")/../testdata/cache" && pwd)"

# ensure_cache_dir creates the cache directory if it does not exist.
ensure_cache_dir() {
    mkdir -p "$CACHE_DIR"
}

# download_if_missing downloads a URL to the cache directory if the file
# does not already exist. Uses curl with retry for resilient downloads.
# Arguments:
#   $1 - URL to download
#   $2 - Output filename (within CACHE_DIR)
download_if_missing() {
    local url="$1"
    local filename="$2"
    local output_path="${CACHE_DIR}/${filename}"

    if [ -f "$output_path" ]; then
        echo "Cache hit: ${filename} already exists at ${output_path}"
        return 0
    fi

    echo "Downloading ${filename} from ${url}..."
    curl -fSL --retry 3 --retry-delay 5 -o "$output_path" "$url"
    echo "Downloaded ${filename} to ${output_path}"
}
