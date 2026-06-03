#!/usr/bin/env bash
# Load the Pagila dataset into PostgreSQL.
# Downloads pagila-schema.sql and pagila-data.sql from github.com/devrimgunduz/pagila,
# caches them locally, and loads schema before data into the target database.
#
# Usage:
#   ./load-pagila.sh [psql-connection-params]
#
# Default connection: -U postgres -h localhost -p 5432 -d testdb
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/download-common.sh"

ensure_cache_dir

SCHEMA_URL="https://raw.githubusercontent.com/devrimgunduz/pagila/master/pagila-schema.sql"
DATA_URL="https://raw.githubusercontent.com/devrimgunduz/pagila/master/pagila-data.sql"
SCHEMA_FILE="pagila-schema.sql"
DATA_FILE="pagila-data.sql"

download_if_missing "$SCHEMA_URL" "$SCHEMA_FILE"
download_if_missing "$DATA_URL" "$DATA_FILE"

# Connection defaults: -U postgres -h localhost -p 5432 -d testdb
CONN_PARAMS="${1:--U postgres -h localhost -p 5432 -d testdb}"

echo "Loading Pagila schema..."
PGPASSWORD="${PG_PASSWORD:-testpass}" psql $CONN_PARAMS -f "${CACHE_DIR}/${SCHEMA_FILE}"

echo "Loading Pagila data..."
PGPASSWORD="${PG_PASSWORD:-testpass}" psql $CONN_PARAMS -f "${CACHE_DIR}/${DATA_FILE}"

echo "Pagila dataset loaded successfully."
