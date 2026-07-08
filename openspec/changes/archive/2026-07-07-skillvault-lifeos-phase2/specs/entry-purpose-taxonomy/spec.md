# Entry Purpose Taxonomy Specification

## Purpose

The `purpose` field classifies entries by their LifeOS purpose, orthogonal to entry type. Five values represent the v7.6 purpose model (minus OBSERVABILITY, deferred). Missing/empty purpose is backward-compatible — existing entries and calls continue working unchanged.

## Requirements

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-PUR-01 | The system SHALL support five purpose values: `WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`. | MUST |
| REQ-PUR-02 | Empty purpose (`""`) SHALL be valid and represent "unset" — backward-compatible default for all existing entries and calls without purpose. | MUST |
| REQ-PUR-03 | The system SHALL reject any purpose value not in the allowed set with a validation error naming the invalid value. | MUST |
| REQ-PUR-04 | `search` CLI command and `search_entries` MCP tool SHALL accept an optional `--purpose` / `purpose` filter that returns only entries matching the given purpose value. | MUST |
| REQ-PUR-05 | `add-entry` CLI SHALL accept optional `--purpose` flag accepting one of the five values. | MUST |
| REQ-PUR-06 | `save_entry` MCP tool SHALL accept an optional `purpose` parameter accepting one of the five values or empty. | MUST |
| REQ-PUR-07 | Export SHALL include the `purpose` field for each entry. Import SHALL restore it — full round-trip fidelity. | MUST |
| REQ-PUR-08 | Purpose SHALL be stored as a `TEXT` column in the entries table, defaulting to empty string (`""`), added via migration 007. | MUST |

## Scenarios

### Scenario: Save entry with valid purpose
- GIVEN a vault with no prior purpose usage
- WHEN `save_entry` is called with `purpose: "KNOWLEDGE"` or `add-entry --purpose KNOWLEDGE`
- THEN the entry is persisted with `purpose = "KNOWLEDGE"`.

### Scenario: Save entry with invalid purpose
- GIVEN a valid entry payload
- WHEN `save_entry` is called with `purpose: "INVALID_VALUE"`
- THEN the save is rejected with a validation error indicating "INVALID_VALUE" is not a recognized purpose.

### Scenario: Save entry with empty purpose (backward compat)
- GIVEN a valid entry payload with no `purpose` field (or empty string)
- WHEN the entry is saved
- THEN the entry is persisted with `purpose = ""` — no error, backward-compatible behavior.

### Scenario: Search filter by purpose
- GIVEN 3 entries: one WORK, one KNOWLEDGE, one with empty purpose
- WHEN `search_entries` is called with `purpose: "WORK"`
- THEN only the WORK entry is returned.

### Scenario: Search without purpose filter (backward compat)
- GIVEN entries with various purposes including empty
- WHEN `search_entries` is called without a `purpose` parameter
- THEN all entries are returned regardless of their purpose value.

### Scenario: CLI add-entry with purpose
- GIVEN a running vault
- WHEN `skillvault add-entry --title "Review" --type reference --purpose LEARNING`
- THEN the entry is persisted with purpose LEARNING.

### Scenario: Import/export round-trip preserves purpose
- GIVEN an export containing entries with purposes WORK, KNOWLEDGE, and empty
- WHEN the export is imported into a fresh vault
- THEN all entries retain their original purpose values.

### Scenario: CLI search with purpose filter
- GIVEN entries with purposes WORK, KNOWLEDGE, LEARNING, and an empty-purpose entry
- WHEN `skillvault search --purpose WORK`
- THEN only WORK entries appear in results.
