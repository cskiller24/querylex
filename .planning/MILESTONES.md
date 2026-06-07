# Milestones

## v1.1 Cleanup (Shipped: 2026-06-07)

**Phases completed:** 2 phases, 2 plans, 6 tasks

**Key accomplishments:**

- Complete AI code removal: internal/ai/ package (11 files), 5 CLI handler files, AI command registrations, AI references in 5 retained files, and embedding dead code in memory/ — with full validation suite passing
- Complete removal of all E2E testing infrastructure: test/ directory hierarchy, Docker Compose files, CI workflow, Makefile targets, and documentation references

---

## v1.0 MVP

**Shipped:** 2026-06-06
**Phases:** 3 | **Plans:** 13 | **Files:** 213 | **LOC:** ~39,520

### Accomplishments

1. **Single-binary build** — Removed standalone `querylex-add-db` and `querylex-stats` binaries; unified under single `querylex` binary via goreleaser
2. **Docker Compose E2E infrastructure** — compose.yaml with 5 profiled database services (MySQL, PostgreSQL, MariaDB, MSSQL, SQLite), healthchecks, tmpfs, random ports
3. **Test helper package** — Connect\* functions, WaitForPort, FixtureRunner, RunQuerylex, GenerateDBName across all 5 engines
4. **Sample dataset provisioning** — Employees DB, Chinook, Pagila, Northwind download/load scripts + committed custom fixtures
5. **MySQL E2E test suite** — 7 test functions (~60 sub-tests) with golden files, exit codes, schema validation, per-test isolation, flag combinatorics
6. **MySQL adapter gap closure** — Implemented Validate/Explain/Joins with differentiated error codes + EXPLAIN FORMAT=JSON parsing
7. **Cross-engine E2E expansion** — PostgreSQL, MariaDB, MSSQL, SQLite test packages (7 tests each) with golden files
8. **Cross-engine SQL validation** — 12 patterns × 5 engines matrix
9. **EXPLAIN golden file infrastructure** — Per-engine golden files with volatile field normalization
10. **AI mock server** — Deterministic E2E testing for AI features without real API calls
11. **CI automation** — GitHub Actions 5-engine matrix workflow, Docker caching, JUnit XML, MSSQL pre-built image

### Key Decisions

| Decision | Outcome |
|----------|---------|
| Remove standalone binaries | ✓ Delivered |
| Docker Compose for E2E | ✓ Delivered |
| Real-world + custom test data | ✓ Delivered |
| MySQL-first E2E | ✓ Delivered, then expanded |
| AI mock server instead of real API | ✓ Delivered |

---

*See .planning/milestones/v1.0-ROADMAP.md for full phase details*
*See .planning/milestones/v1.0-REQUIREMENTS.md for full requirement traceability*
