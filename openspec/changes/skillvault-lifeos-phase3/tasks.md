# Tasks: SkillVault LifeOS Phase 3 — Observability + Workflow Analytics

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~850 total: PR A ~300, PR B ~350, PR C ~200 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR A → PR B → PR C (Feature Branch Chain) |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Aggregation queries + StatsService | PR A | Base: `feature/skillvault-lifeos-phase3`; tests included |
| 2 | CLI `--json`/`--workflow-runs` + 3 MCP tools | PR B | Base: PR A branch; tests included |
| 3 | OBSERVABILITY purpose + migration 008 | PR C | Base: PR B branch; standalone domain change; tests included |

## Phase 1: PR A — RED: Store Tests

- [x] 1.1 Write `TestGetRunStats_MixedStatuses` in `internal/db/workflow_run_store_test.go` — seed 7 completed + 2 failed + 1 running run; assert total=10, completed=7, failed=2, duration stats non-nil
- [x] 1.2 Write `TestGetRunStats_Empty` in `internal/db/workflow_run_store_test.go` — no runs; assert all zero values, no error
- [x] 1.3 Write `TestGetRunStats_PerWorkflow` in `internal/db/workflow_run_store_test.go` — seed 2 workflows (3 runs + 2 runs); filter one; assert scope
- [x] 1.4 Write `TestListAllRuns_WithProgress` in `internal/db/workflow_run_store_test.go` — seed run with 5 steps (3 completed); assert step_ratio=0.6, struct populated

## Phase 2: PR A — GREEN: Store Types + Queries

- [x] 1.5 Add `WorkflowRunStats`, `WorkflowRunPerWorkflow`, `RunProgress` types and `GetRunStats`/`ListAllRuns` methods to `WorkflowRunStore` interface in `internal/db/store.go`
- [x] 1.6 Implement `GetRunStats` in `internal/db/workflow_run_store.go` — SQL: COUNT, SUM CASE status, COALESCE(AVG/MAX/MIN(julianday*86400)), per-workflow GROUP BY, optional workflow_id filter
- [x] 1.7 Implement `ListAllRuns` in `internal/db/workflow_run_store.go` — optional workflow_id, limit/offset, LEFT JOIN run_steps for completed/total ratio per run via subquery

## Phase 3: PR A — RED: StatsService Tests

- [x] 1.8 Write `TestGetStats_WorkflowRunsPopulated` in `internal/app/stats_test.go` — mock returns non-nil stats; assert `VaultStats.WorkflowRuns` populated
- [x] 1.9 Write `TestGetStats_NoWorkflowRunsWhenStoreNil` in `internal/app/stats_test.go` — nil WorkflowRunStore; assert `WorkflowRuns` is nil

## Phase 4: PR A — GREEN: StatsService Wiring

- [x] 1.10 Add `WorkflowRunStore` interface subset + `workflowRunStore` field + `WithWorkflowRunStore()` builder to `StatsService` in `internal/app/stats.go`
- [x] 1.11 Extend `VaultStats` with `WorkflowRuns *WorkflowRunStats \`json:"workflow_runs,omitempty"\``; call `GetRunStats(ctx, nil)` in `GetStats` when store non-nil

## Phase 5: PR B — RED: CLI Tests

- [x] 2.1 Write `TestParseStatsFlags_WorkflowRuns` in `internal/cli/cli_test.go` — table-driven: `--workflow-runs` true, `--json` true, defaults false

## Phase 6: PR B — GREEN: CLI Surface

- [x] 2.2 Add `StatsFlags{WorkflowRuns, JSON bool}` and `ParseStatsFlags()` in `internal/cli/commands.go`
- [x] 2.3 Wire `stats` case in `cmd/skillvault/main.go` — when `--workflow-runs`, call `GetRunStats(nil)` on StatsService; `--json` outputs JSON with `workflow_runs` block; default prints `FormatStats`

## Phase 7: PR B — RED: MCP Tests

- [x] 2.4 Write `TestGetStatsMCP` in `internal/mcp/mcp_test.go` — seed DB with runs; call `get_stats`; assert JSON has `workflow_runs` block with total/completed/duration
- [x] 2.5 Write `TestListWorkflowRunsMCP` in `internal/mcp/mcp_test.go` — seed runs across 2 workflows; call `list_workflow_runs(workflow_id, limit=5)`; assert count=5, step_ratio present
- [x] 2.6 Write `TestGetRunMCP` in `internal/mcp/mcp_test.go` — seed run with 3 steps; call `get_run(run_id)`; assert steps array with status/output (includes not-found case from 2.7)
- [x] 2.7 Write `TestGetRunMCP_NotFound` in `internal/mcp/mcp_test.go` — merged into 2.6

## Phase 8: PR B — GREEN: MCP Tools

- [x] 2.8 Add 3 tool definitions to `registerV2Tools()` in `internal/mcp/tools.go`: `get_stats` (no args), `list_workflow_runs` (opt workflow_id, limit), `get_run` (run_id required)
- [x] 2.9 Add `statsSvc *app.StatsService`, `WithStatsService()`, and 3 handler methods in `internal/mcp/tools.go`
- [x] 2.10 Add 3 `dispatch` cases; wire `statsSvc` chained in `runMCP()` in `cmd/skillvault/main.go`
- [x] 2.11 Update `TestToolsListReturns19Tools` and `TestToolCountIncludesNewTools` in `internal/mcp/mcp_test.go` to expect 22 tools

## Phase 9: PR C — RED: Domain Tests

- [ ] 3.1 Write `TestPurpose_OBSERVABILITY_valid` in `internal/domain/validation_test.go` — `PurposeObservability.IsValid()` true; `ValidatePurpose("OBSERVABILITY")` nil
- [ ] 3.2 Write `TestPurpose_OBSERVABILITY_invalid` in `internal/domain/validation_test.go` — `Purpose("obs").IsValid()` false; `ValidatePurpose("INVALID")` err; assert error msg includes OBSERVABILITY

## Phase 10: PR C — GREEN: Domain + Migration

- [ ] 3.3 Add `PurposeObservability Purpose = "OBSERVABILITY"` and add to `IsValid()` switch case in `internal/domain/entry.go`
- [ ] 3.4 Update `ValidatePurpose` error message in `internal/domain/validation.go` to list OBSERVABILITY
- [ ] 3.5 Create `internal/db/migrations/008_observability_purpose.sql` — rebuild entries (007 pattern): `entries_new` with `purpose TEXT DEFAULT '' CHECK(purpose IN ('', 'WORK', 'KNOWLEDGE', 'LEARNING', 'RELATIONSHIP', 'STATE', 'OBSERVABILITY'))`; explicit INSERT column list; swap; rebuild indexes + FTS5
- [ ] 3.6 Sync `internal/db/schema.sql` — update entries purpose line 31 from `purpose TEXT DEFAULT '',` to `purpose TEXT DEFAULT '' CHECK(purpose IN ('', 'WORK', 'KNOWLEDGE', 'LEARNING', 'RELATIONSHIP', 'STATE', 'OBSERVABILITY')),`

## Phase 11: PR C — RED: Migration Test

- [ ] 3.7 Write `TestMigration008_OBSERVABILITY` in `internal/db/workflow_run_store_test.go` (or new `migration_test.go`) — seed entries with various purposes; run migration 008; assert data preserved; assert OBSERVABILITY entry saves and queries cleanly
