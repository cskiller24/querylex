#!/usr/bin/env bash
# Load the Northwind dataset into Microsoft SQL Server.
# Downloads instnwnd.sql from github.com/microsoft/sql-server-samples,
# caches it locally, and loads via sqlcmd.
#
# Usage:
#   ./load-northwind.sh [sqlcmd-connection-params]
#
# Default connection: -S localhost -U sa -P ${MSSQL_SA_PASSWORD:-TestPass123!} -d ${MSSQL_DATABASE:-testdb}
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/download-common.sh"

ensure_cache_dir

DOWNLOAD_URL="https://raw.githubusercontent.com/microsoft/sql-server-samples/master/samples/databases/northwind-pubs/instnwnd.sql"
SQL_FILE="instnwnd.sql"

download_if_missing "$DOWNLOAD_URL" "$SQL_FILE"

SA_PASSWORD="${MSSQL_SA_PASSWORD:-TestPass123!}"
DB_NAME="${MSSQL_DATABASE:-testdb}"

if command -v sqlcmd &>/dev/null; then
    echo "Loading Northwind dataset into MSSQL..."
    sqlcmd -S localhost -U sa -P "$SA_PASSWORD" -d "$DB_NAME" -i "${CACHE_DIR}/${SQL_FILE}" "$@"
    echo "Northwind dataset loaded successfully."
elif command -v /opt/mssql-tools18/bin/sqlcmd &>/dev/null; then
    echo "Loading Northwind dataset into MSSQL (using mssql-tools18)..."
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PASSWORD" -d "$DB_NAME" -i "${CACHE_DIR}/${SQL_FILE}" "$@"
    echo "Northwind dataset loaded successfully."
else
    echo "Warning: sqlcmd not found."
    echo "The Northwind dataset was downloaded to ${CACHE_DIR}/${SQL_FILE}."
    echo "To load it, install mssql-tools or use the Go mssql driver at test time."
    echo ""
    echo "If the URL is incorrect, alternative Northwind sources include:"
    echo "  - https://github.com/microsoft/sql-server-samples (official)"
    echo "  - Various community-maintained Northwind SQL scripts"
    exit 0
fi
