# SkillVault v1-alpha

Knowledge Operating System — local, portable, structured knowledge library.

**Status**: v1-alpha (pre-release). Usable by CLI and MCP agents.

## Quickstart

```bash
# Build
go build -o ~/tools/skillvault ./cmd/skillvault

# Initialize the vault
skillvault init

# Create a project
echo '{"id":"vitacare","name":"VitaCare","active":true}' > /tmp/proj.json
skillvault project upsert /tmp/proj.json

# Add an entry
echo '{"id":"prd-fastapi","name":"FastAPI PRD","type":"skill","content":"Design FastAPI backend"}' > /tmp/entry.json
skillvault entry upsert /tmp/entry.json

# Search
skillvault search "fastapi"
```

## Architecture

```
cmd/skillvault (main.go)
  ├── internal/cli     (adapter: stdlib flag+os.Args)
  ├── internal/mcp     (adapter: stdio JSON-RPC 2.0)
  └── internal/api     (adapter: v1-final scaffold)
        ↓
  internal/app         (use cases)
        ↓
  internal/domain      (types, validators)
  internal/vars        (variable detection + injection)
        ↓
  internal/db          (SQLite + FTS5)
```

## Database

- Default path: `~/.skillvault/vault.db`
- Engine: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- FTS5 full-text search with `porter unicode61` tokenizer
- Soft delete only (`active=0`), no hard delete

## Commands (17 alpha commands)

| Command | Description |
|---------|-------------|
| `skillvault init` | Initialize vault database |
| `skillvault get <id>` | Get entry by ID |
| `skillvault search <query>` | FTS5 search with filters |
| `skillvault list` | List entries |
| `skillvault entry upsert <file>` | Create/update entry |
| `skillvault entry archive <id>` | Archive (soft-delete) entry |
| `skillvault project upsert <file>` | Create/update project |
| `skillvault project list` | List projects |
| `skillvault series get <id>` | Get series with entries |
| `skillvault series list` | List series |
| `skillvault series upsert <file>` | Create/update series |
| `skillvault series replace <id> <file>` | Replace series entries |
| `skillvault workflow run <id>` | Run workflow (render vars) |
| `skillvault export <file>` | Export vault to JSON |
| `skillvault import <file>` | Import vault from JSON |
| `skillvault mcp` | Start MCP server (stdio JSON-RPC) |
| `skillvault version` | Print version |

## MCP Tools (11 alpha tools)

- `get_entry`, `search_entries`, `list_entries`
- `upsert_entry`, `archive_entry`
- `get_series`, `list_series`, `upsert_series`, `replace_series_entries`
- `get_context`, `run_workflow`

### MCP Setup (Claude Code / OpenCode)

Add to your MCP config:

```json
{
  "mcpServers": {
    "skillvault": {
      "command": "~/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

## Alpha vs Final

**v1-alpha** includes:
- SQLite + FTS5 backend
- Entry, Project, Series, Workflow CRUD
- Variable detection + injection
- CLI (17 commands)
- MCP server (11 tools)
- Import/export (JSON)

**v1-final** will add:
- HTTP API (`net/http`)
- `project_refs`, `archive_series`, `archive_project`
- `copy_entry`, `copy_series`
- `add_entry_to_series`, `remove_entry_from_series`
- Stats, setup commands
- 22 MCP tools total

## Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Integration tests use in-memory SQLite (:memory:)
```

## Dependencies

- Go 1.26+
- `modernc.org/sqlite` (pure Go SQLite driver)
- No frameworks, no ORM, no Cobra, no Fiber

## License

MIT
