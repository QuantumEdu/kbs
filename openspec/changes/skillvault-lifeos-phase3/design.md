# Design: SkillVault LifeOS Phase 3 — Observability + Workflow Analytics

## Technical Approach

Add aggregate SQL queries to the existing `WorkflowRunStore` (no schema changes to `runs`/`run_steps`), wire results into `StatsService`, surface via extended CLI `stats` command and 3 new MCP tools. Add `OBSERVABILITY` purpose via migration 008 following the proven table-rebuild pattern from 006/007. Three chained PRs deliver the work under 400 lines each.

## Architecture Decisions

| Decision | Option A | Option B | Choice | Rationale |
|----------|----------|----------|--------|-----------|
| Duration computation | Store pre-computed `duration_ms` column | Compute `finished_at - started_at` in SQL | **Option B** | No schema change; SQLite `julianday` is fast; no migration risk |
| StatsService dependency | Add `WorkflowRunStore` as 4th field | Extract separate `WorkflowAnalyticsService` | **Option A** | 4 deps is manageable; single `GetStats` call is simpler for consumers |
| MCP tool wiring | Add `statsSvc` field to `ToolRegistry` | Call `WorkflowRunStore` + entry/artifact stores separately in handler | **Option A** | Follows existing `With*` builder pattern; cleaner than reconstructing stats in handler |
| Migration 008 strategy | `ALTER TABLE ADD COLUMN` | Table rebuild (007 pattern) | **Option B** | SQLite doesn't support `ALTER TABLE ADD COLUMN ... CHECK`; rebuild is proven and safe |
| `list_workflow_runs` signature | Accept `workflow_id` + `limit` (existing `ListRuns`) | Add new `ListAllRuns` with optional `workflow_id`, `limit`, `offset` | **Option B** | REQ-MCP-15 needs optional workflow_id; `ListRuns` requires it. New method avoids breaking the existing interface |
| Progress tracking | Compute `completed/total` via SQL `SUM(status='completed')/COUNT(*)` per `run_id` | Store `completed_steps`/`total_steps` denormalized on `runs` | **Option A** | No schema change; computed on read; low row count on `run_steps` per run |

## Data Flow

```
CLI: stats --workflow-runs --json   MCP: get_stats / list_workflow_runs / get_run
       │                                      │
       ▼                                      ▼
  StatsService.GetStats()               ToolRegistry.dispatch()
       │                                      │
       ├── EntryStore.List() ────────── statsSvc.GetStats()
       ├── ArtifactStore.List()              │ or
       ├── ProjectStore.List()         workflowRunSvc.GetRun() / ListAllRuns()
       └── WorkflowRunStore.GetRunStats()    │
                          │                  ▼
                          ▼          sqliteWorkflowRunStore
              SELECT COUNT(*),           │
              AVG(julianday(finished_at)-julianday(started_at))*86400,   │
              ... FROM runs;             ▼
                          └───────→ WorkflowRunStats → VaultStats
```

## Interfaces / Contracts

### WorkflowRunStore additions (`internal/db/store.go`)

```go
type WorkflowRunStats struct {
    TotalRuns       int
    CompletedRuns   int
    FailedRuns      int
    AvgDurationSecs float64
    MaxDurationSecs float64
    MinDurationSecs float64
    FailedStepCount int
    PerWorkflow     []WorkflowRunPerWorkflow
}

type WorkflowRunPerWorkflow struct {
    WorkflowID     string
    TotalRuns      int
    CompletedRuns  int
    AvgDurationSecs float64
}

type RunProgress struct {
    RunID          string
    CompletedSteps int
    TotalSteps     int
}

// Added to WorkflowRunStore interface:
type WorkflowRunStore interface {
    // ... existing methods unchanged ...
    GetRunStats(ctx context.Context, workflowID *string) (*WorkflowRunStats, error)
    ListAllRuns(ctx context.Context, workflowID *string, limit, offset int) ([]domain.WorkflowRun, []RunProgress, error)
}
```

### VaultStats extension (`internal/app/stats.go`)

```go
type VaultStats struct {
    // ... existing fields unchanged ...
    WorkflowRuns *WorkflowRunStats `json:"workflow_runs,omitempty"`
}
```

### ToolRegistry additions (`internal/mcp/tools.go`)

```go
// New fields:
statsSvc *app.StatsService

// New builder:
func (r *ToolRegistry) WithStatsService(svc *app.StatsService) *ToolRegistry

// New tools:
{Name: "get_stats", ...}            // JSON: entry/artifact/project + workflow_runs
{Name: "list_workflow_runs", ...}   // args: workflow_id? (optional), limit (default 20)
{Name: "get_run", ...}              // args: run_id; returns run + step array
```

### CLI flags (`internal/cli/commands.go`)

```go
type StatsFlags struct {
    WorkflowRuns bool
    JSON         bool
}
```

## File Changes

| File | Action | Description | PR |
|------|--------|-------------|-----|
| `internal/db/store.go` | Modify | Add `WorkflowRunStats`, `RunProgress` types; add `GetRunStats`, `ListAllRuns` to `WorkflowRunStore` interface | A |
| `internal/db/workflow_run_store.go` | Modify | Implement `GetRunStats` (aggregate SQL) and `ListAllRuns` (optional `workflow_id`, steps JOIN for progress) | A |
| `internal/db/workflow_run_store_test.go` | Modify | Tests for aggregate queries: empty DB, mixed-status runs, per-workflow filter, zero values | A |
| `internal/app/stats.go` | Modify | Add `WorkflowRunStats` to `VaultStats`; add `WorkflowRunStore` dep; extend `GetStats` to call `GetRunStats(nil)` | A |
| `internal/app/stats_test.go` | Modify | New mock `WorkflowRunStore`; test run stats integration in `VaultStats` | A |
| `internal/cli/commands.go` | Modify | Add `StatsFlags`, `ParseStatsFlags()`, support `--workflow-runs` and `--json` | B |
| `internal/cli/cli_test.go` | Modify | Tests for `ParseStatsFlags` | B |
| `internal/mcp/tools.go` | Modify | Add `statsSvc` field, `WithStatsService`, 3 tool definitions, 3 handlers, 3 dispatch cases | B |
| `internal/mcp/mcp_test.go` | Modify | Tests for `get_stats`, `list_workflow_runs`, `get_run`; update tool count to 22 | B |
| `cmd/skillvault/main.go` | Modify | Wire `WorkflowRunStore` into `StatsService`; add `stats` case with `--json`/`--workflow-runs`; wire `statsSvc` into `ToolRegistry` for MCP | B |
| `internal/domain/entry.go` | Modify | Add `PurposeObservability = "OBSERVABILITY"`; add to `IsValid()` switch | C |
| `internal/domain/validation.go` | Modify | Update `ValidatePurpose` error message to include `OBSERVABILITY` | C |
| `internal/domain/validation_test.go` | Modify | Add `OBSERVABILITY` valid/invalid test cases | C |
| `internal/db/migrations/008_observability_purpose.sql` | Create | Table-rebuild migration: recreate entries with updated `purpose` CHECK constraint including `OBSERVABILITY` | C |
| `internal/db/schema.sql` | Modify | Sync entries `purpose` CHECK constraint to include `OBSERVABILITY` | C |

## SQL Query Strategy (No Schema Changes)

Duration is computed via `julianday()` (SQLite function) on existing `started_at`/`finished_at` columns:

```sql
-- Aggregate run stats (optional per-workflow filter):
SELECT
    COUNT(*)                              AS total_runs,
    SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END) AS completed,
    SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END)    AS failed,
    AVG(julianday(finished_at) - julianday(started_at)) * 86400 AS avg_secs,
    MAX(julianday(finished_at) - julianday(started_at)) * 86400 AS max_secs,
    MIN(julianday(finished_at) - julianday(started_at)) * 86400 AS min_secs
FROM runs WHERE finished_at IS NOT NULL
  AND (? IS NULL OR workflow_id = ?);  -- optional workflow_id filter

-- Per-workflow breakdown:
SELECT workflow_id, COUNT(*), SUM(CASE WHEN status='completed' ...), AVG(...)
FROM runs WHERE finished_at IS NOT NULL
GROUP BY workflow_id;

-- Failed step count (across all runs):
SELECT COUNT(*) FROM run_steps WHERE status = 'failed';

-- Progress per run (completed/total steps):
SELECT rs.run_id, 
       SUM(CASE WHEN rs.status='completed' THEN 1 ELSE 0 END) AS completed,
       COUNT(*) AS total
FROM run_steps rs
WHERE rs.run_id IN (SELECT id FROM runs ... )
GROUP BY rs.run_id;
```

**Empty data handling**: `COALESCE(AVG(...), 0)` ensures zero values, not NULLs. `AVG`/`MAX`/`MIN` on empty sets return NULL, caught with `COALESCE`.

## Non-Interference with RunPipeline/RunPipelineStructured

All changes are **read-only queries** against `runs`/`run_steps`. `CreateRun`, `UpdateStepStatus`, `UpdateRunStatus` are unchanged. `RunPipeline` and `RunPipelineStructured` call only these write paths. No behavioral change.

## PR Slicing (Feature Branch Chain)

| PR | Branch target | Scope | Est. lines | Testable independently? |
|----|--------------|-------|-----------|------------------------|
| A: Analytics Core | `feature/skillvault-lifeos-phase3` | Store queries + StatsService extension + tests | ~300 | Yes — `go test ./internal/db/... ./internal/app/...` |
| B: CLI + MCP Surface | PR A branch | CLI flags + MCP tools + wiring + tests | ~350 | Yes — requires PR A types, integrates full stack |
| C: OBSERVABILITY | PR B branch | Domain + migration + schema sync + tests | ~200 | Yes — migration is standalone; `IsValid()` extended |

PR C is independent of A/B logically but chains on B for merge order. All three under 400 lines.

## Testing Strategy

| Layer | What to Test | Approach | PR |
|-------|-------------|----------|-----|
| Unit — Store | Aggregate SQL returns correct counts/durations with mixed statuses | `:memory:` DB, insert test data, call `GetRunStats`/`ListAllRuns`, assert | A |
| Unit — Store | Empty DB returns zero values, not errors | Same pattern | A |
| Unit — Store | Per-workflow filter scopes correctly | Insert runs for 2 workflows, query one | A |
| Unit — App | `VaultStats.WorkflowRuns` populated when `WorkflowRunStore` present | Mock `WorkflowRunStore`, assert struct fields | A |
| Unit — CLI | `ParseStatsFlags` parses `--workflow-runs` and `--json` | Table-driven tests, existing `cli_test.go` pattern | B |
| Integration — MCP | `get_stats` returns JSON with `workflow_runs` block | `:memory:` DB with seed data, call `reg.Call(ctx, "get_stats", ...)`, parse JSON | B |
| Integration — MCP | `list_workflow_runs` returns runs with step ratio | Seed runs, call tool, assert | B |
| Integration — MCP | `get_run` returns run + steps array | Seed run with steps, call tool | B |
| Integration — MCP | `get_run` errors on nonexistent ID | Call with bad ID, assert `IsError` | B |
| Unit — Domain | `OBSERVABILITY` passes/fails `IsValid()` and `ValidatePurpose` | Extend existing table-driven tests | C |
| Integration — DB | Migration 008 runs, existing data preserved, `OBSERVABILITY` accepted | `:memory:` DB, insert pre-migration data, run migration, verify | C |

## Migration 008 Strategy

Follows `007_purpose.sql` pattern exactly:
1. `PRAGMA foreign_keys=OFF`
2. Create `entries_new` with updated purpose CHECK constraint: `purpose TEXT DEFAULT '' CHECK (purpose IN ('', 'WORK', 'KNOWLEDGE', 'LEARNING', 'RELATIONSHIP', 'STATE', 'OBSERVABILITY'))`
3. `INSERT INTO entries_new (...) SELECT ... FROM entries` (explicit column list, `purpose` copied as-is)
4. Drop `entries`, rename `entries_new` → `entries`
5. Recreate indexes (`idx_entries_type`, `idx_entries_project_id`, `idx_entries_status`, `idx_entries_slug`)
6. Rebuild FTS5: `INSERT INTO entries_fts(entries_fts) VALUES('rebuild')`
7. `PRAGMA foreign_keys=ON`
8. `INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (8, 'observability_purpose')`

**Backward compatibility**: Existing entries with `purpose=''` or any of the 5 existing values are unaffected. Only the CHECK constraint expands.

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| SQLite `julianday()` on NULL `finished_at` for running runs | Medium | Filter `WHERE finished_at IS NOT NULL` in aggregate queries; running runs excluded from duration stats |
| Migration 008 FTS5 compatibility — `entries_new` missing `tags_denorm`/other columns from 001-007 chain | Low | Follow `007_purpose.sql` as template; use same column list as current `entries` schema |
| `list_workflow_runs` step ratio JOIN may be slow on thousands of runs | Low | `run_steps` per run is bounded (workflow steps are ~3-10); indexed by `run_id` already |
| Tool count test breaks (expects 19, becomes 22) | Certain | Update test constants; the `TestToolCountIncludesNewTools` and `TestToolsListReturns19Tools` tests explicitly count tools |

## Open Questions

None — all design decisions resolved by codebase inspection and spec requirements.
