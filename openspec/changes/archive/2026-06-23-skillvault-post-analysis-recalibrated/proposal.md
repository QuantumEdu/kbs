# Proposal: SkillVault Post-Analysis Recalibration

## Intent

Recalibrate the improvement plan after analyzing spec.md claims against the live v2 Hermes codebase. Spec had 4 misdiagnosed bugs (BUG-01/03/04 don't apply; BUG-02 works with current deps). Real issues: 3 bugs, 3 structural gaps, then 2 high-value features.

## Scope

### In Scope (3 Sweeps)

| Sweep | Priority | Items |
|-------|----------|-------|
| 1 — Bugfix | P0 | Migration version conflict (two `002_*.sql`), graceful shutdown MCP stdio (SIGTERM wiring), go.mod `go 1.24` for portability, FTS5 defensive import |
| 2 — Structural | P1 | Partial indexes on `status` (entries) + `active` (entry_links), schema.sql sync, HTTP API graceful shutdown (`Shutdown` not `Close`) |
| 3 — Features | P2 | `search_by_tags` MCP tool (all/any match), `get_context_bundle` MCP tool |

### Out of Scope (Deferred)

- Auth layer for HTTP API — separate change
- File watcher (`--watch` via fsnotify) — dependency decision pending
- Entry versioning, execution logs, semantic search, compare_entries, skill pack export

## Capabilities

### New Capabilities

None — both features extend existing `skillvault` domain.

### Modified Capabilities

- `skillvault` — Add `search_by_tags` MCP tool (all/any match against existing junction-table tags)
- `skillvault` — Add `get_context_bundle` MCP tool (structured bundle: project info + entries grouped by type + artifact refs)

## Approach

3-sweep chained PRs. Each sweep is standalone and testable:

1. **Sweep 1**: Fix real bugs — no new features. Pure remediation.
2. **Sweep 2**: Structural fixes — indexes and schema alignment. Builds on sweep 1.
3. **Sweep 3**: Feature delivery — MCP tools. Builds on clean schema from sweep 2.

## Affected Areas

| Area | Sweep | Change |
|------|-------|--------|
| `go.mod` | 1 | `go 1.24` for portability |
| `internal/db/migrate.go` + migrations | 1 | Rename `002_hermes.sql` → `003_hermes.sql` |
| `internal/app/run.go` | 1 | Wire os.Signal into MCP server context |
| `internal/db/db.go` | 1 | FTS5 side-effect import `_ "modernc.org/sqlite/lib/fts5"` |
| `internal/db/entries_store.go` | 2 | Partial index on `status = 'active'` |
| `internal/db/entry_links_store.go` | 2 | Partial index on `active = 1` |
| `internal/db/schema.sql` | 2 | Sync with actual migration output |
| `internal/api/server.go` | 2 | `Shutdown(ctx)` instead of `Close()` |
| `internal/mcp/tools.go` | 3 | `search_by_tags` + `get_context_bundle` tools |
| DB tag query layer | 3 | all/any tag intersection/union queries |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Migration rename breaks existing DBs | Low | `schema_migrations` table tracks version; rename is safe for fresh installs |
| go.mod version requires features from Go 1.26 | Low | Code uses std Go 1.22+ features; compile-test after change |
| schema.sql sync exposes missing store columns | Low | Store code already references name/content columns via migrations; sync is alignment |

## Rollback

Per-sweep revert. Each PR is independent. Sweep 3 is additive MCP handlers — no DB schema changes, zero risk on revert.

## Dependencies

None external. `fsnotify` deferred to future.

## Success Criteria

- [ ] All 11 test packages pass after each sweep
- [ ] `search_by_tags` returns entries matching `all` tags (intersection) and `any` tags (union)
- [ ] `get_context_bundle` returns structured JSON with project info + entries grouped by type
- [ ] MCP server shuts down on SIGTERM within 5s
- [ ] HTTP API server drains connections on graceful shutdown
- [ ] `go.mod` compiles with standard Go toolchain (not only Go 1.26.3)
