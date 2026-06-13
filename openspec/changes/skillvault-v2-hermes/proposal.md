# Proposal: SkillVault v2 Hermes

## Intent

Rewrite SkillVault from pure SQLite vault to hybrid DB+filesystem vault with Hermes context layer, artifact management, secret detection, and expanded entity model. This supersedes the v1-alpha spec with a full implementation targeting the architectural vision in `skillvault_v1_alpha_spec.md`.

## Scope

### In Scope

- **Domain model rewrite** — 10+ entry types (`prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary`), 5-status model (`draft`, `active`, `archived`, `deprecated`, `canonical`), `Artifact` entity (9 types), `EntryLink` (6 relation types), `Workflow` + `WorkflowStep`, `Series` + `SeriesEntry`, `Tag` entity, `Project` entity
- **SQLite schema v2** — migration 002 (or fresh) with FTS5, supporting all entities, relationships, tags, and search indexes
- **Artifact filesystem layer** (`internal/files/`) — write/read files under `~/.skillvault/objects/YYYY/MM/`, content hashing, MIME type detection, size tracking
- **Hermes context compiler** (`internal/context/`) — 7 modes (`profile`, `project`, `workflow`, `skill`, `planning`, `session_recall`, `full_brief`), configurable max chars, priority-ordered context composition, archived exclusion
- **Secret scanner** (`internal/security/`) — 4 regex patterns (OpenAI key, private key, GitHub PAT, Slack token), reject/redact behavior, warning return
- **10 MCP tools** — `save_entry`, `search_entries`, `get_entry`, `save_artifact`, `get_context`, `compose_series`, `render_workflow`, `session_wrap`, `archive_entry`, `list_projects`
- **14 CLI commands** — `init`, `add-entry`, `search`, `get`, `save-artifact`, `get-context`, `add-project`, `list-projects`, `archive`, `add-workflow`, `render-workflow`, `session-wrap`, `export`, `import`
- **Session wrap service** — creates session entry with decisions, pending items, learnings, linked project, optional artifact
- **Import/export** — JSON format with projects, entries, workflows, steps, series, tags, artifact metadata + manifest; conflict handling on duplicate slugs (suffix or report)
- **Natural language save policy** — small reusable → entry, long output/final doc/PDF analysis/spec/report → artifact file + DB metadata, temporary → cache/don't save
- **Secret detection** — 4 regex patterns with rejection/redaction on `save_entry` and `save_artifact`

### Out of Scope

- Cloud synchronization
- GUI or TUI
- Vector database or embeddings
- Background daemon
- Automatic execution of workflows
- Multi-user support
- Agent marketplace
- PDF parsing engine
- Full AGENTS.md hook implementation

## Approach

Hybrid implementation — evolve existing infrastructure (MCP/CLI pattern, FTS5, migration runner, vars engine from v1-alpha) and rewrite domain/app/files/context/security packages. The existing `skillvault` binary structure under `cmd/` and `internal/` provides the foundation; v2 adds new packages and extends existing ones.

## Build Order

1. Domain entities + SQLite schema v2
2. Store implementations (entries, artifacts, workflows, series, tags, entry_links)
3. App/use case layer
4. Artifact filesystem layer (`internal/files/`)
5. Secret scanner (`internal/security/`)
6. Hermes context compiler (`internal/context/`)
7. CLI commands
8. MCP tools
9. Main wiring + integration

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| No data migration risk (greenfield) | None | v2 starts fresh or clean; no v1 data to migrate |
| Scope creep in Hermes context modes | Medium | Implement `profile` + `project` + `planning` first; add remaining modes iteratively |
| FTS5 incompatibility with in-memory SQLite for tests | Known | Use standalone FTS5 virtual table pattern from v1-alpha |
| CLI ↔ MCP parity drift | Medium | Derive both from same app-layer use cases; test both paths |

## Prior Work

| Phase | Reference | Status |
|-------|-----------|--------|
| v1-alpha spec | `skillvault_v1_alpha_spec.md` (1207 lines) | ✅ Authoritative spec |
| v1-alpha SDD cycle | `sdd/skillvault-v1-alpha/` (Engram) | ✅ Archived |
| Spec analysis | `openspec/changes/skillvault-spec-analysis/` | ✅ Archived |

## Artifacts Produced

| Artifact | Path |
|----------|------|
| Proposal | `openspec/changes/skillvault-v2-hermes/proposal.md` |
| Delta Specs | `openspec/changes/skillvault-v2-hermes/spec.md` |
| Technical Design | `openspec/changes/skillvault-v2-hermes/design.md` |
| Implementation Tasks | `openspec/changes/skillvault-v2-hermes/tasks.md` |
| Verify Plan | `openspec/changes/skillvault-v2-hermes/verify.md` |
| Archive Report | `openspec/changes/skillvault-v2-hermes/archive.md` |

## Success Criteria

- [ ] Proposal approved and Engram-persisted
- [ ] All SDD phases produce corresponding files under `openspec/changes/skillvault-v2-hermes/`
- [ ] Spec covers all entities, tools, commands, security, and import/export from v1-alpha spec
- [ ] Design covers package structure, interfaces, data flow, schema v2, Hermes compiler architecture
- [ ] Tasks ordered for inside-out implementation per build order
- [ ] Verify plan traces to spec acceptance criteria (AC1–AC10)
