# SkillVault Spec Analysis — Implementation Tasks

> **Status**: Retrospective — the implementation was already completed and archived under Engram topic key `sdd/skillvault-v1-alpha/tasks` (observation #60). This document captures the tasks THAT WERE FOLLOWED, presented as the ordered implementation plan.
> **Build order**: Domain → Vars → DB → Store → App → CLI → MCP → Main → API scaffold → README/Makefile (inside-out, each task depends on everything above it).
> **TDD approach**: Each store task includes integration tests (SQLite `:memory:` + real migrations). Each domain/vars task includes pure unit tests. App tasks use real store implementations. MCP tasks include contract tests over stdio.

---

## Phase 1: Foundation (T-01–T-06)

### T-01 — Go Module & Project Skeleton

| Field | Value |
|-------|-------|
| **Description** | Initialize Go module `github.com/quantum-6/skillvault`, create `cmd/skillvault/main.go` (stub), create all `internal/` package directories (`domain`, `vars`, `db`, `app`, `cli`, `mcp`, `api`), add `.gitignore`, and verify `go build ./...` compiles |
| **Depends on** | None |
| **Files to create** | `go.mod`, `cmd/skillvault/main.go`, all `internal/` dirs, `.gitignore` |
| **Files to modify** | None |
| **Test requirements** | `go build ./...` compiles, `go vet ./...` passes |
| **Complexity** | S |
| **Commit message** | `chore: init Go module and project skeleton` |

### T-02 — Domain Types

| Field | Value |
|-------|-------|
| **Description** | Define core domain types: `Entry`, `EntryType` enum (skill/agent/workflow/prompt/context/note), `Project`, `Series`, `SeriesEntry`, `WorkflowStep`, `WorkflowRole` (system/user/assistant), filter structs (`EntryFilter`, `SeriesFilter`, `SearchQuery`), result structs (`EntryResult`, `EntryListResult`, `EntrySearchResult`, `SeriesResult`, `SeriesListResult`, `SeriesEntryResult`, `SeriesRef`, `RenderedStep`, `VaultExport`, `VaultData`), and supporting row types (`EntryTagRow`, `SeriesEntryRow`). All types in pure Go without external dependencies |
| **Depends on** | T-01 |
| **Files to create** | `internal/domain/entry.go`, `internal/domain/project.go`, `internal/domain/series.go`, `internal/domain/workflow.go` |
| **Files to modify** | None |
| **Test requirements** | Table-driven type-check tests: entry type constants valid, filter struct defaults, time fields, pointer vs value semantics |
| **Complexity** | M |
| **Commit message** | `feat(domain): add core domain types` |

### T-03 — Domain Validation

| Field | Value |
|-------|-------|
| **Description** | Implement pure validators: `NormalizeTags` (trim whitespace, lowercase, spaces→dashes, reject empty, deduplicate), `ValidateSeriesScope` (global series→global entries only; project series→global or same-project entries; reject cross-project), `ValidateStepNumbers` (sequential from 1, no gaps), `ValidateEntryType` (must be one of 6 valid types), `ValidateSearchQuery` (limit bounds, query non-empty if set). No DB or I/O — pure functions only |
| **Depends on** | T-02 |
| **Files to create** | `internal/domain/validation.go`, `internal/domain/tags.go` |
| **Files to modify** | None |
| **Test requirements** | Tag normalization (spaces, case, empty, duplicates), scope validation (global→global OK, project→same-project OK, project→cross-project fails), step numbering (valid, gaps, not starting at 1), entry type validation (valid types pass, invalid fail) |
| **Complexity** | M |
| **Commit message** | `feat(domain): add pure validators for tags, scope, steps, and types` |

### T-04 — Variable Detection & Injection

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/vars/detect.go` (regex `{{key}}` extraction from content) and `internal/vars/resolver.go` (replace `{{key}}` with values from provided map, leave missing keys visible, collect `missing_vars` list). Built-in globals: `{{date}}` → current ISO date, `{{project}}` → project_id if set. Case-sensitive resolution. No code execution, no expressions, no conditionals — pure functions only |
| **Depends on** | T-01 (Go module) — no domain types needed, vars are pure string ops |
| **Files to create** | `internal/vars/detect.go`, `internal/vars/resolver.go` |
| **Files to modify** | None |
| **Test requirements** | Detection finds `{{key}}` in text, resolution replaces values, missing vars left visible and reported, globals `{{date}}` and `{{project}}`, case sensitivity, no false positives on literal text |
| **Complexity** | M |
| **Commit message** | `feat(vars): add variable detection and injection engine` |

### T-05 — DB Migrations

| Field | Value |
|-------|-------|
| **Description** | Create `internal/db/migrations/001_init.sql` with all 8 tables (`schema_migrations`, `projects`, `entries`, `entry_tags`, `series`, `series_entries`, `workflow_steps`, `entries_fts`), 9 indexes, FTS5 virtual table with `porter unicode61` tokenizer. Create `internal/db/schema.sql` as consolidated reference (kept in sync manually). Implement `internal/db/migrate.go` with `go:embed` directive, `ApplyMigrations(db *sql.DB)` function that reads SQL files, applies pending versions, and records in `schema_migrations`. `init` must be idempotent |
| **Depends on** | T-01 |
| **Files to create** | `internal/db/migrations/001_init.sql`, `internal/db/schema.sql`, `internal/db/migrate.go` |
| **Files to modify** | None |
| **Test requirements** | `ApplyMigrations` creates all 8 tables + FTS5 + 9 indexes in in-memory SQLite, `schema_migrations` records version 1, repeated application is idempotent (no error, no duplicated rows) |
| **Complexity** | M |
| **Commit message** | `feat(db): add embedded SQLite migrations with schema` |

### T-06 — DB Connection & Store Interfaces

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/db/store.go` with `Open(dbPath string) (*Store, error)` using `modernc.org/sqlite` driver (no CGO). Define store interfaces: `EntryStore` (UpsertEntry, GetEntry, ListEntries, ArchiveEntry), `ProjectStore` (UpsertProject, ListProjects), `SeriesStore` (UpsertSeries, GetSeries, ListSeries, ReplaceSeriesEntries), `WorkflowStore` (UpsertWorkflowSteps, GetWorkflowSteps), `SearchStore` (SearchEntries, RebuildFTS), `ImportExportStore` (ExportAll, ImportAll). `Store` struct composes all sub-stores + `*sql.DB` |
| **Depends on** | T-02 (domain types for interface signatures), T-05 (migrations for DB setup) |
| **Files to create** | `internal/db/store.go` |
| **Files to modify** | None |
| **Test requirements** | `Open(":memory:")` succeeds, `Store` struct compiles with all interface fields, `DB.Ping()` returns nil |
| **Complexity** | M |
| **Commit message** | `feat(db): add SQLite connection and store interfaces` |

---

## Phase 2: Store Implementation (T-07–T-12)

### T-07 — Entry Store

| Field | Value |
|-------|-------|
| **Description** | Implement `EntryStore` SQLite: `UpsertEntry` (transaction: INSERT OR REPLACE into entries, DELETE + INSERT for entry_tags, DELETE + INSERT for workflow_steps when type=workflow, sync FTS5 via UPDATE), `GetEntry` (with/without `include_archived` — archived returns `ErrEntryArchived`), `ListEntries` (filter by type/active/project), `ArchiveEntry` (UPDATE active=0). All operations use context and `database/sql` |
| **Depends on** | T-06 (store interfaces, connection) |
| **Files to create** | `internal/db/entries_store.go` |
| **Files to modify** | None |
| **Test requirements** | Upsert creates entry with tags, upsert updates without duplicating tags, get returns active entry, get archived returns archived error, get archived with include_archived=true returns content, list filters by type/active, archive sets active=0 |
| **Complexity** | L |
| **Commit message** | `feat(db): implement entry store with CRUD and archive` |

### T-08 — Project Store

| Field | Value |
|-------|-------|
| **Description** | Implement `ProjectStore` SQLite: `UpsertProject` (INSERT OR REPLACE), `ListProjects(includeArchived)` — basic project CRUD. `projects.id` is stable slug, `projects.active` defaults to 1 |
| **Depends on** | T-06 (store interfaces) |
| **Files to create** | `internal/db/projects_store.go` |
| **Files to modify** | None |
| **Test requirements** | Upsert creates project, upsert updates existing, list returns projects, list with include_archived=true includes archived, list defaults to active only |
| **Complexity** | S |
| **Commit message** | `feat(db): implement project store` |

### T-09 — Series Store

| Field | Value |
|-------|-------|
| **Description** | Implement `SeriesStore` SQLite: `UpsertSeries` (metadata only — id, name, project_id, description, vars), `GetSeries` (return series header + ordered entries with `step_num`/`total_steps` calculated dynamically via COUNT), `ListSeries` (filter by project_id, active), `ReplaceSeriesEntries` (transaction: DELETE all old series_entries for series, INSERT new rows with `step_num` renumbered 1..N sequentially, validate scope: global series→global entries only, project series→global or same-project entries). Store-level validation catches scope violations before DB write |
| **Depends on** | T-06 (store interfaces), T-07 (entry store for scope validation reads) |
| **Files to create** | `internal/db/series_store.go` |
| **Files to modify** | None |
| **Test requirements** | Replace renumbers from 1, no gaps after replace, get series returns step_num/total_steps, scope validation rejects cross-project entries, global series rejects project entries, project series accepts global and same-project entries |
| **Complexity** | L |
| **Commit message** | `feat(db): implement series store with transactional replace` |

### T-10 — Workflow Store

| Field | Value |
|-------|-------|
| **Description** | Implement `WorkflowStore` SQLite: `UpsertWorkflowSteps` (DELETE old + INSERT new with sequential step_num, called as part of entry upsert when type=workflow), `GetWorkflowSteps` (ordered by step_num). Steps store: role (system/user/assistant), content, label. Store-level renumbering ensures sequential from 1 |
| **Depends on** | T-06 (store interfaces), T-07 (entries FK) |
| **Files to create** | `internal/db/workflow_store.go` |
| **Files to modify** | None |
| **Test requirements** | Upsert replaces all steps, get returns ordered steps with correct roles, renumbering eliminates gaps, roles (system/user/assistant) preserved |
| **Complexity** | M |
| **Commit message** | `feat(db): implement workflow step store` |

### T-11 — FTS5 Search Store

| Field | Value |
|-------|-------|
| **Description** | Implement `SearchStore` SQLite: `SearchEntries` using FTS5 MATCH on `entries_fts` virtual table with content-sync mode. Supports filters: query (FTS5 MATCH), project_id, series_id (via JOIN on series_entries), type, tags (via subquery on entry_tags), active, include_archived, limit. Each result includes light series metadata (max 3 refs: series_id, series_name, step_num, total_steps, label) via LEFT JOIN + correlated subquery. `RebuildFTS()` for bulk recreation (used after import). FTS5 query sanitization: wrap terms in double quotes to prevent operator injection |
| **Depends on** | T-05 (FTS5 table in migrations), T-06 (store interfaces) |
| **Files to create** | `internal/db/fts.go` |
| **Files to modify** | None |
| **Test requirements** | Search finds by name/description/content/tags, filter by project_id/series_id/type/tags, archived excluded by default, included with include_archived, limit works, series refs returned (max 3), sanitized query handles special characters |
| **Complexity** | L |
| **Commit message** | `feat(db): implement FTS5 search with filters and series refs` |

### T-12 — Import/Export Store

| Field | Value |
|-------|-------|
| **Description** | Implement `ImportExportStore` SQLite: `ExportAll` (SELECT from all 6 data tables, assemble `VaultExport` with metadata: schema_version=1, app_version, exported_at, source="skillvault"), `ImportAll` (transaction: INSERT OR REPLACE into projects/entries/entry_tags/series/series_entries/workflow_steps, validate schema_version exists and ≤ supported, validate all referenced IDs exist, reject on structural inconsistency, rebuild FTS5 after import). Import is atomic — any error rolls back the entire transaction |
| **Depends on** | T-07 (entries), T-08 (projects), T-09 (series), T-10 (workflow steps) |
| **Files to create** | `internal/db/import_export_store.go` |
| **Files to modify** | None |
| **Test requirements** | Export produces all sections with metadata, import processes valid data, import rejects missing schema_version, import rejects unsupported version, import fails atomically on bad refs, round-trip export→import preserves all data |
| **Complexity** | L |
| **Commit message** | `feat(db): implement import/export store with transactional import` |

---

## Phase 3: App Layer (T-13–T-17)

### T-13 — EntryService

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/app/entries.go` with `EntryService`: `UpsertEntry(input)` — normalize tags via `domain.NormalizeTags`, validate type via `domain.ValidateEntryType`, delegate to store; `GetEntry(id, includeArchived)` — handle `ErrEntryArchived` mapping to structured error; `SearchEntries(q)` — validate query via `domain.ValidateSearchQuery`, delegate; `ArchiveEntry(id)` — delegate; `ListEntries(filter)` — delegate. Depends on `EntryStore` interface + `vars.Resolver` for entry content rendering when fetched |
| **Depends on** | T-04 (vars resolver), T-07 (entry store), T-11 (search store) |
| **Files to create** | `internal/app/entries.go` |
| **Files to modify** | None |
| **Test requirements** | Upsert normalizes tags in flow, get returns entry with resolved content (vars injected), get archived returns structured error, search delegates correctly, list filters work |
| **Complexity** | M |
| **Commit message** | `feat(app): add entry service with tag normalization and archiving` |

### T-14 — SeriesService

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/app/series.go` with `SeriesService`: `UpsertSeries(input)` — validate, delegate; `GetSeries(id, includeArchived, vars)` — get series header + entries, render each entry's content with provided vars via `vars.Resolver`; `ListSeries(filter)` — delegate; `ReplaceSeriesEntries(seriesID, entries[])` — validate scope via `domain.ValidateSeriesScope` before store call (fail fast); `GetContext(projectID)` returns project with its entries and series |
| **Depends on** | T-04 (vars resolver), T-09 (series store) |
| **Files to create** | `internal/app/series.go` |
| **Files to modify** | None |
| **Test requirements** | Scope validation fails before store call for invalid entries, get series renders entry content with var injection, replace validates all entries, list returns correct filter results |
| **Complexity** | M |
| **Commit message** | `feat(app): add series service with scope validation and var rendering` |

### T-15 — WorkflowService

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/app/workflows.go` with `WorkflowService`: `RunWorkflow(id, vars map)` — validate entry exists, is not archived, has `type=workflow`; fetch steps via store; for each step, resolve content with merged vars (user-provided + built-in globals `{{date}}` and `{{project}}`); collect `missing_vars` per step; return rendered steps preserving roles. No LLM execution, no code evaluation, no step branching |
| **Depends on** | T-04 (vars resolver), T-07 (entry store for validation), T-10 (workflow store) |
| **Files to create** | `internal/app/workflows.go` |
| **Files to modify** | None |
| **Test requirements** | RunWorkflow renders vars in steps, returns missing_vars, preserves system/user/assistant roles, rejects non-workflow entries, rejects archived entries, no LLM calls (pure rendering only) |
| **Complexity** | M |
| **Commit message** | `feat(app): add workflow service with var rendering` |

### T-16 — VaultExportService / VaultImportService

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/app/import_export.go`: `VaultExportService.Export(path)` — delegate to store, marshal JSON with `json.MarshalIndent`, write to file; `VaultImportService.Import(path)` — read file, unmarshal JSON, validate `schema_version` (exists, ≤ supported), delegate to store (transactional). Both use `ImportExportStore` interface |
| **Depends on** | T-12 (import/export store) |
| **Files to create** | `internal/app/import_export.go` |
| **Files to modify** | None |
| **Test requirements** | Export generates valid JSON with metadata, import processes valid file, import rejects bad schema_version, import file read errors handled gracefully |
| **Complexity** | M |
| **Commit message** | `feat(app): add import/export orchestration services` |

### T-17 — ContextService

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/app/context.go` with `ContextService`: `GetContext(projectID)` — return project info plus its entries (active, non-archived) and series (active, non-archived). In v1-alpha, this is a basic listing without `project_refs` (v1-final feature). Serves MCP tool `get_context` |
| **Depends on** | T-07 (entry store), T-08 (project store), T-09 (series store) |
| **Files to create** | `internal/app/context.go` |
| **Files to modify** | None |
| **Test requirements** | GetContext returns project details, entries for project, series for project; archived entries/series excluded by default |
| **Complexity** | S |
| **Commit message** | `feat(app): add context service for project context` |

---

## Phase 4: Adapters (T-18–T-20)

### T-18 — CLI Adapter

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/cli/commands.go` with 17 subcommands using `flag` + `os.Args` (no Cobra): `init`, `get`, `search`, `list`, `entry upsert`, `entry archive`, `project upsert`, `project list`, `series get`, `series list`, `series upsert`, `series replace`, `workflow run`, `export`, `import`, `mcp`, `version`. Implement `internal/cli/output.go` with human-readable table/JSON formatter. CLI layer formats output and handles exit codes — no SQLite access, no business logic |
| **Depends on** | T-13 through T-17 (all app services) |
| **Files to create** | `internal/cli/commands.go`, `internal/cli/output.go` |
| **Files to modify** | None |
| **Test requirements** | Subcommand parsing recognizes all 17 commands, flags parsed correctly, error outputs to stderr with non-zero exit, version prints correct string |
| **Complexity** | L |
| **Commit message** | `feat(cli): add CLI adapter with 17 subcommands using stdlib` |

### T-19 — MCP Server

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/mcp/server.go` (stdio JSON-RPC 2.0 read/route/write loop — read line from stdin, parse, route, write response to stdout, log to stderr), `internal/mcp/jsonrpc.go` (types: `JSONRPCRequest`, `JSONRPCResponse`, `RPCError`, `Tool`, `ToolCallResult`, `ContentItem`), `internal/mcp/tools.go` (11 tool definitions with JSON Schemas, handler dispatch to app services). Supports `initialize`, `tools/list`, `tools/call`, `notifications/initialized`, `$/cancelRequest`. Error codes: -32700 (parse), -32601 (method not found), -32602 (invalid params), -32000 (domain errors). Server exits when stdin closes |
| **Depends on** | T-13 through T-17 (all app services) |
| **Files to create** | `internal/mcp/server.go`, `internal/mcp/jsonrpc.go`, `internal/mcp/tools.go` |
| **Files to modify** | None |
| **Test requirements** | tools/list returns 11 tools, tools/call get_entry works, tools/call search_entries works, tools/call get_series works, tools/call run_workflow works, tools/call upsert_entry works, tools/call replace_series_entries works, invalid method returns -32601, parse error returns -32700 |
| **Complexity** | L |
| **Commit message** | `feat(mcp): add MCP server with 11 tools over stdio JSON-RPC` |

### T-20 — Binary Wiring (main.go)

| Field | Value |
|-------|-------|
| **Description** | Wire everything in `cmd/skillvault/main.go`: parse `os.Args[0]`, route `init` → `db.Migrate(ctx, dbPath)`, `mcp` → `mcp.Server.Run(ctx, appServices)`, `version` → print version, everything else → `cli.Run(ctx, os.Args[1:], appServices)`. Initialize `Store` with `db.Open()`, construct all app services with constructor injection (store interfaces + vars.Resolver), wire into CLI and MCP. DB path defaults to `~/.skillvault/vault.db` (expand `~` via `os.UserHomeDir`) |
| **Depends on** | T-05 (migrations for init), T-18 (CLI), T-19 (MCP) |
| **Files to modify** | `cmd/skillvault/main.go` (was stub in T-01) |
| **Test requirements** | Binary compiles, `skillvault version` prints version, `skillvault init` creates DB, `skillvault mcp` starts MCP server |
| **Complexity** | M |
| **Commit message** | `feat(cmd): wire main entry point with CLI, MCP, and service injection` |

---

## Phase 5: Polish (T-21–T-23)

### T-21 — HTTP API Scaffold (v1-final)

| Field | Value |
|-------|-------|
| **Description** | Implement `internal/api/server.go` (stub `net/http` server, `ListenAndServe` on `127.0.0.1:7438`, placeholder route for `/health` returning 200), `internal/api/handlers.go` (empty handler stubs for all v1-final endpoints returning 501 Not Implemented). Compiles cleanly but all functional routes return 501. Serves as extension point for v1-final |
| **Depends on** | T-20 (binary needs to compile) |
| **Files to create** | `internal/api/server.go`, `internal/api/handlers.go` |
| **Files to modify** | `cmd/skillvault/main.go` (add `serve` subcommand scaffold) |
| **Test requirements** | API server starts and stops cleanly, `/health` returns 200, all other routes return 501 |
| **Complexity** | S |
| **Commit message** | `feat(api): add HTTP API scaffold for v1-final` |

### T-22 — README

| Field | Value |
|-------|-------|
| **Description** | Write `README.md` with: quickstart (build, init, upsert project/entry, search, mcp), DB path (`~/.skillvault/vault.db`), binary path (`~/tools/skillvault`), alpha vs final scope summary, MCP setup instructions for Claude Code and OpenCode, import/export usage, test instructions (`go test ./...`), architecture overview, and links to spec/design docs |
| **Depends on** | T-20 (binary works for copy-paste verification) |
| **Files to create** | `README.md` |
| **Files to modify** | None |
| **Test requirements** | Quickstart commands copy-paste verifiable (run through once before commit) |
| **Complexity** | S |
| **Commit message** | `docs: add README with quickstart and architecture overview` |

### T-23 — Makefile

| Field | Value |
|-------|-------|
| **Description** | Create `Makefile` with targets: `build` (compile `cmd/skillvault` → `~/tools/skillvault`), `test` (run `go test ./...` with verbose), `clean` (remove built binary), `install` (copy to `$GOPATH/bin` or `~/tools/`), `lint` (run `go vet ./...`). Cross-compilation friendly (use `$GOOS`/`$GOARCH`) |
| **Depends on** | T-01 (Go module exists) |
| **Files to create** | `Makefile` |
| **Files to modify** | None |
| **Test requirements** | `make build` produces working binary at expected path, `make test` runs all tests and passes, `make clean` removes binary |
| **Complexity** | S |
| **Commit message** | `chore: add Makefile with build, test, clean, and install targets` |

---

## Task Dependency Graph

```
T-01 (module)
  ├── T-02 (domain types) ──── T-03 (validators)
  ├── T-04 (vars)
  ├── T-05 (migrations) ────── T-06 (store interfaces)
  └── T-23 (Makefile)
                                  │
                    ┌─────────────┴──────────────┐
                    │             │              │
               T-07 (entries)  T-08 (proj)  T-11 (search)
                    │             │              │
               T-09 (series)◄────┘              │
                    │                           │
               T-10 (workflow)                  │
                    │                           │
               T-12 (import/export)◄────────────┘
                    │
          ┌─────────┼─────────┬──────────┐
          │         │         │          │
      T-13 (entry) T-14 T-15 T-16 T-17 (ctx)
                    (series) (wf) (i/e)
          └─────────┼─────────┴──────────┘
                    │
          ┌─────────┼─────────┐
          │         │         │
      T-18 (CLI) T-19 (MCP) T-20 (main)
                              │
                          T-21 (API scaffold)
                          T-22 (README)
```

**Build order (inside-out)**: T-01 → T-02 → T-03 → T-04 → T-05 → T-06 → T-07 → T-08 → T-09 → T-10 → T-11 → T-12 → T-13 → T-14 → T-15 → T-16 → T-17 → T-18 → T-19 → T-20 → T-21 → T-22 → T-23

---

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 4000–5000 |
| 400-line budget risk | High |
| Chained PRs recommended | No (exception-ok) |
| Suggested split | Single PR (size-exception accepted) |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

### Risk Analysis

| Risk | Mitigation |
|------|------------|
| Large total diff (4k–5k lines) | Single PR with size-exception accepted; code is additive (new project) with clear separation by phase |
| Phase 2 stores have highest complexity (FTS5, transactional import, scope validation) | Each store is independently testable with in-memory SQLite; tests catch regressions before phase 3 |
| MCP protocol adherence (edge cases in JSON-RPC) | Contract tests for tools/list, tools/call, error codes, and notifications |
| CLI ergonomics (17 subcommands, some with two words) | Parse `os.Args` with clear state machine; test argument parsing independently |
| FTS5 content-sync mode (manual sync on upsert) | `UpsertEntry` transaction includes `UPDATE entries_fts`; `RebuildFTS()` available for bulk operations |

### Test Count Estimate

| Layer | Tests | Est. Lines |
|-------|-------|------------|
| Domain (unit) | 6 files | ~300 |
| Vars (unit) | 2 files | ~200 |
| Store (integration, SQLite `:memory:`) | 6 files | ~1200 |
| App (integration) | 5 files | ~600 |
| CLI (subprocess) | 1 file | ~200 |
| MCP (subprocess stdio) | 1 file | ~300 |
| Migrations (integration) | 1 file | ~100 |
| **Total** | **22 files** | **~2900 lines of test** |

---

## Summary Table

| Task | Area | Complexity | Dependencies | Tests |
|------|------|------------|-------------|-------|
| T-01 | Go module + skeleton | S | None | go build |
| T-02 | Domain types | M | T-01 | Unit (table-driven) |
| T-03 | Domain validation | M | T-02 | Unit (pure) |
| T-04 | Variable engine | M | T-01 | Unit (pure) |
| T-05 | DB migrations | M | T-01 | Integration (:memory:) |
| T-06 | Store interfaces | M | T-02, T-05 | Integration |
| T-07 | Entry store | L | T-06 | Integration |
| T-08 | Project store | S | T-06 | Integration |
| T-09 | Series store | L | T-06, T-07 | Integration |
| T-10 | Workflow store | M | T-06, T-07 | Integration |
| T-11 | FTS5 search | L | T-05, T-06 | Integration |
| T-12 | Import/export store | L | T-07–T-10 | Integration |
| T-13 | Entry service | M | T-04, T-07, T-11 | Integration |
| T-14 | Series service | M | T-04, T-09 | Integration |
| T-15 | Workflow service | M | T-04, T-07, T-10 | Integration |
| T-16 | Import/export app | M | T-12 | Integration |
| T-17 | Context service | S | T-07, T-08, T-09 | Integration |
| T-18 | CLI adapter | L | T-13–T-17 | Subprocess |
| T-19 | MCP server | L | T-13–T-17 | Subprocess stdio |
| T-20 | Main wiring | M | T-05, T-18, T-19 | go build |
| T-21 | API scaffold | S | T-20 | HTTP test |
| T-22 | README | S | T-20 | Manual |
| T-23 | Makefile | S | T-01 | make build |

---

*Derived from `skillvault-spec-v1.md` (1974 lines). Retrospective documentation of implementation plan that was followed under `sdd/skillvault-v1-alpha/tasks` (Engram observation #60).*
