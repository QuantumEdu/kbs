# Tasks: service-hardening

> **Chain strategy**: Single PR (estimated ~300 lines)
> **Review budget**: Low (under 400)
> **Branch**: `feat/service-hardening`

---

## Phase 1: save_result MCP Tool

- [ ] 1.1 Add `save_result` tool definition + dispatch case — `internal/mcp/tools.go`
- [ ] 1.2 Implement `handleSaveResult` handler — `internal/mcp/tools.go`
- [ ] 1.3 Add `WithSaveResultService` builder — `internal/mcp/tools.go`
- [ ] 1.4 Wire `saveResultSvc` into ToolRegistry in `runMCP()` — `cmd/skillvault/main.go`
- [ ] 1.5 Extend `mcp_test.go`: test save_result tool with valid input, missing required fields, and tool count 19 — `internal/mcp/mcp_test.go`

## Phase 2: HTTP Auth Layer

- [ ] 2.1 Add `apiKey` field and `WithAPIKey` builder to `Server` — `internal/api/server.go`
- [ ] 2.2 Implement `authMiddleware` — `internal/api/server.go`
- [ ] 2.3 Apply middleware in `Start()` — `internal/api/server.go`
- [ ] 2.4 Add `--api-key` flag to `http` command parsing — `internal/cli/commands.go` and `cmd/skillvault/main.go`
- [ ] 2.5 Extend `server_test.go`: test auth is skipped when no key, required when key set, wrong key returns 401, health always open — `internal/api/server_test.go`

## Phase 3: Graceful Shutdown

- [ ] 3.1 Move `Start()` to use `signal.NotifyContext` + goroutine for `Shutdown` — `internal/api/server.go`
- [ ] 3.2 Extend `server_test.go`: test graceful shutdown returns cleanly — `internal/api/server_test.go`

## Phase 4: docs/vars.md

- [ ] 4.1 Write `docs/vars.md` covering: frontmatter, variable syntax, `--vars` flag, `-i` flag, examples — `docs/vars.md`

## Phase 5: docs/commands.md Sync

- [ ] 5.1 Update command table to 21 commands with new ones: `entry history`, `entry restore`, `setup-vectors`, `reindex-embeddings`, `compare-entries`, `graph`, `memory index/reindex/list-external`, `entry ref add/list/remove`, `run` — `docs/commands.md`
- [ ] 5.2 Add pack flags to export/import commands — `docs/commands.md`

---

## Verification

| Command | Expected |
|---------|----------|
| `go build ./...` | Builds clean |
| `go test ./...` | All 13+ packages pass |
| MCP `save_result` with name+content | Returns entry_id |
| MCP `save_result` missing name | Returns error |
| HTTP `/health` without key | Returns 200 |
| HTTP POST `/entries` without key when key set | Returns 401 |
| HTTP POST `/entries` with valid key | Returns 200 |
| HTTP graceful shutdown | Server stops within 5s |

## Rollback

Revert `internal/mcp/tools.go` (save_result additions), `internal/api/server.go` (auth + shutdown), `cmd/skillvault/main.go` (wiring), restored docs from git.
