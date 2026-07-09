# Tasks: Entry Versioning + Skill Pack Export

> **Chain strategy**: Feature Branch Chain — 400-line budget per PR.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: Low

| Unit | Goal | PR | Base | Lines |
|------|------|----|------|-------|
| 1 | DB migration + domain + store | #1 | main | ~310 |
| 2 | App service + CLI + MCP tools | #2 | #1 | ~380 |
| 3 | Pack export/import + prefix | #3 | #2 | ~350 |

---

## PR #1: Store Layer — `01-store` (base: main)

### Phase 1: Migration + Domain
- [x] 1.1 `009_entry_versions.sql`: table, index, version 9 migration — `internal/db/migrations/`
- [x] 1.2 `EntryVersion` struct (VersionID, EntryID, VersionNumber, Title, Summary, BodyOptional, SavedAt) — `internal/domain/entry.go`

### Phase 2: Store
- [x] 2.1 `EntryVersionStore` interface (SaveVersion, ListVersions, GetVersion) + Store field — `internal/db/store.go`
- [x] 2.2 `sqliteEntryVersionStore`: auto-increment Save, ListVersions DESC, GetVersion by number — `internal/db/entry_versions_store.go` (new)
- [x] 2.3 `Save()`: archive old content in tx before UPSERT when changed — `internal/db/entries_store.go`

### Phase 3: Tests
- [x] 3.1 Update `schema.sql` with entry_versions DDL — `internal/db/schema.sql`
- [x] 3.2 `entry_versions_store_test.go`: Save, List, Get, empty, auto-increment
- [x] 3.3 Extend `entries_store_test.go`: archives on change; no archive when unchanged

**✅**: `go test ./internal/db/...` — migration 009 applied

---

## PR #2: App + CLI + MCP — `02-cli-mcp` (base: PR #1)

### Phase 4: App
- [x] 4.1 `EntryVersionService`: ListVersions, RestoreVersion (get version→get entry→Save restored; Save auto-archives) — `internal/app/entry_versions.go` (new)
- [x] 4.2 `entry_versions_test.go`: ListVersions descending, RestoreVersion round-trip, error on nonexistent — `internal/app/`

### Phase 5: CLI
- [x] 5.1 Extend `ParseCommand` entry block: `history`, `restore` subcommands — `internal/cli/commands.go`
- [x] 5.2 `EntryHistoryFlags` + `EntryRestoreFlags` structs + parsers — `internal/cli/commands.go`
- [x] 5.3 `entry-history`/`entry-restore` dispatch in `runCLI` — `cmd/skillvault/main.go`

### Phase 6: MCP
- [x] 6.1 Register `list_entry_versions` + `restore_entry_version` — `internal/mcp/tools.go`
- [x] 6.2 Implement handlers + dispatch — `internal/mcp/tools.go`
- [x] 6.3 Extend `mcp_test.go` for both tools

### Phase 7: Wiring
- [x] 7.1 Wire `EntryVersionService` into `openVault()`/`vaultServices` + `ToolRegistry` — `cmd/skillvault/main.go`, `internal/mcp/tools.go`
- [x] 7.2 Extend `cli_test.go` for entry history/restore parsing

**✅**: `go test ./...`; `entry history <id>`; `entry restore <id> --version 1`

---

## PR #3: Pack Export + Import — `03-pack-export` (base: PR #2)

### Phase 8: Domain + App
- [x] 8.1 `VaultPackExport` + `PackMetadata` — `internal/domain/filters.go`
- [x] 8.2 `VaultPackExportService.ExportPack` (ExportAll + metadata → JSON) — `internal/app/pack_export.go` (new)
- [x] 8.3 `Import(ctx, path)` → `Import(ctx, path, prefix)`: detect pack key, applyPrefix, delegate — `internal/app/import_export.go`

### Phase 9: Prefix
- [x] 9.1 `applyPrefix(prefix, *VaultData)`: all ID fields across Entries/Projects/Artifacts/Workflows/Series/Tags/Links/Runs — `internal/app/import_export.go`

### Phase 10: CLI
- [x] 10.1 `ParseCommand`: export `--pack`; import `--pack --prefix` — `internal/cli/commands.go`
- [x] 10.2 `ExportPackFlags` (Pack, Author, Version, Description, OutputPath) + parser; extend ImportFlags — `internal/cli/commands.go`
- [x] 10.3 `runCLI` dispatch: export → ExportPack; import → Import with prefix — `cmd/skillvault/main.go`

### Phase 11: Tests + Wiring
- [x] 11.1 `pack_export_test.go`: round-trip prefix, bare compat, empty prefix — `internal/app/`
- [x] 11.2 Extend `cli_test.go` for export/import pack flags
- [x] 11.3 Wire `VaultPackExportService` into `openVault()`/`vaultServices` — `cmd/skillvault/main.go`

**✅**: `go test ./...`; `export --pack "X" --author "a" --version "1.0" --output x.svpack`; `import --pack x.svpack --prefix "ns/"`; bare import still works

---

Chain: main → 01-store → 02-cli-mcp → 03-pack-export.
Rollback: #1 drop migration 009; #2 revert app/CLI/MCP; #3 revert pack code.
