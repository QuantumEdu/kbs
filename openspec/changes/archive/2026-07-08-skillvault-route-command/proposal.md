# Proposal: skillvault route <scenario>

## Intent

AI agents in coding harnesses need to discover which workflow or skill handles a given scenario without manually searching YAML routing bodies. Human power users need the same via CLI. The `routing` entry type exists in the domain model and database, but no resolution command exists — users must grep YAML bodies by hand.

## Scope

### In Scope
- `skillvault route <scenario>` CLI command, human-readable output by default
- `--json` flag for machine-parseable output (scenario name, route type, target slug, description)
- Resolve routing entries by exact title, tag, or FTS5 body search
- Parse YAML routing bodies, verify target workflow/skill exists
- Friendly error messages for missing entries, malformed YAML, stale references

### Out of Scope
- MCP tool or HTTP endpoint (Phase 2)
- Creating routing entry type — already exists in domain model (`EntryTypeRouting`)
- Skill-level resolution (routes to workflow slugs only; skill resolution deferred)

## Capabilities

### New Capabilities

None. The route command extends the existing CLI Commands capability.

### Modified Capabilities

- **CLI Commands** (Capability 11): Add `route` command with scenario resolution, `--json` flag, YAML body parsing, and target verification. New requirement: REQ-CLI-ROUTE.

## Approach

**App Service Method** (Approach 1 from exploration). Add `RouteScenario(ctx, scenario string) (*RouteResult, error)` to `EntryService` in `internal/app/entries.go`. Encapsulates search → YAML parse → resolve logic. Testable, reusable by future MCP/HTTP. CLI handler in `cmd/skillvault/main.go` formats output.

Resolution cascade:
1. FTS5 search (type=routing, query=scenario)
2. Fallback: tag search (`workflow-route`)
3. Parse YAML bodies for exact scenario key match
4. Verify target workflow/skill exists via `WorkflowService.Get()`
5. Output human-readable or `--json` dump

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/skillvault/main.go` | Modified | New `case "route":` handler, command description |
| `internal/cli/commands.go` | Modified | `RouteFlags`, `ParseRouteFlags()`, add to dispatch |
| `internal/app/entries.go` | Modified | New `RouteScenario()` method |
| `internal/app/app_test.go` | Modified | Acceptance test for route resolution |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| No routing entries exist | Low | Clear message with creation hint (`add-entry --type routing`) |
| Malformed YAML in routing body | Medium | Skip bad entries, warn to stderr, continue resolution |
| Workflow/skill slug stale | Medium | Warn "Referenced X not found", continue checking others |

## Rollback Plan

Revert the commit. No database schema changes — routing entry type and FTS5 already exist. Command is purely additive with no side effects.

## Dependencies

- `yaml.v3` already in `go.mod` (used by `internal/db/workflow_import_store.go`)
- `WorkflowService.Get()` and `EntryService.SearchEntries()`/`SearchByTags()` already exist

## Success Criteria

- [ ] `skillvault route research` resolves to workflow/skill when routing entry exists
- [ ] `skillvault route --json research` outputs valid JSON with scenario, type, target, description
- [ ] Nonexistent scenario prints helpful "no routing entries" message with creation hint
- [ ] Malformed YAML in one routing entry does not break resolution of others
- [ ] Acceptance test in `internal/app/app_test.go` passes
