# Design: Entry Versioning + Skill Pack Export

## Architecture Overview

```
┌───────────────────────────────────────────────────┐
│  CLI (cmd/skillvault/main.go)                     │
│  entry history → EntryVersionService.ListVersions  │
│  entry restore → EntryVersionService.RestoreVersion│
│  export --pack → VaultPackExportService.Export     │
│  import --pack → VaultImportService.ImportPack     │
└──────────────┬────────────────────────────────────┘
               │
┌──────────────▼────────────────────────────────────┐
│  App Layer (internal/app/)                        │
│  EntryVersionService     (new)                    │
│  VaultPackExportService  (new)                    │
│  VaultExportService      (modified: pack path)    │
│  VaultImportService      (modified: pack detection)│
└──────────────┬────────────────────────────────────┘
               │
┌──────────────▼────────────────────────────────────┐
│  Store Layer (internal/db/)                       │
│  EntryVersionStore interface   (new in store.go)   │
│  sqliteEntryVersionStore       (new file)         │
│  sqliteEntryStore.Save()       (modified)         │
│  sqliteImportExportStore       (modified: prefix) │
└──────────────┬────────────────────────────────────┘
               │
┌──────────────▼────────────────────────────────────┐
│  Domain (internal/domain/)                        │
│  EntryVersion struct            (new in entry.go) │
│  VaultPackExport struct         (new in filters.go)│
└──────────────────────────────────────────────────┘
```

## New Components

### 1. `entry_versions` Table (Migration 009)

```sql
-- 009_entry_versions.sql
CREATE TABLE IF NOT EXISTS entry_versions (
    version_id      TEXT PRIMARY KEY,
    entry_id        TEXT NOT NULL REFERENCES entries(id),
    version_number  INTEGER NOT NULL,
    title           TEXT NOT NULL,
    summary         TEXT DEFAULT '',
    body_optional   TEXT DEFAULT '',
    saved_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entry_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_entry_versions_entry ON entry_versions(entry_id);

INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (9, 'entry_versions');
```

### 2. Domain Structs

```go
// internal/domain/entry.go (add)
type EntryVersion struct {
    VersionID     string
    EntryID       string
    VersionNumber int
    Title         string
    Summary       string
    BodyOptional  string
    SavedAt       time.Time
}
```

```go
// internal/domain/filters.go (add)
type VaultPackExport struct {
    Pack PackMetadata  `json:"pack"`
    Data VaultExport   `json:"data"`
}

type PackMetadata struct {
    PackID      string `json:"pack_id"`
    Author      string `json:"author"`
    Version     string `json:"version"`
    Description string `json:"description"`
    ExportedAt  string `json:"exported_at"` // RFC 3339
}
```

### 3. Store Interface + Implementation

```go
// internal/db/store.go (add to Store struct + interface)
type EntryVersionStore interface {
    SaveVersion(ctx context.Context, v domain.EntryVersion) error
    ListVersions(ctx context.Context, entryID string) ([]domain.EntryVersion, error)
    GetVersion(ctx context.Context, entryID string, versionNumber int) (domain.EntryVersion, error)
}

// internal/db/store.go (add to Store struct)
type Store struct {
    // ... existing fields ...
    EntryVersions EntryVersionStore
}

// internal/db/store.go (add to NewStore)
EntryVersions: &sqliteEntryVersionStore{db: db},
```

**`sqliteEntryStore.Save()` modification** — ARCHIVE before UPSERT:

```go
func (s *sqliteEntryStore) Save(ctx context.Context, entry domain.Entry, tags []string) error {
    tx, err := s.db.BeginTx(ctx, nil)
    // ... existing setup ...

    // ARCHIVE old version if content changed
    if entry.ID != "" {
        existing, _ := s.getCurrentForVersioning(ctx, tx, entry.ID)
        if existing != nil && contentChanged(existing, &entry) {
            nextVersion := existing.currentMaxVersion + 1
            _, err = tx.ExecContext(ctx, `
                INSERT INTO entry_versions (version_id, entry_id, version_number, title, summary, body_optional)
                VALUES (?, ?, ?, ?, ?, ?)
            `, newID(), entry.ID, nextVersion, existing.Title, existing.Summary, existing.BodyOptional)
            // ... handle error ...
        }
    }

    // ... existing UPSERT ...
    return tx.Commit()
}

func contentChanged(old *versionSnapshot, new *domain.Entry) bool {
    return old.Title != new.Title || old.Summary != new.Summary || old.BodyOptional != new.BodyOptional
}
```

### 4. App Services

**`EntryVersionService`** (`internal/app/entry_versions.go`):

```go
type EntryVersionService struct {
    versions EntryVersionStore
    entries  EntryStore
}

func (s *EntryVersionService) ListVersions(ctx context.Context, entryID string) ([]domain.EntryVersion, error)
func (s *EntryVersionService) RestoreVersion(ctx context.Context, entryID string, versionNumber int) (*domain.Entry, error)
```

`RestoreVersion` flow:
1. Call `GetVersion(entryID, versionNumber)` to retrieve archived content
2. Call `GetEntry(entryID)` to get current entry
3. Call `Save()` with version's title/summary/body overwriting current fields
4. The `Save()` call itself archives the pre-restore state (creates version N+1)

**`VaultPackExportService`** (`internal/app/pack_export.go`):

```go
type VaultPackExportService struct {
    exportSvc *VaultExportService
}

func (s *VaultPackExportService) ExportPack(ctx context.Context, input PackExportInput) error
```

`ExportPack` flow:
1. Call `exportSvc.ExportAll()` to get `VaultExport`
2. Wrap in `VaultPackExport` with input metadata
3. Marshal to JSON and write to output path

### 5. Import Pack Detection + Prefix

**`VaultImportService.Import()` modification**:

```go
// Signature: Import(ctx context.Context, path string, prefix string) error
// The prefix parameter is optional (empty string default) — backward-compatible.
func (s *VaultImportService) Import(ctx context.Context, path string, prefix string) error {
    jsonBytes, err := os.ReadFile(path)

    // Detect pack format
    var raw map[string]json.RawMessage
    json.Unmarshal(jsonBytes, &raw)
    if _, isPack := raw["pack"]; isPack {
        var pack VaultPackExport
        json.Unmarshal(jsonBytes, &pack)
        if prefix != "" {
            s.applyPrefix(&pack.Data, prefix)
        }
        return s.ImportVault(ctx, pack.Data)
    }

    // Bare export (backward compat)
    var data VaultExport
    json.Unmarshal(jsonBytes, &data)
    return s.ImportVault(ctx, data)
}
```

`applyPrefix` prepends `prefix` to all IDs in `VaultData`:
- `Entries[].ID`, `Entries[].ProjectID`, `Entries[].ArtifactID`
- `Projects[].ID`
- `Artifacts[].ID`, `Artifacts[].ProjectID`, `Artifacts[].SourceEntryID`
- `Workflows[].ID`, `WorkflowSteps[].WorkflowID`, `WorkflowSteps[].ID`
- `Series[].ID`, `SeriesEntries[].SeriesID`, `SeriesEntries[].EntryID`
- `EntryTags[].EntryID`, `EntryLinks[].FromEntryID`, `EntryLinks[].ToEntryID`
- `WorkflowRuns[].WorkflowID`, `WorkflowRunSteps[].RunID`, `WorkflowRunSteps[].EntryID`

### 6. CLI Specification

#### `entry history`
```
Usage: skillvault entry history <entry_id_or_slug>
Output: table with columns Version, Title, Saved At
        1  "Original Title" 2026-06-28T12:00:00Z
        2  "Updated Title"  2026-06-28T13:00:00Z
```

#### `entry restore`
```
Usage: skillvault entry restore <entry_id_or_slug> --version <N>
Output: "Restored entry <id> to version N (previous state saved as version N+1)"
```

Both added to `entry` subcommand dispatch in `cli/commands.go`:

```go
case "entry":
    sub2 := args[2]
    switch sub2 {
    case "ref":  return "entry-ref", nil
    case "history":
        if len(args) < 4 { return error }
        return "entry-history", nil
    case "restore":
        if len(args) < 4 { return error }
        return "entry-restore", nil
    }
```

#### `export --pack`
```
Usage: skillvault export --pack "Pack Name" --author "user" --version "1.0" [--description "text"] --output pack.svpack
Flags: --pack (string, required for pack mode), --author (string, required), --version (semver string, required),
       --description (string, optional), --output (path, required)
```

#### `import --pack --prefix`
```
Usage: skillvault import --pack pack.svpack --prefix "namespace/"
Flags: --pack (path, required), --prefix (string, optional, default "")
Backward compat: skillvault import file.json works as before
```

### 7. MCP Tool Specification

#### `list_entry_versions`
```
Input:  { "entry_id": string (required) }
Output: JSON array of { version_id, entry_id, version_number, title, summary, body_optional, saved_at }
```

#### `restore_entry_version`
```
Input:  { "entry_id": string (required), "version": number (required) }
Output: { "entry": Entry, "restored_from_version": number, "new_version_created": number }
```

### 8. Testing Strategy

| Layer | Test File | What |
|-------|-----------|------|
| DB | `internal/db/entry_versions_store_test.go` | `SaveVersion`, `ListVersions`, `GetVersion`, version auto-increment |
| DB | `internal/db/entries_store_test.go` (extend) | `Save()` archives old content; no archive on no-change |
| App | `internal/app/entry_versions_test.go` | `ListVersions`, `RestoreVersion` round-trip |
| App | `internal/app/pack_export_test.go` | Pack export round-trip; import with prefix; bare backward compat |
| CLI | `internal/cli/cli_test.go` (extend) | `ParseCommand` for `entry history`, `entry restore`; export/import flag parsing |
| MCP | `internal/mcp/mcp_test.go` (extend) | `list_entry_versions`, `restore_entry_version` dispatch |
| Integration | Existing acceptance tests | All existing tests pass; no regression |

### 9. Schema Version Policy

- **Schema version**: remains `2` (additive only, no breaking changes)
- **App version**: `v3` (unchanged)
- Migration 009 is additive — no ALTER on existing tables
- Bare `VaultExport` format unchanged; pack is a wrapper
- Existing exports import unchanged
