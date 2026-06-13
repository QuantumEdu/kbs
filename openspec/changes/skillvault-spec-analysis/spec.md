# SkillVault Spec Analysis — Delta Specifications

> All capabilities are **ADDED** (new project, no existing specs).
> **Authoritative source**: `skillvault-spec-v1.md` (1974 lines).
> **Existing SDD reference**: Engram topic key `sdd/skillvault-v1-alpha/spec` (observation #58) — this document ports that analysis into file-based OpenSpec format.

## Capability 1: DB Schema & Migrations

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-01 | `skillvault init` creates SQLite DB at `~/.skillvault/vault.db` via `modernc.org/sqlite` | MUST |
| REQ-02 | `001_init.sql` creates 8 tables: `schema_migrations`, `projects`, `entries`, `entry_tags`, `series`, `series_entries`, `workflow_steps`, `entries_fts` | MUST |
| REQ-03 | 9 indexes created: entries(type/project_id/active), series(project_id/active), series_entries(series_id,step_num), entry_tags(tag), workflow_steps(entry_id,step_num) | MUST |
| REQ-04 | Migrations embedded via `go:embed`; applied from `internal/db/migrations/` | MUST |
| REQ-05 | `schema_migrations` records applied versions; `init` is idempotent | MUST |
| REQ-06 | `schema.sql` reference file kept in sync with migration source | MUST |
| REQ-07 | FTS5 virtual table `entries_fts` uses `porter unicode61` tokenizer | MUST |

**Scenarios**:

- GIVEN no DB exists, WHEN `skillvault init` runs, THEN `vault.db` is created with all 8 tables, 9 indexes, and FTS5 virtual table; version 1 recorded in `schema_migrations`.
- GIVEN DB already initialized, WHEN `skillvault init` runs again, THEN no error; `schema_migrations` is unchanged.

## Capability 2: Domain Types & Validation

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-08 | `Entry` type: id (unique global), name, type (skill|agent|workflow|prompt|context|note), content, project_id (nullable=global), vars, tags, active | MUST |
| REQ-09 | Tag normalization: trim whitespace, lowercase, spaces→dashes, reject empty, deduplicate | MUST |
| REQ-10 | Series scope rule: global series accepts only global entries; project series accepts global or same-project entries; cross-project entries rejected | MUST |
| REQ-11 | Workflow is self-contained: steps have direct content (no external refs), role (system|user|assistant), no nested workflows/series, no LLM execution | MUST |
| REQ-12 | `step_num` sequential from 1, no gaps, renumbered by store in transaction; DB enforces `UNIQUE(series_id,step_num)` / `UNIQUE(entry_id,step_num)` | MUST |
| REQ-13 | `Project`: id (slug stable), name (display), active (0=archived), description | MUST |

**Scenarios**:

- GIVEN entry with tags `[" Go ", "Go", "", "cli-tool"]`, WHEN normalized, THEN result is `["go", "cli-tool"]` (trimmed, lowercased, empty rejected, deduplicated).
- GIVEN series with `project_id=A`, WHEN adding entry with `project_id=B` (B ≠ A), THEN store rejects with scope validation error.
- GIVEN workflow with `type=workflow`, WHEN `run_workflow` called, THEN content is rendered (vars injected) but no LLM call is made.

## Capability 3: Store Layer

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-14 | `UpsertEntry`: creates or updates entry + tags + workflow_steps (if type=workflow) in single transaction | MUST |
| REQ-15 | `GetEntry(id, includeArchived)`: returns entry with tags and steps; returns `archived` error if entry archived and `includeArchived=false` | MUST |
| REQ-16 | `SearchEntries(q)`: FTS5 search with filter struct (query, project_id, series_id, type, tags, active, include_archived, limit) | MUST |
| REQ-17 | `ArchiveEntry(id)`: sets `active=0`; soft delete only | MUST |
| REQ-18 | `UpsertProject` / `ListProjects(includeArchived)`: basic project CRUD | MUST |
| REQ-19 | `UpsertSeries` / `GetSeries` / `ListSeries`: metadata-only for series header | MUST |
| REQ-20 | `ReplaceSeriesEntries(seriesID, entries[])`: transactional replace — delete old rows, insert new with renumbered step_num 1..N | MUST |
| REQ-21 | Workflow steps stored directly in `workflow_steps` with autoincrement id, sequential step_num | MUST |

**Scenarios**:

- GIVEN entry E1 with tags `["go"]`, WHEN `UpsertEntry` called with same id and new tags `["go","cli"]`, THEN entry updated, tags are `["go","cli"]` (no duplicates).
- GIVEN series with entries at steps [1,2,3], WHEN `ReplaceSeriesEntries` called with 5 new entries, THEN old rows deleted, new rows inserted at steps 1-5 in one transaction.

## Capability 4: Variable Detection & Injection

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-22 | Pattern `{{key}}` detected in `entries.content`, `workflow_steps.content`, and series entry content | MUST |
| REQ-23 | Resolution replaces `{{key}}` with value from provided `vars` map; missing variables left visible as `{{key}}` | MUST |
| REQ-24 | `missing_vars` returned alongside resolved content | MUST |
| REQ-25 | Built-in globals: `{{date}}` → current ISO date, `{{project}}` → entry/series project_id if set | MUST |
| REQ-26 | Case-sensitive: `{{Project}}` and `{{project}}` are distinct keys | MUST |
| REQ-27 | No code execution, no expressions, no conditionals | MUST |

**Scenarios**:

- GIVEN content `"Hello {{name}}"` and vars `{"name":"World"}`, WHEN resolved, THEN result is `"Hello World"` with empty `missing_vars`.
- GIVEN content `"{{name}} {{role}}"` and vars `{"name":"A"}` only, WHEN resolved, THEN result is `"A {{role}}"` and `missing_vars=["role"]`.

## Capability 5: App Layer (Services)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-28 | `EntryService`: orchestrates UpsertEntry, GetEntry, SearchEntries, ArchiveEntry, ListEntries | MUST |
| REQ-29 | `SeriesService`: orchestrates UpsertSeries, GetSeries, ListSeries, ReplaceSeriesEntries with scope validation | MUST |
| REQ-30 | `WorkflowService`: orchestrates RunWorkflow (render steps with vars) | MUST |
| REQ-31 | `VaultExportService`: orchestrates ExportVault, ImportVault (transactional with validation) | MUST |
| REQ-32 | App layer depends on store interfaces, not SQLite directly | SHOULD |
| REQ-33 | All services receive `context.Context` as first parameter | MUST |

**Scenarios**:

- GIVEN valid entry JSON, WHEN `EntryService.UpsertEntry` called, THEN tags normalized, store upserted, entry returned.
- GIVEN series replace request with cross-project entry, WHEN `SeriesService.ReplaceSeriesEntries` called, THEN scope validation fails before store transaction begins.

## Capability 6: CLI Commands

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-34 | CLI built with Go stdlib (`flag` + `os.Args`); no Cobra | MUST |
| REQ-35 | Subcommands: `init`, `get <id>`, `search <q>`, `list`, `entry upsert <file>`, `entry archive <id>`, `project upsert <file>`, `project list`, `series get <id>`, `series list`, `series upsert <file>`, `series replace <id> <file>`, `workflow run <id>`, `export <file>`, `import <file>`, `mcp`, `version` | MUST |
| REQ-36 | `search` supports flags: `--project`, `--series`, `--type`, `--tag`, `--include-archived` | MUST |
| REQ-37 | CLI layer formats human-readable output; does not access SQLite directly | MUST |
| REQ-38 | `version` prints binary version string | MUST |

**Scenarios**:

- GIVEN built binary, WHEN `skillvault init` runs, THEN DB created at `~/.skillvault/vault.db`; exit code 0.
- GIVEN DB with 3 entries, WHEN `skillvault search "fastapi" --type skill --include-archived`, THEN matching entries output in human-readable format.

## Capability 7: MCP Server

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-39 | stdio JSON-RPC server; own implementation, no external MCP SDK (unless compat blocked) | MUST |
| REQ-40 | 11 tools: `get_entry`, `search_entries`, `list_entries`, `upsert_entry`, `archive_entry`, `get_series`, `list_series`, `upsert_series`, `replace_series_entries`, `get_context`, `run_workflow` | MUST |
| REQ-41 | Responds to `tools/list` and `tools/call` JSON-RPC methods | MUST |
| REQ-42 | Server validates input superficially, delegates to `internal/app` for business logic | MUST |
| REQ-43 | Launched via `skillvault mcp` subcommand | MUST |

**Scenarios**:

- GIVEN MCP server running on stdio, WHEN `tools/list` request sent, THEN response contains all 11 tools with name, description, and inputSchema.
- GIVEN `tools/call` with tool=`get_entry` and args=`{"id":"prd-fastapi"}`, WHEN entry exists, THEN MCP returns entry JSON.

## Capability 8: Import/Export JSON

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-44 | Export produces single JSON with `schema_version`, `app_version`, `exported_at`, `source`, and `data` containing projects/entries/entry_tags/series/series_entries/workflow_steps | MUST |
| REQ-45 | Export includes both active and archived records | MUST |
| REQ-46 | Import runs in single transaction; upserts (no delete of absent records) | MUST |
| REQ-47 | Import validates `schema_version` exists and is ≤ supported; rejects otherwise | MUST |
| REQ-48 | Import validates IDs and referential integrity; fails atomically on structural inconsistency | MUST |
| REQ-49 | CLI-only (no MCP tools for import/export) | MUST |

**Scenarios**:

- GIVEN vault with 2 entries and 1 project, WHEN `skillvault export out.json`, THEN file contains all records, `schema_version:1`, `app_version:v1-alpha`, and valid `exported_at` timestamp.
- GIVEN import file without `schema_version`, WHEN `skillvault import bad.json`, THEN import rejected with clear error.
- GIVEN vault with data, WHEN export→import round-trip completes, THEN no data loss; all records preserved.

## Capability 9: FTS5 Search

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-50 | `search_entries` uses FTS5 on `entries_fts` (id, name, description, content, tags_denorm) | MUST |
| REQ-51 | Supports filters: `query`, `project_id`, `series_id`, `type`, `tags`, `active`, `include_archived`, `limit` | MUST |
| REQ-52 | Each result includes light series metadata (max 3 refs): series_id, series_name, step_num, total_steps, label | MUST |
| REQ-53 | Archived entries excluded by default; included when `include_archived=true` | MUST |
| REQ-54 | Search finds matches by name, description, content, and denormalized tags | MUST |

**Scenarios**:

- GIVEN entry with name="FastAPI PRD" and content="FastAPI backend design", WHEN `search "fastapi"`, THEN entry returned in results.
- GIVEN entry tagged "python" is archived, WHEN `search "python"` without `include_archived`, THEN entry excluded from results.
- GIVEN entry E1 appears in series S1 (step 3 of 6) and S2 (step 1 of 4), WHEN search finds E1, THEN series metadata shows both refs (max 3).

## Capability 10: Archived Entry Handling

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-55 | `archive_entry` sets `active=0` (soft delete); no physical deletion | MUST |
| REQ-56 | All list/search/get operations exclude archived entries by default | MUST |
| REQ-57 | `include_archived=true` flag restores visibility of archived entries in all operations | MUST |
| REQ-58 | Direct `get_entry(id)` without `include_archived` returns error: `{"error":"archived","message":"Entry exists but is archived. Retry with include_archived=true.","id":"<id>","type":"entry"}` | MUST |
| REQ-59 | Archiving an entry does not affect series membership; archived entry still appears in series with `include_archived` | MUST |
| REQ-60 | No hard delete, purge, or destructive cleanup in v1 | MUST |

**Scenarios**:

- GIVEN entry E1 is active, WHEN `archive_entry E1`, THEN `active=0` set; subsequent `get_entry E1` returns `archived` error.
- GIVEN entry E1 is archived, WHEN `get_entry E1 --include-archived`, THEN full entry content returned.
- GIVEN series S1 contains archived entry E1, WHEN `get_series S1` without `include_archived`, THEN E1 excluded from steps; total_steps excludes archived entries.

---

## Coverage Summary

| Capability | Requirements | Scenarios |
|-----------|-------------|-----------|
| DB Schema & Migrations | 7 (REQ-01–07) | 2 |
| Domain Types & Validation | 6 (REQ-08–13) | 3 |
| Store Layer | 8 (REQ-14–21) | 2 |
| Variable Detection & Injection | 6 (REQ-22–27) | 2 |
| App Layer (Services) | 6 (REQ-28–33) | 2 |
| CLI Commands | 5 (REQ-34–38) | 2 |
| MCP Server | 5 (REQ-39–43) | 2 |
| Import/Export JSON | 6 (REQ-44–49) | 3 |
| FTS5 Search | 5 (REQ-50–54) | 3 |
| Archived Entry Handling | 6 (REQ-55–60) | 3 |
| **Total** | **60** | **24** |

## Edge Case Notes

- **Tag normalization**: empty strings, whitespace, case, duplicates
- **Missing vars**: `{{missing}}` left visible; `missing_vars` list returned
- **Archived retrieval**: direct get without flag returns structured error
- **Cross-project scope**: global/project series membership enforced
- **Duplicate upsert**: tags merge without duplication
- **FTS5 filtered exclusion**: archived, wrong type, wrong project
- **Import validation**: missing or invalid `schema_version` rejected atomically
- **Step renumbering**: gaps eliminated in transaction

## Authoritative Source Reference

This spec is derived from `skillvault-spec-v1.md` (all 1974 lines). The same analysis was previously stored in Engram under topic key `sdd/skillvault-v1-alpha/spec` (observation #58). This document is the file-based OpenSpec port of that analysis.
