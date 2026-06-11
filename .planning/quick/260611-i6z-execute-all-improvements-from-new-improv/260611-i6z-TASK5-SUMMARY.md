# Task 5 Summary: Improve workspace-stats DB Availability Check

**Commit:** (included in final commit)
**Date:** 2026-06-11

## Done

- [x] Added `TestConnect(ctx context.Context, dsn string) error` to `db.Adapter` interface
- [x] Implemented `TestConnect` in all 5 adapters (MySQL, MariaDB, PostgreSQL, MSSQL, SQLite) with 3s timeout
- [x] Added `Connectivity` field to `DatabaseHealth` struct in `run_stats.go`
- [x] Added `checkConnectivity()` function with 30s TTL cache using `sync.Map`
- [x] Integrated connectivity check into `buildHealthReport()`: reads database config, retrieves credentials, builds DSN, tests connection
- [x] Connectivity status mapped: `online`, `unreachable`, `auth_failed`, `timeout`, `not_checked`
- [x] SQLite always reported as `online` (local file)
- [x] Added connectivity to human-readable output (`RenderStatsHuman`)
- [x] Fixed all mock adapters in test files to implement `TestConnect`

## Verification

```bash
go build ./cmd/querylex/     # passes
go vet ./...                  # passes
go test ./... -short -count=1 # all tests pass
```
