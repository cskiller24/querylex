// Package testhelper provides reusable test utilities for QueryLex E2E tests.
//
// Connect functions (ConnectMySQL, ConnectPostgreSQL, ConnectMariaDB, ConnectMSSQL)
// resolve DSN from environment variables, wait for the database port to be reachable,
// open a connection, and create a unique per-test database isolated from other tests.
// Each function registers t.Cleanup handlers for connection close and database drop.
//
// WaitForPort performs TCP dial retry with exponential backoff for container readiness.
// FixtureRunner loads SQL fixture files from test/testdata/fixtures/ against a live *sql.DB.
// RunQuerylex wraps os/exec to run the compiled querylex binary and capture output.
// GenerateDBName produces e2e_ prefixed UUIDs for per-test database names.
package testhelper
