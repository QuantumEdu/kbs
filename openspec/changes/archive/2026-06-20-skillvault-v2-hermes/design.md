# SkillVault v2 Hermes — Technical Design

> Supersedes v1-alpha. All capabilities are **ADDED** in v2.
> Spec: `spec.md` (this directory), authoritative source `skillvault_v1_alpha_spec.md`.
> v1-alpha design archived at `sdd/skillvault-v1-alpha/design` (Engram #59).

---

## 1. Architecture Overview

**Clean Architecture Light extended** — 11 packages, strict one-way dependency:

```
cmd/skillvault (main.go)
    ├── internal/cli       (adapter: flag + os.Args, 14 commands)
    ├── internal/mcp       (adapter: stdio JSON-RPC 2.0, 10 tools)
              ↓
    internal/app           (use cases: SaveEntry, GetContext, SessionWrap, etc.)
              ↓
    internal/domain        (pure entities, validators, constants — imports nothing)
    internal/vars          (path resolver, pure functions — imports nothing)
              ↓
    internal/db            (SQLite stores + migrations + FTS5 — implements domain ifaces)
    internal/files         (NEW — artifact filesystem writer/reader/hasher)
    internal/context       (NEW — Hermes context compiler, 7 modes)
    internal/security      (NEW — secret scanner/redactor)
    internal/search        (EVOLVED from db/fts — FTS5 search abstraction)
    internal/export        (EVOLVED from db/import_export — JSON import/export)
```

**Dependency rule**: Upper layers depend on lower. `db`/`files`/`context`/`security`/`search`/`export` never import `app`/`cli`/`mcp`. `domain`/`vars` import nothing internal.

### Package structure

```
cmd/skillvault/
├── main.go                    (wire DI, init vault, serve CLI or MCP)

internal/
├── cli/
│   └── commands.go            (14 commands, flag parsing)
├── mcp/
│   ├── server.go              (JSON-RPC 2.0 over stdio)
│   ├── handlers.go            (tool call dispatch)
│   └── types.go               (Tool, ToolCallParams, JSONRPC types)
├── app/
│   ├── entry.go               (SaveEntry, GetEntry, SearchEntries, ArchiveEntry)
│   ├── artifact.go            (SaveArtifact)
│   ├── project.go             (AddProject, ListProjects)
│   ├── workflow.go            (AddWorkflow, RenderWorkflow)
│   ├── series.go              (ComposeSeries)
│   ├── session.go             (SessionWrap)
│   ├── context.go             (GetContext)
│   ├── export.go              (ExportVault)
│   └── import.go              (ImportVault)
├── domain/
│   ├── entry.go               (Entry struct, ValidateEntry, EntryType constants)
│   ├── project.go             (Project struct, ValidateProject)
│   ├── artifact.go            (Artifact struct, ArtifactType constants)
│   ├── workflow.go            (Workflow, WorkflowStep)
│   ├── series.go              (Series, SeriesEntry)
│   ├── tag.go                 (Tag, NormalizeTags, DeduplicateTags)
│   ├── link.go                (EntryLink, RelationType constants)
│   ├── status.go              (Status constants, ValidateStatus)
│   ├── context.go             (ContextRequest, ContextMode constants)
│   └── search.go              (SearchQuery, SearchResult)
├── db/
│   ├── store.go               (top-level Store — composes sub-stores)
│   ├── entry_store.go         (EntryStore SQLite impl)
│   ├── project_store.go       (ProjectStore SQLite impl)
│   ├── artifact_store.go      (ArtifactStore SQLite impl)
│   ├── workflow_store.go      (WorkflowStore SQLite impl)
│   ├── series_store.go        (SeriesStore SQLite impl)
│   ├── tag_store.go           (TagStore SQLite impl)
│   ├── link_store.go          (EntryLinkStore SQLite impl)
│   ├── migrations.go          (go:embed + sequential apply)
│   ├── migrations/
│   │   ├── 001_init.sql       (v1 schema — 8 tables)
│   │   └── 002_hermes.sql     (v2 schema — 11 tables)
│   └── schema.go              (DB path, init, connect)
├── files/
│   ├── store.go               (ArtifactFileService impl)
│   └── types.go               (WriteResult, MIME detection)
├── context/
│   ├── compiler.go            (Hermes context compiler — 7 modes)
│   └── modes.go               (mode-specific queries + priority)
├── security/
│   ├── scanner.go             (SecretScanner impl — 4 regex patterns)
│   └── types.go               (ScanResult, RedactResult)
├── search/
│   ├── fts.go                 (FTS5 search abstraction)
│   └── filters.go             (query builder with type/project/tag/status filters)
├── export/
│   ├── exporter.go            (ExportVault impl)
│   └── importer.go            (ImportVault impl)
└── vars/
    └── resolver.go            (detect + resolve functions, path helpers)
```

---

## 2. Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| DB driver | `modernc.org/sqlite` | No CGO, single binary, pure Go (same as v1) |
| CLI framework | `flag` + `os.Args` | Zero deps, matching spec constraint |
| MCP protocol | JSON-RPC 2.0 over stdio | Same as v1, full control |
| Migrations | `go:embed` + sequential SQL files | Single binary, no runtime file reads |
| Delete strategy | Status-based archive (soft) | No hard delete in v2; recoverable |
| Vault root | `~/.skillvault/` | Local-first, portable (same as v1) |
| Test DB | `SKILLVAULT_DB=:memory:` env var | In-memory SQLite for integration tests |
| Secret detection | Reject + warning (no force-save flag) | Spec REQ-SEC-03: do NOT save secret value |
| Artifact dedup | SHA-256 content hash | Spec REQ-ART-04: deduplication + integrity |
| Context truncation | Lowest priority sections first | Spec REQ-HRM-06: respects max_chars |
| MIME detection | Extension-based with content fallback | Spec REQ-ART-05: auto-detect |
| Import conflict | Conflict suffix on duplicate slug | Spec REQ-IEX-05: no silent overwrite |
| No HTTP API | Removed from v2 | v1-alpha scaffold only; not in spec §16 |
| No var detection | Removed from v2 | v2 focuses on Hermes + artifact storage |

---

## 3. Interface Definitions

```go
// internal/domain — all interfaces defined at the app/domain boundary

// EntryStore persists and retrieves entries.
type EntryStore interface {
    Save(ctx context.Context, e domain.Entry, tags []domain.Tag) error
    Get(ctx context.Context, idOrSlug string, includeArchived bool) (domain.Entry, error)
    Search(ctx context.Context, q domain.SearchQuery) ([]domain.SearchResult, error)
    Archive(ctx context.Context, id string) error
    List(ctx context.Context, filter domain.EntryFilter) ([]domain.Entry, error)
}

// ArtifactStore persists artifact metadata (content lives on filesystem).
type ArtifactStore interface {
    Save(ctx context.Context, a domain.Artifact) error
    Get(ctx context.Context, idOrSlug string) (domain.Artifact, error)
    List(ctx context.Context, projectID *string) ([]domain.Artifact, error)
}

// WorkflowStore persists workflows and their steps.
type WorkflowStore interface {
    Save(ctx context.Context, w domain.Workflow, steps []domain.WorkflowStep) error
    Get(ctx context.Context, idOrSlug string) (domain.Workflow, error)
    GetSteps(ctx context.Context, workflowID string) ([]domain.WorkflowStep, error)
    List(ctx context.Context, includeArchived bool) ([]domain.Workflow, error)
}

// SeriesStore persists series and their ordered entry links.
type SeriesStore interface {
    Save(ctx context.Context, s domain.Series) error
    Get(ctx context.Context, idOrSlug string) (domain.Series, error)
    Compose(ctx context.Context, seriesID string) ([]domain.SeriesEntry, error)
    List(ctx context.Context, includeArchived bool) ([]domain.Series, error)
}

// TagStore persists and normalizes tags.
type TagStore interface {
    Save(ctx context.Context, tags []domain.Tag) error
    Search(ctx context.Context, query string) ([]domain.Tag, error)
    List(ctx context.Context) ([]domain.Tag, error)
}

// EntryLinkStore persists directed entry-to-entry relationships.
type EntryLinkStore interface {
    Save(ctx context.Context, link domain.EntryLink) error
    GetByEntry(ctx context.Context, entryID string, relationType *string) ([]domain.EntryLink, error)
    ListByRelation(ctx context.Context, relationType string) ([]domain.EntryLink, error)
}

// ProjectStore persists projects.
type ProjectStore interface {
    Save(ctx context.Context, p domain.Project) error
    Get(ctx context.Context, idOrSlug string) (domain.Project, error)
    List(ctx context.Context, includeArchived bool) ([]domain.Project, error)
}

// HermesContextService compiles agent context packs based on mode and filters.
type HermesContextService interface {
    Compile(ctx context.Context, req domain.ContextRequest) (string, error)
}

// SecretScanner scans content for secret patterns and optionally redacts.
type SecretScanner interface {
    Scan(content string) []domain.SecretMatch
    ScanAndRedact(content string) (redacted string, matches []domain.SecretMatch, ok bool)
}

// ArtifactFileService writes/reads artifact content on the filesystem.
type ArtifactFileService interface {
    Write(ctx context.Context, content string, ext string) (domain.ArtifactFileResult, error)
    Read(ctx context.Context, relativePath string) (string, error)
    Hash(content []byte) string
    DetectMIME(filename string, content []byte) string
}
```

### App service constructors

```go
// internal/app — use cases receive store interfaces via constructor injection.

type Services struct {
    Entry     *EntryService
    Artifact  *ArtifactService
    Project   *ProjectService
    Workflow  *WorkflowService
    Series    *SeriesService
    Session   *SessionService
    Context   *ContextService
    Export    *ExportService
    Import    *ImportService
}

func NewServices(
    stores *db.Store,
    files files.ArtifactFileService,
    scanner security.SecretScanner,
    compiler context.HermesContextService,
    vars *vars.Resolver,
) *Services { /* ... */ }
```

---

## 4. SQL Schema v2

### 11 tables + 1 FTS5 virtual table

Migration `001_init.sql` (v1-alpha, 8 tables — preserved for upgrade path):
`schema_migrations`, `projects`, `entries`, `entry_tags`, `series`, `series_entries`, `workflow_steps`, `entries_fts`.

Migration `002_hermes.sql` (v2, adds 3 new tables + evolves existing):

```sql
-- v2 additions — artifacts, links, tags as separate table, workflows
CREATE TABLE artifacts (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK(type IN ('markdown','json','txt','html','pdf_reference','ai_output','pdf_analysis','spec','report','session_output')),
    file_path TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    summary TEXT,
    content_hash TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    project_id TEXT REFERENCES projects(id),
    source_entry_id TEXT REFERENCES entries(id),
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('draft','active','archived','deprecated','canonical')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('active','archived','draft')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE workflow_steps (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id),
    order_index INTEGER NOT NULL,
    title TEXT NOT NULL,
    instruction TEXT NOT NULL,
    required INTEGER NOT NULL DEFAULT 1,
    expected_output TEXT,
    UNIQUE(workflow_id, order_index)
);

CREATE TABLE entry_links (
    from_entry_id TEXT NOT NULL REFERENCES entries(id),
    to_entry_id TEXT NOT NULL REFERENCES entries(id),
    relation_type TEXT NOT NULL CHECK(relation_type IN ('references','supersedes','related_to','part_of','derived_from','implements')),
    PRIMARY KEY (from_entry_id, to_entry_id, relation_type),
    CHECK(from_entry_id != to_entry_id)
);

-- Evolved from v1: tags now a proper table (not denormalized)
CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE
);

CREATE TABLE entry_tags (
    entry_id TEXT NOT NULL REFERENCES entries(id),
    tag_id TEXT NOT NULL REFERENCES tags(id),
    PRIMARY KEY (entry_id, tag_id)
);

-- FTS5 for entries + artifact summaries
CREATE VIRTUAL TABLE content_fts USING fts5(
    entry_id UNINDEXED,
    title,
    summary,
    body,
    artifact_summary,
    tokenize='porter unicode61'
);

-- v1 schema preserved tables: entries, projects, series, series_entries
-- entries gains: artifact_id, status (evolved from `active` boolean)
-- ALTER TABLE entries ADD COLUMN status TEXT NOT NULL DEFAULT 'draft';
-- ALTER TABLE entries ADD COLUMN artifact_id TEXT REFERENCES artifacts(id);
-- ALTER TABLE entries DROP COLUMN active; (handled in migration logic)
```

### Indexes

```sql
CREATE INDEX idx_entries_type ON entries(type);
CREATE INDEX idx_entries_project ON entries(project_id);
CREATE INDEX idx_entries_status ON entries(status);
CREATE INDEX idx_artifacts_project ON artifacts(project_id);
CREATE INDEX idx_artifacts_slug ON artifacts(slug);
CREATE INDEX idx_workflows_slug ON workflows(slug);
CREATE INDEX idx_workflow_steps_workflow ON workflow_steps(workflow_id);
CREATE INDEX idx_entry_links_from ON entry_links(from_entry_id);
CREATE INDEX idx_entry_links_to ON entry_links(to_entry_id);
CREATE INDEX idx_tags_slug ON tags(slug);
CREATE INDEX idx_series_entries_series ON series_entries(series_id);
```

---

## 5. Data Flows

### SaveEntry

```
CLI/MCP → app.EntryService.SaveEntry(input)
  → domain.ValidateEntry(input)                          // type, status, required fields
  → domain.NormalizeTags(input.Tags)                     // lowercase, trim, deduplicate
  → security.SecretScanner.Scan(body + summary)          // reject if secret found
  → domain.GenerateSlug(title, type)                     // auto-slug if not provided
  → store.Entries.Save(ctx, entry, normalizedTags)
    → SQLite: INSERT INTO entries (tx)
    → SQLite: INSERT OR IGNORE INTO tags (tx)
    → SQLite: INSERT INTO entry_tags (tx)
    → SQLite: INSERT INTO content_fts (tx)
  → return entry with id, slug
```

### GetContext

```
CLI/MCP → app.ContextService.GetContext(req)
  → domain.ValidateContextRequest(req)                    // mode, project, max_chars
  → context.HermesContextService.Compile(ctx, req)
    → 1. priority filter: exclude_archived/deprecated
    → 2. mode-specific queries:
      - profile:       query entries WHERE type IN ('user','feedback') AND status='active'
      - project:       query entries WHERE project_id=? AND status IN ('active','canonical')
      - workflow:      query workflow + steps by slug
      - skill:         query entries WHERE type='skill' AND status='active'
      - planning:      combine profile + project + workflow
      - session_recall: query entries WHERE type='session' ORDER BY created_at DESC LIMIT N
      - full_brief:    combine all modes, exclude archived
    → 3. build structured text (Scope, Preferences, State, Decisions, Workflows, Sessions, Artifacts)
    → 4. apply max_chars truncation (remove lowest priority sections first)
  → return context string
```

### SaveArtifact

```
CLI/MCP → app.ArtifactService.SaveArtifact(input)
  → domain.ValidateArtifact(input)                       // type, at least one of content/file_path
  → security.SecretScanner.Scan(content)                 // reject if secret found
  → files.ArtifactFileService.Write(content, ext)
    → detect MIME from extension/content
    → compute SHA-256 hash
    → write to objects/YYYY/MM/<slug>.<ext>
    → return WriteResult{path, hash, size, mime}
  → store.Artifacts.Save(ctx, artifact)                  // metadata + file path + hash + size
  → optionally: store.Entries.Save(ctx, artifactSummaryEntry)
  → return artifact
```

### SessionWrap

```
CLI/MCP → app.SessionService.SessionWrap(input)
  → domain.BuildSessionEntry(summary, decisions, pending, learnings)
  → store.Entries.Save(ctx, sessionEntry)                // type='session'
  → store.Projects.Get(ctx, projectSlug)                 // link project if specified
  → optionally: save artifacts referenced in input.Artifacts
  → return session entry with links
```

### RenderWorkflow

```
CLI/MCP → app.WorkflowService.RenderWorkflow(slug)
  → store.Workflows.Get(ctx, slug)                       // load workflow
  → store.Workflows.GetSteps(ctx, workflow.ID)           // load ordered steps
  → domain.ValidateWorkflow(workflow)                    // at least one step
  → return ordered steps with metadata
```

### ArchiveEntry

```
CLI/MCP → app.EntryService.ArchiveEntry(idOrSlug)
  → store.Entries.Get(ctx, idOrSlug)                     // verify exists
  → store.Entries.Archive(ctx, id)                       // SET status='archived'
  → return confirmation
```

### ExportVault

```
CLI → app.ExportService.Export(path)
  → store (all): projects, entries, workflows + steps, series + entries, tags, entry_tags, artifacts
  → domain.BuildExport(payload)                           // wrap in schema version + timestamp
  → write JSON to path
  → optionally: write artifact manifest (paths + hashes)
  → return confirmation
```

### ImportVault

```
CLI → app.ImportService.Import(path)
  → read JSON file
  → domain.ValidateExport(data)                           // check schema version
  → verify no structural errors before any write
  → begin transaction
  → for each entity: INSERT OR conflict handling on slug
  → commit
  → rebuild FTS index
  → return import summary
```

---

## 6. Hermes Context Compiler Design

### 7 modes

| Mode | Queries | Sections |
|------|---------|----------|
| `profile` | type IN ('user','feedback'), status='active' | User Preferences |
| `project` | project_id=, status IN ('active','canonical') | Project State, Active Decisions |
| `workflow` | workflow slug → steps | Relevant Workflow |
| `skill` | type='skill', status='active' | Relevant Skills |
| `planning` | profile + project + workflow | All except sessions |
| `session_recall` | type='session', ORDER BY created_at DESC LIMIT 10 | Recent Sessions |
| `full_brief` | ALL (subject to max_chars) | All sections |

### Priority order (lowest truncated first)

1. User feedback/preferences
2. Active project state
3. Canonical decisions
4. Relevant workflow
5. Recent sessions
6. Artifact summaries
7. References
8. Archived index lines (only when requested)

### Compiler algorithm

```go
func (c *Compiler) Compile(ctx context.Context, req domain.ContextRequest) (string, error) {
    var sections []section

    // 1. Build sections based on mode
    switch req.Mode {
    case domain.ModeProfile:
        sections = append(sections, c.userPreferences(ctx))
        sections = append(sections, c.feedbackEntries(ctx))
    case domain.ModeProject:
        sections = append(sections, c.projectState(ctx, req.Project))
        sections = append(sections, c.activeDecisions(ctx, req.Project))
        sections = append(sections, c.artifactSummaries(ctx, req.Project))
    // ... each mode builds appropriate sections
    case domain.ModeFullBrief:
        sections = append(sections, c.userPreferences(ctx))
        sections = append(sections, c.projectState(ctx, req.Project))
        sections = append(sections, c.activeDecisions(ctx, req.Project))
        sections = append(sections, c.workflowSteps(ctx, req.Workflow))
        sections = append(sections, c.recentSessions(ctx, req.Project))
        sections = append(sections, c.artifactSummaries(ctx, req.Project))
        sections = append(sections, c.references(ctx, req.Project))
    }

    // 2. Apply include[] filter if specified
    if len(req.Include) > 0 {
        sections = filterSections(sections, req.Include)
    }

    // 3. Apply max_chars — truncate from lowest priority
    result := buildContextString(sections)
    if len(result) > req.MaxChars {
        result = truncateSections(sections, req.MaxChars)
    }

    return result, nil
}
```

### Context pack output format

```
# CONTEXT PACK

## Scope
Mode: <mode>
Project: <project>
Generated: <timestamp>

## Section Title
- item 1
- item 2

## Next Section
...
```

---

## 7. Secret Scanner Design

### Patterns (spec REQ-SEC-02)

| Pattern | Regex | Example |
|---------|-------|---------|
| OpenAI API key | `sk-[A-Za-z0-9_-]{20,}` | `sk-proj-AbCdEf...` |
| Private key | `-----BEGIN (RSA \|EC \|OPENSSH \|)?PRIVATE KEY-----` | `-----BEGIN RSA PRIVATE KEY-----` |
| GitHub PAT | `ghp_[A-Za-z0-9_]{20,}` | `ghp_abc123...` |
| Slack token | `xox[baprs]-[A-Za-z0-9-]{20,}` | `xoxb-12345...` |

### Behavior

```go
func (s *Scanner) Scan(content string) []SecretMatch {
    // 1. Compile all 4 patterns
    // 2. FindAllStringIndex across content
    // 3. Return matches with type, start, end
}

func (s *Scanner) ScanAndRedact(content string) (string, []SecretMatch, bool) {
    // 1. Scan
    // 2. If matches found:
    //    a. Replace matched text with "[REDACTED <type>]"
    //    b. Return redacted content, matches, ok=false (rejected)
    // 3. If no matches: return content, nil, ok=true (approved)
}
```

### Save flow integration

Both `SaveEntry` and `SaveArtifact` in `app/` call `scanner.ScanAndRedact()` before any store operation. If `ok=false`, the use case returns an error with `ErrSecretDetected` and a list of matched types. The entry/artifact is NOT saved.

No force-save flag in v2 (spec REQ-SEC-03: do NOT save secret value).

---

## 8. Artifact File Service Design

```go
func (s *FileService) Write(content string, ext string) (ArtifactFileResult, error) {
    // 1. Compute SHA-256 hash of content
    // 2. Generate slug from content prefix + hash[:8]
    // 3. Build path: objects/YYYY/MM/<slug>.<ext>
    // 4. Ensure directory exists
    // 5. Write content to file
    // 6. Detect MIME (extension first, content sniff fallback)
    // 7. Return {relativePath, hash, sizeBytes, mimeType}
}

func (s *FileService) Read(relativePath string) (string, error) {
    // 1. Resolve relative path against vault root
    // 2. Read file
    // 3. Return content
}

func (s *FileService) Hash(content []byte) string {
    return fmt.Sprintf("%x", sha256.Sum256(content))
}

func (s *FileService) DetectMIME(filename string, content []byte) string {
    // Extension map: .md → text/markdown, .json → application/json,
    // .txt → text/plain, .html → text/html, .pdf → application/pdf
    // Fallback: http.DetectContentType(content)
}
```

---

## 9. Migration from v1-alpha

### Schema evolution

| v1-alpha (001_init.sql) | v2 Hermes (002_hermes.sql) | Change |
|-------------------------|---------------------------|--------|
| `schema_migrations` | preserved | unchanged |
| `projects` | preserved | unchanged |
| `entries` | evolved | ADD `status` (replaces `active` boolean), ADD `artifact_id` |
| `entry_tags` | evolved | tag_id now references `tags` table (was denormalized string) |
| `series` | preserved | unchanged |
| `series_entries` | preserved | unchanged |
| `workflow_steps` | evolved | NOW references `workflows` table (was per-entry) |
| `entries_fts` | evolved → `content_fts` | expanded to include artifact summaries |
| — | `artifacts` (NEW) | artifact metadata storage |
| — | `workflows` (NEW) | workflow entity separate from entries |
| — | `tags` (NEW) | tag entity with id + name + slug |
| — | `entry_links` (NEW) | directed entry relationships |

### Migration execution

`skillvault init` applies pending migrations sequentially:
1. `001_init.sql` — creates v1 schema if not present (idempotent via `schema_migrations`)
2. `002_hermes.sql` — runs ALTER TABLE / CREATE TABLE for v2 additions

Data migration within `002_hermes.sql`:
- `entries`: `ALTER TABLE entries ADD COLUMN status TEXT NOT NULL DEFAULT 'active'`
- `entry_tags`: rebuild with `tags` table reference (script handles old format)

---

## 10. Build Order

| Phase | Package | Depends on | Output |
|-------|---------|-----------|--------|
| 1 | `internal/domain` | nothing | All entities, validators, constants |
| 2 | `internal/vars` | nothing | Path resolver |
| 3a | `internal/db` (migrations) | nothing (embed) | `001_init.sql` + `002_hermes.sql` |
| 3b | `internal/db` (stores) | domain, 3a | All store implementations |
| 4 | `internal/security` | domain | SecretScanner |
| 5 | `internal/search` | domain, db | FTS5 search abstraction |
| 6 | `internal/files` | vars | ArtifactFileService |
| 7 | `internal/context` | domain, db, search | HermesContextService |
| 8 | `internal/export` | domain, db | ExportVault + ImportVault |
| 9 | `internal/app` | domain, db, security, search, files, context, export | All use cases |
| 10 | `internal/cli` | app, vars | 14 CLI commands |
| 11 | `internal/mcp` | app | 10 MCP tools |
| 12 | `cmd/skillvault/main.go` | db (migrations), cli, mcp | Runnable binary |
| 13 | Tests | all | Unit + integration + acceptance |

### TDD order within each component

1. Write failing test
2. Implement minimum to pass
3. Refactor

Domain unit tests first (pure, no deps), then store integration tests (SQLite `:memory:` with migrations), then app/service tests (mock stores), then CLI/MCP adapter tests.
