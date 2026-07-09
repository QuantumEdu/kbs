# Delta for CLI Commands

## ADDED Requirements

### Requirement: Route Scenario Resolution (REQ-CLI-ROUTE)

The system SHALL provide a `route <scenario>` CLI command that resolves a scenario string to a matching workflow or skill by searching routing-type entries. The resolution cascade: (1) FTS5 search on routing entries, (2) tag search fallback (`workflow-route`), (3) YAML body parse for exact key match, (4) target verification via `WorkflowService.Get()`. Human-readable output by default; `--json` flag for machine-parseable output. Malformed YAML in one entry MUST NOT block resolution of others (warn to stderr, continue). Stale workflow references SHALL warn and continue.

#### Scenario: Route resolves to workflow

- GIVEN a routing entry with YAML body mapping `research: {workflow: research-workflow}`
- WHEN `skillvault route research` runs
- THEN workflow name, description, and steps are displayed
- AND exit code is 0

#### Scenario: JSON output

- GIVEN a routing entry mapping `onboarding: {skill: onboarding-skill}`
- WHEN `skillvault route --json onboarding` runs
- THEN valid JSON prints with fields: scenario, type, target, description
- AND exit code is 0

#### Scenario: No matching routing entries

- GIVEN no routing entries match the scenario
- WHEN `skillvault route nonexistent` runs
- THEN message shows "No routing entries found" with creation hint (`add-entry --type routing`)
- AND exit code is non-zero

#### Scenario: Malformed YAML does not block resolution

- GIVEN two routing entries: one with invalid YAML, one valid matching the scenario
- WHEN `skillvault route <scenario>` runs
- THEN malformed entry is skipped with stderr warning
- AND valid entry resolves and displays

#### Scenario: Stale workflow reference

- GIVEN routing entry references a deleted workflow slug
- WHEN `skillvault route <scenario>` runs
- THEN warning "Referenced workflow X not found" prints
- AND resolution continues to other entries
