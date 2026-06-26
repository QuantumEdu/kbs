# Design: SkillVault v3 Workflow Pipelines

## Technical Approach

Extend workflows from renderable checklists to sequential prompt-composition pipelines. SkillVault resolves `{{input}}`/`{{previous_output}}`/`{{final_output}}` system variables into entry bodies per step, outputs composed prompts to stdout, reads step results from stdin, and persists run records. No LLM calls — SkillVault is the prompt orchestrator; the external agent/LLM executes each step.

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|----------|--------|----------|-----------|
| Run ID type | TEXT PK with `run-` prefix (matches `wf-`, `proj-` pattern) | INTEGER AUTOINCREMENT | Consistent with project UUID convention |
| RunStep ID type | TEXT PK with `rst-` prefix | INTEGER (like `workflow_steps.id`) | New table, no legacy constraint; TEXT is standard for all new entities |
| Status model | Custom `RunStatus`: `pending`/`running`/`completed`/`failed` | Reuse `domain.Status` | Domain status (draft/active/archived) is lifecycle; run status is transient execution state |
| Step-entry link | Add `entry_slug TEXT` to `workflow_steps` via ALTER TABLE (nullable, additive) | New junction table | Single column is enough; nullable preserves backward compat (null = renderable-only) |
| Entry resolution | Pre-flight: resolve all slugs → entry IDs → validate active status; store resolved `entry_id` in `run_steps` | Lazy resolve at step time | Pre-flight prevents partial runs; stored entry_id survives entry renames mid-pipeline |
| Truncation | Cap `{{previous_output}}` at 32768 chars; log warning if truncated | No truncation | 32K prevents context explosion; Go string slicing, no allocation |
| Interactive model | Per-step: stdout composed prompt → read step result from stdin → store | One-shot non-interactive | Agent/LLM needs per-step prompt composition; Unix pipe model fits agent orchestration |
| CLI args | `run <workflow> <file|-|--stdin> [--save <path>]` | `run <workflow> --input <file>` | Matches spec: positional workflow + file, optional --save; `-` means stdin |
| Version bump | `const version = "v3"` in main.go + import_export_store.go + MCP server.go; SchemaVersion stays 2 | Bump SchemaVersion | Data is additive (runs/run_steps); old exports import cleanly into v3 |

## Data Flow

```
Input (file/stdin)
     │
     ▼
┌─────────────────────────────────┐
│  WorkflowRunService.RunPipeline │
│                                 │
│  1. Pre-flight: resolve slugs   │
│     → validate entries active   │
│  2. Create run + run_steps      │
│     (status: pending)           │
│  3. For each step:              │
│     a. vars.Resolve(body,       │
│        {{input}}/{{prev}})      │
│     b. stdout: composed prompt  │
│     c. stdin: step result       │
│     d. UpdateStepStatus(result) │
│  4. Concatenate {{final_output}}│
│     → stdout or --save file     │
└─────────────────────────────────┘
     │                    │
     ▼                    ▼
  Store (runs,        stdout/--save
  run_steps rows)        file
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/domain/workflow.go` | Modify | Add `WorkflowRun`, `WorkflowRunStep`, `RunStatus` types; add `EntrySlug string` to `WorkflowStep` |
| `internal/domain/filters.go` | Modify | Add `WorkflowRuns`, `WorkflowRunSteps` to `VaultData` |
| `internal/db/store.go` | Modify | Add `WorkflowRunStore` interface + `sqliteWorkflowRunStore` struct |
| `internal/db/workflow_run_store.go` | Create | `WorkflowRunStore` impl: CreateRun, GetRun, ListRuns, AddSteps, UpdateStepStatus |
| `internal/db/workflow_store.go` | Modify | `Save()` writes `entry_slug`; `GetSteps()` reads `entry_slug` |
| `internal/db/migrations/004_workflow_pipelines.sql` | Create | `CREATE TABLE runs` + `run_steps`; `ALTER TABLE workflow_steps ADD COLUMN entry_slug TEXT` |
| `internal/db/schema.sql` | Modify | Add runs/run_steps DDL; add `entry_slug` to workflow_steps |
| `internal/db/import_export_store.go` | Modify | Bump `AppVersion` to `v3`; export/import runs + run_steps |
| `internal/app/workflow_runs.go` | Create | `WorkflowRunService` with `RunPipeline(ctx, workflowSlug, input string, savePath *string) error` |
| `internal/cli/commands.go` | Modify | Add `RunFlags` + `ParseRunFlags`; register `run` in `ParseCommand` |
| `cmd/skillvault/main.go` | Modify | Bump `version` to `v3`; add `run` case + wire `WorkflowRunService` |
| `internal/mcp/server.go` | Modify | Bump server info `version` to `v3` |
| `internal/vars/resolver.go` | Reference | Used as-is; `Resolve()` with `providedVars` map keys `input`, `previous_output`, `final_output` |

## Interfaces / Contracts

```go
// domain/workflow.go additions

type RunStatus string
const (
    RunStatusPending   RunStatus = "pending"
    RunStatusRunning   RunStatus = "running"
    RunStatusCompleted RunStatus = "completed"
    RunStatusFailed    RunStatus = "failed"
)

type WorkflowStep struct {
    // ... existing fields unchanged ...
    EntrySlug string  // NEW: nullable; empty = renderable-only step
}

type WorkflowRun struct {
    ID         string
    WorkflowID string
    Input      string
    Output     string
    Status     RunStatus
    StartedAt  time.Time
    FinishedAt *time.Time
}

type WorkflowRunStep struct {
    ID        string
    RunID     string
    StepID    int64    // FK to workflow_steps.id (INTEGER)
    EntryID   string   // FK to entries.id, resolved at pre-flight
    Input     string
    Output    string
    Status    RunStatus
    StartedAt time.Time
    FinishedAt *time.Time
}

// db/store.go addition
type WorkflowRunStore interface {
    CreateRun(ctx context.Context, run domain.WorkflowRun, steps []domain.WorkflowRunStep) error
    GetRun(ctx context.Context, id string) (domain.WorkflowRun, []domain.WorkflowRunStep, error)
    ListRuns(ctx context.Context, workflowID string, limit int) ([]domain.WorkflowRun, error)
    UpdateStepStatus(ctx context.Context, stepID string, status domain.RunStatus, output string) error
}
```

## Migration 004 DDL

```sql
CREATE TABLE IF NOT EXISTS runs (
    id          TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id),
    input       TEXT DEFAULT '',
    output      TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK(status IN ('pending','running','completed','failed')),
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
);

CREATE TABLE IF NOT EXISTS run_steps (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    step_id     INTEGER NOT NULL REFERENCES workflow_steps(id),
    entry_id    TEXT NOT NULL REFERENCES entries(id),
    input       TEXT DEFAULT '',
    output      TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK(status IN ('pending','running','completed','failed')),
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
);

ALTER TABLE workflow_steps ADD COLUMN entry_slug TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_runs_workflow ON runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_run_steps_run ON run_steps(run_id);

INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (4, 'workflow_pipelines');
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `RunStatus` validation | Table-driven Go test in `domain/workflow_test.go` |
| Unit | `vars.Resolve()` with `input`/`previous_output`/`final_output` keys | Existing `vars/*_test.go` conventions; add pipeline-specific test |
| Integration | `WorkflowRunStore` CRUD | SQLite `:memory:`, follow `workflow_store_test.go` pattern |
| Integration | `WorkflowRunService.RunPipeline` full lifecycle | `app/app_test.go` pattern: init store, create workflow+entries, call RunPipeline, assert run/step records |
| Integration | Pre-flight validation rejects missing/archived entries | Separate test: archived entry → expect error before any INSERT |
| Integration | Truncation warning at 32K boundary | Step with >32K input → assert truncation + warning stderr |
| CLI | `run` command arg parsing | Table-driven `TestParseRunFlags` |
| E2E | Version bump `v3` | Update `main_test.go` expected string |

## Migration / Rollout

- Migration 004 is additive (new tables + one ALTER TABLE column). No data migration.
- Rollback: delete 004 SQL, recompile. Existing workflows render unchanged.
- SchemaVersion stays 2; v2 exports import cleanly into v3.
- `entry_slug` defaults to `''` — existing steps remain renderable-only.

## Open Questions

None — all decisions resolved against existing codebase patterns and spec requirements.
