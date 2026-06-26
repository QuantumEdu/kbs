# Tasks: SkillVault Post-Analysis Recalibration

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 250-300 |
| 400-line budget risk | Low |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Sweep 1 bugfixes (base=main) → PR 2: Sweep 2 structural (base=PR1) → PR 3: Sweep 3 features+tests (base=PR2) |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Fix 4 P0 bugs | PR 1 | go.mod, migration rename, FTS5 import, signal handling |
| 2 | Fix 4 structural gaps | PR 2 | partial indexes, schema.sql sync, HTTP Shutdown |
| 3 | Deliver 2 MCP features | PR 3 | search_by_tags, get_context_bundle + full tests |

## Phase 1: Sweep 1 — P0 Bugfixes

- [x] 1.1 Change `go 1.26.3` → `go 1.24` in `go.mod` (REQ-CI-01)
- [x] 1.2 Rename `internal/db/migrations/002_hermes.sql` → `003_hermes.sql`; fix version marker line 200 from `(2, '002_hermes')` → `(3, '003_hermes')`
- [x] 1.3 Add `_ "modernc.org/sqlite/lib/fts5"` defensive import in `cmd/skillvault/main.go` (REQ-CI-04) — **DEVIATION**: import path `modernc.org/sqlite/lib/fts5` does not exist in v1.52.0. FTS5 is built into `_ "modernc.org/sqlite"`. Verified by `CGO_ENABLED=0 go test` and `TestFTS5VirtualTable` passing.
- [x] 1.4 Wire `os.Signal` (SIGTERM/SIGINT) → context cancel in `runMCP()` at `cmd/skillvault/main.go:735`; MCP server drains active calls on cancel, exits within 5s (REQ-CI-02)
- [x] 1.5 Run `go test ./...` — 11/11 packages must pass post-sweep-1

## Phase 2: Sweep 2 — P1 Structural

- [x] 2.1 Replace `idx_entries_status` regular index with partial `WHERE status = 'active'` in `003_hermes.sql` line 183 (REQ-CI-05)
- [x] 2.2 Replace `idx_entry_links_active` regular index with partial `WHERE active = 1` in `002_entry_refs_and_handoff.sql` line 228
- [x] 2.3 Sync `internal/db/schema.sql`: add `name`, `content` columns to entries table; add partial indexes; ensure `external_ref` in FTS5 matches migration output
- [x] 2.4 Replace `srv.Close()` with `srv.Shutdown(ctx)` + 5s deadline in `internal/api/server.go:81-85` Stop() method (REQ-CI-03)
- [x] 2.5 Run `go test ./...` — 11/11 packages must pass post-sweep-2

## Phase 3: Sweep 3 — P2 Features

- [x] 3.1 Add `SearchByTags(ctx, tags []string, matchAll bool, typePtr, projectPtr *string, limit int) ([]domain.EntrySearchResult, error)` to `EntryStore` interface at `internal/db/store.go:10` and implement in `internal/db/entries_store.go` using `entry_tags` junction table with `GROUP BY`/`HAVING COUNT` for intersection (REQ-TQR-01/02/03)
- [x] 3.2 Register `search_by_tags` MCP tool and handler in `internal/mcp/tools.go`: params `tags(array,req)`, `match(all|any)`, `type(opt)`, `project(opt)`, `limit(20)`. Returns id, title, type, summary, project, status, tags as text. Dispatches to `SearchByTags` (REQ-MCP-12)
- [x] 3.3 Register `get_context_bundle` MCP tool and handler in `internal/mcp/tools.go`: param `project(opt)`. Returns structured JSON with project info, entries grouped by type, artifact refs. Composes from existing `EntryStore.List` + `ProjectStore.Get` + `ArtifactStore.List` (REQ-MCP-13)
- [x] 3.4 Add `search_by_tags` and `get_context_bundle` cases to `dispatch()` in `internal/mcp/tools.go:161`; wire new store method into `ToolRegistry` if needed

## Phase 4: Testing

- [x] 4.1 Write table-driven test for `SearchByTags` intersection/union in `internal/db/entries_store_test.go`: seed entries with tags `["go","cli"]`, `["go"]`, `["cli"]`; verify all-match returns 1, any-match returns 3
- [x] 4.2 Write MCP handler test for `search_by_tags` and `get_context_bundle` in `internal/mcp/mcp_test.go` — use in-memory store with seeded data
- [x] 4.3 Run `CGO_ENABLED=0 go test ./...` to verify FTS5 works without CGO
- [x] 4.4 Run full `go test ./...` — 11/11 packages must pass (now 12/12 with new tests)
