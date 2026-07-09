# SkillVault Spec Analysis — Technical Design

> **Status**: Port of `sdd/skillvault-v1-alpha/design` (Engram #59) into file-based OpenSpec format
> **Authoritative source**: `skillvault-spec-v1.md`
> **Existing reference**: Engram topic key `sdd/skillvault-v1-alpha/design` (observation #59)

## 1. Architecture Overview

**Clean Architecture Light** — 6 packages, strict dependency direction:

```
cmd/skillvault (main.go)
    ├── internal/cli       (adapter: stdlib flag+os.Args)
    ├── internal/mcp       (adapter: stdio JSON-RPC)
    └── internal/api       (adapter: v1-final, scaffold only)
              ↓
    internal/app           (use cases: EntryService, SeriesService, etc.)
              ↓
    internal/domain        (pure types, validators, constants)
    internal/vars          (detect + resolve, pure functions)
              ↓
    internal/db            (SQLite store, migrations, FTS5)
```

**Dependency rule**: Upper layers depend on lower via interfaces. `db` never imports `app`/`cli`/`mcp`. `domain` imports nothing internal. `vars` is pure, imports nothing internal. All cross-package communication goes through `internal/app` — CLI, MCP, and API never call `db` or `domain` directly.

### 1.1 Layer Responsibilities

| Layer | Responsibility | Key Constraints |
|-------|---------------|-----------------|
| `cmd/skillvault` | Entry point, route to CLI/MCP/serve | No domain logic |
| `internal/cli` | Parse args, format human output, dispatch to app | No SQLite access, no Cobra |
| `internal/mcp` | JSON-RPC 2.0 stdio, 11 tools, delegate to app | No business rules, no SQLite |
| `internal/api` | net/http routes (v1-final only, scaffold in alpha) | No SQL, no complex validation |
| `internal/app` | Use cases, orchestrate operations, apply domain rules | Depends on store interfaces |
| `internal/domain` | Types, constants, pure validators | Nothing internal |
| `internal/vars` | `{{key}}` detection + resolution | Pure functions, no DB |
| `internal/db` | SQLite connection, migrations, store implementations, FTS5 | No output formatting |

## 2. Package Structure

```
skillvault/
├── cmd/
│   └── skillvault/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── commands.go      -- 17 subcommands via flag+os.Args
│   │   └── output.go        -- human-readable table/JSON formatter
│   ├── mcp/
│   │   ├── server.go        -- stdio JSON-RPC 2.0 read/route/write loop
│   │   ├── jsonrpc.go       -- request/response/error types
│   │   └── tools.go         -- 11 tool definitions + handler dispatch
│   ├── api/
│   │   ├── server.go        -- net/http stub (v1-final scaffold)
│   │   └── handlers.go      -- empty handlers returning 501
│   ├── app/
│   │   ├── entries.go       -- EntryService
│   │   ├── projects.go      -- ProjectService
│   │   ├── series.go        -- SeriesService
│   │   ├── workflows.go     -- WorkflowService
│   │   ├── context.go       -- ContextService
│   │   ├── import_export.go -- VaultExportService / VaultImportService
│   │   └── stats.go         -- StatsService (v1-final)
│   ├── domain/
│   │   ├── entry.go         -- Entry, EntryType, EntryFilter, EntryResult
│   │   ├── project.go       -- Project, ProjectFilter
│   │   ├── series.go        -- Series, SeriesEntry, SeriesFilter, SeriesResult
│   │   ├── workflow.go      -- WorkflowStep, WorkflowRole
│   │   ├── tags.go          -- NormalizeTags pure function
│   │   └── validation.go    -- ValidateSeriesScope, ValidateStepNumbers, etc.
│   ├── vars/
│   │   ├── detect.go        -- VarDetect(content) → []string
│   │   └── resolver.go      -- Resolve(content, vars) → (resolved, missing)
│   └── db/
│       ├── migrations/
│       │   └── 001_init.sql -- executable SQL source
│       ├── schema.sql       -- consolidated reference (manual sync)
│       ├── migrate.go       -- go:embed + apply pending migrations
│       ├── store.go         -- Store struct + interface definitions
│       ├── entries_store.go -- EntryStore SQLite implementation
│       ├── projects_store.go
│       ├── series_store.go
│       ├── workflow_store.go
│       ├── import_export_store.go
│       └── fts.go           -- SearchStore SQLite implementation
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 3. Interface Definitions

All interfaces live in `internal/db/store.go`. App services receive them via constructor injection.

```go
type EntryStore interface {
    UpsertEntry(ctx context.Context, e domain.Entry, tags []string, steps []domain.WorkflowStep) error
    GetEntry(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error)
    ListEntries(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error)
    ArchiveEntry(ctx context.Context, id string) error
}

type ProjectStore interface {
    UpsertProject(ctx context.Context, p domain.Project) error
    ListProjects(ctx context.Context, includeArchived bool) ([]domain.Project, error)
}

type SeriesStore interface {
    UpsertSeries(ctx context.Context, s domain.Series) error
    GetSeries(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error)
    ListSeries(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error)
    ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntryInput) error
}

type WorkflowStore interface {
    UpsertWorkflowSteps(ctx context.Context, entryID string, steps []domain.WorkflowStep) error
    GetWorkflowSteps(ctx context.Context, entryID string) ([]domain.WorkflowStep, error)
}

type SearchStore interface {
    SearchEntries(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error)
    RebuildFTS(ctx context.Context) error
}

type ImportExportStore interface {
    ExportAll(ctx context.Context) (domain.VaultExport, error)
    ImportAll(ctx context.Context, data domain.VaultExport) error
}
```

The top-level `Store` struct composes all sub-stores:

```go
type Store struct {
    Entries       EntryStore
    Projects      ProjectStore
    Series        SeriesStore
    Workflows     WorkflowStore
    Search        SearchStore
    ImportExport  ImportExportStore
    DB            *sql.DB
}
```

### 3.1 Injection Pattern

```go
type EntryService struct {
    store EntryStore
    vars  *vars.Resolver
}

func NewEntryService(s EntryStore, r *vars.Resolver) *EntryService {
    return &EntryService{store: s, vars: r}
}
```

## 4. Data Flows

### 4.1 UpsertEntry

```
CLI/MCP → app.EntryService.UpsertEntry(input)
  → domain.ValidateEntry(input)                    // type, id, content
  → domain.NormalizeTags(tags)                     // trim, lower, deduplicate
  → store.Entries.UpsertEntry(ctx, entry, tags, steps)
    → BEGIN TRANSACTION
    → INSERT OR REPLACE INTO entries (id, name, type, category, project_id, ...)
    → DELETE FROM entry_tags WHERE entry_id = ?
    → INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?) ...
    → IF type=workflow:
        → DELETE FROM workflow_steps WHERE entry_id = ?
        → INSERT INTO workflow_steps (entry_id, step_num, role, content, label) ...
    → UPDATE entries_fts SET name=?, description=?, content=?, tags_denorm=? WHERE id=?
    → COMMIT
  → return EntryResult
```

### 4.2 SearchEntries

```
CLI/MCP → app.EntryService.SearchEntries(q)
  → domain.ValidateSearchQuery(q)
  → store.Search.SearchEntries(ctx, q)
    → Build FTS5 query from q.Query (escape special chars, quote terms)
    → SELECT e.id, e.name, e.type, e.description, e.project_id,
             e.active, e.created_at,
             GROUP_CONCAT(DISTINCT et.tag) as tags
      FROM entries_fts fts
      JOIN entries e ON e.id = fts.id
      LEFT JOIN entry_tags et ON et.entry_id = e.id
      WHERE entries_fts MATCH ?
        AND e.active = COALESCE(NULLIF(?, 0), e.active)
        AND (e.project_id = ? OR ? IS NULL)
        AND e.type = COALESCE(?, e.type)
      GROUP BY e.id
      ORDER BY rank
      LIMIT ?
    → For each result, fetch light series refs (max 3):
      SELECT s.id, s.name, se.step_num, se.label,
        (SELECT COUNT(*) FROM series_entries WHERE series_id = s.id AND active = 1) as total_steps
      FROM series_entries se
      JOIN series s ON s.id = se.series_id
      WHERE se.entry_id = ? AND se.active = 1 AND s.active = 1
    → If tags filter present: filter results client-side or via subquery
  → return []EntrySearchResult
```

### 4.3 RunWorkflow

```
MCP → app.WorkflowService.RunWorkflow(id, vars)
  → store.Entries.GetEntry(ctx, id)                // validate exists, not archived, type=workflow
  → store.Workflows.GetWorkflowSteps(ctx, id)      // ordered steps
  → For each step:
    → vars.Resolve(step.content, mergedVars)
      └── inject {{date}} → time.Now().ISOFormat()
      └── inject {{project}} → entry.project_id if set
      └── collect missing_vars
  → return []RenderedStep{role, content, label, missing_vars}
```

### 4.4 Import (VaultImportService)

```
CLI → app.VaultImportService.Import(path)
  → Read + unmarshal JSON file into domain.VaultExport
  → Validate schema_version exists AND ≤ supported
  → Validate structural integrity (all refs resolve)
  → store.ImportExport.ImportAll(ctx, data)
    → BEGIN TRANSACTION
    → For each project: INSERT OR REPLACE INTO projects
    → For each entry: INSERT OR REPLACE INTO entries
    → For each entry_tag: INSERT OR IGNORE INTO entry_tags
    → For each series: INSERT OR REPLACE INTO series
    → For each series_entry: INSERT OR REPLACE INTO series_entries
    → For each workflow_step: INSERT OR REPLACE INTO workflow_steps
    → Rebuild FTS5: INSERT OR REPLACE INTO entries_fts ...
    → COMMIT
  → return summary (counts)
```

### 4.5 Export (VaultExportService)

```
CLI → app.VaultExportService.Export(path)
  → store.ImportExport.ExportAll(ctx)
    → SELECT * FROM projects
    → SELECT * FROM entries (active + archived)
    → SELECT * FROM entry_tags
    → SELECT * FROM series
    → SELECT * FROM series_entries
    → SELECT * FROM workflow_steps
  → Assemble domain.VaultExport{
      SchemaVersion: 1,
      AppVersion: "v1-alpha",
      ExportedAt: time.Now().UTC(),
      Source: "skillvault",
      Data: { projects, entries, entry_tags, series, series_entries, workflow_steps },
    }
  → Marshal JSON + write to file
  → return
```

## 5. MCP Protocol Design

### 5.1 Transport

JSON-RPC 2.0 over stdio. Line-delimited JSON — one message per line, terminated by `\n`. Server reads `os.Stdin`, writes to `os.Stdout`, logs to `os.Stderr`.

### 5.2 JSON-RPC Types

```go
type JSONRPCRequest struct {
    JSONRPC string          `json:"jsonrpc"`       // "2.0"
    ID      json.RawMessage `json:"id"`            // number, string, or null
    Method  string          `json:"method"`         // tools/list | tools/call | initialize
    Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      interface{} `json:"id"`
    Result  interface{} `json:"result,omitempty"`
    Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
    Code    int         `json:"code"`    // -32700, -32601, -32602, -32000
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```

### 5.3 Tool Definition Schema

```go
type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"` // JSON Schema draft-07
}
```

### 5.4 Tool Call Result

```go
type ToolCallResult struct {
    Content []ContentItem `json:"content"`
    IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
    Type string `json:"type"` // "text" | "resource"
    Text string `json:"text,omitempty"`
}
```

### 5.5 11 v1-alpha Tools

| # | Tool | Method | Key Args | Delegates To |
|---|------|--------|----------|-------------|
| 1 | `get_entry` | `tools/call` | `id`, `include_archived` (opt) | `app.EntryService.GetEntry` |
| 2 | `search_entries` | `tools/call` | `query`, `project_id`, `series_id`, `type`, `tags`, `include_archived`, `limit` | `app.EntryService.SearchEntries` |
| 3 | `list_entries` | `tools/call` | `project_id`, `type`, `include_archived` | `app.EntryService.ListEntries` |
| 4 | `upsert_entry` | `tools/call` | Full entry JSON | `app.EntryService.UpsertEntry` |
| 5 | `archive_entry` | `tools/call` | `id` | `app.EntryService.ArchiveEntry` |
| 6 | `get_series` | `tools/call` | `id`, `include_archived`, `vars` (opt) | `app.SeriesService.GetSeries` |
| 7 | `list_series` | `tools/call` | `project_id`, `include_archived` | `app.SeriesService.ListSeries` |
| 8 | `upsert_series` | `tools/call` | Full series JSON | `app.SeriesService.UpsertSeries` |
| 9 | `replace_series_entries` | `tools/call` | `series_id`, `entries[]` | `app.SeriesService.ReplaceSeriesEntries` |
| 10 | `get_context` | `tools/call` | `project_id` | `app.ContextService.GetContext` |
| 11 | `run_workflow` | `tools/call` | `id`, `vars` (opt) | `app.WorkflowService.RunWorkflow` |

### 5.6 Error Codes

| Code | Meaning | When |
|------|---------|------|
| -32700 | Parse error | Invalid JSON in request |
| -32601 | Method not found | Unknown method |
| -32602 | Invalid params | Malformed tool args |
| -32000 | Domain error | Archived entry, validation failure, not found |

### 5.7 Server Lifecycle

1. `initialize` — client sends capabilities. Server responds with protocol version, tool list, capabilities.
2. `tools/list` — server returns all 11 tool definitions with JSON Schemas.
3. `tools/call` — server validates args, calls app service, returns result or error.
4. `notifications` — `$/cancelRequest` (honored), `notifications/initialized` (ack).
5. Shutdown — server exits when stdin closes.

## 6. SQL Schema

### 6.1 8 Tables (v1-alpha)

```sql
-- Migration registration
CREATE TABLE IF NOT EXISTS schema_migrations (
  version     INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Logical project containers
CREATE TABLE projects (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  description TEXT,
  active      INTEGER DEFAULT 1,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Knowledge units (entries)
CREATE TABLE entries (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL,
  type            TEXT NOT NULL CHECK(type IN ('skill','agent','workflow','prompt','context','note')),
  category        TEXT,
  project_id      TEXT REFERENCES projects(id),
  description     TEXT,
  content         TEXT NOT NULL,
  vars            TEXT,
  source_entry_id TEXT REFERENCES entries(id),
  active          INTEGER DEFAULT 1,
  created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_used       DATETIME
);

-- Entry tags (normalized)
CREATE TABLE entry_tags (
  entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  tag      TEXT NOT NULL,
  PRIMARY KEY (entry_id, tag)
);

-- Series (compositions of entries)
CREATE TABLE series (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  project_id  TEXT REFERENCES projects(id),
  description TEXT,
  vars        TEXT,
  active      INTEGER DEFAULT 1,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Series membership (ordered entries)
CREATE TABLE series_entries (
  series_id TEXT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
  entry_id  TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  step_num  INTEGER NOT NULL,
  label     TEXT,
  required  INTEGER DEFAULT 1,
  notes     TEXT,
  active    INTEGER DEFAULT 1,
  PRIMARY KEY (series_id, entry_id),
  UNIQUE(series_id, step_num)
);

-- Workflow steps (self-contained in entries of type=workflow)
CREATE TABLE workflow_steps (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  entry_id  TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  step_num  INTEGER NOT NULL,
  role      TEXT NOT NULL CHECK(role IN ('system','user','assistant')),
  content   TEXT NOT NULL,
  label     TEXT,
  UNIQUE(entry_id, step_num)
);

-- Full-text search virtual table
CREATE VIRTUAL TABLE entries_fts USING fts5(
  id,
  name,
  description,
  content,
  tags_denorm,
  content='entries',
  tokenize='porter unicode61'
);
```

### 6.2 9 Indexes

```sql
CREATE INDEX idx_entries_type ON entries(type);
CREATE INDEX idx_entries_project_id ON entries(project_id);
CREATE INDEX idx_entries_active ON entries(active);
CREATE INDEX idx_series_project_id ON series(project_id);
CREATE INDEX idx_series_active ON series(active);
CREATE INDEX idx_series_entries_series_step ON series_entries(series_id, step_num);
CREATE INDEX idx_entry_tags_tag ON entry_tags(tag);
CREATE INDEX idx_workflow_steps_entry_step ON workflow_steps(entry_id, step_num);
```

### 6.3 FTS5 Strategy

- FTS5 declared with `content='entries'` for content-sync mode (simpler than external content).
- `tags_denorm` column stores space-separated tags for search.
- On upsert: update FTS5 content via `UPDATE entries_fts SET ... WHERE id=?` (content-sync keeps the `entries` table as the external content; FTS5 rebuild is not needed on every write).
- `RebuildFTS()` available via store for bulk operations (import).
- `porter unicode61` tokenizer for stemming + unicode support.

## 7. Build Order (Inside-Out)

Ordered by dependency chain. Each component depends on everything above it.

| # | Component | Files | Depends On | Enables |
|---|-----------|-------|------------|---------|
| 1 | Go module + skeleton | `go.mod`, `cmd/skillvault/main.go` (stub), all `internal/` dirs | None | Project compiles |
| 2 | `internal/domain` | `entry.go`, `project.go`, `series.go`, `workflow.go`, `tags.go`, `validation.go` | 1 (module) | Domain types + validators |
| 3 | `internal/vars` | `detect.go`, `resolver.go` | 1 (module) | Var detection/injection |
| 4 | `internal/db` migrations | `migrations/001_init.sql`, `schema.sql`, `migrate.go` | 1 (module) | DB creation, `init` |
| 5 | `internal/db` store interfaces | `store.go` (interfaces + `Store` struct) | 2, 4 | Store contracts |
| 6 | `internal/db` EntryStore | `entries_store.go` | 5 | Entry CRUD |
| 7 | `internal/db` ProjectStore | `projects_store.go` | 5 | Project CRUD |
| 8 | `internal/db` SeriesStore | `series_store.go` | 5, 6 | Series CRUD |
| 9 | `internal/db` WorkflowStore | `workflow_store.go` | 5, 6 | Workflow step CRUD |
| 10 | `internal/db` SearchStore | `fts.go` | 4, 5 | FTS5 search |
| 11 | `internal/db` ImportExportStore | `import_export_store.go` | 6–9 | Export/import |
| 12 | `internal/app` EntryService | `entries.go` | 3, 6, 10 | Entry use cases |
| 13 | `internal/app` SeriesService | `series.go` | 3, 8 | Series use cases |
| 14 | `internal/app` WorkflowService | `workflows.go` | 3, 6, 9 | Workflow run |
| 15 | `internal/app` ContextService | `context.go` | 6, 7, 8 | Project context |
| 16 | `internal/app` ImportExport | `import_export.go` | 11 | Export/import orchestration |
| 17 | `internal/cli` | `commands.go`, `output.go` | 12–16 | CLI commands |
| 18 | `internal/mcp` | `server.go`, `jsonrpc.go`, `tools.go` | 12–16 | MCP server |
| 19 | `cmd/skillvault/main.go` | wire everything | 4, 17, 18 | Runnable binary |
| 20 | `internal/api` (scaffold) | `server.go`, `handlers.go` | 12–16 | v1-final HTTP |

### 7.1 TDD Sequence Within Each Component

1. **Write failing test** — domain unit tests first (pure, no DB), then store integration tests (SQLite `:memory:` + migrations), then app tests, then MCP contract tests.
2. **Implement minimum** — just enough to pass.
3. **Refactor** — without breaking tests.

### 7.2 Test Infrastructure

- **Domain**: pure Go table-driven tests, no setup needed.
- **Vars**: pure function tests with expected inputs/outputs.
- **Store**: `SKILLVAULT_DB=:memory:` env var opens in-memory SQLite for each test; migrations applied fresh; each test gets isolated DB.
- **App**: uses real store implementations with in-memory SQLite (no mocks for pure alpha pragmatism).
- **MCP**: subprocess stdio test — run MCP server, send JSON-RPC lines, assert responses.
- **CLI**: subprocess test — run `skillvault` binary with args, assert stdout/stderr/exit code.

## 8. Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| DB driver | `modernc.org/sqlite` | No CGO, pure Go, portable single binary |
| CLI framework | `flag` + `os.Args` | Zero dependencies, matches spec constraint (§6, §21) |
| MCP protocol | Custom JSON-RPC 2.0 over stdio | Full control, no external MCP SDK unless compat blocked (§20.4) |
| HTTP framework | `net/http` | No Fiber/Gin/Gin/Echo (§6, §22) |
| Migrations | `go:embed` + sequential SQL files | Single binary, no runtime file reads (§12.3) |
| Delete strategy | Soft delete (`active=0`) | No hard delete in v1, fully recoverable (§15) |
| DB path | `~/.skillvault/vault.db` | Local-first, OS standard config dir |
| Test DB | In-memory SQLite via `:memory:` | Fast isolated tests, real migrations |
| Series numbering | Store-level renumbering in tx | DB only enforces `UNIQUE(series_id, step_num)`; Go store fills gaps (§16.5–16.6) |
| Workflow rendering | Pure var injection, no execution | Self-contained recetas; no LLM calls, no expression eval (§17.2) |
| Global vars | `{{date}}`, `{{project}}` baked in resolver | Zero config auto-injection (§13.3) |
| FTS5 sync | Content-sync mode (`content='entries'`) + `tags_denorm` | Simplest sync; no triggers needed for basic case |
| Import validation | Atomic transaction + schema_version check | No partial imports (§23.3) |
| No ORM | Raw SQL | Full control, zero dependency, spec constraint (§2.5) |

## 9. FTS5 Content Sync Note

The FTS5 table is declared with `content='entries'` which enables SQLite's external content sync mode. When `entries` rows change, FTS5 does NOT automatically update — the application must call `UPDATE entries_fts SET ... WHERE id=?` explicitly in the upsert transaction. This is by design: it gives the application control over when sync happens and avoids trigger complexity.

For bulk operations (import), `RebuildFTS()` can be called which does `INSERT OR REPLACE INTO entries_fts SELECT id, name, description, content, tags_denorm FROM entries` in a single statement.

## 10. Transactional Guarantees

| Operation | Transaction Scope | Rollback Conditions |
|-----------|-----------------|---------------------|
| `UpsertEntry` | Single tx: entry + tags + steps + FTS5 | Any SQL error |
| `ReplaceSeriesEntries` | Single tx: delete old + insert new + renumber | Validation error, scope violation, SQL error |
| `ImportAll` | Single tx: all rows across all tables | Schema_version mismatch, ref integrity error, any SQL error |
| `ExportAll` | Read-only, no tx needed (snapshot isolation) | N/A |

## 11. CLI Dispatch Flow

```
skillvault <subcommand> [args...] [flags...]

main.go:
  switch os.Args[1]:
    case "init"  → db.Migrate(ctx, dbPath)
    case "mcp"   → mcp.Server.Run(ctx, appServices)
    case "version" → fmt.Println(version)
    default      → cli.Run(ctx, os.Args[1:], appServices)

cli.Run:
  1. Parse primary verb: get, search, list, entry, project, series, workflow, export, import
  2. Parse secondary verb if present: entry upsert, entry archive, etc.
  3. Parse flags with flag.FlagSet
  4. Call app service
  5. Format output (table, text, JSON)
  6. Exit with appropriate code
```

## 12. Error Handling Strategy

| Layer | Error Type | Handling |
|-------|-----------|----------|
| Domain | Validation errors | Return typed error (e.g., `ErrInvalidEntryType`, `ErrCrossProjectScope`). Propagated as-is through app → adapter. |
| DB | SQLite errors | Wrap with context (`entry not found`, `constraint violation`). App layer maps to domain errors. |
| App | Business logic errors | Wrap domain errors with operation context. Return structured error to adapter. |
| MCP | Domain → RPC error | Map to JSON-RPC error code -32000 with structured `data` field. |
| CLI | Domain → exit code | Print error to stderr, exit non-zero. |
| Archived entry | `ErrEntryArchived` | MCP: `code:-32000, data:{error:"archived", id, type:"entry"}`. CLI: print message + suggest `--include-archived`. |

## 13. Domain File Structure Reference

### `internal/domain/entry.go`
```go
type EntryType string
const (
    EntryTypeSkill    EntryType = "skill"
    EntryTypeAgent    EntryType = "agent"
    EntryTypeWorkflow EntryType = "workflow"
    EntryTypePrompt   EntryType = "prompt"
    EntryTypeContext  EntryType = "context"
    EntryTypeNote     EntryType = "note"
)

type Entry struct {
    ID          string
    Name        string
    Type        EntryType
    Category    string
    ProjectID   *string
    Description string
    Content     string
    Vars        []string
    SourceEntryID *string
    Active      bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
    LastUsed    *time.Time
}

type EntryFilter struct {
    ProjectID       *string
    Type            *EntryType
    IncludeArchived bool
}

type EntryResult struct {
    Entry
    Tags  []string
    Steps []WorkflowStep
}

type EntryListResult struct {
    ID        string
    Name      string
    Type      EntryType
    ProjectID *string
    Active    bool
    Tags      []string
    CreatedAt time.Time
}
```

### `internal/domain/project.go`
```go
type Project struct {
    ID          string
    Name        string
    Description string
    Active      bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### `internal/domain/series.go`
```go
type Series struct {
    ID          string
    Name        string
    ProjectID   *string
    Description string
    Vars        []string
    Active      bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type SeriesEntryInput struct {
    EntryID string
    Label   string
    Notes   string
}

type SeriesResult struct {
    Series
    Entries []SeriesEntryResult
}

type SeriesEntryResult struct {
    EntryID    string
    Name       string
    Type       EntryType
    StepNum    int
    TotalSteps int
    Label      string
    Required   bool
}

type SeriesFilter struct {
    ProjectID       *string
    IncludeArchived bool
}

type SeriesListResult struct {
    ID          string
    Name        string
    ProjectID   *string
    Active      bool
    EntryCount  int
}
```

### `internal/domain/workflow.go`
```go
type WorkflowRole string
const (
    WorkflowRoleSystem    WorkflowRole = "system"
    WorkflowRoleUser      WorkflowRole = "user"
    WorkflowRoleAssistant WorkflowRole = "assistant"
)

type WorkflowStep struct {
    ID      int
    EntryID string
    StepNum int
    Role    WorkflowRole
    Content string
    Label   string
}

type RenderedStep struct {
    Role       WorkflowRole
    Content    string
    Label      string
    MissingVars []string
}

type SearchQuery struct {
    Query           string
    ProjectID       *string
    SeriesID        *string
    Type            *EntryType
    Tags            []string
    Active          *bool
    IncludeArchived bool
    Limit           int
}

type EntrySearchResult struct {
    EntryListResult
    Score      float64
    SeriesRefs []SeriesRef
}

type SeriesRef struct {
    SeriesID   string
    SeriesName string
    StepNum    int
    TotalSteps int
    Label      string
}

type VaultExport struct {
    SchemaVersion int       `json:"schema_version"`
    AppVersion    string    `json:"app_version"`
    ExportedAt    time.Time `json:"exported_at"`
    Source        string    `json:"source"`
    Data          VaultData `json:"data"`
}

type VaultData struct {
    Projects      []Project      `json:"projects"`
    Entries       []Entry        `json:"entries"`
    EntryTags     []EntryTagRow  `json:"entry_tags"`
    Series        []Series       `json:"series"`
    SeriesEntries []SeriesEntryRow `json:"series_entries"`
    WorkflowSteps []WorkflowStep `json:"workflow_steps"`
}
```

### `internal/domain/validation.go`
- `NormalizeTags(raw []string) []string` — trim, lowercase, spaces→dashes, reject empty, deduplicate
- `ValidateSeriesScope(seriesProjectID *string, entryProjectID *string) error` — enforce global/project membership rules
- `ValidateStepNumbers(steps []WorkflowStep) error` — check sequential from 1, no gaps
- `ValidateEntryType(t string) error` — must be one of the 6 valid types
- `ValidEntryTypes() []EntryType` — returns valid types for serialization
- `ValidateSearchQuery(q SearchQuery) error` — limit bounds, query non-empty if set

## 14. v1-final Extension Points

The following structures have extension points designed for v1-final:

- **`internal/db/projects_store.go`**: Add `project_refs` table queries without changing existing interfaces.
- **`internal/db/import_export_store.go`**: Add `project_refs` to export/import data without breaking v1-alpha format (optional field in JSON).
- **`internal/api/`**: Full HTTP implementation using `net/http` with routes mapping to the same `internal/app` services.
- **`internal/app/stats.go`**: New service for vault statistics (counts by type/project/status).
- **`internal/cli/commands.go`**: Add new subcommands for copy, archive-series, archive-project, setup, serve.
- **`internal/mcp/tools.go`**: Expand from 11 to 22 tools following the same handler pattern.

---

*Derived from `skillvault-spec-v1.md` (1974 lines). Ported from Engram observation #59 (`sdd/skillvault-v1-alpha/design`).*
