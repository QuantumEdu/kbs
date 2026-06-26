# Delta for SkillVault

## ADDED Requirements

### Requirement: Workflow Run Persistence

The system MUST persist pipeline execution records. A `runs` table SHALL track each execution with `id`, `workflow_id`, `input`, `output`, `status` (pending|running|completed|failed), `started_at`, and `finished_at`. A `run_steps` table SHALL track each step with `id`, `run_id`, `step_id`, `entry_id`, `input`, `output`, `status`, `started_at`, and `finished_at`.

#### Scenario: Run lifecycle

- GIVEN a workflow with 3 steps
- WHEN `skillvault run` executes
- THEN a `run` record is created with status `pending`
- AND 3 `run_step` records are created with status `pending`
- AND statuses transition to `running` then `completed` (or `failed` on error)

#### Scenario: Failed run preserves intermediate state

- GIVEN step 2 of 3 fails
- WHEN execution halts
- THEN run status is `failed`
- AND step 1 status is `completed`, step 2 status is `failed`
- AND step 3 status remains `pending`

### Requirement: Sequential Pipeline Execution

The `WorkflowRunService` MUST execute steps in `order_index` ascending order. Each step's output SHALL become `{{previous_output}}` for the next. Execution MUST halt on step failure — subsequent steps SHALL NOT execute. The service SHALL NOT call LLMs; it composes prompts and records step results.

#### Scenario: Linear pipeline with output propagation

- GIVEN workflow steps 1→2→3 all succeed
- WHEN executed
- THEN step output propagates as `{{previous_output}}` to each subsequent step
- AND final run status is `completed`

#### Scenario: Pre-flight validation rejects missing entries

- GIVEN workflow step references entry slug `extract_wisdom` that does not exist
- WHEN `run` is invoked
- THEN execution is rejected BEFORE any step runs
- AND no `run` or `run_step` records are created

#### Scenario: Pre-flight validates entry status

- GIVEN workflow step references an archived entry
- WHEN `run` is invoked
- THEN execution is rejected with a validation error indicating the entry is not active

### Requirement: System Variables

The system MUST support three variables during pipeline execution. `{{input}}` SHALL resolve to initial file or stdin content. `{{previous_output}}` SHALL resolve to the last completed step's output, truncated at 32K characters with a warning on truncation. `{{final_output}}` SHALL resolve to all completed step outputs concatenated.

#### Scenario: Variable substitution across steps

- GIVEN initial input "Hello World"
- WHEN step 1 entry body contains "Process: {{input}}"
- THEN step executes with "Process: Hello World"
- AND step output is stored for next step's `{{previous_output}}`

#### Scenario: Truncation threshold

- GIVEN step output exceeds 32K characters
- WHEN next step substitutes `{{previous_output}}`
- THEN variable is truncated at 32K
- AND a truncation warning is emitted

### Requirement: Step-Entry Linking

Each `workflow_step` MAY include an `entry_slug` referencing a specific entry. When `entry_slug` is NOT set, the step SHALL remain a renderable checklist item per REQ-WKF-01 through REQ-WKF-04. Existing workflow rendering MUST continue to work unchanged for all steps regardless of `entry_slug` value.

#### Scenario: Mixed executable and renderable steps

- GIVEN workflow with step 1 (`entry_slug` set) and step 2 (`entry_slug` null)
- WHEN `render-workflow` renders the workflow
- THEN both steps appear in order with full instructions
- WHEN `skillvault run` executes the workflow
- THEN only step 1 executes; step 2 is skipped

### Requirement: CLI Run Command

The CLI MUST support `skillvault run <workflow> <file> [--save output.md]`. Input SHALL be read from `<file>` or stdin if file is `-`. Output SHALL be written to stdout by default, or to the path specified by `--save`.

#### Scenario: File input to stdout

- GIVEN file `article.md` exists
- WHEN `skillvault run research_article article.md` executes
- THEN final composed output is printed to stdout

#### Scenario: Stdin input with --save

- GIVEN input piped via stdin
- WHEN `echo "test" | skillvault run my_workflow - --save out.md` runs successfully
- THEN `out.md` contains `{{final_output}}` content

#### Scenario: Nonexistent workflow

- GIVEN workflow `missing_wf` does not exist
- WHEN `skillvault run missing_wf file.md` is invoked
- THEN command exits with error indicating workflow not found

### Requirement: Version Bump

The application version MUST be updated from `v2-quantum` to `v3` in `cmd/skillvault/main.go`, `internal/db/import_export_store.go`, and `internal/mcp/server.go`. The import/export schema version SHALL remain `2` (additive change only).

#### Scenario: Version command

- GIVEN no prior runs exist
- WHEN `skillvault version` is executed
- THEN output is `v3`

#### Scenario: v2 export imports into v3

- GIVEN a v2 export file with schema version 2
- WHEN imported into v3
- THEN import succeeds without data loss
- AND new `runs`/`run_steps` fields are absent (null/empty)

## MODIFIED Requirements

### Requirement: REQ-WKF-05

Workflows are primarily renderable instruction checklists. When steps have `entry_slug` set, they MAY be executed as sequential pipeline steps via `skillvault run`.
(Previously: Workflows are not executable — they are renderable instruction checklists)

#### Scenario: Renderable checklist (unchanged)

- GIVEN a workflow with 3 steps
- WHEN `render_workflow` is called
- THEN steps are returned in order with title, instruction, and required flag

#### Scenario: Executable pipeline (new)

- GIVEN a workflow with 3 steps all having `entry_slug` set to valid entries
- WHEN `skillvault run` is invoked
- THEN steps execute sequentially with system variable substitution

### Requirement: REQ-WFR-01

Workflows are primarily renderable instruction checklists. When a step has `entry_slug` set referencing a valid active entry, that step MAY be executed as part of a sequential pipeline via `skillvault run`.
(Previously: Workflows are renderable instruction checklists — not executable automation)

#### Scenario: Renderable workflow (unchanged)

- GIVEN workflow "spec-plan-task" has 6 steps
- WHEN `render_workflow` is called with workflow slug
- THEN steps 1–6 are returned in sequential order with full instructions and required flags

#### Scenario: Executable step in workflow (new)

- GIVEN workflow step has `entry_slug: summarize` referencing an active entry
- WHEN `skillvault run` executes that step
- THEN the referenced entry body is composed with system variables and executed in order
