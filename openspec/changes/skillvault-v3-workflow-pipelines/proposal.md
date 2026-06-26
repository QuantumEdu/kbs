# Proposal: SkillVault v3 Workflow Pipelines

## Intent

Workflows today are **renderable instruction checklists** (REQ-WKF-05: "not executable"). Users need sequential execution — composing entries into pipelines where each step's output feeds the next. This bridges SkillVault from pure retrieval to minimal execution, keeping the "Agnosticismo de Modelo" principle (SkillVault composes context; the external LLM/agent executes).

## Scope

### In Scope
- New DB tables: `runs`, `run_steps` (migration 004)
- New domain entities: `WorkflowRun`, `WorkflowRunStep`
- `entry_slug` column on `workflow_steps` to link steps to executable entries
- `WorkflowRunService` in `internal/app/` with `Run()` method
- CLI command: `skillvault run <workflow> <file> [--save output.md]`
- System variables: `{{input}}`, `{{previous_output}}`, `{{final_output}}` using existing `vars` package
- Version bump: `v2-quantum` → `v3` in `main.go`, `import_export_store.go`, `mcp/server.go`
- Docs update: README, `docs/commands.md`, `docs/architecture.md`, `docs/quickstart.md`

### Out of Scope
- Named variables (`input:`/`output:` fields per step) — **deferred to v4**
- MCP `workflow_run` tool — **deferred to v4**
- Export/import of workflow runs — **deferred to v4**
- External adapters (GraphRAG, Mem0) — **deferred to v5**

## Capabilities

### New Capabilities
None — extends existing `skillvault` domain.

### Modified Capabilities
- **skillvault**: REQ-WKF-05 changes from "Workflows are not executable — they are renderable instruction checklists" to "Workflows are executable sequential pipelines composing entries via system variables." REQ-WFR-01/02 gain execution semantics. New REQ-WKF-06 through REQ-WKF-10 for pipeline execution, run persistence, step ordering, variable substitution, and pre-flight validation.

## Approach

**Simple mode only** (Approach 1 from exploration). Sequential execution with three system variables substituted via `internal/vars/resolver.go`. Steps reference entries by slug (`entry_slug` on `workflow_steps`). The `run` CLI command:

1. Loads workflow steps → resolves entry_slug → validates all entries exist (pre-flight)
2. Creates a `run` record + `run_step` records (status: pending)
3. For each step: substitutes `{{input}}` or `{{previous_output}}` in entry body, outputs composed prompt to stdout, reads step result from agent/user, persists to `run_steps`
4. On completion: marks run status, optionally writes `{{final_output}}` to file

SkillVault does NOT call LLMs — it composes prompts. The external agent/executor provides step output back.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/domain/workflow.go` | Modified | Add `WorkflowRun`, `WorkflowRunStep` types; add `EntrySlug` to `WorkflowStep` |
| `internal/domain/filters.go` | Modified | Add `WorkflowRuns`, `WorkflowRunSteps` to `VaultData` |
| `internal/db/store.go` | Modified | Add `WorkflowRunStore` interface and `sqliteWorkflowRunStore` |
| `internal/db/workflow_store.go` | New file | `WorkflowRunStore` implementation (CRUD for runs/run_steps) |
| `internal/db/migrations/004_workflow_pipelines.sql` | New | `runs` + `run_steps` tables; `entry_slug` column on `workflow_steps` |
| `internal/db/workflow_store.go` (existing) | Modified | `GetSteps` selects new `entry_slug` column; `Save` persists it |
| `internal/db/import_export_store.go` | Modified | Export/import runs + run_steps; bump `AppVersion` to `v3` |
| `internal/app/workflows.go` | Modified | Add `WorkflowRunService` with `Run()` method |
| `internal/cli/commands.go` | Modified | Add `RunFlags`, `ParseRunFlags`; register `run` in `ParseCommand` |
| `cmd/skillvault/main.go` | Modified | Add `run` case; bump `version` to `v3`; wire `WorkflowRunService` |
| `internal/mcp/server.go` | Modified | Bump server info `version` from `v1-alpha` to `v3` |
| `README.md` | Modified | Update version, add pipeline feature, bump CLI/MCP counts |
| `docs/commands.md` | Modified | Add `run` command docs |
| `docs/architecture.md` | Modified | Add pipeline data flow, new entities |
| `docs/quickstart.md` | Modified | Add run example |
| Test files (5 paths) | Modified | Update hardcoded `"v2-quantum"` to `"v3"` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Entry resolution failure mid-pipeline (archived/missing entry) | Low | Pre-flight validation: resolve all entry_slugs before creating run |
| `{{previous_output}}` too large for substitution | Med | Truncation strategy: cap at 32K chars; warn on truncation |
| Version bump breaks import of v2 exports | Low | `runs`/`run_steps` are additive; old exports auto-upgrade on import; SchemaVersion stays 2 |
| YAML workflow definition format (spec shows YAML, current code uses JSON) | Med | Accept both YAML and JSON for `add-workflow`; `entry_slug` field is additive |

## Rollback Plan

1. Migration 004 is additive — no destructive column changes to existing data
2. Revert: delete migration 004 SQL, recompile, existing workflows still render as checklists
3. `runs` table can be truncated without affecting workflows/workflow_steps
4. Version bump is cosmetic — no DB schema incompatibility introduced

## Dependencies

None — all infrastructure (store pattern, migration system, vars resolver, CLI flag parsing) exists in codebase.

## Success Criteria

- [ ] `skillvault run <workflow> <file>` executes steps sequentially, substituting `{{input}}`/`{{previous_output}}`/`{{final_output}}`
- [ ] Pre-flight validation rejects runs referencing non-existent entries BEFORE any step executes
- [ ] `--save` flag writes final output to specified file
- [ ] Run and run_steps records persisted with correct status transitions (pending → running → completed/failed)
- [ ] `skillvault version` outputs `v3` (was `v2-quantum`)
- [ ] All existing tests pass; new tests cover run lifecycle, variable substitution, pre-flight validation
- [ ] v2 exports import successfully into v3 without data loss
