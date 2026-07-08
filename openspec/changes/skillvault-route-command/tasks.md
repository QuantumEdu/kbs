# Tasks: skillvault route \<scenario\>

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~235 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

## Phase 1: Foundation — Types and CLI Flags

- [x] 1.1 Add `RouteTarget` (yaml struct: `workflow`, `skill` fields), `RouteResult` (json struct) types to `internal/app/entries.go`
- [x] 1.2 Add `RouteFlags{Scenario string, JSON bool}` + `ParseRouteFlags()` to `internal/cli/commands.go` (~25 LOC)
- [x] 1.3 Add `workflowStore db.WorkflowStore` field + `SetWorkflowStore()` to `EntryService` in `internal/app/entries.go`

## Phase 2: Core Implementation

- [x] 2.1 Implement `RouteScenario(ctx, scenario)` on `EntryService`: FTS5 search (type=routing) → tag fallback (`workflow-route`) → YAML body key match → `workflowStore.Get()` verify target (~70 LOC)
- [x] 2.2 Wire `entrySvc.SetWorkflowStore(store.Workflows)` in `openVault()` at `cmd/skillvault/main.go`

## Phase 3: CLI Integration

- [x] 3.1 Add `"route"` (requires 1 arg) to `ParseCommand` switch in `internal/cli/commands.go`
- [x] 3.2 Add `"route"` to `commandDescs` map in `cmd/skillvault/main.go`
- [x] 3.3 Add `case "route":` handler in `runCLI` switch: parse flags, call `RouteScenario`, output human-readable (name + steps) or `--json` dump (~50 LOC)

## Phase 4: Testing

- [x] 4.1 Unit test `ParseRouteFlags` — positional arg + `--json` — in `internal/cli/cli_test.go`
- [x] 4.2 Integration: route resolves to workflow (create routing entry + workflow, call `RouteScenario`) — `internal/app/app_test.go`
- [x] 4.3 Integration: malformed YAML skipped, valid entry resolves — `internal/app/app_test.go`
- [x] 4.4 Integration: stale workflow reference warns and continues — `internal/app/app_test.go`
- [x] 4.5 Integration: no matching entries returns error with creation hint — `internal/app/app_test.go`
