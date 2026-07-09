# SkillVault Delta Spec — versioning-pack

## ADDED Requirements

### REQ-VER-01: Entry Versions Table

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-VER-01 | `entry_versions` table with columns: `version_id` TEXT PK, `entry_id` TEXT NOT NULL REFERENCES entries(id), `version_number` INTEGER NOT NULL, `title` TEXT NOT NULL, `summary` TEXT DEFAULT '', `body_optional` TEXT DEFAULT '', `saved_at` DATETIME DEFAULT CURRENT_TIMESTAMP. UNIQUE on `(entry_id, version_number)`. | MUST |

**Scenarios**:
- GIVEN entry E exists, WHEN migration 009 runs, THEN `entry_versions` table is created with correct schema.
- GIVEN entry E is saved twice with title changes, WHEN `entry_versions` is queried by E's `entry_id`, THEN two version rows exist with `version_number` 1 and 2.

### REQ-VER-02: Version History Query

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-VER-02 | `ListVersions(entry_id)` returns all versions for an entry ordered by `version_number` DESC. Each result includes `version_id`, `entry_id`, `version_number`, `title`, `summary`, `body_optional`, `saved_at`. | MUST |

**Scenarios**:
- GIVEN entry E has 3 versions, WHEN `skillvault entry history <E>` is called, THEN versions are listed in descending order showing version number, title, and saved_at.
- GIVEN entry E has 0 versions (never updated), WHEN `ListVersions` is called, THEN empty list returned with no error.
- GIVEN entry ID "nonexistent" does not exist, WHEN `ListVersions` is called, THEN returns empty list (no error — entry may exist in future).

### REQ-VER-03: Version Restore

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-VER-03 | `RestoreVersion(entry_id, version_number)` retrieves version content (title, summary, body_optional) and calls `Save()` to create a new current version with that content. The restore itself auto-creates a new version row capturing the state before restore. | MUST |

**Scenarios**:
- GIVEN entry E has version 1 (title "A") and current version 2 (title "B"), WHEN `skillvault entry restore <E> --version 1` runs, THEN current title is "A", AND a new version 3 is created recording "B" as previous state.
- GIVEN entry E has version 5, WHEN `restore --version 5` is called, THEN version 5 content becomes current, AND version 6 captures the pre-restore state.
- GIVEN version 99 does not exist for entry E, WHEN `restore --version 99` is called, THEN error: "version 99 not found for entry <E>".

### REQ-PACK-01: Pack Export Format

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-PACK-01 | `VaultPackExport` struct wraps `VaultExport` with additional fields: `pack_id` (unique, auto-generated), `author`, `version` (semver string), `description`, `exported_at` (RFC 3339). Output JSON has top-level `pack` key containing pack metadata and nested `data` key with `VaultExport`. CLI: `skillvault export --pack "Name" --author "user" --version "1.0" --output pack.svpack`. When `--pack` is omitted, export produces bare `VaultExport` (existing behavior). | MUST |

**Scenarios**:
- GIVEN vault with 5 entries and 1 project, WHEN `skillvault export --pack "My Pack" --author "alice" --version "1.0" --output pack.svpack`, THEN output JSON has `pack` key with `pack_id`, `author`, `version`, `description`, `exported_at`, and nested `data` matching `VaultExport`.
- GIVEN vault with 3 entries, WHEN `skillvault export --output bare.json` (no `--pack`), THEN output is bare `VaultExport` JSON with `schema_version`, `app_version`, `exported_at`, `source`, `data` (no `pack` key).
- GIVEN pack export runs, WHEN `exported_at` is set, THEN timestamp is RFC 3339 format.

### REQ-PACK-02: Pack Import with Prefix

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-PACK-02 | Import detects pack format by the presence of a `pack` key at the top level. When `pack` is present: entry IDs, project IDs, and all foreign key references are prefixed with the `--prefix` value (e.g., `"shared/"`). When `pack` is absent, import proceeds as bare `VaultExport` (backward compat). CLI: `skillvault import --pack pack.svpack --prefix "imported/"`. Prefix default: `""` (no prefix). Slug conflict resolution applies after prefixing. | MUST |

**Scenarios**:
- GIVEN pack file `pack.svpack` with entry ID `abc-123`, WHEN `skillvault import --pack pack.svpack --prefix "shared/"` runs, THEN imported entry ID becomes `shared/abc-123`, AND all foreign keys referencing `abc-123` are rewritten to `shared/abc-123`.
- GIVEN bare export file `bare.json` (no `pack` key), WHEN `skillvault import bare.json` runs, THEN import proceeds as bare `VaultExport` with no prefixing (backward compat).
- GIVEN pack import with prefix `"shared/"`, WHEN entry slug `my-slug` conflicts with existing entry, THEN conflict suffix is appended: `my-slug-import-<hex>` (existing behavior, applied after prefix).
- GIVEN pack import with empty prefix, WHEN import runs, THEN entry IDs are imported as-is (no namespace).

## MODIFIED Requirements

### REQ-CLI-02: CLI Command Count

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-CLI-02 | Required commands include all existing commands plus: `entry history`, `entry restore`. | MUST |

**Scenarios** (new):
- GIVEN entry E with 3 versions, WHEN `skillvault entry history <E>` runs, THEN version list is printed with version_number, title, saved_at.
- GIVEN entry E exists with version 1, WHEN `skillvault entry restore <E> --version 1` runs, THEN version 1 content is restored and a new version captures the pre-restore state.

### REQ-MCP-01: MCP Tool Count

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MCP-01 | 24 MCP tools: all existing 22 tools plus `list_entry_versions`, `restore_entry_version`. | MUST |

*Delta: tool count 22 → 24. Added `list_entry_versions` and `restore_entry_version`.*

**Scenarios** (new):
- GIVEN entry E has 2 versions, WHEN MCP `list_entry_versions` is called with `entry_id=E`, THEN versions returned in descending order with version_number, title, saved_at.
- GIVEN entry E has version 1, WHEN MCP `restore_entry_version` is called with `entry_id=E, version=1`, THEN version 1 content becomes current and a new version captures pre-restore state.
