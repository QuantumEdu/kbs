# Delta for MCP Tools

## MODIFIED Requirements

### Requirement: Required MCP Tools (REQ-MCP-01)

The system MUST expose 22 MCP tools — adds `get_stats`, `list_workflow_runs`, `get_run` to the existing 19.
(Previously: 19 tools)

## ADDED Requirements

### Requirement: Get Stats (REQ-MCP-14)

`get_stats` SHALL return vault statistics with `workflow_runs` block: total_runs, success_rate, avg/max/min duration, failed_step_count, per-workflow metrics.

#### Scenario: Complete stats response
- GIVEN vault with entries, artifacts, projects, and workflow runs
- WHEN `get_stats` called
- THEN response includes entry/artifact/project counts AND workflow_runs analytics block

### Requirement: List Workflow Runs (REQ-MCP-15)

`list_workflow_runs` SHALL accept optional `workflow_id` and `limit` (default 20), returning run id, workflow_id, status, timestamps, and step completion ratio per run.

#### Scenario: Filtered listing
- GIVEN 10 runs across 2 workflows
- WHEN `list_workflow_runs(workflow_id: "wf-1", limit: 5)` called
- THEN up to 5 runs for wf-1 returned with status and step_ratio

### Requirement: Get Run Detail (REQ-MCP-16)

`get_run` SHALL accept `run_id` and return run metadata plus all run_steps with step_index, status, entry_id, output, and error (if failed).

#### Scenario: Run with steps
- GIVEN run R with 3 steps
- WHEN `get_run(run_id: "R")` called
- THEN run metadata and steps array returned, each with status and output/error

#### Scenario: Run not found
- GIVEN run_id "nonexistent"
- WHEN `get_run` called
- THEN error: run not found
