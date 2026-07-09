# Workflow Analytics Specification

## Purpose
Aggregate run metrics and progress tracking from existing `runs`/`run_steps`. No schema changes.

## Requirements

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-WFA-01 | Compute total_runs, success_rate (completed/total), avg/max/min duration, and failed_step_count from runs/run_steps | SHALL |
| REQ-WFA-02 | Compute completed/total step ratio per run for progress tracking | SHALL |
| REQ-WFA-03 | All metrics SHALL support per-workflow filtering | SHALL |
| REQ-WFA-04 | Empty data SHALL return zero values, not errors | SHALL |

#### Scenario: Mixed-status analytics
- GIVEN 10 runs: 7 completed, 2 failed, 1 running
- WHEN analytics queried
- THEN total_runs=10, success_rate=0.7, failed_steps=count of run_steps with status 'failed'

#### Scenario: Per-workflow and empty
- GIVEN workflow A has 5 runs, workflow B has 2 runs
- WHEN analytics queried for workflow A
- THEN metrics scoped to A only

#### Scenario: No data
- GIVEN no runs exist
- WHEN analytics queried
- THEN all metrics=0, no error

#### Scenario: Step progress
- GIVEN run R: 5 steps (3 completed, 2 pending)
- WHEN progress queried for R
- THEN completed=3, total=5, ratio=0.6
