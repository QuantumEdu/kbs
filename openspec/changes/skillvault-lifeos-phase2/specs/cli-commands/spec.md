# Delta for CLI Commands

## MODIFIED Requirements

### Requirement: REQ-CLI-04 — add-entry flags

`add-entry` SHALL accept `--title`, `--type`, `--summary` (required), `--body`, `--project`, `--tags`, `--status`, `--purpose` (optional).

(Previously: `add-entry` did not accept `--purpose` flag.)

#### Scenario: add-entry with purpose
- GIVEN a running vault
- WHEN `skillvault add-entry --title "Go Patterns" --type reference --purpose KNOWLEDGE`
- THEN the entry is persisted with purpose KNOWLEDGE.

#### Scenario: add-entry without purpose (backward compat)
- GIVEN a running vault
- WHEN `skillvault add-entry --title "Go Patterns" --type reference` without `--purpose`
- THEN the entry is persisted with empty purpose — no error.

#### Scenario: add-entry with invalid purpose
- GIVEN a running vault
- WHEN `skillvault add-entry --title "Bad" --type reference --purpose INVALID`
- THEN the command exits with a validation error indicating the purpose value is not recognized.

### Requirement: REQ-CLI-11 — search filters

`search` SHALL support `--query`, `--type`, `--project`, `--tags`, `--include-archived`, `--limit`, `--vector`, `--purpose`.

(Previously: `search` did not support `--purpose` filter.)

#### Scenario: search filtered by purpose
- GIVEN entries with purposes WORK, KNOWLEDGE, and empty
- WHEN `skillvault search --purpose KNOWLEDGE`
- THEN only KNOWLEDGE entries are returned.

#### Scenario: search without purpose filter (backward compat)
- GIVEN entries with various purposes
- WHEN `skillvault search --query "patterns"` without `--purpose`
- THEN all matching entries are returned regardless of purpose.
