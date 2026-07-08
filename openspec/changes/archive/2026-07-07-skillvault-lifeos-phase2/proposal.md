# Proposal: SkillVault LifeOS Phase 2 — Purpose + Run Bridge + MCP Tools

## Intent

Close remaining Phase 2 gaps from the SkillVault ↔ Workflow-Builder + LifeOS comparison: (5) purpose taxonomy, (6) structured run bridge, (7) MCP exposure of route/run metadata. Item (4) `skillvault route` is already implemented (PR #16) — treated as completed baseline, not new scope.

## Scope

### In Scope

- **Purpose taxonomy**: Add `purpose` field to Entry (WORK, KNOWLEDGE, LEARNING, RELATIONSHIP, STATE). Orthogonal to entry type. Filter/search by purpose, CLI `--purpose`, MCP param, migration 007.
- **Run bridge**: Add `RunPipelineStructured()` accepting step inputs as structured arguments (not stdin). Expose `run_workflow` MCP tool returning step-by-step JSON-RPC results with progress semantics.
- **Route/run MCP tools**: Expose `route_scenario` MCP tool (wraps existing `RouteScenario`). Wire `WorkflowRunService` into `ToolRegistry`.

### Out of Scope

- OBSERVABILITY purpose (deferred — LifeOS v7.6 has 6 purposes; Phase 2 starts with 5).
- Windows `agentcore_publish` integration (target repo inaccessible; portable SkillVault-side changes only).
- Conditional/parallel workflow execution (run bridge stays sequential).
- `progress.yaml` file I/O (run bridge returns progress structure, does not write files).

## Capabilities

### New Capabilities

- `entry-purpose-taxonomy`: Purpose field on Entry with 5-value enum, validation, search/filter, import/export.
- `workflow-run-bridge`: Structured pipeline execution method + `run_workflow` MCP tool.
- `mcp-route-tool`: `route_scenario` MCP tool exposing existing route resolution.

### Modified Capabilities

- `cli-commands`: add-entry gains `--purpose`; search gains `--purpose` filter.
- `mcp-tools`: save_entry gains `purpose` param; search_entries gains `purpose` filter; new `run_workflow` and `route_scenario` tools (count increases from 16→18).

## Approach

Two chained PRs, each under 400 changed lines:

1. **PR A (purpose taxonomy)**: Domain types + validation, migration 007, store CRUD, CLI/MCP params, import/export. Pure additive — no behavior change for existing entries (purpose defaults empty).
2. **PR B (run bridge + MCP tools)**: `RunPipelineStructured()` in `WorkflowRunService`, wire into `ToolRegistry`, `run_workflow` and `route_scenario` MCP tools. Builds on PR A foundation (ToolRegistry gains `workflowRunSvc` field parallel to existing `workflowSvc`).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/domain/entry.go` | Modified | +`Purpose` type, constants, `Entry.Purpose`, `EntryFilter.Purpose`, `SearchQuery.Purpose` |
| `internal/domain/filters.go` | Modified | +`Purpose` in `SearchQuery` |
| `internal/domain/validation.go` | Modified | +`ValidatePurpose()` |
| `internal/db/migrations/007_purpose.sql` | New | Rebuild entries with `purpose TEXT DEFAULT ''` column |
| `internal/db/schema.sql` | Modified | +`purpose` column, +index |
| `internal/db/entries_store.go` | Modified | INSERT/UPDATE/SELECT include purpose |
| `internal/db/import_export_store.go` | Modified | Export/import includes purpose column |
| `internal/app/entries.go` | Modified | `SaveEntry`/`SearchEntries` accept purpose |
| `internal/app/workflow_runs.go` | Modified | +`RunPipelineStructured()` |
| `internal/mcp/tools.go` | Modified | +`workflowRunSvc` field, +`run_workflow`, +`route_scenario` tools |
| `internal/cli/commands.go` | Modified | +`--purpose` flags |
| `cmd/skillvault/main.go` | Modified | Wire `WorkflowRunService` into MCP, pass purpose to entry ops |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Migration 007 rebuilds entries table — data loss if columns mismatch | Low | Follow 006 pattern: `INSERT INTO entries_new SELECT * FROM entries` then rename. Add `purpose` column with default. |
| Purpose field backward compat — old entries lack purpose | Low | Default empty string; nullable in domain, `DEFAULT ''` in schema. Existing CLI/MCP calls work unchanged. |
| Structured run differs from stdin model | Med | Keep `RunPipeline` for CLI compat. `RunPipelineStructured` is additive — new method, new MCP tool. Both use same store layer. |

## Rollback Plan

- Migration 007 is additive — `purpose` column only. Rollback: rebuild entries without purpose column (reverse migration). No data loss (purpose is metadata).
- MCP tools are additive — remove from `registerV2Tools()` to revert. Existing 16 tools unaffected.
- `RunPipelineStructured` is new code path — CLI `run` command still uses `RunPipeline`.

## Dependencies

- PR #16 (`feature/skillvault-route-command`) — completed. `RouteScenario` already in `EntryService`, ready for MCP exposure.
- Migration 006 (routing entry type) — already applied in live DBs.

## Success Criteria

- [ ] `add-entry --purpose KNOWLEDGE` persists purpose; `search --purpose WORK` filters correctly.
- [ ] `run_workflow` MCP tool executes a workflow with structured step input and returns JSON results with step statuses.
- [ ] `route_scenario` MCP tool returns matched workflow/skill for a scenario string.
- [ ] `go test ./...` passes; all 3 tests for new behavior included per work unit.
- [ ] Import/export round-trips preserve purpose field.

## Delivery Strategy

- PR strategy: **force-chained** (2 PRs from `feature/skillvault-lifeos-phase2`)
- PR A: purpose taxonomy — ~300 lines
- PR B: run bridge + MCP tools — ~350 lines
- Review budget: 400 changed lines per PR
