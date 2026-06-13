# Proposal: SkillVault Spec Analysis

## Intent

Re-analyze `skillvault-spec-v1.md` (1974 lines) and generate complete SDD artifacts — spec, design, tasks, and verify plan — as persistent OpenSpec files under `openspec/`. The previous `skillvault-v1-alpha` cycle stored artifacts only in Engram memory; this change ports the same analysis into file-based OpenSpec format for portability, review, and traceability.

## Scope

### In Scope

- Read `skillvault-spec-v1.md` and extract all requirements, rules, and constraints
- Generate delta specs (`openspec/changes/skillvault-spec-analysis/spec.md`) with capability tables, requirement IDs, and GIVEN/WHEN/THEN scenarios
- Generate technical design (`openspec/changes/skillvault-spec-analysis/design.md`) covering architecture, data flow, interfaces, schema design, and build order
- Generate implementation tasks (`openspec/changes/skillvault-spec-analysis/tasks.md`) broken into ordered work units with TDD notes
- Generate archive/verify artifacts as needed to complete the SDD lifecycle for this change
- All artifacts written to `openspec/changes/skillvault-spec-analysis/`

### Out of Scope

- Writing any Go source code, tests, or implementation
- Modifying the existing codebase (`cmd/`, `internal/`, `Makefile`, `README.md`)
- Running the existing test suite
- Creating new requirements or altering the spec

## Approach

1. **Read source spec** — parse all 1974 lines covering vision, scope, stack, architecture, schema, CLI, MCP, and domain rules
2. **Reference existing Engram artifacts** — reuse the structure and decisions from `sdd/skillvault-v1-alpha/` (already archived) to ensure consistency
3. **Extract capabilities** — map spec sections to SDD capabilities (DB, domain, store, vars, app, CLI, MCP, import/export, FTS5, archiving)
4. **Generate delta specs** — produce requirement tables with MUST/SHOULD strength and GIVEN/WHEN/THEN scenarios per capability
5. **Generate design** — produce architecture overview, package structure, interface definitions, data flows, MCP protocol, schema design, and build order
6. **Generate tasks** — break into ordered implementation tasks using inside-out dependency order
7. **Write all files** — persist as OpenSpec markdown under `openspec/changes/skillvault-spec-analysis/`

## Prior Work

The spec was already fully analyzed and implemented under `sdd/skillvault-v1-alpha` (archived). This proposal references that prior work:

| Phase | Engram Topic Key | Status |
|-------|-----------------|--------|
| Initial proposal | `sdd/skillvault-v1-alpha/proposal` | ✅ Archived |
| Delta specs | `sdd/skillvault-v1-alpha/spec` | ✅ Archived |
| Technical design | `sdd/skillvault-v1-alpha/design` | ✅ Archived |
| Implementation tasks | `sdd/skillvault-v1-alpha/tasks` | ✅ Archived |
| Archive report | `sdd/skillvault-v1-alpha/archive-report` | ✅ Archived |

Key decisions already documented (reused as-is):
- Clean Architecture Light with 6 packages
- `modernc.org/sqlite` as sole dependency
- FTS5 standalone virtual table (not content='entries') due to in-memory SQLite incompatibility
- Tags stored in `entry_tags` table, denormalized for FTS5
- `series_entries` renumbering in app layer
- Workflows as self-contained entries with inline steps

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Spec is very detailed (1974 lines); this is primarily a formatting exercise | High | Reuse prior Engram artifact structure; no new requirements to discover |
| Duplicating effort already done in v1-alpha | High — by design | This is a deliberate port to OpenSpec file format for persistence outside Engram |
| OpenSpec directory may need conventions not yet established | Low | Check existing `openspec/` conventions if any exist; otherwise use standard SDD phase naming |

## Artifacts Produced

| Artifact | Path |
|----------|------|
| Proposal | `openspec/changes/skillvault-spec-analysis/proposal.md` |
| Delta Specs | `openspec/changes/skillvault-spec-analysis/spec.md` |
| Technical Design | `openspec/changes/skillvault-spec-analysis/design.md` |
| Implementation Tasks | `openspec/changes/skillvault-spec-analysis/tasks.md` |
| Verify Plan | `openspec/changes/skillvault-spec-analysis/verify.md` |
| Archive Report | `openspec/changes/skillvault-spec-analysis/archive.md` |

## Success Criteria

- [ ] All 6 SDD phases have corresponding files under `openspec/changes/skillvault-spec-analysis/`
- [ ] Specs cover 10 capabilities with requirement IDs and scenarios
- [ ] Design covers architecture, data flow, interfaces, schema, and build order
- [ ] Tasks are broken into ordered units suitable for inside-out implementation
- [ ] Engram memory updated with topic_key `sdd/skillvault-spec-analysis/proposal`
