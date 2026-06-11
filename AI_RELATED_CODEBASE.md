# AI-Related Codebase Overview

## Commands

### `querylex sql "<question>"` — SQL Generation from Natural Language

**What it does:** Generates dialect-correct SQL from a natural language question using live database context (schema, terminology, joins, stats, indexes).

**AI concepts:** Chat completion (unstructured), prompt engineering (tiered context assembly), token budgeting, retry loop with validation feedback.

**Flow (12 steps in `run_sql.go`):**
1. `PreflightForAICommand()` — loads workspace state and AI config
2. `RunResolve()` — deterministic table/column matching against schema
3. `RunMemory()` — checks for cached result in memory store
4. Read `terminologies.md` — domain-specific terminology from indexing
5. Parallel context fetch (errgroup): schema, stats, joins, indexes
6. Assemble `SQLGenerationContext` with all collected context
7. `BuildSQLGenerationPrompt()` — tiered prompt within token budget
8. `ChatCompletion()` — calls OpenAI, unstructured output, temperature ≈ 0
9. Validate returned SQL against database schema
10. Retry up to 3 attempts on validation failure (feeds prior attempt + error)
11. Display human-readable output
12. Optionally save to memory

**Key files:**
- `internal/cli/run_sql.go` — full pipeline
- `internal/ai/prompt.go:29-63` — `BuildSQLGenerationPrompt()`
- `internal/ai/chat.go` — `ChatCompletion()`
- `internal/ai/tokens.go` — `TokenBudget`
- `internal/cli/preflight.go` — `PreflightForAICommand()`

---

### `querylex optimize "<sql>"` — AI-Driven Query Optimization

**What it does:** Optimizes SQL using AI-driven three-strategy rewrite, validates each attempt, compares explain plans before/after.

**AI concepts:** Structured output (JSON schema), multi-strategy exploration, explain plan comparison, heuristic fallback, prior attempts as context.

**Flow:**
1. `PreflightForAICommand()` — loads workspace + AI config
2. `RunMemory()` — checks for cached optimization
3. `RunValidate(sql)` — validates input SQL
4. `RunExplain(sql, analyze)` — get explain plan
5. Extract referenced tables from validation response
6. Parallel context fetch: schema, stats, joins, indexes
7. If no API key → `heuristicFallback()` (rule-based, no AI)
8. AI path: try 3 strategies serially:
   - `predicate_rewrite` — filter/predicate optimization
   - `join_subquery_rewrite` — join/subquery restructuring
   - `aggregation_rewrite` — aggregation/grouping optimization
9. Per strategy: `BuildOptimizationPrompt()` + `ChatCompletion(useStructuredOutput=true)` → `ParseOptimizationResponse()`
10. Validate optimized SQL, retry up to 3 times per strategy
11. Compare explain plans before/after via `RunExplain()`
12. Track all attempts as `PriorAttempt` for context in retries

**Key files:**
- `internal/cli/run_optimize.go` — full pipeline, heuristic fallback
- `internal/ai/chat.go:91-107` — structured output mode (JSON schema)
- `internal/ai/prompt.go:65-110` — `BuildOptimizationPrompt()`
- `internal/ai/tokens.go` — token budgeting for optimization context

---

### `querylex ai-config` — AI Provider Configuration

**What it does:** Interactive terminal wizard to set up AI provider credentials, model selection, and endpoint.

**AI concepts:** Credential management (OS keychain), config file persistence, env var fallback.

**Flow:**
1. Prompt for provider, endpoint, model, embedding model, API key, max tokens (via `survey`)
2. Store API key in credential store (OS keychain → encrypted file → env vars)
3. Write `~/.querylex/ai_config.json` atomically

**Key files:**
- `internal/cli/run_ai_config.go` — interactive prompts
- `internal/ai/config.go` — `AIConfig` struct, `ResolveAIConfig()`, `LoadAIConfig()`, `SaveAIConfig()`
- `internal/credentials/store.go` — credential storage abstraction
- `internal/credentials/factory.go` — `SelectCredentialStore()` priority chain

---

### `querylex save`, `memory`, `history` — Query Memory (Embeddings NOT Wired)

**What they do:** Save natural-language→SQL pairs to SQLite memory, search by semantic similarity, browse by topic.

**AI concepts:** Embeddings pipeline exists (Generate → Store → Score) but is NOT connected to any CLI command. All three use lexical-only scoring and emit `EMBEDDINGS_UNAVAILABLE` warning.

**Current behavior:**
- `save` — upserts into `memory.sqlite`, rebuilds keyword index (token-based), emits warning
- `memory` — calls `Search()` (lexical-only), returns best match if similarity ≥ 0.86
- `history` — calls `Search()` (lexical-only), returns all results with composite score

**Existing embedding infrastructure (not wired):**
- `ai.GenerateEmbedding()` → calls OpenAI `text-embedding-3-small`
- `MemoryIndex.EmbeddingVectors` → map of `entryID → EmbeddingMetadata`
- `SearchWithEmbeddings()` → 5-component scoring
- `ComputeSimilarity()` with `embeddingsActive=true` → embedding cosine (0.45) weight

**Key files:**
- `internal/cli/run_save.go` — save pipeline
- `internal/cli/run_memory.go` — memory search pipeline
- `internal/cli/run_history.go` — history search pipeline
- `internal/memory/store.go` — SQLite storage, `SaveEntry()`, `ListEntries()`
- `internal/memory/index.go` — `MemoryIndex`, `EmbeddingMetadata`, `RebuildIndex()`
- `internal/memory/search.go` — `Search()`, `SearchWithEmbeddings()`
- `internal/memory/scoring.go` — `ComputeSimilarity()`, `cosineSimilarity()`
- `internal/ai/embed.go` — `GenerateEmbedding()`, `CosineSimilarity()`

---

## Architecture Concepts

### Prompt Engineering (`internal/ai/prompt.go`)

Two prompt builders, both tiered with token budget management:

**`BuildSQLGenerationPrompt()`** — tiers in order:
1. Schema context (columns, types, keys)
2. Resolve output (matched tables/columns)
3. Terminology (domain glossary)
4. Join graph relationships
5. Index information
6. Table statistics (row counts, cardinality)
7. History references (similar past queries)

**`BuildOptimizationPrompt()`** — sections:
1. Original SQL
2. Explain plan (current execution)
3. Schema context for referenced tables
4. Join relationships
5. Table statistics
6. Existing indexes
7. Prior attempts (on retry, feeds previous strategy + result + validation error)

### Token Budget (`internal/ai/tokens.go`)

- `TokenBudget` struct: `MaxTokens`, `Used`, `Warnings`
- Default max: 128,000 tokens
- Cap at 80% of max (102,400 tokens) for prompt content
- Token estimation: 4 characters ≈ 1 token (heuristic, not actual tokenizer)
- `AddTier(name, content)` — truncates content to fit remaining budget, records warning if truncated

### Chat Completion (`internal/ai/chat.go`)

Two modes:

**Unstructured** (used by `sql`):
- Free-form text response from OpenAI
- Returned SQL extracted and validated against database schema
- Temperature set to `math.SmallestNonzeroFloat32` for deterministic output

**Structured** (used by `optimize`):
- OpenAI returns JSON matching `OptimizationResponse` Go struct
- Uses `CreateChatCompletion` with `ResponseJSONSchema` and strict mode
- `ParseOptimizationResponse()` unmarshals into typed struct
- Schema: `Issues[]`, `RewriteStrategy`, `OptimizedSQL`, `ExpectedPlanChanges`, `RequiresNewIndex`, `Confidence`

Error handling:
- HTTP 401 → `ErrAIServiceUnavailable` (bad key)
- HTTP 429 → `ErrAIServiceUnavailable` (rate limited)
- HTTP 500/502/503 → `ErrAIServiceUnavailable` (server error)

### Retry Loop (both `run_sql.go` and `run_optimize.go`)

- **For `sql`:** After AI generates SQL, validate against database. On failure, feed `priorAttempt + validationError` into next prompt. Up to 3 total attempts.
- **For `optimize`:** Per strategy, validate optimized SQL. On failure, feed back as context. Up to 3 retries per strategy. Track all attempts in `[]PriorAttempt`.

### Embeddings (`internal/ai/embed.go`)

- `GenerateEmbedding(ctx, client, model, text)` → `[]float32`
- Default model: `text-embedding-3-small`
- Calls `client.CreateEmbeddings()` with `openai.EmbeddingRequest`
- `CosineSimilarity(a, b)` → `float64` in [0, 1]
- Used by scoring when `embeddingsActive=true`:
  - Embedding cosine: weight 0.45
  - Schema entity overlap: weight 0.25
  - Intent classification: weight 0.15
  - Filter/temporal overlap: weight 0.10
  - Recency decay: weight 0.05

### Fallback (`internal/cli/run_optimize.go`)

- If `AIConfig.APIKey == ""` → `heuristicFallback()` → rule-based optimization only
- No AI fallback for `sql` — fails with `ErrCodeAIConfigMissing`

### Credential Resolution (`internal/ai/config.go`)

`ResolveAIConfig(home)` priority:
1. Env vars: `QUERYLEX_AI_API_KEY`, `QUERYLEX_AI_ENDPOINT`, `QUERYLEX_AI_MODEL`, `QUERYLEX_AI_EMBEDDING_MODEL`, `QUERYLEX_AI_MAX_TOKENS`
2. Config file `~/.querylex/ai_config.json` + credential store (OS keychain → encrypted file → `QUERYLEX_AI_KEY` env var)

Defaults:
- Endpoint: `https://api.openai.com/v1`
- Model: `gpt-4o`
- Embedding model: `text-embedding-3-small`
- Max tokens: 128,000

### Error Codes (`internal/format/error.go`)

| Code | Message | When |
|------|---------|------|
| `AI_SERVICE_UNAVAILABLE` | AI service is unreachable or not configured. | HTTP errors, no API key |
| `AI_CONFIG_MISSING` | AI provider not configured. Run `querylex ai-config` to set up. | `ResolveAIConfig()` fails |
| `AI_GENERATION_FAILED` | AI failed to generate a valid response. | After exhausting retries |
| `AI_CONTEXT_OVERFLOW` | Context exceeds model token limit. | Schema too large for 128k window |

---

## All Files

### `internal/ai/` (11 files)

| File | Purpose |
|------|---------|
| `client.go` | Creates OpenAI client from `AIConfig` |
| `config.go` | `AIConfig` struct, env var resolution, config file load/save |
| `config_test.go` | Tests for config resolution, round-trip, atomic writes |
| `chat.go` | Chat completion, structured output, optimization response parsing |
| `chat_test.go` | Tests for optimization response schema, parsing, round-trip |
| `embed.go` | `GenerateEmbedding()`, `CosineSimilarity()` |
| `embed_test.go` | Tests for cosine similarity |
| `prompt.go` | `BuildSQLGenerationPrompt()`, `BuildOptimizationPrompt()` |
| `prompt_test.go` | Tests for prompt building with budget truncation |
| `tokens.go` | `TokenBudget` struct, tiered content management |
| `tokens_test.go` | Tests for token budget add/truncation/defaults |

### CLI layer

| File | AI Content |
|------|------------|
| `internal/cli/preflight.go` | `PreflightForAICommand()` — workspace + AI config loading |
| `internal/cli/run_sql.go` | Full SQL generation pipeline (12 steps) |
| `internal/cli/run_optimize.go` | Full optimization pipeline, heuristic fallback |
| `internal/cli/run_ai_config.go` | Interactive AI config wizard |
| `internal/cli/run_memory.go` | Memory search (embeddings NOT wired, lexical-only) |
| `internal/cli/run_history.go` | History search (embeddings NOT wired, lexical-only) |
| `internal/cli/run_save.go` | Save to memory (embeddings NOT wired, lexical-only) |

### Memory layer

| File | AI Content |
|------|------------|
| `internal/memory/index.go` | `MemoryIndex.EmbeddingVectors` field, `EmbeddingMetadata` struct |
| `internal/memory/search.go` | `SearchWithEmbeddings()` (5-component, gated by `embeddingsActive`) |
| `internal/memory/scoring.go` | `ComputeSimilarity()` with embedding branch, `cosineSimilarity()` |

### Supporting layer

| File | AI Content |
|------|------------|
| `internal/rootcmd/root.go` | Command registration: `sqlCmd`, `optimizeCmd`, `aiConfigCmd` |
| `internal/format/error.go` | 4 AI-specific error codes |
| `internal/credentials/factory.go` | Credential store priority chain (keychain → file → env) |
| `internal/credentials/env.go` | `EnvStore` reads `QUERYLEX_AI_KEY` |
