#!/bin/bash
set -euo pipefail

SA_PASSWORD="${1:?SA_PASSWORD required}"

# Start MSSQL in background
echo "Starting MSSQL..."
/opt/mssql/bin/sqlservr &
MSSQL_PID=$!

# Wait for MSSQL to be ready (up to 60 seconds)
echo "Waiting for MSSQL to start..."
for i in $(seq 1 30); do
  if /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "${SA_PASSWORD}" -No -Q "SELECT 1" 2>/dev/null; then
    echo "MSSQL is ready."
    break
  fi
  echo "Attempt $i/30..."
  sleep 2
done

# Restore AdventureWorksLT
echo "Restoring AdventureWorksLT..."
/opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "${SA_PASSWORD}" -No \
  -Q "RESTORE DATABASE AdventureWorksLT FROM DISK = '/tmp/AdventureWorksLT.bak' WITH MOVE 'AdventureWorksLT2022_Data' TO '/var/opt/mssql/data/AdventureWorksLT.mdf', MOVE 'AdventureWorksLT2022_Log' TO '/var/opt/mssql/data/AdventureWorksLT_log.ldf'"
echo "Restore complete."

# Shut down MSSQL gracefully
/opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "${SA_PASSWORD}" -No -Q "SHUTDOWN WITH NOWAIT"
wait $MSSQL_PID 2>/dev/null || true
echo "MSSQL stopped. Image ready."
