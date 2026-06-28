# Proposal: Entry Versioning + Skill Pack Export

## Intent

Users lose entry history on every `Save()` (UPSERT overwrites content). Recovering past versions of prompts, decisions, or workflows is impossible. Also, sharing curated vault content between instances requires wrapping the bare JSON export with pack metadata (author, version, description) and a namespaced import path.

## Scope

### In Scope
- **Entry versioning**: Archive old title/summary/body before UPSERT; query version history; restore any version as current
- **Skill pack export**: Wrap `VaultExport` with pack metadata (`pack_id`, `author`, `version`, `description`, `exported_at`); export to `.svpack` file
- **Skill pack import**: Detect pack vs bare format; prefix entry IDs with namespace; backward-compatible with v2 JSON imports
- **CLI**: `entry history`, `entry restore`, `export --pack`, `import --pack --prefix`
- **MCP**: `list_entry_versions`, `restore_entry_version`

### Out of Scope
- Diff between versions (use existing `compare-entries`)
- Version pruning/retention policies
- Pack signing or integrity checks beyond existing SHA-256
- Pack merge or conflict resolution for overlapping IDs
- Artifact versioning (entry content only)

## Capabilities

### New Capabilities
- `entry-versioning`: Immutable entry version history, version query, version restore
- `skill-pack-export`: Wrapped export format with pack metadata and namespaced import

### Modified Capabilities
- `skillvault`: CLI command count 18→20 (`entry history`, `entry restore`); MCP tool count 16→18 (`list_entry_versions`, `restore_entry_version`)

## Approach

**Entry Versioning**: New `entry_versions` table stores snapshots of `title`, `summary`, `body_optional` with auto-incremented `version_number` and `saved_at`. `sqliteEntryStore.Save()` compares old vs new values before UPSERT; only archives when content changes. New `EntryVersionStore` interface and `EntryVersionService` app layer.

**Skill Pack Export**: `VaultPackExport` wraps `VaultExport` + pack metadata. `VaultPackExportService` writes a `.svpack` file (JSON with `pack` envelope key). Import detects `pack` key → strips prefix onto entry IDs → delegates to existing `ImportAll`.

**Schema version stays at 2**. Both features are additive.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/db/migrations/` | New | `006_entry_versions.sql` |
| `internal/db/store.go` | Modified | Add `EntryVersionStore` interface + wire `sqliteEntryVersionStore` |
| `internal/db/entries_store.go` | Modified | `Save()` archives before UPSERT |
| `internal/domain/entry.go` | Modified | Add `EntryVersion` struct |
| `internal/domain/filters.go` | Modified | Add `VaultPackExport` struct |
| `internal/app/` | New/Modified | `EntryVersionService`, `VaultPackExportService`, extend `VaultExportService`/`VaultImportService` |
| `internal/cli/commands.go` | Modified | `entry` subcommands (`history`, `restore`); `export --pack`; `import --pack --prefix` |
| `internal/mcp/tools.go` | Modified | Register `list_entry_versions`, `restore_entry_version` |
| `cmd/skillvault/main.go` | Modified | runCLI dispatch for new commands |
| `internal/db/schema.sql` | Modified | Add `entry_versions` DDL |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Version table grows unbounded | Medium | Document retention policy for future; skip for v1 |
| Pack prefix collision with existing IDs | Low | Prefix is namespace separator; import validates no overlap |
| Race condition on concurrent `Save()` + version insert | Low | Transaction wraps both operations |

## Rollback Plan

- `entry_versions` is append-only: drop migration 006 + rollback code, no data loss in entries table
- Pack export is a new code path; bare export unchanged
- Pack import prefix is additive; bare import unchanged

## Dependencies

- Migration 006 runs after existing migrations 001–005

## Success Criteria

- [ ] `Save()` on existing entry with changed body creates a version row with previous content
- [ ] `entry history <id>` lists all versions with version number, saved_at, and summary
- [ ] `entry restore <id> --version 1` restores title/summary/body from that version (creates new current version)
- [ ] `export --pack "My Pack" --author "user" --output pack.svpack` produces valid JSON with `pack` key
- [ ] `import --pack pack.svpack --prefix "shared/"` prefixes all entry IDs; bare import still works
- [ ] MCP `list_entry_versions` and `restore_entry_version` mirror CLI behavior
- [ ] Existing tests pass; new tests cover version archive, restore, pack round-trip
