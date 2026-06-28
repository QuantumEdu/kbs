# Tasks: Entry Versioning + Skill Pack Export

> **Chain strategy**: Feature Branch Chain  
> **Review budget**: 400 lines per PR  
> **Tracker branch**: `feature/versioning-pack` (draft, no-merge)  
> **PR #1 targets**: `feature/versioning-pack`  
> **PR #2 targets**: `feature/versioning-pack-entry-versions` (PR #1 branch)  
> **PR #3 targets**: `feature/versioning-pack-entry-versions` (PR #1 branch; independent of PR #2)

---

## PR #1: Entry Versioning — Store Layer

**Branch**: `feature/versioning-pack-entry-versions`  
**Base**: `feature/versioning-pack`  
**Estimated**: ~310 lines  
**400-line budget risk**: Low

### Phase 1: Migration + Domain

- [ ] 1.1 Create migration `006_entry_versions.sql` with `entry_versions` table DDL + schema_migrations insert — `internal/db/migrations/006_entry_versions.sql`
- [ ] 1.2 Add `EntryVersion` struct to domain package — `internal/domain/entry.go`

### Phase 2: Store Interface + Implementation

- [ ] 2.1 Add `EntryVersionStore` interface (`SaveVersion`, `ListVersions`, `GetVersion`) and wire `sqliteEntryVersionStore` — `internal/db/store.go`
- [ ] 2.2 Implement `sqliteEntryVersionStore` (SaveVersion, ListVersions, GetVersion) — `internal/db/entry_versions_store.go` (new)
- [ ] 2.3 Modify `sqliteEntryStore.Save()` to archive old content before UPSERT when title/summary/body changed — `internal/db/entries_store.go`

### Phase 3: Schema Sync + Tests

- [ ] 3.1 Update `schema.sql` with `entry_versions` table DDL and index — `internal/db/schema.sql`
- [ ] 3.2 Write `entry_versions_store_test.go`: test SaveVersion, ListVersions descending, GetVersion by number, empty list for no versions, version auto-increment — `internal/db/entry_versions_store_test.go` (new)
- [ ] 3.3 Extend `entries_store_test.go`: verify Save() archives old content when title/summary/body changes; verify Save() does NOT archive when unchanged — `internal/db/entries_store_test.go`

**Verification**: `go test ./internal/db/...` — all tests pass including migration 006

---

## PR #2: Entry Versioning — App + CLI + MCP

**Branch**: `feature/versioning-pack-entry-versions-cli-mcp`  
**Base**: `feature/versioning-pack-entry-versions`  
**Depends on**: PR #1  
**Estimated**: ~380 lines  
**400-line budget risk**: Medium

### Phase 4: App Service

- [x] 4.1 Implement `EntryVersionService` with `ListVersions` and `RestoreVersion`. `RestoreVersion` retrieves version content, gets current entry, calls `SaveEntry` with restored fields (the `Save()` call auto-archives pre-restore state) — `internal/app/entry_versions.go` (new)
- [x] 4.2 Write `entry_versions_test.go`: test ListVersions returns versions in descending order; test RestoreVersion creates new version with pre-restore content; test restore of nonexistent version returns error — `internal/app/entry_versions_test.go` (new)

### Phase 5: CLI

- [x] 5.1 Extend `ParseCommand` in `entry` subcommand block: add `history` (requires entry ID, returns `"entry-history"`) and `restore` (requires entry ID, parses `--version` flag, returns `"entry-restore"`) — `internal/cli/commands.go`
- [x] 5.2 Add `EntryHistoryFlags` and `EntryRestoreFlags` structs with parsing functions (`ParseEntryHistoryFlags`, `ParseEntryRestoreFlags`). `EntryRestoreFlags` accepts `--version` (int, required) — `internal/cli/commands.go`
- [x] 5.3 Add `entry-history` and `entry-restore` cases to `runCLI` dispatch: history prints table (version_number, title, saved_at); restore prints restored version and new version created — `cmd/skillvault/main.go`

### Phase 6: MCP Tools

- [x] 6.1 Register `list_entry_versions` (input: `entry_id` string) and `restore_entry_version` (input: `entry_id` string, `version` number) tools in `registerV2Tools` — `internal/mcp/tools.go`
- [x] 6.2 Add dispatch cases for `list_entry_versions` and `restore_entry_version` in `ToolRegistry.dispatch` — `internal/mcp/tools.go`
- [x] 6.3 Implement `handleListEntryVersions` and `handleRestoreEntryVersion` handlers. `list_entry_versions` outputs JSON array of versions. `restore_entry_version` returns restored entry with metadata — `internal/mcp/tools.go`
- [x] 6.4 Extend `mcp_test.go`: test `list_entry_versions` returns correct count; test `restore_entry_version` restores correct content — `internal/mcp/mcp_test.go`

### Phase 7: Service Wiring

- [x] 7.1 Instantiate `EntryVersionService` in `openVault()` and add to `vaultServices` struct — `cmd/skillvault/main.go`
- [x] 7.2 Wire `EntryVersionService` into `ToolRegistry` via `WithEntryVersionService` builder method — `internal/mcp/tools.go`
- [x] 7.3 Extend `cli_test.go` for `entry history` and `entry restore` command parsing — `internal/cli/cli_test.go`

**Verification**: `go test ./...` — all tests pass; `skillvault entry history <id>` lists versions; `skillvault entry restore <id> --version 1` restores content

---

## PR #3: Skill Pack Export + Import

**Branch**: `feature/versioning-pack-pack-export`  
**Base**: `feature/versioning-pack-entry-versions` (can merge independently of PR #2)  
**Estimated**: ~350 lines  
**400-line budget risk**: Low

### Phase 8: Domain + App

- [ ] 8.1 Add `VaultPackExport` and `PackMetadata` structs — `internal/domain/filters.go`
- [ ] 8.2 Implement `VaultPackExportService` — wraps `VaultExportService.ExportAll()`, adds `PackMetadata`, marshals to JSON — `internal/app/pack_export.go` (new)
- [ ] 8.3 Modify `VaultImportService.Import()` to detect pack format (`pack` key) vs bare `VaultExport`. On pack detection: apply prefix to all IDs in `VaultData`; then delegate to `ImportVault` — `internal/app/import_export.go`

### Phase 9: Prefix Application

- [ ] 9.1 Implement `applyPrefix(prefix string, data *VaultData)` — iterates all slices, prepends prefix to every ID field: `Entries[].ID`, `Entries[].ProjectID`, `Entries[].ArtifactID`, `Projects[].ID`, `Artifacts[].ID`, `Artifacts[].ProjectID`, `Artifacts[].SourceEntryID`, `Workflows[].ID`, `WorkflowSteps[].WorkflowID`, `WorkflowSteps[].ID`, `Series[].ID`, `SeriesEntries[].SeriesID`, `SeriesEntries[].EntryID`, `EntryTags[].EntryID`, `EntryLinks[].FromEntryID`, `EntryLinks[].ToEntryID`, `WorkflowRuns[].WorkflowID`, `WorkflowRunSteps[].RunID`, `WorkflowRunSteps[].EntryID` — `internal/app/import_export.go`

### Phase 10: CLI

- [ ] 10.1 Extend `ParseCommand` in `export` path: detect `--pack` flag for pack export mode; extend `entry` subcommand is already handled in PR #2 — `internal/cli/commands.go`
- [ ] 10.2 Add `ExportPackFlags` struct (`Pack` string, `Author` string, `Version` string, `Description` string, `OutputPath` string) and `ParseExportPackFlags` function — `internal/cli/commands.go`
- [ ] 10.3 Add `ImportPackFlags` struct (`FilePath` string, `Prefix` string) — extend existing `ImportFlags` — `internal/cli/commands.go`
- [ ] 10.4 Extend `runCLI` dispatch: `export` case detects `--pack` flag, calls `VaultPackExportService.ExportPack`; `import` case passes `--prefix` to `VaultImportService.Import` — `cmd/skillvault/main.go`

### Phase 11: Tests

- [ ] 11.1 Write `pack_export_test.go`: test pack export round-trip (export then import with prefix, verify IDs have prefix); test bare export still works; test pack detection on import; test empty prefix imports as-is — `internal/app/pack_export_test.go` (new)
- [ ] 11.2 Extend `cli_test.go`: test `export --pack` and `import --pack --prefix` flag parsing — `internal/cli/cli_test.go`

### Phase 12: Service Wiring

- [ ] 12.1 Instantiate `VaultPackExportService` in `openVault()`, add to `vaultServices` — `cmd/skillvault/main.go`

**Verification**: `go test ./...` — all tests pass; `skillvault export --pack "Test" --author "me" --version "1.0" --output t.svpack` produces valid pack; `skillvault import --pack t.svpack --prefix "p/"` imports with prefix; bare import still works

---

## Chain Context

```
feature/versioning-pack (tracker, draft, no-merge)
├── feature/versioning-pack-entry-versions (PR #1 — DB + Domain + Store)
│   ├── feature/versioning-pack-entry-versions-cli-mcp (PR #2 — App + CLI + MCP) 📍
│   └── feature/versioning-pack-pack-export (PR #3 — Pack Export + Import)
```

PR #2 and PR #3 are independent of each other — both depend only on PR #1. They can be developed and reviewed in parallel.

## Verification Plan

| PR | Command | Expected |
|----|---------|----------|
| #1 | `go test ./internal/db/...` | All tests pass; migration 006 applied |
| #1 | `go test ./internal/db/... -run TestEntryVersion` | Version store tests pass |
| #2 | `go test ./...` | All tests pass |
| #2 | `skillvault entry history <id>` | Version list printed |
| #2 | `skillvault entry restore <id> --version 1` | Content restored, new version created |
| #3 | `go test ./...` | All tests pass |
| #3 | `skillvault export --pack "X" --author "alice" --version "1.0" --output x.svpack` | Pack JSON valid |
| #3 | `skillvault import --pack x.svpack --prefix "ns/"` | Imported with prefix |
| #3 | `skillvault import bare.json` | Bare import still works |

## Rollback Per PR

| PR | Rollback |
|----|----------|
| #1 | Revert migration 006 + remove `sqliteEntryVersionStore` + undo `Save()` modification |
| #2 | Revert app/CLI/MCP files; store layer remains (no-op without surface) |
| #3 | Revert pack export/import code; bare export/import untouched |

---

**Decision needed before apply**: Yes — chained PRs recommended (3 PRs, each ≤400 lines)  
**Chained PRs recommended**: Yes  
**400-line budget risk**: Low (each PR under 400 lines)
