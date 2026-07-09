# Delta for CLI Commands

## MODIFIED Requirements

### Requirement: Required Commands (REQ-CLI-02)

The system MUST support `stats` in addition to the existing command list.
(Previously: stats was absent)

## ADDED Requirements

### Requirement: Stats Workflow Runs (REQ-CLI-13)

`skillvault stats --workflow-runs` SHALL display per-workflow run metrics: total runs, success rate, avg duration, step completion ratio.

#### Scenario: Per-workflow output
- GIVEN workflow "research" has 5 runs
- WHEN `skillvault stats --workflow-runs` runs
- THEN per-workflow total_runs, success_rate, avg_duration, step_ratio displayed

### Requirement: Stats JSON Output (REQ-CLI-14)

`skillvault stats --json` SHALL output all stats including workflow run analytics as structured JSON.

#### Scenario: JSON includes workflow_runs
- GIVEN vault with entries and workflow runs
- WHEN `skillvault stats --json` runs
- THEN valid JSON returned with `workflow_runs` block containing totals and per-workflow metrics
