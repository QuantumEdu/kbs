# Archive Report: skillvault-v3-workflow-pipelines

**Date**: 2026-06-26
**Archived to**: `openspec/changes/archive/2026-06-26-skillvault-v3-workflow-pipelines/`
**Mode**: openspec

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| skillvault | Updated | 6 requirements added, 2 modified, 0 removed |

### Merged Changes

**MODIFIED Requirements**:
- **REQ-WKF-05** (Capability 5): Changed from "not executable" to "primarily renderable instruction checklists, steps with `entry_slug` MAY be executed via `skillvault run`"
- **REQ-WFR-01** (Capability 15): Changed from "not executable automation" to "primarily renderable, steps with `entry_slug` MAY be executed via `skillvault run`"

**ADDED Requirements**:
- **REQ-WKF-06** (Capability 5): Step-entry linking via `entry_slug` on `workflow_step`
- **REQ-CLI-12** (Capability 11): `skillvault run` CLI command with file/stdin input and `--save` flag
- **REQ-CI-06** (Capability 18): Version bump to v3, schema version 2 compatibility
- **REQ-RUN-01** through **REQ-RUN-07** (Capability 20): Pipeline Execution Engine — runs/run_steps tables, sequential execution, system variables (`{{input}}`, `{{previous_output}}`, `{{final_output}}`), pre-flight validation, truncation

**Updated Scenarios**:
- Capability 5: +2 scenarios (mixed renderable/executable steps, full pipeline execution)
- Capability 11: +3 scenarios (run to stdout, stdin with --save, nonexistent workflow error)
- Capability 15: +1 scenario (executable step with entry_slug)
- Capability 18: +2 scenarios (version command output v3, v2 export import into v3)
- Capability 20: +6 scenarios (run lifecycle, failed run state, variable substitution, truncation, missing entry rejection, archived entry rejection)

## Source of Truth Updated

- `openspec/specs/skillvault/spec.md` — now at 135 requirements across 20 capabilities (was 123 requirements across 19 capabilities)

## Coverage Summary

| Capability | Requirements | Scenarios |
|-----------|-------------|-----------|
| Hybrid Storage Model | 7 | 3 |
| Entry Entity + 10 Types | 6 | 3 |
| Project Entity | 5 | 3 |
| Artifact Entity + File-Backed Storage | 8 | 3 |
| Workflow + WorkflowStep Entities | 6 | 5 |
| Series + SeriesEntry Entities | 6 | 3 |
| Tag Entity | 5 | 3 |
| EntryLink + Relation Types | 5 | 3 |
| Multi-Status Model | 7 | 3 |
| Hermes Context Layer (7 Modes) | 11 | 3 |
| CLI Commands (15) | 12 | 6 |
| MCP Tools (12) | 13 | 5 |
| Secret Detection | 7 | 3 |
| Search (FTS5 with Filters) | 5 | 3 |
| Workflow Rendering | 4 | 4 |
| Import/Export | 7 | 3 |
| Session Wrap | 5 | 3 |
| Code Integrity | 6 | 5 |
| Tag Query Support | 3 | 2 |
| Pipeline Execution Engine | 7 | 6 |
| **Total** | **135** | **70** |

## Archive Contents

- proposal.md ✅
- design.md ✅
- specs/skillvault/spec.md ✅ (delta spec)
- tasks.md ✅
- verify-report.md ✅

## Verification

- [x] Main spec updated correctly — all deltas applied
- [x] Change folder moved to archive
- [x] Archive contains all artifacts
- [x] Active changes directory no longer has this change
- [x] No CRITICAL issues in verification report

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.
Ready for the next change.
