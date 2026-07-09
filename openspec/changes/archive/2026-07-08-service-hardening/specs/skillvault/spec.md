# SkillVault Delta Spec — service-hardening

## ADDED Requirements

### REQ-MCP-02: save_result MCP Tool

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MCP-02 | `save_result` MCP tool accepting: `name` (required), `content` (required), `type`, `category`, `tags`, `project_id`, `source_prompt_id`, `model`. Returns `entry_id`, `name`, `type`, `project_id`. Wraps existing `SavePromptResultService`. | MUST |

**MCP tool count**: 18→19.

### REQ-HTTP-01: API Key Authentication

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-HTTP-01 | HTTP server supports optional API key auth via `--api-key` flag. When set, all write endpoints require `Authorization: Bearer <key>`. `/health` remains unauthenticated. When unset, all endpoints work as before (backward compat). | MUST |

**Endpoints requiring auth** when key is set: POST/PUT/DELETE on `/entries`, `/entries/*`, `/artifacts`, `/context`, `/projects`, `/sessions/wrap`, `/workflows`, `/workflows/*`, `/export`, `/import`.

### REQ-GRACE-01: Graceful Shutdown

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-GRACE-01 | HTTP server `Start()` wraps `srv.ListenAndServe` with signal handling (SIGINT/SIGTERM). On signal, calls `s.Shutdown(ctx)` with 5s timeout. Both MCP and HTTP servers shut down gracefully. | MUST |

### REQ-DOCS-01: Vars Documentation

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-DOCS-01 | `docs/vars.md` documents: frontmatter parsing (`---\nkey: value\n---`), `{{variable}}` detection/injection, `--vars` flag, inline vs in-place replace (`-i`). | MUST |

### REQ-DOCS-02: Command Docs Sync

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-DOCS-02 | `docs/commands.md` reflects 21 commands with: `entry history`, `entry restore`, `setup-vectors`, `reindex-embeddings`, `compare-entries`, `graph`, `memory index/reindex/list-external`, `entry ref add/list/remove`, `run`, pack flags (`export --pack --author --version --description`, `import --pack --prefix`). | MUST |

## MODIFIED Requirements

### REQ-CLI-02: CLI Command Count
Command count updated to 21.

### REQ-MCP-01: MCP Tool Count
Tool count updated to 19.
