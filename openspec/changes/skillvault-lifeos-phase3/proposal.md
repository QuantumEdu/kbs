# Proposal: SkillVault LifeOS Phase 3 — Observability + Workflow Analytics

## Intent
SkillVault captures workflow run data but has no analytics, progress tracking, or observability purpose. Agents and users cannot answer: "How many runs completed? What failed? Where am I in this workflow?" Phase 3 adds minimal analytics on existing data plus the OBSERVABILITY purpose deferred from Phase 2.

## Scope

### In Scope
- Workflow run aggregation: total runs, success rate, avg/min/max duration, failed step counts — from existing `runs`/`run_steps` tables
- Completed-vs-total step ratio per run (progress tracking)
- CLI `stats --workflow-runs` output and new MCP tools (`get_stats`, `list_workflow_runs`, `get_run`)
- `OBSERVABILITY` purpose value added to taxonomy (was deferred, now shipped)

### Out of Scope
- ISA / Ideal State Artifact
- Purpose-aware context compiler
- Workflow-builder export
- Route miss tracking (needs new logging infrastructure — deferred to Phase 3b)
- Schema changes to `runs`/`run_steps` tables
- Changes to `RunPipeline` or `RunPipelineStructured`

## Capabilities

### New Capabilities
- `workflow-analytics`: Aggregated run metrics — total runs, success rate, duration stats, completed/total step ratio. Pure aggregate queries on existing data.

### Modified Capabilities
- `entry-purpose-taxonomy`: Add `OBSERVABILITY` to the 5 existing purpose values (was explicitly deferred in v3 spec REQ-PUR-01)
- `cli-commands`: Extend `stats` command with `--workflow-runs` flag and per-workflow breakdown
- `mcp-tools`: Add `get_stats` (run analytics), `list_workflow_runs`, and `get_run` tools

## Approach

Three chained PRs under 400 lines each — Feature Branch Chain strategy:

| PR | Scope | Est. Lines | Dependencies |
|----|-------|-----------|--------------|
| A: Analytics Core | Aggregate queries on `WorkflowRunStore`, `WorkflowRunStats` type, wire into `StatsService`, tests | ~300 | None |
| B: CLI + MCP Surface | `stats --workflow-runs` CLI, `get_stats`/`list_workflow_runs`/`get_run` MCP tools, tests | ~350 | PR A (`WorkflowRunStats`) |
| C: OBSERVABILITY Purpose | Migration 008, add `OBSERVABILITY` to `Purpose` type, update validation, schema sync | ~200 | None (standalone) |

### OBSERVABILITY Decision
**INCLUDE as PR C.** Risk is low — same table-rebuild migration pattern as 006/007. Backward-compatible (existing entries default to empty purpose). If risk materializes during apply, PR C can be reverted independently.

## Affected Areas

| Area | Impact | Lines |
|------|--------|-------|
| `db/store.go` — `WorkflowRunStore` interface | +3 methods | ~15 |
| `db/workflow_run_store.go` — SQL aggregate queries | New queries | ~60 |
| `app/stats.go` — `WorkflowRunStats`, dependency | Extended | ~100 |
| `app/stats_test.go` | New tests | ~80 |
| `cli/commands.go` — `stats --workflow-runs` | Extended | ~40 |
| `mcp/tools.go` — 3 new tools + registry | New | ~120 |
| `cmd/skillvault/main.go` — wiring | Extended | ~30 |
| `domain/entry.go` — `+OBSERVABILITY` | +3 lines | ~3 |
| `domain/validation.go` — updated error msg | +1 line | ~1 |
| `db/migrations/008_observability_purpose.sql` | New | ~60 |
| `db/schema.sql` | Sync | ~2 |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| PR B exceeds 350 line estimate | Low | Trim to essential MCP tools only; `list_workflow_runs` can be deferred |
| Migration 008 table rebuild data loss | Low | Follow proven 006/007 pattern; test with populated DB |
| StatsService grows beyond single responsibility | Low | 4 dependencies still manageable; if >5, extract `WorkflowAnalyticsService` |

## Rollback Plan

- PR A: Revert `WorkflowRunStore` interface additions and `StatsService` wiring — no schema changes
- PR B: Revert CLI/MCP tool registrations — no schema changes
- PR C: Migration 008 is additive only; revert migration file and remove `OBSERVABILITY` from `Purpose.IsValid()` — existing entries with empty purpose are unaffected

## Success Criteria

- [ ] `WorkflowRunStats` returns total runs, success rate, avg/max/min duration, and step completion ratio from real `runs`/`run_steps` data
- [ ] `skillvault stats --workflow-runs` prints per-workflow run metrics
- [ ] `get_stats` MCP tool returns `workflow_runs` block alongside entry/artifact stats
- [ ] `OBSERVABILITY` purpose passes `ValidatePurpose`, migration 008 preserves all existing data
- [ ] All 3 PRs individually pass `go test ./...` with `CGO_ENABLED=0`
- [ ] No changes to `RunPipeline` or `RunPipelineStructured` behavior
