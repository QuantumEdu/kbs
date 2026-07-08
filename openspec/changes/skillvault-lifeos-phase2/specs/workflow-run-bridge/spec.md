# Workflow Run Bridge Specification

## Purpose

Provides structured pipeline execution (`RunPipelineStructured`) that accepts step inputs as structured arguments (not stdin/file) and returns a JSON-shaped run result. Exposed as the `run_workflow` MCP tool. Additive — existing CLI `run` (`RunPipeline`) is unchanged.

## Requirements

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-RBR-01 | `RunPipelineStructured(workflowRef, steps)` SHALL accept a workflow reference (slug or ID) and a map of step index → input text. It SHALL execute steps sequentially in `order_index` order, substituting system variables identically to `RunPipeline`. | MUST |
| REQ-RBR-02 | The return value SHALL be a structured result containing: `run_id`, `workflow_id`, `workflow_slug`, `status` (completed|failed), `steps` (array of {step_index, status, output, error}), `started_at`, `finished_at`. | MUST |
| REQ-RBR-03 | If a step fails, execution SHALL halt. The run status SHALL be `failed`, the failing step's entry in `steps` SHALL include `status: "failed"` and an `error` field, and unexecuted steps SHALL have `status: "pending"`. | MUST |
| REQ-RBR-04 | `RunPipelineStructured` SHALL create `runs` and `run_steps` records in the database identically to `RunPipeline`. | MUST |
| REQ-RBR-05 | Pre-flight validation SHALL reject execution if any referenced entry slug does not exist or is archived, BEFORE any step executes. | MUST |
| REQ-RBR-06 | The existing `RunPipeline` method and CLI `run` command SHALL remain unchanged — stdin/file input path is preserved. | MUST |
| REQ-RBR-07 | `{{previous_output}}` truncation at 32K SHALL apply to structured runs identically to existing pipeline behavior. | MUST |
| REQ-RBR-08 | The system SHALL NOT execute steps in parallel; execution is strictly sequential. | MUST |

## Scenarios

### Scenario: Successful structured run
- GIVEN a workflow "research_article" with 2 steps both referencing valid active entries
- WHEN `RunPipelineStructured("research_article", {1: "Analyze: REST vs GraphQL", 2: ""})` is called
- THEN a run record is created with status `completed`, AND `steps` array shows both steps with status `completed` and their respective outputs.

### Scenario: Step failure halts execution
- GIVEN step 1 succeeds, step 2 fails
- WHEN structured run executes
- THEN run status is `failed`, step 1 shows `completed` with output, step 2 shows `failed` with error, and any step 3 shows `pending`.

### Scenario: Pre-flight rejects missing entry
- GIVEN a workflow step references entry slug "nonexistent_entry" that does not exist
- WHEN `RunPipelineStructured` is invoked
- THEN execution is rejected before any step runs, AND no run/run_step records are created.

### Scenario: CLI run unchanged
- GIVEN a workflow and input file
- WHEN `skillvault run my_workflow input.md` is invoked
- THEN the existing `RunPipeline` path executes via stdin/file — behavior is identical to before structured run was added.

### Scenario: MCP run_workflow tool
- GIVEN a valid workflow slug and step inputs
- WHEN the `run_workflow` MCP tool is called
- THEN it delegates to `RunPipelineStructured` and returns the structured result as JSON-RPC response.
