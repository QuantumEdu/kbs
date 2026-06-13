# SkillVault v2 Hermes — Implementation Tasks

> Generated from design.md, spec.md, and skillvault_v1_alpha_spec.md (§19 AC1–AC10).
> Build order follows the dependency graph from design §10, grouped into 11 phases.

---

## Phase 1: Foundation (domain + schema)

### T-01 — Domain entities, constants, and validators

**Description**: Define all domain structs, type constants, and validation functions. Pure Go — no external imports. Includes: Entry (10 types), Project, Artifact (10 types), Workflow, WorkflowStep, Series, SeriesEntry, Tag, EntryLink (6 relation types), ContextRequest, ContextMode (7 modes), SearchQuery, SearchResult, Status (5 values), EntryFilter. Validation: ValidateEntry, ValidateProject, ValidateArtifact, ValidateWorkflow, ValidateSeries, ValidateTag, ValidateLink, ValidateStatus, ValidateContextRequest.

**Dependencies**: None

**Files to create**:
- `internal/domain/entry.go`
- `internal/domain/project.go`
- `internal/domain/artifact.go`
- `internal/domain/workflow.go`
- `internal/domain/series.go`
- `internal/domain/tag.go`
- `internal/domain/link.go`
- `internal/domain/status.go`
- `internal/domain/context.go`
- `internal/domain/search.go`

**Test requirements**:
- Entry type validation accepts all 10 valid types, rejects unknown
- Status validation accepts all 5 values, rejects unknown
- Tag validation rejects empty names, normalizes to lowercase-slug
- Link validation rejects invalid relation types and self-references
- ContextRequest validation rejects invalid modes
- Artifact validation rejects without content/file_path

**Complexity**: L

**Commit message**: `feat(domain): add entities, type constants, and validators`

---

### T-02 — Vars path resolver

**Description**: Pure functions for vault root detection and resolution. `VaultRoot()` returns `~/.skillvault/`. Subdirectory resolvers for `objects/`, `exports/`, `cache/`. Respects `SKILLVAULT_HOME` env var override. No internal imports.

**Dependencies**: None

**Files to create**:
- `internal/vars/resolver.go`

**Test requirements**:
- Default vault root resolves to `~/.skillvault/`
- `SKILLVAULT_HOME` override works
- Subdirectory paths resolve correctly

**Complexity**: S

**Commit message**: `feat(vars): add vault root path resolver`

---

### T-03 — SQL migration files

**Description**: Two sequential migration files embedded via `go:embed`. `001_init.sql`: v1-alpha schema (schema_migrations, projects, entries, entry_tags, series, series_entries, workflow_steps, entries_fts). `002_hermes.sql`: v2 additions (artifacts, workflows, workflow_steps evolved, entry_links, tags, content_fts, ALTER TABLE entries ADD status/artifact_id, indexes). Idempotent via schema_migrations tracking.

**Dependencies**: None (embedded, no runtime deps)

**Files to create**:
- `internal/db/migrations/001_init.sql`
- `internal/db/migrations/002_hermes.sql`
- `internal/db/migrations.go` (embed directive + ApplyMigrations function)

**Test requirements**:
- Applying 001_init creates all 8 v1 tables
- Applying 002_hermes adds 3 new tables + evolves existing
- Re-applying is idempotent (no errors)
- FTS5 virtual table `content_fts` is created

**Complexity**: M

**Commit message**: `feat(db): add SQL migration files for v2 schema`

---

### T-04 — DB connection and migration runner

**Description**: Database initialization (`InitDB`) using `modernc.org/sqlite`. Opens/reuses the vault DB via vault root path. Runs pending migrations. Supports `SKILLVAULT_DB=:memory:` for test mode. Provides `Store` struct fields and `NewStore()` constructor.

**Dependencies**: T-01, T-02, T-03

**Files to create**:
- `internal/db/schema.go`

**Test requirements**:
- In-memory DB opens and applies migrations without error
- File-based DB creates vault.db at expected path
- Reopening an existing migrated DB works
- Migrations apply in order

**Complexity**: M

**Commit message**: `feat(db): add DB connection, init, and migration runner`

---

## Phase 2: Store implementations

### T-05 — ProjectStore + TagStore

**Description**: SQLite implementations of `ProjectStore` and `TagStore` interfaces. `ProjectStore`: Save (INSERT/UPDATE), Get (by ID or slug), List (with includeArchived filter). `TagStore`: Save (INSERT OR IGNORE by slug), Search (by name prefix), List (all). Tag normalization: lowercase, trim, spaces to dashes, deduplicate.

**Dependencies**: T-01, T-04

**Files to create**:
- `internal/db/project_store.go`
- `internal/db/tag_store.go`

**Test requirements**:
- Create and retrieve project by ID and slug
- List projects excludes archived by default
- Tag deduplication and normalization
- Empty tag name rejected at store level

**Complexity**: M

**Commit message**: `feat(db): add ProjectStore and TagStore implementations`

---

### T-06 — EntryStore + EntryLinkStore

**Description**: SQLite implementations of `EntryStore` and `EntryLinkStore`. `EntryStore`: Save (INSERT entry + tags + FTS in transaction), Get (by ID or slug), Search (delegates to search package), Archive (SET status='archived'), List (with EntryFilter for type/project/status). `EntryLinkStore`: Save (INSERT directed link), GetByEntry (outgoing/incoming links), ListByRelation. All operations use transactions.

**Dependencies**: T-01, T-04, T-05

**Files to create**:
- `internal/db/entry_store.go`
- `internal/db/link_store.go`

**Test requirements**:
- Save entry with tags and verify via Get
- Get entry by ID and by slug
- List entries filters by type, project, status
- Archive sets status without deleting row
- Create entry link and retrieve by entry
- Self-referencing link rejected at store level

**Complexity**: L

**Commit message**: `feat(db): add EntryStore and EntryLinkStore implementations`

---

### T-07 — ArtifactStore

**Description**: SQLite implementation of `ArtifactStore`. Save (INSERT artifact metadata), Get (by ID or slug), List (by optional project filter). Stores references to file paths and content hashes computed by the files layer.

**Dependencies**: T-01, T-04

**Files to create**:
- `internal/db/artifact_store.go`

**Test requirements**:
- Save and retrieve artifact metadata
- Get by ID and by slug
- List by project returns only matching artifacts
- Missing slug/ID returns error

**Complexity**: S

**Commit message**: `feat(db): add ArtifactStore implementation`

---

### T-08 — WorkflowStore + SeriesStore

**Description**: SQLite implementations of `WorkflowStore` and `SeriesStore`. `WorkflowStore`: Save (workflow + steps in transaction), Get (by ID or slug), GetSteps (ordered by order_index), List (with includeArchived). `SeriesStore`: Save, Get, Compose (ordered SeriesEntry with entry metadata JOIN), List.

**Dependencies**: T-01, T-04

**Files to create**:
- `internal/db/workflow_store.go`
- `internal/db/series_store.go`

**Test requirements**:
- Save workflow with steps and retrieve ordered
- Get workflow by slug
- Steps returned in correct order
- Series compose returns entries with metadata in order
- Entry appears in multiple series

**Complexity**: M

**Commit message**: `feat(db): add WorkflowStore and SeriesStore implementations`

---

### T-09 — Top-level Store composition

**Description**: Composed `Store` struct that embeds all sub-stores (EntryStore, ProjectStore, ArtifactStore, WorkflowStore, SeriesStore, TagStore, EntryLinkStore). `NewStore(db)` constructor. Includes `Close()` method and accessor helpers. The single `Store` struct is passed to app services.

**Dependencies**: T-05, T-06, T-07, T-08

**Files to create**:
- `internal/db/store.go`

**Test requirements**:
- Store composes all sub-stores
- All stores accessibly via Store struct
- Close works

**Complexity**: S

**Commit message**: `feat(db): add top-level Store composition`

---

### T-10 — Search (FTS5 abstraction)

**Description**: FTS5 search abstraction layer. `Search(ctx, query string, filters SearchFilters)` queries `content_fts` virtual table with proper quoting to prevent FTS5 syntax errors. Filters: type, project, tag, status, includeArchived, limit. Returns `[]SearchResult` with id, title, type, summary, project, status, tags, artifact_ref. Handles FTS5 special characters by sanitizing input.

**Dependencies**: T-01, T-04

**Files to create**:
- `internal/search/fts.go`
- `internal/search/filters.go`

**Test requirements**:
- Full-text search returns matching entries
- Filter by type narrows results
- Filter by project narrows results
- Archived entries excluded by default
- includeArchived=true includes archived
- Sanitized query with special chars does not crash

**Complexity**: M

**Commit message**: `feat(search): add FTS5 search abstraction with filters`

---

## Phase 3: Secret scanner

### T-11 — SecretScanner

**Description**: Pure regex-based scanner. `Scan(content)` returns all matches with type, start, end. `ScanAndRedact(content)` returns redacted content (replaces matches with `[REDACTED <type>]`), list of matches, and ok bool. 4 patterns: OpenAI key (`sk-[A-Za-z0-9_-]{20,}`), private key (`-----BEGIN (RSA |EC |OPENSSH |)?PRIVATE KEY-----`), GitHub PAT (`ghp_[A-Za-z0-9_]{20,}`), Slack token (`xox[baprs]-[A-Za-z0-9-]{20,}`). Returns `ErrSecretDetected` with matched types on scan failure.

**Dependencies**: T-01

**Files to create**:
- `internal/security/scanner.go`
- `internal/security/types.go`

**Test requirements**:
- Detects OpenAI API key pattern
- Detects private key PEM block
- Detects GitHub PAT pattern
- Detects Slack token pattern
- Allows safe content (no false positives for similar text)
- ScanAndRedact replaces secrets with `[REDACTED ...]`
- ScanAndRedact returns ok=true when no secrets

**Complexity**: S

**Commit message**: `feat(security): add SecretScanner with 4 regex patterns`

---

## Phase 4: Artifact filesystem

### T-12 — ArtifactFileService

**Description**: Filesystem-backed artifact storage. `Write(content, ext)` computes SHA-256 hash, generates slug from content prefix + hash[:8], builds path `objects/YYYY/MM/<slug>.<ext>`, creates dirs, writes file, detects MIME (extension map with content fallback), returns `ArtifactFileResult{relativePath, hash, sizeBytes, mimeType}`. `Read(relativePath)` resolves against vault root and returns content. `Hash(content)` returns hex SHA-256. `DetectMIME(filename, content)` uses extension map then http.DetectContentType.

**Dependencies**: T-02

**Files to create**:
- `internal/files/store.go`
- `internal/files/types.go`

**Test requirements**:
- Write stores file under `objects/YYYY/MM/` with correct extension
- SHA-256 hash computed correctly
- MIME detected from extension (.md → text/markdown, .json → application/json)
- MIME falls back to content detection for unknown extensions
- Read returns written content
- Hash is deterministic for same content

**Complexity**: M

**Commit message**: `feat(files): add ArtifactFileService with write/read/hash/MIME`

---

## Phase 5: Hermes context compiler

### T-13 — Context compiler (7 modes)

**Description**: Hermes context compiler implementing `HermesContextService`. `Compile(ctx, req)` builds context pack from DB queries organized by mode. 7 modes: `profile` (user + feedback entries), `project` (active project state + decisions + artifact summaries), `workflow` (workflow steps), `skill` (active skill entries), `planning` (profile + project + workflow combined), `session_recall` (last 10 session entries), `full_brief` (all sections). Priority-ordered sections (1–8). Truncation removes lowest-priority sections first when content exceeds `max_chars`. Output format: structured Markdown with `## Scope`, `## Section Title` sections. `include[]` filter whitelists sections.

**Dependencies**: T-01, T-04, T-10

**Files to create**:
- `internal/context/compiler.go`
- `internal/context/modes.go`

**Test requirements**:
- `profile` mode returns user + feedback entries
- `project` mode returns active decisions and project state for specific project
- `planning` mode combines profile + project + workflow
- `full_brief` mode returns all available context
- Archived/deprecated entries excluded by default
- `max_chars` limit respected; lowest priority sections truncated first
- Output format matches structured Markdown spec
- `include[]` filter limits returned sections
- Empty vault still produces valid minimal context pack

**Complexity**: L

**Commit message**: `feat(context): add Hermes context compiler with 7 modes and truncation`

---

## Phase 6: Import/Export

### T-14 — ExportService

**Description**: JSON export of entire vault. Exports all projects, entries (with tags), workflows + steps, series + entries, tags, artifact metadata, and artifact manifest (paths + hashes). Wraps in schema version and timestamp. Writes to specified file path. Uses `encoding/json` with indented output.

**Dependencies**: T-01, T-04 (needs stores via app service)

**Files to create**:
- `internal/export/exporter.go`

**Test requirements**:
- Export produces valid JSON
- JSON contains all entities with correct structure
- Schema version field present
- Timestamp present
- Empty vault exports produce valid minimal JSON

**Complexity**: M

**Commit message**: `feat(export): add ExportService for full vault JSON export`

---

### T-15 — ImportService

**Description**: JSON import of SkillVault export data. Reads JSON file, validates schema version, validates structure before any writes (pre-commit validation). Uses transaction for atomic import. On duplicate slug: adds conflict suffix instead of silent overwrite. Rebuilds FTS index after import. Returns import summary (counts of imported entities, conflicts).

**Dependencies**: T-01, T-04

**Files to create**:
- `internal/export/importer.go`

**Test requirements**:
- Import valid JSON restores all entities
- Duplicate slug creates conflict suffix, does not overwrite
- Invalid schema version rejected before any writes
- Invalid JSON format rejected
- Import runs in transaction (rollback on partial failure)
- FTS index rebuilt after import
- Import summary returned with counts

**Complexity**: L

**Commit message**: `feat(export): add ImportService with conflict-safe import`

---

## Phase 7: App/use case layer

### T-16 — EntryService (SaveEntry, GetEntry, SearchEntries, ArchiveEntry)

**Description**: Entry use cases. `SaveEntry(input)` → validate entry, normalize tags, scan for secrets (reject if found), auto-generate slug, persist via store, return entry with ID and slug. `GetEntry(idOrSlug)` → retrieve entry + tags + artifact ref. `SearchEntries(query)` → delegate to search package, return results. `ArchiveEntry(idOrSlug)` → verify entry exists, set status to archived. Integrates SecretScanner into Save path.

**Dependencies**: T-01, T-06, T-11, T-10 (search)

**Files to create**:
- `internal/app/entry.go`

**Test requirements**:
- SaveEntry validates entry, persists, returns with ID
- SaveEntry rejects content with secrets
- GetEntry returns entry with associated tags
- SearchEntries returns filtered results
- ArchiveEntry changes status without deletion
- Missing entry returns error on Get/Archive

**Complexity**: M

**Commit message**: `feat(app): add EntryService with secret-safe save and archive`

---

### T-17 — ArtifactService (SaveArtifact)

**Description**: Artifact use case. `SaveArtifact(input)` → validate artifact, scan content for secrets (reject if found), write file via ArtifactFileService, compute hash/size/MIME, persist metadata via store, optionally create artifact_summary entry, return artifact with metadata. Integrates SecretScanner and ArtifactFileService.

**Dependencies**: T-01, T-07, T-11, T-12

**Files to create**:
- `internal/app/artifact.go`

**Test requirements**:
- SaveArtifact writes file, stores metadata, returns with hash and path
- SaveArtifact rejects content with secrets
- SaveArtifact without content or file_path is rejected
- Artifact links to project when specified
- Optional artifact_summary entry created when flag is set

**Complexity**: M

**Commit message**: `feat(app): add ArtifactService with file-backed save`

---

### T-18 — ProjectService + WorkflowService + SeriesService

**Description**: `ProjectService`: AddProject (validate + persist), ListProjects (with includeArchived). `WorkflowService`: AddWorkflow (validate + persist with steps), RenderWorkflow (get by slug + ordered steps). `SeriesService`: ComposeSeries (ordered entries with metadata), ListSeries.

**Dependencies**: T-01, T-05, T-08

**Files to create**:
- `internal/app/project.go`
- `internal/app/workflow.go`
- `internal/app/series.go`

**Test requirements**:
- AddProject creates project with slug and active status
- ListProjects excludes archived by default
- AddWorkflow validates at least one step
- RenderWorkflow returns ordered steps with metadata
- ComposeSeries returns entries in correct order

**Complexity**: M

**Commit message**: `feat(app): add ProjectService, WorkflowService, and SeriesService`

---

### T-19 — SessionService + ContextService

**Description**: `SessionService.SessionWrap(input)` → build session-type entry from summary + decisions + pending + learnings, persist entry, link to project if specified, optionally link artifacts. `ContextService.GetContext(req)` → validate request, delegate to Hermes compiler, return context string.

**Dependencies**: T-01, T-06, T-13

**Files to create**:
- `internal/app/session.go`
- `internal/app/context.go`

**Test requirements**:
- SessionWrap creates session entry with decisions array serialized into body
- Session entry linked to specified project
- GetContext validates request, delegates to compiler
- Context string returned matches compiler output format

**Complexity**: M

**Commit message**: `feat(app): add SessionService and ContextService`

---

### T-20 — Services composition

**Description**: `Services` struct composing all services (Entry, Artifact, Project, Workflow, Series, Session, Context, Export, Import). `NewServices(stores, files, scanner, compiler, vars)` constructor wires all dependencies. `ExportService` and `ImportService` adapters wired here.

**Dependencies**: T-16, T-17, T-18, T-19, T-14, T-15

**Files to create**:
- `internal/app/export.go`
- `internal/app/import.go`
- (or extend existing app files)

**Test requirements**:
- Services struct composes all service instances
- NewServices wires dependencies without nil references
- ExportService accessible via Services.Export
- ImportService accessible via Services.Import

**Complexity**: S

**Commit message**: `feat(app): add Services composition and Export/Import service adapters`

---

## Phase 8: CLI commands

### T-21 — CLI adapter (14 commands)

**Description**: All 14 CLI commands using `flag` + `os.Args` (zero external deps). Commands: `init`, `add-entry`, `search`, `get`, `save-artifact`, `get-context`, `add-project`, `list-projects`, `archive`, `add-workflow`, `render-workflow`, `session-wrap`, `export`, `import`. Each command parses flags, calls appropriate app service, formats output as human-readable text. Subcommand routing via `os.Args[1]`. Help text for each command. Structured as flat command dispatch in `commands.go`.

**Dependencies**: T-20 (all services via NewServices), T-02

**Files to create**:
- `internal/cli/commands.go`

**Test requirements**:
- Each command dispatches to correct app service
- Missing required flags produce helpful error
- Init creates vault structure
- Add-entry saves entry and prints result
- Search prints formatted results
- Archive confirms status change
- Export writes file
- Help text available for each command

**Complexity**: L

**Commit message**: `feat(cli): add all 14 CLI commands`

---

## Phase 9: MCP tools

### T-22 — MCP server + handlers (10 tools)

**Description**: JSON-RPC 2.0 over stdio MCP server. Lists 10 tools via `list_tools`. Dispatches `call_tool` to handler. Tools: `save_entry`, `search_entries`, `get_entry`, `save_artifact`, `get_context`, `compose_series`, `render_workflow`, `session_wrap`, `archive_entry`, `list_projects`. Input validation, error responses as JSON-RPC errors. Uses the same app services as CLI. Three files: server.go (stdio read/write loop), handlers.go (tool dispatch), types.go (Tool, ToolCallParams, JSONRPCRequest/Response structs).

**Dependencies**: T-20 (all services), T-02

**Files to create**:
- `internal/mcp/server.go`
- `internal/mcp/handlers.go`
- `internal/mcp/types.go`

**Test requirements**:
- `list_tools` returns all 10 tools with schemas
- `save_entry` tool saves and returns entry
- `search_entries` returns filtered results
- `get_context` returns context pack (same as CLI)
- Invalid tool call returns JSON-RPC error
- Tool schemas match spec definitions
- `archive_entry` changes status

**Complexity**: L

**Commit message**: `feat(mcp): add JSON-RPC 2.0 MCP server with 10 tools`

---

## Phase 10: Main wiring + integration

### T-23 — main.go (DI + vault init)

**Description**: Binary entry point. Detects subcommand (`skillvault` → CLI, no subcommand or `serve-mcp` → MCP). Initializes vault: detect vault root, create directories if needed, open DB, run migrations. Wires all dependencies: `Store` → `Services` → CLI or MCP. Handles `SKILLVAULT_DB=:memory:` for test mode. Handles `SKILLVAULT_HOME` override. Single `main()` function.

**Dependencies**: T-04, T-20, T-21, T-22, T-02

**Files to create**:
- `cmd/skillvault/main.go`

**Test requirements**:
- `init` command creates vault structure and runs migrations
- CLI mode dispatches to commands
- MCP mode starts stdio server
- Missing vault directory causes init on first run
- Env vars respected

**Complexity**: M

**Commit message**: `feat(cmd): add main.go with DI wiring and vault init`

---

## Phase 11: Tests for all acceptance criteria

### T-24 — Acceptance tests covering AC1–AC10

**Description**: Comprehensive integration and acceptance test suite. Uses `SKILLVAULT_DB=:memory:`. Covers all 10 acceptance criteria. Each AC is a named test case. Tests exercise the full stack from CLI commands and MCP tool calls through to store persistence. Validates output formats, error states, and edge cases.

**Test scenarios per AC**:

| AC | Test scenario | Validation |
|----|---------------|------------|
| AC1 | Run `init`, verify vault structure | `~/.skillvault/vault.db`, `objects/`, `exports/`, `cache/` exist |
| AC2 | Save entry via `add-entry`, search by title/tag/body | Entry returned with metadata in search results |
| AC3 | Save long artifact via `save-artifact`, verify file + DB metadata | File under `objects/YYYY/MM/`, DB has hash/size/path |
| AC4 | Create profile/feedback/decision/workflow entries, call `get-context --project X --mode planning` | Structured context pack with all expected sections |
| AC5 | Archive an entry, search without/with `--include-archived` | Excluded by default, included with flag |
| AC6 | Save entry with API key in body | Rejected with secret-detected warning |
| AC7 | Create workflow with steps, call `render-workflow` | Ordered steps returned with title/instruction/required |
| AC8 | Call `session-wrap` with project + summary + decisions | Session entry created, linked to project |
| AC9 | Export vault, import into fresh vault, verify entities preserved | Round-trip, same count/slugs (conflicts get suffixes) |
| AC10 | Call MCP `get_context` (via tool dispatch), compare to CLI output | Same structured context pack |

**Dependencies**: All prior tasks

**Files to create**:
- `cmd/skillvault/main_test.go` (or `internal/test/acceptance_test.go`)

**Test requirements**:
- All 10 ACs have passing test cases
- Edge cases covered: secret detection, duplicate slug import, archived exclusion, empty tag, self-referencing link, missing artifact content, max_chars truncation
- In-memory DB used throughout
- Tests are deterministic and idempotent

**Complexity**: L

**Commit message**: `test: add acceptance tests covering all 10 ACs`

---

## Review Workload Forecast

| Metric | Value |
|--------|-------|
| **Total tasks** | 24 |
| **Estimated total lines** | ~5,000–6,500 (Go) |
| **Domain/validation** | ~800 lines |
| **SQL migrations** | ~300 lines |
| **Store implementations** | ~1,200 lines |
| **Search (FTS5)** | ~300 lines |
| **Secret scanner** | ~200 lines |
| **Artifact filesystem** | ~250 lines |
| **Context compiler** | ~500 lines |
| **Import/Export** | ~500 lines |
| **App services** | ~700 lines |
| **CLI commands** | ~600 lines |
| **MCP server + handlers** | ~500 lines |
| **main.go wiring** | ~150 lines |
| **Tests** | ~800 lines |
| **Complexity breakdown** | S: 6, M: 11, L: 7 |
| **Budget risk** | Medium — Context compiler (T-13), EntryStore (T-06), CLI (T-21), MCP (T-22), and tests (T-24) are the 5 highest-risk items due to mode count, query surface, and edge cases |
| **Mitigation** | Build domain + stores first (low risk), then infra layers (medium risk), then app + adapters (higher risk). Test incrementally with each task's test requirements. |

---

## Acceptance Criteria Traceability Matrix

| AC | Description | Covered by tasks |
|----|-------------|-----------------|
| **AC1** | Initialize vault | T-03 (migrations), T-04 (DB init), T-21 (CLI init), T-23 (main.go), T-24 (test) |
| **AC2** | Save and search entry | T-06 (EntryStore), T-10 (Search), T-16 (EntryService), T-21 (CLI), T-22 (MCP), T-24 (test) |
| **AC3** | Save long artifact | T-07 (ArtifactStore), T-12 (ArtifactFileService), T-17 (ArtifactService), T-21 (CLI), T-22 (MCP), T-24 (test) |
| **AC4** | Context generation | T-13 (Context compiler), T-19 (ContextService), T-21 (CLI), T-22 (MCP), T-24 (test) |
| **AC5** | Archived content behavior | T-06 (EntryStore.Archive), T-10 (Search exclude_archived), T-13 (Compiler archived exclusion), T-16 (ArchiveEntry), T-24 (test) |
| **AC6** | Secret protection | T-11 (SecretScanner), T-16 (EntryService integration), T-17 (ArtifactService integration), T-24 (test) |
| **AC7** | Workflow rendering | T-08 (WorkflowStore), T-18 (WorkflowService), T-21 (CLI), T-22 (MCP), T-24 (test) |
| **AC8** | Session wrap | T-19 (SessionService), T-21 (CLI), T-22 (MCP), T-24 (test) |
| **AC9** | Import/export | T-14 (ExportService), T-15 (ImportService), T-21 (CLI), T-24 (test) |
| **AC10** | MCP agent use | T-22 (MCP server + handlers), T-23 (main.go wire MCP), T-24 (test) |

Each AC is covered by 4–6 tasks spanning the implementation stack (store → service → adapter → test), ensuring end-to-end traceability.
