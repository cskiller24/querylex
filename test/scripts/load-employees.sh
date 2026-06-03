#!/usr/bin/env bash
# Load the Employees DB (test_db) into MySQL or PostgreSQL.
# Downloads the dataset from github.com/datacharmer/test_db, caches it,
# and loads it into the target database.
#
# Usage:
#   ./load-employees.sh mysql [connection-params]
#   ./load-employees.sh postgresql [connection-params]
#
# Connection defaults:
#   MySQL:      -u root -p${MYSQL_PWD:-testpass} -h ${MYSQL_HOST:-localhost} -P ${MYSQL_PORT:-3306}
#   PostgreSQL: -U postgres -h ${PG_HOST:-localhost} -p ${PG_PORT:-5432} -d ${PG_DATABASE:-testdb}
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/download-common.sh"

DB_TYPE="${1:-}"
if [ -z "$DB_TYPE" ]; then
    echo "Usage: $0 <mysql|postgresql> [connection-params]"
    exit 1
fi
shift || true

ensure_cache_dir

# Download the Employees DB archive
DOWNLOAD_URL="https://github.com/datacharmer/test_db/archive/refs/heads/master.zip"
ARCHIVE_NAME="test_db-master.zip"
EXTRACT_DIR="${CACHE_DIR}/test_db-extracted"

download_if_missing "$DOWNLOAD_URL" "$ARCHIVE_NAME"

# Extract if not already extracted
if [ ! -d "$EXTRACT_DIR/test_db-master" ]; then
    echo "Extracting ${ARCHIVE_NAME}..."
    unzip -o "${CACHE_DIR}/${ARCHIVE_NAME}" -d "$EXTRACT_DIR"
    echo "Extracted to ${EXTRACT_DIR}/test_db-master"
fi

SQL_DIR="${EXTRACT_DIR}/test_db-master"

case "$DB_TYPE" in
    mysql)
        echo "Loading Employees DB into MySQL..."
        cd "$SQL_DIR"
        mysql -u root -p"${MYSQL_PWD:-testpass}" \
            -h "${MYSQL_HOST:-localhost}" \
            -P "${MYSQL_PORT:-3306}" \
            "$@" < employees.sql
        echo "Employees DB loaded into MySQL successfully."
        ;;
    postgresql)
        echo "Loading Employees DB into PostgreSQL..."
        cd "$SQL_DIR"
        PGPASSWORD="${PG_PASSWORD:-testpass}" psql \
            -U postgres \
            -h "${PG_HOST:-localhost}" \
            -p "${PG_PORT:-5432}" \
            -d "${PG_DATABASE:-testdb}" \
            "$@" -f postgresql/employees.sql
        echo "Employees DB loaded into PostgreSQL successfully."
        ;;
    *)
        echo "Unsupported database type: ${DB_TYPE}. Use 'mysql' or 'postgresql'."
        exit 1
        ;;
esac
