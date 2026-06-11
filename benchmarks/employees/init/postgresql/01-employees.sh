#!/bin/bash
set -e

echo "Loading employee sample database (PostgreSQL)..."

# Run the PostgreSQL version of the employee database
cd /employees-data/postgresql

# Install pgcrypto if not already available
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;" 2>/dev/null || true

# Load the schema and data
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f employees_pg.sql

echo "Employee database loaded successfully (PostgreSQL)."
