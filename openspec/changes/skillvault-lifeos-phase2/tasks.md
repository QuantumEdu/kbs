# Tasks: SkillVault LifeOS Phase 2 — Purpose + Run Bridge + MCP Tools

## Review Workload Forecast

Estimated changed lines: PR A ~280, PR B ~320. Both under 400-line review budget.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: Low

**Work units:** PR A (purpose taxonomy, base: feature/skillvault-lifeos-phase2) → PR B (run bridge + MCP, base: PR A branch).

**Gate warnings addressed:** (1) Migration 007 explicit INSERT column list + `''` default (task 2.2). (2) `EntryFilter.Purpose` excluded (dead API). (3) `route_scenario` rejects empty scenario before service call (task 4.3).

## Phase 1: Domain (PR A)

- [x] 1.1 RED: Add `TestPurpose_IsValid` + `TestValidatePurpose` in `internal/domain/validation_test.go`.
- [x] 1.2 GREEN: Add `Purpose` type, 5 constants, `IsValid()` in `internal/domain/entry.go`. Add `ValidatePurpose()` in `internal/domain/validation.go`. Add `Purpose` to `Entry`. Add `Purpose *string` to `SearchQuery` in `filters.go`. Exclude `EntryFilter` (gate #2).

## Phase 2: Database (PR A)

- [x] 2.1 RED: Purpose persistence round-trip test in `internal/app/app_test.go`.
- [x] 2.2 GREEN: Create `internal/db/migrations/007_purpose.sql` — explicit column list `INSERT INTO entries_new (..., purpose) SELECT ..., '' FROM entries`. Recreate indexes + FTS5. Add `purpose TEXT DEFAULT ''` + index to `internal/db/schema.sql`.
- [x] 2.3 GREEN: Update `internal/db/entries_store.go`: add `purpose` to `Save()` INSERT/ON CONFLICT, to `Get()`/`Search()`/`List()`/`SearchByTags()` SELECTs. Add `AND e.purpose = ?` in `Search()` when filter present.
- [x] 2.4 GREEN: Update `internal/db/import_export_store.go`: add `purpose` to `exportEntries()` SELECT and `ImportAll()` entry INSERT. Bump `exportSchemaVersion` to 3.

## Phase 3: App + CLI + MCP (PR A)

- [x] 3.1 GREEN: Add `Purpose` to `SaveEntryInput` in `internal/app/entries.go`. Validate non-empty in `SaveEntry()`, set on Entry. Pass through `SearchEntries()`.
- [x] 3.2 GREEN: Add `Purpose` to `AddEntryFlags`/`SearchFlags` in `internal/cli/commands.go` + `--purpose` flag. Test in `internal/cli/cli_test.go`.
- [x] 3.3 GREEN: Add `purpose` param to `save_entry`/`search_entries` MCP schemas in `internal/mcp/tools.go`. Extract in handlers.

## Phase 4: Run Bridge + MCP (PR B)

- [ ] 4.1 RED: Add `TestRunPipelineStructured_Success`/`_StepFailure`/`_PreFlightReject`/`_CLIUnchanged` in `internal/app/workflow_runs_test.go`.
- [ ] 4.2 GREEN: Add `StructuredRunResult` + `StructuredStepResult` to `internal/domain/workflow.go`. Implement `RunPipelineStructured(ctx, workflowRef, stepInputs)` in `internal/app/workflow_runs.go` — reuses store, `maxPreviousOutputLen`, `vars.Resolve`. Step failures produce `status:"failed"` not Go error.
- [ ] 4.3 RED: Add `TestRouteScenario_MCP` (match, no-match, empty rejection per gate #3) in `internal/mcp/mcp_test.go`.
- [ ] 4.4 GREEN: Add `workflowRunSvc` + `WithWorkflowRunService()` to `ToolRegistry` in `internal/mcp/tools.go`. Register `run_workflow` + `route_scenario` in `registerV2Tools()`. `handleRouteScenario` MUST reject empty scenario before service call.
- [ ] 4.5 GREEN: Wire `WorkflowRunService` via `WithWorkflowRunService()` in `cmd/skillvault/main.go`.

## Phase 5: Verification

- [ ] 5.1 `go test ./...` — all pass. Backward compat: old entries without purpose survive.
- [ ] 5.2 `go vet ./...` — clean.
- [ ] 5.3 Verify PR A independently before PR B.
