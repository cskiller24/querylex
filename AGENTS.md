<!-- GSD:project-start source:PROJECT.md -->
## Project

**QueryLex**

QueryLex is a context-aware SQL generator and optimizer CLI tool for AI agents. It provides a comprehensive command interface that helps agents introspect database schemas, generate SQL from natural language, optimize queries through explain-plan analysis, and maintain workspace state across multiple database connections. It exposes a structured JSON envelope for all deterministic commands and integrates with OpenAI-compatible APIs for AI-powered features.

**Core Value:** AI agents can reliably introspect any supported database, generate correct SQL from natural language descriptions, and optimize queries — all through a single CLI tool with structured machine-readable output.

### Constraints

- **Language**: Go (must stay Go, no language migration)
- **Database parity**: All 5 databases must remain supported with equal capability
- **Backward compatibility**: JSON envelope format and command interface must not break existing consumers
- **CGO_ENABLED=0**: Build constraint for portability (already configured in goreleaser)
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.26.3 - The entire codebase: CLI entry points (`cmd/`), core logic (`internal/`)
## Runtime
- Go toolchain (compiled statically with `CGO_ENABLED=0`)
- Go modules (standard library `go mod`)
- Lockfile: `go.sum` present
## Frameworks
- `github.com/spf13/cobra` v1.9.1 - CLI framework for the `querylex` command tree
- Go standard testing (`testing` package + `go test`)
- Test files co-located with source: `*_test.go` files alongside implementations (e.g., `internal/cli/run_sql_test.go`, `internal/db/mysql/adapter_test.go`)
- Test run command: `go test ./... -short -count=1`
- `goreleaser` (via `.goreleaser.yaml`) - Cross-platform release builds for Linux, macOS, Windows (amd64, arm64, 386)
- `golangci-lint` - Linting in CI (`golangci-lint-action@v6`)
- `go vet ./...` - Local linting via Makefile
- `Makefile` - Local dev tasks (build, test, clean, install, lint, release, completions)
## Key Dependencies
- `github.com/go-sql-driver/mysql` v1.10.0 - MySQL/MariaDB driver (used by both `internal/db/mysql/` and `internal/db/mariadb/`)
- `github.com/jackc/pgx/v5` v5.9.2 - PostgreSQL driver (used by `internal/db/postgresql/`)
- `github.com/microsoft/go-mssqldb` v1.10.0 - Microsoft SQL Server driver (used by `internal/db/mssql/`)
- `modernc.org/sqlite` v1.51.0 - Pure-Go SQLite driver (used by `internal/db/sqlite/` and `internal/memory/store.go` for the memory store)
- `github.com/sashabaranov/go-openai` v1.41.2 - OpenAI API client for SQL generation (`internal/ai/client.go`)
- `github.com/spf13/cobra` v1.9.1 - CLI framework, shell completions
- `github.com/zalando/go-keyring` v0.2.8 - OS-native keychain access (macOS Keychain, Windows Credential Manager, Linux Secret Service via D-Bus)
- `github.com/AlecAivazis/survey/v2` v2.3.7 - Interactive terminal prompts (`internal/cli/prompts.go`, `internal/cli/run_ai_config.go`)
- `github.com/google/uuid` v1.6.0 - UUID generation for trace IDs (`internal/format/response.go`)
- `github.com/oklog/ulid/v2` v2.1.1 - ULID generation for memory entry IDs (`internal/memory/store.go`)
- `golang.org/x/crypto` v0.52.0 - `scrypt` key derivation for encrypted credential store (`internal/credentials/encrypted.go`)
- `golang.org/x/term` v0.43.0 - Terminal detection for interactive prompts
- `golang.org/x/sync` v0.20.0 - `errgroup` for parallel context fetching (`internal/cli/run_sql.go`)
- `gopkg.in/yaml.v3` v3.0.1 - YAML parsing (used directly)
## Configuration
- Config files stored in `$HOME/.querylex/`:
- `QUERYLEX_AI_API_KEY` — AI provider API key (inline config, bypasses keychain)
- `QUERYLEX_AI_ENDPOINT` — API endpoint override (default: `https://api.openai.com/v1`)
- `QUERYLEX_AI_MODEL` — Model name (default: `gpt-4o`)
- `QUERYLEX_AI_MAX_TOKENS` — Max context tokens (default: `128000`)
- `QUERYLEX_DB_PASSWORD` — Database password fallback (env store)
- `QUERYLEX_AI_KEY` — AI key fallback (env store)
- `QUERYLEX_KEYCHAIN_PASSPHRASE` — Passphrase for encrypted file store (CI/non-interactive use)
- `Makefile` with ldflags injection: `-X github.com/cskiller24/querylex/internal/version.Version`, `.Commit`, `.BuildDate`
- `.goreleaser.yaml` — release automation (v2 format)
- Go module defined in `go.mod` (module: `github.com/cskiller24/querylex`)
## Platform Requirements
- Go 1.26.3+
- `goreleaser` (for release builds)
- `golangci-lint` (for linting)
- Linux, macOS, Windows (cross-compiled via `CGO_ENABLED=0`)
- Architectures: amd64, arm64, 386
- Deployment targets: `.deb`/`.rpm` packages (Linux), Homebrew tap (macOS), Scoop bucket (Windows), manual tarball/zip
- No runtime dependencies beyond the OS and network access
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- All `.go` source files use `snake_case`: `run_schema.go`, `join_graph.go`, `lock_provider.go`
- Test files use `_test.go` suffix: `run_schema_test.go`, `response_test.go`
- Package directory names are single-word lowercase: `cli`, `db`, `ai`, `state`, `format`, `index`, `credentials`
- Exported functions: PascalCase — `RunSchema`, `NewSuccessResponse`, `PreflightForCommand`
- Unexported functions: camelCase — `runSchemaWithAdapter`, `formatResolveOutput`, `findDatabaseEntry`
- Constructor functions: `New*` prefix — `NewTokenBudget`, `NewFileLock`, `NewFileWorkspaceStore`
- Test helpers use `t.Helper()` annotation and descriptive names: `setupPreflightTestWorkspace`, `createStatsTestManifest`
- Exported sentinel errors: `Err*` prefix — `ErrConnectionFailed`, `ErrNotImplemented`, `ErrUnsupportedDatabase`
- Unexported private vars: camelCase — `knownArtifactPaths`, `artifactStatePresent`
- Constants: CamelCase or UPPER_CASE depending on context —
- Exported structs: PascalCase — `ResponseMeta`, `SchemaResult`, `DatabaseEntry`, `TokenBudget`
- Type aliases: PascalCase — `ErrorCode string`, `DatabaseStatus string`
- Interface names describe behavior: `Adapter`, `WorkspaceStore`, `FileLock`
- All struct fields use `snake_case` JSON tags with `omitempty` where appropriate:
## Code Style
- Standard `gofmt` formatting (no `.prettierrc` or custom formatter config)
- Tabs for indentation (Go standard)
- No external formatter configuration detected
- `go vet ./...` via Makefile target `lint` (line 28 of `Makefile`)
- No `.golangci-lint` or other linting config detected
- No strict limit enforced; practical maximum ~130 columns (observed in `run_sql.go`, `root.go`)
## Import Organization
- Not used (no `import alias "path"` patterns detected)
## Response Envelope Pattern
- Success responses: `format.NewSuccessResponse(data, traceID, activeDBID)`
- Error responses: `format.NewErrorResponse[DataType](code, message, retryable, traceID)`
- Duration tracking: `resp.Complete(startTime)` called before returning
- Trace IDs: Generated via `format.GenerateTraceID()` (UUID v4) or `uuid.New().String()`
## Command Function Pattern
## Error Handling
- Defined at package level as `var ErrXxx = errors.New("description")`:
- Wrap error codes with underlying errors for `errors.Is` / `errors.Unwrap`:
- Use `fmt.Errorf("context: %w", err)` consistently:
- All error codes are typed constants `ErrorCode string` in `internal/format/error.go`
- Used as `format.ErrCodeConnectionFailed`, `format.ErrCodeWorkspaceStateInvalid`, etc.
- Each has a description in `ErrorCodeDescriptions` map for LLM-friendly messages
- `convertSchemaError`, `convertValidateError`, `convertExplainError`, `convertMemoryError` — all follow identical pattern:
- Exit code 1 for general failures
- Exit code 130 (128 + SIGINT) for signal-cancelled contexts (`cmd/querylex/main.go` line 20)
## Logging
- Warnings use `fmt.Fprintf(os.Stderr, "Warning: %s\n", ...)` — non-fatal, do not exit
- Errors use `fmt.Fprintf(os.Stderr, "Error: %s\n", ...)` — followed by `os.Exit(1)` or `return err`
- Human-readable output: `fmt.Printf(...)` for interactive commands like `RunSQLGeneration`
- Response output: `fmt.Println(string(data))` / `fmt.Println(json)` — always stdout
## Comments
- Every package has a `// Package {name} provides ...` comment at the top of its primary file:
- Always documented with `// Name description.` style:
- Single-line // comments used exclusively
- Block comments `/* */` not used
- GoDoc-style comments on all exported identifiers
- Inline comments explain non-obvious logic (e.g., `// Context was cancelled by signal — defers already ran (lock.Release())`)
- Every exported type, function, constant, and variable
- Complex algorithm steps (numbered steps in `PreflightForCommand`, `RunSQLGeneration`)
- Non-obvious design decisions (why a constraint exists, why a fallback is needed)
## Function Design
- Most functions are under 60 lines
- Larger functions (100-200 lines) are data processing (Schema extraction, parsing) with clear sections
- Longest function: `MySQLAdapter.Schema` at ~300 lines (row scanning + aggregation — acceptable complexity for database schema extraction)
- `context.Context` always first parameter for I/O-bound functions
- Adapter/dependencies passed as parameters rather than struct fields (dependency injection style)
- Functions accepting business data use descriptive parameter names
- Functions returning both value and error: `(result, error)` pattern
- Command functions return `*format.Response[T]`
- Preflight functions return `(*PreflightResult, *format.Response[any])` — tuple of result and potential error
- Named return values: not used
## Module Design
- Each `internal/` package exposes a focused set of exported types and functions
- Package `format` exports: `Response`, `ResponseMeta`, `ErrorDetail`, `Warning`, `ErrorCode`, all error code constants, constructors
- Package `db` exports: `Adapter` interface, `Register`/`Open` factory, all result types in `types.go`
- Not used; each package has distinct primary files (`response.go`, `adapter.go`, `types.go`)
- Types are defined near where they're primarily used, not centralized
## Interface Patterns
- Registration via `db.Register(name, factory)` in `init()` functions
- Dialect adapters imported via blank-import `_ "github.com/cskiller24/querylex/internal/db/mysql"`
- Two implementations: `FileWorkspaceStore` (production, file-backed) and `InMemoryWorkspaceStore` (testing)
## Context Usage
- `context.Background()` used at entry points (`main.go`, command invocations)
- `context.WithTimeout(ctx, duration)` for all database operations (10s for connect/ping, 30s for queries)
- `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` for graceful shutdown in `cmd/querylex/main.go`
- `errgroup.WithContext(ctx)` for parallel context fetching in `RunSQLGeneration`
- Context cancellation checked: `if ctx.Err() != nil { ... }` in main signal handler
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## System Overview
```text
```
## Component Responsibilities
| Component | Responsibility | File |
|-----------|----------------|------|
| `rootcmd` | Shared Cobra command definitions; imported by both `querylex` binary and completion generator | `internal/rootcmd/root.go` |
| `cli/preflight` | Workspace loading, active DB resolution, status gating, DSN construction, adapter connection | `internal/cli/preflight.go` |
| `cli/run_*.go` | Per-subcommand handler functions (validate, explain, schema, optimize, etc.) | `internal/cli/run_*.go` |
| `db` | Adapter interface, types, registry factory — the database abstraction layer | `internal/db/adapter.go`, `internal/db/types.go`, `internal/db/factory.go` |
| `db/mysql` (et al.) | Database-specific SQL adapters implementing the `db.Adapter` interface | `internal/db/mysql/adapter.go`, etc. |
| `ai` | OpenAI client wrapper, chat completions, prompt builders, token budgeting | `internal/ai/client.go`, `internal/ai/chat.go`, `internal/ai/prompt.go`, `internal/ai/tokens.go` |
| `index` | Schema indexing pipeline (6 phases), join graph, domain clustering, manifest | `internal/index/pipeline.go`, `internal/index/schema.go`, `internal/index/schema_map.go` |
| `memory` | SQLite-backed persistent query memory with keyword index, keyword scoring | `internal/memory/store.go`, `internal/memory/search.go`, `internal/memory/scoring.go` |
| `state` | Workspace CRUD, atomic file writes, advisory flock-based locking, crash recovery | `internal/state/workspace.go`, `internal/state/atomic.go`, `internal/state/lock.go` |
| `credentials` | Multi-backend secret storage: OS keychain, AES-256-GCM encrypted file, env vars | `internal/credentials/store.go`, `internal/credentials/factory.go` |
| `format` | Standardized JSON response envelope (`Response[T]`), error codes, trace IDs | `internal/format/response.go`, `internal/format/error.go` |
| `queryutil` | Natural language token resolution (5-pass), SQL safety validation (DML/DCL block) | `internal/queryutil/resolve.go` |
| `analysis` | Heuristic explain plan analysis (full scans, non-sargable predicates, etc.) | `internal/analysis/heuristics.go` |
| `cache` | Fingerprint-based explain plan cache with TTL-based invalidation | `internal/cache/explain_cache.go` |
| `version` | Build-time injected version metadata (via ldflags) | `internal/version/version.go` |
## Pattern Overview
- Single cobra `RootCmd` defined in `internal/rootcmd/` and shared between main binary and shell completion generator — avoids Go's restriction on importing `package main`
- Each subcommand maps to a `Run*` function in `internal/cli/` that follows a standard pattern: preflight → execute → return `format.Response[T]`
- Database adapters self-register via `init()` functions in their packages; the root command imports them as blank imports (`_ "github.com/cskiller24/querylex/internal/db/mysql"`)
- AI features (SQL generation, query optimization) are optional and degrade gracefully — SQL generation validates through a retry loop, optimization falls back to heuristics
- Workspace state (`querylex.json`) uses atomic writes with advisory file locking for concurrency safety
- Credentials are stored in OS keychain by reference only — `database.json` stores a `CredentialReference`, never the actual password
## Layers
- Purpose: Parse args, signal handling, invoke command tree
- Location: `cmd/querylex/main.go`, `cmd/querylex-add-db/main.go`, `cmd/querylex-stats/main.go`
- Contains: `main()` functions with context setup and error handling
- Depends on: `internal/rootcmd`, `internal/cli`
- Used by: End user (command line)
- Purpose: Define cobra command tree with flags, args validation, and subcommand wiring
- Location: `internal/rootcmd/root.go`
- Contains: 16 subcommands (`add-db`, `workspace-stats`, `schema`, `stats`, `indexes`, `explain`, `validate`, `joins`, `save`, `memory`, `history`, `delete`, `resolve`, `ai-config`, `sql`, `optimize`, `completion`)
- Depends on: `internal/cli`, `internal/state`, `internal/version`
- Used by: `cmd/querylex/main.go`, `cmd/generate_completions/main.go`
- Purpose: Implement each subcommand's business logic — preflight, execute, respond
- Location: `internal/cli/run_*.go` (17 files) + `internal/cli/preflight.go`
- Contains: Three preflight variants (`PreflightForCommand`, `PreflightForMemoryCommand`, `PreflightForAICommand`) and per-command Run functions
- Depends on: `internal/db`, `internal/ai`, `internal/index`, `internal/memory`, `internal/state`, `internal/credentials`, `internal/format`, `internal/queryutil`
- Used by: `internal/rootcmd/root.go`
- Purpose: Provide domain logic — DB interaction, AI integration, indexing, memory, state management, credential storage
- Location: `internal/db/`, `internal/ai/`, `internal/index/`, `internal/memory/`, `internal/state/`, `internal/credentials/`, `internal/queryutil/`, `internal/analysis/`, `internal/cache/`
- Contains: Interface definitions, implementations, algorithms
- Depends on: External packages (go-openai, go-sql-driver/mysql, jackc/pgx, modernc.org/sqlite, etc.), standard library
- Used by: CLI handler layer
- Purpose: Standardized response envelope, error codes, warnings
- Location: `internal/format/response.go`, `internal/format/error.go`
- Contains: Generic `Response[T]` struct, `ErrorCode` constants, `NewSuccessResponse`, `NewErrorResponse`, trace ID generation
- Depends on: `github.com/google/uuid`
- Used by: CLI handler layer, occasionally state layer
## Data Flow
### Primary Request Path (e.g., `querylex explain "SELECT * FROM orders"`)
### AI SQL Generation Flow (e.g., `querylex sql "show me all orders from last month"`)
### Workspace State Flow
- Workspace state is file-based with advisory locking — no in-process global singleton
- Memory store is SQLite-backed (WAL mode) with `SetMaxOpenConns(1)` for concurrent safety
- Indexed artifacts are immutable-once-written; staleness detected via manifest checksum verification (`internal/cli/preflight.go:132-160`)
## Key Abstractions
- Purpose: Uniform interface for all database engines (MySQL, MariaDB, PostgreSQL, SQLite, MSSQL)
- Examples: `internal/db/mysql/adapter.go`, `internal/db/postgresql/adapter.go`, `internal/db/sqlite/adapter.go`, `internal/db/mariadb/adapter.go`, `internal/db/mssql/adapter.go`
- Pattern: Strategy/Adapter pattern with a registry. Each adapter registers itself in `init()`: `db.Register("mysql", func(dsn string) (db.Adapter, error) {...})`
- Methods: `Connect`, `Ping`, `Close`, `Schema`, `Explain`, `Validate`, `Stats`, `Indexes`, `Joins`, `DatabaseType`
- Purpose: Abstract secret storage with multiple backends (OS keychain → encrypted file → env vars)
- Examples: `internal/credentials/keychain.go`, `internal/credentials/encrypted.go`, `internal/credentials/env.go`
- Pattern: Chain-of-responsibility: `SelectCredentialStore()` tries keychain first, falls back to encrypted file, then env vars
- Key invariant: Credentials never stored as plaintext on disk; `database.json` stores only `CredentialReference` structs
- Purpose: Abstraction over workspace state persistence
- Examples: `FileWorkspaceStore` (production), `InMemoryWorkspaceStore` (tests) — both in `internal/state/workspace.go`
- Pattern: Repository pattern with atomic save, revision tracking, and lock-guarded access
- Purpose: Standardized JSON response for all commands
- Examples: Used in every `Run*` function in `internal/cli/`
- Pattern: Generic type parameter for typed data, with `Success`, `Error`, `Warnings`, `Meta` (trace ID, protocol version, duration)
- Versioning: Protocol version `1.0.0` constant across all responses
- Purpose: Orchestrate 6-phase schema indexing process
- Location: `internal/index/pipeline.go`
- Pattern: Synchronous pipeline with status tracking via `index_status.json`, graceful degradation on optional phases (terminology generation is non-fatal)
- Phases: Schema Extraction (0-15%) → Join Graph (15-30%) → Schema Map (30-45%) → Domain Clustering (45-75%) → Terminology Generation (75-85%) → Output Assembly (85-100%)
## Entry Points
- Location: `cmd/querylex/main.go`
- Triggers: User CLI invocation
- Responsibilities: Signal handling (`SIGINT`/`SIGTERM` → exit code 130), cobra execution, error formatting to stderr
- Location: `cmd/querylex-add-db/main.go`
- Triggers: User invocation, typically once per database
- Responsibilities: Interactive DB setup, credential storage, initial schema indexing — standalone binary for cleaner GoReleaser packaging
- Location: `cmd/querylex-stats/main.go`
- Triggers: User invocation, health checks
- Responsibilities: Display workspace status across connected databases
- Location: `cmd/generate_completions/main.go`
- Triggers: GoReleaser `before` hook or `make completions`
- Responsibilities: Generate static bash/zsh/fish/powershell completion files for release archives
## Architectural Constraints
- **Threading:** Single-threaded CLI tool. Concurrent operations within a command use `errgroup` (`golang.org/x/sync`) for parallel context fetching (schema, stats, joins, indexes).
- **Global state:** Adapter registry in `internal/db/factory.go` uses a `sync.RWMutex`-protected global map. Version variables in `internal/version/version.go` are package-level globals (injected at build time). No other global mutable state.
- **Circular imports:** No known circular dependency chains. The dependency graph is strictly layered: `cmd/` → `rootcmd/` → `cli/` → `{db,ai,index,memory,state,credentials,queryutil,format}`. `format` package is the leaf with no internal dependencies.
- **Database connections:** Set to `SetMaxOpenConns(1)` for all adapters — single connection per adapter instance, connected on demand during preflight, closed via `defer adapter.Close()`.
- **File locking:** Uses `flock(F_RDLCK)`/`flock(F_WRLCK)` on Unix (via `internal/state/flock_unix.go` using `golang.org/x/sys/unix`) and `LockFileEx` on Windows (via `internal/state/lockfileex_windows.go`). Lock retries with exponential backoff capped at 5 attempts with jitter.
## Anti-Patterns
### `any`-typed Error Response Conversions
### Guard Clause Copy-Paste in Preflight Functions
### Stale Detection Only in Full Preflight
## Error Handling
- Preflight errors are propagated as `*format.Response[any]` — the `Error` field is checked by callers via `errResp.Error != nil`
- Adapter errors wrap sentinel errors (`db.ErrConnectionFailed`, `db.ErrNotImplemented`) with context
- AI operations return `ErrAIServiceUnavailable` with the underlying error — callers decide whether to fallback (heuristic) or fail
- Memory operations use sentinel error prefixes like `MEMORY_WRITE_FAILED`, `MEMORY_STORE_UNAVAILABLE` for programmatic detection
## Cross-Cutting Concerns
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
