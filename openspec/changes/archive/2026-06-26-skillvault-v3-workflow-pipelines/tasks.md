# Tasks: SkillVault v3 Workflow Pipelines

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~450–550 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | DB migrations + domain + store CRUD | PR 1 | Independent; compiles with no callers |
| 2 | Run service + CLI wiring + version bump | PR 2 | Depends on PR 1 |
| 3 | Full tests + import/export runs | PR 3 | Depends on PR 2 |

## Phase 1: Foundation — DB + Domain + Store (PR 1)

- [x] 1.1 Create `internal/db/migrations/004_workflow_pipelines.sql`: `runs` table (TEXT PK, `run-` prefix), `run_steps` (TEXT PK, `rst-` prefix), `ALTER TABLE workflow_steps ADD COLUMN entry_slug TEXT DEFAULT ''`, indexes, migration insert
- [x] 1.2 Add `RunStatus` type, `WorkflowRun`, `WorkflowRunStep` structs to `internal/domain/workflow.go`; add `EntrySlug string` to `WorkflowStep`
- [x] 1.3 Add `WorkflowRuns`, `WorkflowRunSteps` to `VaultData` in `internal/domain/filters.go`
- [x] 1.4 Add `WorkflowRunStore` interface (CreateRun, GetRun, ListRuns, UpdateStepStatus) and `sqliteWorkflowRunStore` to `internal/db/store.go`; wire into `NewStore()`
- [x] 1.5 Create `internal/db/workflow_run_store.go` — implement all 4 methods; CreateRun uses TX for atomic run+steps insert
- [x] 1.6 Modify `internal/db/workflow_store.go`: `Save()` inserts `entry_slug`; `GetSteps()` SELECT includes `COALESCE(entry_slug,'')`
- [x] 1.7 Sync `internal/db/schema.sql` — add runs/run_steps DDL and `entry_slug` to workflow_steps

## Phase 2: App Service + CLI + Wiring (PR 2)

- [x] 2.1 Create `internal/app/workflow_runs.go` — `WorkflowRunService.RunPipeline()`: pre-flight resolve entry_slugs → validate active; create run+steps pending; sequential loop substituting `{{input}}`/`{{previous_output}}`/`{{final_output}}` via `vars.Resolve()`, stdout prompt, stdin result, update status; truncate previous_output at 32K with warning
- [x] 2.2 Add `RunFlags` + `ParseRunFlags()` to `internal/cli/commands.go`; register `"run"` in `ParseCommand()` (positional `<workflow> <file>`, `--save <path>`)
- [x] 2.3 Wire `run` case in `cmd/skillvault/main.go`: add `workflowRunSvc` to vaultServices, parse flags, read input (file or stdin), call RunPipeline, write `--save` output
- [x] 2.4 Bump version `"v2-quantum"` → `"v3"` in `cmd/skillvault/main.go`, `internal/db/import_export_store.go` (AppVersion), and `internal/mcp/server.go` (server version `"v1-alpha"` → `"v3"`)

## Phase 3: Tests + Import/Export (PR 3)

- [x] 3.1 Extend `internal/db/import_export_store.go`: `ExportAll()` exports runs+run_steps; `ImportAll()` imports them with idempotent upsert
- [x] 3.2 Create `internal/db/workflow_run_store_test.go` — table-driven: CreateRun (success, empty, duplicate), GetRun, ListRuns, UpdateStepStatus transitions
- [x] 3.3 Create `internal/app/workflow_runs_test.go` — full lifecycle, pre-flight rejection (missing slug, archived entry), truncation at 32K, skip-steps-without-entry_slug
- [x] 3.4 Update `"v2-quantum"` → `"v3"` in test files: `main_test.go:150`, `import_export_store_test.go:65–66`, `filters_test.go:143,152–153`
- [x] 3.5 Add `run` CLI parsing tests to `internal/cli/cli_test.go` — valid, missing args, `--save`
