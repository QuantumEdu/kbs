# MCP Route Tool Specification

## Purpose

Expose the existing `RouteScenario` capability (from PR #16) as an MCP tool. The `route_scenario` MCP tool accepts a scenario string and returns the matched workflow, skill, or entry information — enabling agent-driven scenario-to-workflow resolution.

## Dependencies

PR #16 (`feature/skillvault-route-command`) — `RouteScenario` already implemented in `EntryService`. This spec covers MCP exposure only; CLI route implementation is the baseline.

## Requirements

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MRT-01 | The system SHALL expose a `route_scenario` MCP tool that wraps `EntryService.RouteScenario`. | MUST |
| REQ-MRT-02 | Input: `scenario` (string, required) — the scenario description to resolve. | MUST |
| REQ-MRT-03 | Output SHALL include the matched workflow (ID, slug, name, steps) and any matched skill/entry (ID, slug, title, type), encoded as a JSON object. | MUST |
| REQ-MRT-04 | The tool SHALL return a meaningful error message when no workflow or skill matches the scenario string. | MUST |
| REQ-MRT-05 | The tool SHALL return a validation error when `scenario` is empty or missing. | MUST |
| REQ-MRT-06 | Args and results SHALL be JSON-RPC-compatible — all values serializable as JSON without custom types. | MUST |

## Scenarios

### Scenario: Match found
- GIVEN a workflow "spec-plan-task" is associated with a routing entry tagged for "spec writing"
- WHEN `route_scenario` is called with `scenario: "write a specification"`
- THEN the response includes the matched workflow ID, slug "spec-plan-task", steps, and related skill/entry metadata.

### Scenario: No match
- GIVEN no routing entries match the scenario
- WHEN `route_scenario` is called with `scenario: "do something unknown"`
- THEN the response is an error indicating no workflow or skill matched the scenario.

### Scenario: Empty scenario rejected
- GIVEN an empty string scenario
- WHEN `route_scenario` is called with `scenario: ""`
- THEN the call is rejected with a validation error: "scenario is required".

### Scenario: JSON-RPC result format
- GIVEN a valid scenario that matches a workflow
- WHEN `route_scenario` returns
- THEN the result is a JSON object with string/array/number values only — no Go-specific types, function references, or channels.
