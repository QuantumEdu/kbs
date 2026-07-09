# Exploration: SkillVault LifeOS Phase 3 — Observability + Workflow Analytics

## Current State

### Workflow Run Persistence (Phase 2)
The `runs` and `run_steps` tables already capture everything needed for analytics:

```
runs: id | workflow_id | input | output | status(pending|running|completed|failed) | started_at | finished_at
run_steps: id | run_id | step_id | entry_id | input | output | status | started_at | finished_at
```

- **Duration is computable**: `finished_at - started_at` on both run and step level — no schema change needed.
- **Status transitions**: pending → running → completed|failed, tracked per-step and per-run.
- **`WorkflowRunStore` interface**: `CreateRun`, `GetRun`, `ListRuns(workflowID, limit)`, `UpdateStepStatus`, `UpdateRunStatus`. No aggregate queries, no cross-workflow listing, no status-filtered queries.
- **Export**: runs/run_steps are included in `VaultData` for import/export round-trip.

### Existing Stats Service
`internal/app/stats.go` — `VaultStats` today: entries, artifacts, projects, char counts, today's counts, token estimate. No workflow run data. `StatsService` has no `WorkflowRunStore` dependency.

### Route Command (Phase 2)
`route` CLI / `route_scenario` MCP: resolves scenario to workflow/skill. Route misses are returned as errors inline — **not persisted, not tracked, not capturable** for analytics without new infrastructure.

### Purpose Taxonomy (Phase 2)
5 current values: `WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`. `OBSERVABILITY` was explicitly deferred in the spec (REQ-PUR-01: "minus OBSERVABILITY, deferred"). Purpose is validated via `domain.Purpose.IsValid()` and stored as a TEXT column in entries.

### Existing MCP Tools
20 tools registered: `save_entry`, `search_entries`, `get_entry`, `save_artifact`, `get_context`, `compose_series`, `render_workflow`, `session_wrap`, `archive_entry`, `list_projects`, `save_entry_ref`, `list_entry_refs`, `get_entry_graph`, `search_by_tags`, `get_context_bundle`, `compare_entries`, `save_result`, `run_workflow`, `route_scenario`.

No MCP tools exist for: stat retrieval, run listing, run detail retrieval, or workflow progress tracking.

## Affected Areas

- `internal/domain/workflow.go` — `WorkflowRun`, `WorkflowRunStep` — stable, no changes needed
- `internal/domain/entry.go` — `Purpose` type — add `OBSERVABILITY` (+3 lines)
- `internal/domain/validation.go` — `ValidatePurpose` — update error message (+1 line)
- `internal/db/store.go` — `WorkflowRunStore` interface — add aggregate query methods (+~15 lines)
- `internal/db/workflow_run_store.go` — SQLite implementation — add aggregate queries (+~60 lines)
- `internal/app/stats.go` — `StatsService` — new `WorkflowRunStats`, new dependency, extended `GetStats` (+~100 lines)
- `internal/cli/commands.go` — `stats` command — new flags for run analytics, JSON output (+~40 lines)
- `internal/mcp/tools.go` — `ToolRegistry` — new `get_stats`, `list_workflow_runs`, `get_run` tools (+~120 lines)
- `cmd/skillvault/main.go` — wire `WorkflowRunStore` into `StatsService`, new CLI switch cases (+~30 lines)
- `internal/db/migrations/008_observability_purpose.sql` — new migration (+~60 lines)
- `internal/db/schema.sql` — sync after migration (+~2 lines)
- `internal/db/migrate.go` — embed migration 008 (+~3 lines)
- `internal/app/stats_test.go` — new test cases for run stats (+~80 lines)
- `internal/db/workflow_run_store_test.go` — new test cases for aggregate queries (+~60 lines)
- `internal/mcp/mcp_test.go` — new test cases for observability MCP tools (+~80 lines)
- `openspec/specs/skillvault/spec.md` — delta spec updates (+~30 lines)

## Approaches

### 1. Single-PR: Everything in one changeset
- **Pros**: Fastest to deliver, no PR chaining overhead
- **Cons**: Estimated 600+ changed lines across domain, store, app, CLI, MCP, migration, tests — exceeds 400-line review budget by ~50%
- **Effort**: Medium

### 2. Three Chained PRs: Layer by layer (recommended)
- **PR A: Workflow Run Analytics Core** (~300 lines): Add aggregate queries to `WorkflowRunStore`, extend `VaultStats` with `WorkflowRunStats`, wire `WorkflowRunStore` into `StatsService`, add corresponding tests. No CLI/MCP changes yet.
- **PR B: CLI + MCP Observability Surface** (~350 lines): New `stats --workflow-runs` CLI output, new `get_stats` MCP tool (run analytics), new `list_workflow_runs` MCP tool, new `get_run` MCP tool. Integration tests.
- **PR C: Purpose OBSERVABILITY** (~200 lines): Add `OBSERVABILITY` to purpose taxonomy, migration 008, update validation, FTS5/schema sync, CLI/MCP help text, spec updates.
- **Pros**: Each PR ≤400 lines, independently testable, progressive value delivery
- **Cons**: 3 PRs to review; PR B depends on PR A's `WorkflowRunStats` type
- **Effort**: Medium (but well-structured)

### 3. Two PRs: Data + Surface combined, Purpose separate
- **PR A: Run Analytics + Observability Surface** (~550 lines): PRs A+B from option 2 merged
- **PR B: Purpose OBSERVABILITY** (~200 lines): Same as PR C above
- **Pros**: Fewer PRs, faster delivery
- **Cons**: PR A exceeds 400 lines, needs `size:exception` or `auto-chain` strategy
- **Effort**: Medium

## Recommendation

**Approach 2 — Three chained PRs.** Each slice is under 400 changed lines, independently testable, and delivers value incrementally:

1. **PR A** establishes the data layer — aggregate SQL queries on existing tables return real analytics. Testable in isolation via `go test`.
2. **PR B** surfaces that data through CLI and MCP — agents and users can immediately query workflow run history, progress, and success rates.
3. **PR C** adds the `OBSERVABILITY` purpose to round out the taxonomy — low-risk enum addition with full backward compatibility.

### Route Miss Tracking — deferred

Route misses are currently not persisted. Capturing them requires:
- A new `route_log` or `route_misses` table
- A `RouteService` or modifying `EntryService.RouteScenario` to log attempts
- Additional analytics queries

This is **out of scope** for the minimal Phase 3. Route miss analytics should be a separate change (Phase 3b) or bundled with Phase 4 if the user wants it. The current route mismatch is already returned as a helpful error message, which is adequate for progress tracking.

### What NOT to do (per explicit user constraints)
- Do NOT build ISA/Ideal State Artifact
- Do NOT build purpose-aware context compiler
- Do NOT build workflow-builder export
- Do NOT change existing `RunPipeline` or `RunPipelineStructured` behavior
- Do NOT modify the runs/run_steps schema (current fields are sufficient)

## Risks

- **StatsService growing**: Adding `WorkflowRunStore` as a 4th dependency is manageable but the constructor signature will change. Tests use mock stores; this is a straightforward update.
- **Migration 008 rebuild**: Following the existing 006/007 pattern (rebuild entries table, copy data, drop old, rename). This is proven and safe but requires careful schema sync with `schema.sql`.
- **MCP tool count**: Adding 3 new tools brings the total to 23. The registry is flat and dispatch is a switch statement — no scaling concern yet.
- **Purpose CHECK constraint**: Migration 008 must update the CHECK constraint to include `OBSERVABILITY`. Existing entries with empty purpose or invalid purpose are unaffected (rebuild pattern preserves data, default empty).

## Ready for Proposal

Yes. The data model supports analytics out of the box. The implementation is additive — extend existing `StatsService`, add aggregate queries to existing `WorkflowRunStore`, surface via new CLI flags and MCP tools. No schema redesign, no breaking changes, no data migration on the runs/run_steps side. Purpose OBSERVABILITY is a low-risk enum addition following the same migration pattern as 006/007.

### Measurable Outcomes (all achievable without schema changes)

| Metric | Data Source | Query |
|--------|-------------|-------|
| Total runs | `runs` table | `SELECT COUNT(*) FROM runs` |
| Success rate | `runs.status` | `completed / total` |
| Failed steps | `run_steps.status` | `WHERE status='failed'` |
| Avg/max/min duration | `runs.started_at`, `runs.finished_at` | Time diff aggregations |
| Completed/total phases per run | `run_steps.status` per `run_id` | Grouped counts |
| Runs per workflow | `runs.workflow_id` | `GROUP BY workflow_id` |
| Route misses | ❌ Not captured | Needs new infrastructure — deferred |
