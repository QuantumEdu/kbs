# Delta for MCP Tools

## MODIFIED Requirements

### Requirement: REQ-MCP-01 — tool count

The system SHALL expose 19 MCP tools: `save_entry`, `search_entries`, `get_entry`, `save_artifact`, `get_context`, `compose_series`, `render_workflow`, `session_wrap`, `archive_entry`, `list_projects`, `search_by_tags`, `get_context_bundle`, `save_entry_ref`, `list_entry_refs`, `get_entry_graph`, `compare_entries`, `save_result`, `run_workflow`, `route_scenario`.

(Previously: 17 tools — `run_workflow` and `route_scenario` were not present.)

### Requirement: REQ-MCP-02 — save_entry params

`save_entry` SHALL accept: `title`, `type`, `summary`, `body`(opt), `project`(opt), `tags`, `status`, `purpose`(opt); SHALL reject secrets.

(Previously: `save_entry` did not accept `purpose` parameter.)

#### Scenario: save_entry with purpose
- GIVEN a valid entry payload
- WHEN `save_entry` is called with `purpose: "LEARNING"`
- THEN the entry is persisted with purpose LEARNING.

#### Scenario: save_entry without purpose (backward compat)
- GIVEN a valid entry payload with no `purpose` field
- WHEN `save_entry` is called
- THEN the entry is persisted with empty purpose — no error.

### Requirement: REQ-MCP-03 — search_entries filters

`search_entries` SHALL accept: `query`, `type`(opt), `project`(opt), `tags`, `purpose`(opt), `include_archived`(default false), `limit`(default 10), `vector`(opt bool, default false).

(Previously: `search_entries` did not accept `purpose` parameter.)

#### Scenario: search_entries filtered by purpose
- GIVEN entries with purposes WORK and KNOWLEDGE
- WHEN `search_entries` is called with `purpose: "WORK"`
- THEN only WORK entries are returned.

## ADDED Requirements

### Requirement: REQ-MCP-18 — run_workflow MCP tool

The system SHALL expose a `run_workflow` MCP tool that delegates to `WorkflowRunService.RunPipelineStructured`. Input: `workflow` (slug or ID, required), `steps` (map of step index → input text, required). Output: structured run result with `run_id`, `workflow_id`, `workflow_slug`, `status`, `steps` array (each with `step_index`, `status`, `output`, `error`), `started_at`, `finished_at`. All values SHALL be JSON-RPC-compatible.

#### Scenario: run_workflow executes successfully
- GIVEN a workflow "research" with 2 valid steps
- WHEN `run_workflow(workflow: "research", steps: {1: "topic: Go", 2: ""})` is called
- THEN the response includes `status: "completed"` and both steps with outputs.

#### Scenario: run_workflow step fails
- GIVEN step 2 references a missing entry
- WHEN `run_workflow` is called
- THEN pre-flight validation rejects the call before any execution.

### Requirement: REQ-MCP-19 — route_scenario MCP tool

The system SHALL expose a `route_scenario` MCP tool that wraps `EntryService.RouteScenario`. Input: `scenario` (string, required). Output: matched workflow (ID, slug, name, steps) and skill/entry metadata as a JSON object. Empty scenario SHALL be rejected with validation error. No-match SHALL return a meaningful error.

#### Scenario: route_scenario matches a workflow
- GIVEN a routing entry associates scenario "write spec" with workflow "spec-plan-task"
- WHEN `route_scenario(scenario: "write spec")` is called
- THEN the response includes workflow slug "spec-plan-task" and related metadata.

#### Scenario: route_scenario no match
- GIVEN no routing entry matches "unknown task"
- WHEN `route_scenario(scenario: "unknown task")` is called
- THEN the response is an error indicating no workflow matched.
