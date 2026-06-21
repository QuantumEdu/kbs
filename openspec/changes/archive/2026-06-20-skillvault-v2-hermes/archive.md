# SkillVault v2 Hermes — Archive Report

**Change**: skillvault-v2-hermes
**Version**: v2 (supersedes v1-alpha, supersedes spec-analysis)
**Archived**: 2026-06-20
**Mode**: hybrid (openspec + Engram)
**Final Status**: ✅ ARCHIVED

---

## Change Summary

Complete rewrite of SkillVault from v1-alpha (pure SQLite vault) to v2 Hermes — a hybrid DB+filesystem vault with a context compilation engine, artifact management layer, secret detection, expanded entity model, 10 MCP tools, and 14 CLI commands. This change delivers the full architectural vision from the v1-alpha authoritative spec (`skillvault_v1_alpha_spec.md`), implementing all new capabilities as a superseding greenfield v2.

---

## What Was Delivered

### All 17 Capabilities (115 requirements, 51 scenarios)

| # | Capability | REQs | Compliant | Partial | Status |
|---|------------|------|-----------|---------|--------|
| 1 | Hybrid Storage Model | 7 | 7 | 0 | ✅ |
| 2 | Entry Entity + 10 Types | 6 | 6 | 0 | ✅ |
| 3 | Project Entity | 5 | 5 | 0 | ✅ |
| 4 | Artifact Entity + File-Backed Storage | 8 | 8 | 0 | ✅ |
| 5 | Workflow + WorkflowStep Entities | 5 | 5 | 0 | ✅ |
| 6 | Series + SeriesEntry Entities | 6 | 6 | 0 | ✅ |
| 7 | Tag Entity | 5 | 5 | 0 | ✅ |
| 8 | EntryLink + Relation Types | 5 | 5 | 0 | ✅ |
| 9 | Multi-Status Model | 7 | 7 | 0 | ✅ |
| 10 | Hermes Context Layer (7 Modes) | 11 | 11 | 0 | ✅ |
| 11 | CLI Commands (14) | 11 | 11 | 0 | ✅ |
| 12 | MCP Tools (10) | 11 | 11 | 0 | ✅ |
| 13 | Secret Detection | 7 | 6 | 1 | ✅ |
| 14 | Search (FTS5 with Filters) | 5 | 5 | 0 | ✅ |
| 15 | Workflow Rendering | 4 | 3 | 1 | ✅ |
| 16 | Import/Export | 7 | 7 | 0 | ✅ |
| 17 | Session Wrap | 5 | 5 | 0 | ✅ |
| **Total** | | **115** | **113** | **2** | **✅ PASS** |

### All 10 Acceptance Criteria

| AC | Description | Result |
|----|-------------|--------|
| AC1 | Initialize vault | ✅ PASS |
| AC2 | Save and search entry | ✅ PASS |
| AC3 | Save long artifact | ✅ PASS |
| AC4 | Context generation (planning mode) | ✅ PASS |
| AC5 | Archived content behavior | ✅ PASS |
| AC6 | Secret protection | ✅ PASS |
| AC7 | Workflow rendering | ✅ PASS |
| AC8 | Session wrap | ✅ PASS |
| AC9 | Import/export round-trip | ✅ PASS |
| AC10 | MCP agent use (parity with CLI) | ✅ PASS |

### All 24 Tasks Complete

| Phase | Tasks | Status |
|-------|-------|--------|
| 1. Foundation (domain + schema) | T-01 through T-04 | ✅ |
| 2. Store implementations | T-05 through T-10 | ✅ |
| 3. Secret scanner | T-11 | ✅ |
| 4. Artifact filesystem | T-12 | ✅ |
| 5. Hermes context compiler | T-13 | ✅ |
| 6. Import/Export | T-14 through T-15 | ✅ |
| 7. App/use case layer | T-16 through T-20 | ✅ |
| 8. CLI commands | T-21 | ✅ |
| 9. MCP tools | T-22 | ✅ |
| 10. Main wiring + integration | T-23 | ✅ |
| 11. Acceptance tests | T-24 | ✅ |

### Package Architecture

11 Go packages delivered in clean one-way dependency order:
`cmd/skillvault` → `internal/cli`, `internal/mcp` → `internal/app` → `internal/domain`, `internal/vars` → `internal/db`, `internal/files`, `internal/context`, `internal/security`

Key new packages: `internal/files` (artifact filesystem), `internal/context` (Hermes compiler), `internal/security` (secret scanner).

---

## Test Results Summary

| Metric | Value |
|--------|-------|
| Build | ✅ Passed |
| Test packages | 11 of 11 passing |
| Test functions | 397 passing (406 including sub-tests) |
| Failures | 0 |
| Skipped | 0 |
| Coverage | Not measured |

All 10 acceptance criteria have dedicated tests. Spec compliance matrix shows 113/115 requirements fully compliant.

---

## Open Items / Known Gaps

Two SHOULD-level partials — neither blocks archive:

| ID | Requirement | Level | Description |
|----|-------------|-------|-------------|
| REQ-SEC-07 | Hard delete requires explicit confirmation | SHOULD | Not implemented. Archive (soft delete via status change) is the primary deletion strategy per REQ-SEC-06. Users can set status to `archived` instead of deleting rows. |
| REQ-WFR-04 | Example workflow `spec-plan-task` has 6 steps | SHOULD | Not explicitly embedded as a built-in example. Workflows are user-defined entities; the schema and renderer support arbitrary step counts. Users create their own workflows via `add-workflow` CLI or `save_entry` MCP. |

### Suggestions from Verify Report (non-blocking)
- Add test coverage measurement to CI/CD (`go test -coverprofile`)
- Document `SKILLVAULT_DB=:memory:` test mode convention in README
- No CRITICAL or WARNING issues at archive time

### Design Deviations (architectural refinements, not violations)
- `internal/search/fts.go` merged into `internal/db/fts.go` — search is a DB concern
- `internal/export/` split between `internal/db/` (data extraction) and `internal/app/` (JSON serialization)
- `internal/context/modes.go` inlined into `compiler.go` — single-file compiler, all 7 modes present

---

## Artifact Inventory

| Artifact | Path | Status |
|----------|------|--------|
| Proposal | `openspec/changes/archive/2026-06-20-skillvault-v2-hermes/proposal.md` | ✅ |
| Delta Specs | `openspec/changes/archive/2026-06-20-skillvault-v2-hermes/specs/spec.md` | ✅ |
| Technical Design | `openspec/changes/archive/2026-06-20-skillvault-v2-hermes/design.md` | ✅ |
| Implementation Tasks | `openspec/changes/archive/2026-06-20-skillvault-v2-hermes/tasks.md` | ✅ (24/24 `[x]`) |
| Verification Report | `openspec/changes/archive/2026-06-20-skillvault-v2-hermes/verify.md` | ✅ |
| Archive Report | `openspec/changes/archive/2026-06-20-skillvault-v2-hermes/archive.md` | ✅ |

### Main Specs Updated

| Domain | Action | Path |
|--------|--------|------|
| skillvault | Created (initial full spec) | `openspec/specs/skillvault/spec.md` |

No existing main spec was overwritten — this is the first main spec for the skillvault domain.

### Engram Persistence

| Artifact | Topic Key |
|----------|-----------|
| Archive Report | `sdd/skillvault-v2-hermes/archive-report` |

---

## SDD Cycle Complete

The SkillVault v2 Hermes change has been:

1. **Proposed** — Scope, approach, risks, and build order defined
2. **Specified** — 17 capabilities, 115 requirements, 51 scenarios, 10 acceptance criteria
3. **Designed** — 11-package Clean Architecture, SQL schema v2 (11 tables + FTS5), Hermes compiler (7 modes), data flows, interface contracts
4. **Task-broken** — 24 tasks across 11 phases, inside-out TDD build order
5. **Implemented** — All 24 tasks complete, 11 packages, zero external dependencies (modernc.org/sqlite only)
6. **Verified** — 406 tests passing, 113/115 requirements compliant, 10/10 ACs passing
7. **Archived** — Delta specs synced to main specs, change folder moved to archive, audit trail complete

Ready for the next change.
