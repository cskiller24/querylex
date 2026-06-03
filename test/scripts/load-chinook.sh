#!/usr/bin/env bash
# Load the Chinook dataset into SQLite.
# Downloads the Chinook_Sqlite.sql script from github.com/lerocha/chinook-database
# and generates a chinook.db SQLite database file.
#
# Usage:
#   ./load-chinook.sh
#
# The generated chinook.db is placed in test/testdata/cache/.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/download-common.sh"

ensure_cache_dir

DOWNLOAD_URL="https://github.com/lerocha/chinook-database/releases/download/v1.4.5/Chinook_Sqlite.sql"
SQL_SCRIPT="Chinook_Sqlite.sql"
DB_FILE="chinook.db"

download_if_missing "$DOWNLOAD_URL" "$SQL_SCRIPT"

if command -v sqlite3 &>/dev/null; then
    echo "Generating Chinook SQLite database..."
    sqlite3 "${CACHE_DIR}/${DB_FILE}" < "${CACHE_DIR}/${SQL_SCRIPT}"
    echo "Chinook SQLite database created at ${CACHE_DIR}/${DB_FILE}"
else
    echo "Warning: sqlite3 CLI not found — install sqlite3 or generate chinook.db via Go driver at test time."
    echo "The Go modernc.org/sqlite driver can create chinook.db from ${CACHE_DIR}/${SQL_SCRIPT} during test setup."
    exit 0
fi
