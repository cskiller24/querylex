#!/bin/bash
set -e

echo "Loading employee sample database..."

cd /employees-data
mysql -u root -p"$MYSQL_ROOT_PASSWORD" < employees.sql

echo "Employee database loaded successfully."
echo "Verifying data..."
mysql -u root -p"$MYSQL_ROOT_PASSWORD" -t < test_employees_sha2.sql
