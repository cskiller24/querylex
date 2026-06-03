#!/bin/bash
set -euo pipefail
SERVICE="$1"
CONTAINER_PORT="$2"
HOST_PORT=$(docker compose port "$SERVICE" "$CONTAINER_PORT" 2>/dev/null | cut -d: -f2)
if [ -z "$HOST_PORT" ]; then
  echo "Error: could not resolve port for $SERVICE:$CONTAINER_PORT" >&2
  exit 1
fi
# Output the full DSN based on service type
case "$SERVICE" in
  mysql)     echo "root:testpass@tcp(localhost:${HOST_PORT})/testdb?parseTime=true" ;;
  postgresql) echo "postgres:testpass@localhost:${HOST_PORT}/testdb?sslmode=disable" ;;
  mariadb)   echo "root:testpass@tcp(localhost:${HOST_PORT})/testdb?parseTime=true" ;;
  mssql)     echo "sqlserver://sa:TestPass123!@localhost:${HOST_PORT}?database=testdb&encrypt=false" ;;
  *)         echo "Unknown service: $SERVICE" >&2; exit 1 ;;
esac
