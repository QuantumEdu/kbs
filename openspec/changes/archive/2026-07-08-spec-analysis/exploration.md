## Exploration: spec.md Analysis for SkillVault

### Current State

The codebase at `/home/ubuntu/dev/kbs` is a working v2 Hermes SkillVault. All packages compile, all tests pass (11/11 packages OK). Go 1.26.3 is the installed toolchain on this machine. The system uses:
- `status` column (TEXT: draft/active/archived/deprecated/canonical) for entries, projects, series, workflows — migrated from the legacy `active` INTEGER column
- `modernc.org/sqlite` v1.52.0 with SQLite 3.53.2, FTS5 compiled in by default
- `net/http` standard library for HTTP API, NOT Fiber
- Tags via `tags` + `entry_tags` junction tables + `tags_denorm` for FTS5
- Single-pass variable resolution in `internal/vars` (no recursion, no cycle risk)
- Import/export in plain JSON format
- No authentication on HTTP API

### Affected Areas

- `go.mod` — declares `go 1.26.3` (works on this machine, may not on standard Go)
- `internal/vars/resolver.go` — single-pass resolve, no depth limit or cycle detection (but no recursion, so current risk is low)
- `internal/mcp/server.go` — `Run()` takes context but no SIGTERM/SIGINT wiring
- `internal/db/migrate.go` — two migration files with version=2 (conflict: only one runs)
- `internal/db/schema.sql` — reference schema differs from actual migration output (missing `name`, `content` columns; missing `external_ref` in FTS5)
- `internal/api/server.go` — zero auth, no rate limiting
- `internal/cli/commands.go` — no `upsert <file>` command exists; spec's BUG-04 is moot
- `internal/db/entry_links_store.go` — uses `active` INTEGER (not `status` TEXT); queries on `active = 1` do NOT use partial indexes
- `docs/` — no `vars.md` exists

### Per-Item Assessment

#### P0 Bugfixes

| ID | Spec Claim | Reality | Verdict |
|----|-----------|---------|---------|
| BUG-01 | `go 1.26` is invalid version | Go 1.26.3 IS installed and code compiles | **NOT A BUG on this machine.** Would fail on standard Go 1.22/1.23/1.24. Consider `go 1.24` for portability. |
| BUG-02 | FTS5 not compiled into modernc.org/sqlite | FTS5 tests pass; SQLite 3.53.2 has FTS5 by default in v1.52.0 | **NOT A BUG with current version.** Side-effect import `_ "modernc.org/sqlite/lib/fts5"` is defensive best practice for older versions. |
| BUG-03 | Variable resolution causes infinite loops | Resolver is SINGLE-PASS — no recursion exists. Cannot loop. | **NOT A BUG currently.** If recursive resolution is added in the future, depth limits become critical. |
| BUG-04 | `upsert <file>` lacks validation | No `upsert` command exists in CLI, MCP, or app layer | **NON-EXISTENT FEATURE.** Spec describes a command that doesn't exist. `add-entry` uses flags, not files. MCP `save_entry` takes JSON params. |

#### P1 Structural

| ID | Spec Claim | Reality | Verdict |
|----|-----------|---------|---------|
| STRUCT-01 | Partial indexes on `active` column | Entries use `status` TEXT column! `active` is legacy. Regular `idx_entries_status` exists. `entry_links.active` still INTEGER. | **CONCEPT VALID, DETAILS WRONG.** Partial indexes should be on `status = 'active'` not `active = 1`. `entry_links.active = 1` partial index IS valid. |
| STRUCT-02 | Graceful shutdown for MCP stdio | No signal handling anywhere in codebase. `context.Background()` passed to server. | **REAL ISSUE.** MCP server stops on EOF from stdin, but SIGTERM/SIGINT are ignored. |

#### P2-P4 Features

| ID | Feature | Exists? | Verdict |
|----|---------|---------|---------|
| FEAT-01 | Tag system | Tags already implemented (tags table + entry_tags junction + tags_denorm + CLI flags + MCP search filter) | **ALREADY EXISTS** in different form. Spec proposes JSON array column — current is junction table. `search_by_tags` MCP tool with all/any match is NEW. |
| FEAT-02 | Entry versioning | No entry_versions table | **NEW FEATURE.** Valuable for prompt engineering workflow. |
| FEAT-03 | Execution logs | No workflow_runs table | **NEW FEATURE.** Valuable for agent debugging. |
| FEAT-04 | get_context_bundle MCP tool | `get_context` tool exists but returns text packs, not structured bundles with project info + grouped entries | **PARTIALLY EXISTS.** Current tool is close but not the structured bundle spec describes. |
| FEAT-05 | Ranked/semantic search | Pure FTS5, no scoring | **NEW FEATURE.** Phase 1 (hybrid scoring) is feasible without embeddings. |
| FEAT-06 | compare_entries | Does not exist | **NEW FEATURE.** Requires unified diff implementation. |
| FEAT-07 | File watcher (--watch) | Does not exist. `fsnotify` not in go.mod | **NEW FEATURE.** Requires external dependency (fsnotify) — violates "no frameworks" convention. Consider using `syscall` or stdlib. |
| FEAT-08 | Skill pack export | Import/export exists as plain JSON, not "skill pack" format with pack_id/author/version metadata | **ENHANCEMENT of existing feature.** Extends current export. |
| SEC-01 | Auth layer for HTTP | Zero auth. No API key, no mTLS. HTTP server binds to 127.0.0.1:7438 without any middleware. | **MISSING — critical before exposing HTTP.** Also note: spec uses `fiber.Handler` but project uses `net/http`. |
| DOC-01 | vars engine spec | No `docs/vars.md`. Internal/vars has 3 functions, minimal godoc. | **MISSING DOCUMENTATION.** Easy to produce, high value for contributors. |

#### Additional Issues Discovered (NOT in spec.md)

1. **Migration version conflict**: Two SQL files share version=2 (`002_entry_refs_and_handoff.sql` and `002_hermes.sql`). Only the first (alphabetically) runs on fresh install. The second's schema changes (workflow_steps v2 recreation, entries type CK update in 002_hermes) are silently skipped.

2. **Schema drift**: `schema.sql` (reference) differs from actual migration output. Missing columns: `entries.name`, `entries.content`, `entries.external_ref` in FTS5. Present but deprecated: `entries.active`.

3. **Entry store INSERT references stale columns**: `entries_store.go` line 43 INSERT references `name` and `content` columns which exist in migrations but NOT in `schema.sql` reference. If schema.sql were the source of truth, this would be a bug.

4. **save-result MCP tool missing**: CLI has `save-result` command but MCP tools don't expose it. Agents can't save prompt results via MCP.

5. **HTTP API has no graceful shutdown**: `api.Server.Stop()` calls `srv.Close()` not `srv.Shutdown(ctx)`. No context-aware shutdown.

### Recommendations

1. **Fix go.mod FIRST**: Change to `go 1.24` for portability. 5 minutes, unblocks everything.

2. **Add FTS5 side-effect import**: Defensive. Costs nothing, prevents runtime surprise on older modernc versions.

3. **Fix STRUCT-02 (graceful shutdown)**: Real bug. Wire os.Signal handling into `runMCP()`. ~1h.

4. **Adjust STRUCT-01**: Partial indexes belong on `status` (entries) and `active` (entry_links), not `active` (entries).

5. **Drop BUG-03 and BUG-04 from bugfix list**: These are NOT bugs in current code. BUG-03 is a forward-looking risk for recursive resolution. BUG-04 describes a nonexistent command.

6. **Decide on tag system direction**: Current junction-table approach works. JSON array in `entries.tags` (spec proposal) is simpler but denormalized. Either way, `search_by_tags` MCP tool is the real feature gap.

7. **Fix migration version conflict**: Rename one of the two version-2 migrations to version 3. Otherwise fresh installs get incomplete schema.

8. **Prioritize FEAT-04 (get_context_bundle)** and **SEC-01 (auth layer)** if HTTP API is being used. These are the highest-impact features.

### Ready for Proposal

**Yes** — but the proposal should focus on:
1. A P0 cleanup sweep (go.mod version + migration version conflict + FTS5 import)
2. A P1 structural fix (graceful shutdown + partial indexes on correct columns)
3. Then decide feature priority based on user needs (tag search tool, context bundle, or auth first)

**Recommend NOT creating a proposal for**: BUG-01 (as stated, it's not broken on this machine), BUG-03 (not recursive), BUG-04 (feature doesn't exist).

### Risks

- Changing `go.mod` version from 1.26.3 to 1.24 could break if Go 1.26 features are used (none apparent)
- Adding `modernc.org/sqlite/lib/fts5` import may cause build-time issues if the package path is incorrect in v1.52.0
- Migration version conflict could cause data loss if both migrations are needed on a real database
- `fsnotify` dependency for file watcher violates "no frameworks" convention
