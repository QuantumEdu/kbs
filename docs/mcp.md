# MCP Server — 19 tools for AI agents

SkillVault runs as an MCP (Model Context Protocol) server over stdio JSON-RPC 2.0. This lets agents like Claude Code, OpenCode, or any MCP client read and write directly to your vault.

---

## Setup

### 1. Installation

```bash
go build -o ~/tools/skillvault ./cmd/skillvault
```

### 2. Configuration

Add to your `opencode.json` (or `claude_desktop_config.json`, depending on the client):

```json
{
  "mcpServers": {
    "skillvault": {
      "command": "/home/your-user/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

### 3. Symlink for direct access (optional)

```bash
ln -sf ~/tools/skillvault ~/tools/mcp
# Now agents can call "mcp" directly
# The binary detects the symlink and enters MCP mode automatically
```

When the binary runs as `mcp` (via the symlink name), it enters MCP mode directly without needing the argument.

### 4. Verification

```bash
skillvault mcp
# Waits for stdio JSON-RPC connections. Test with:
echo '{"jsonrpc":"2.0","id":1,"method":"list_projects","params":{}}' | skillvault mcp
```

---

## Tools

### `save_entry`

Save any type of entry in the vault.

**Parameters:**
- `title` (string, required) — Title
- `type` (string, required) — Type: `prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary`, `handoff`, `routing`
- `summary` (string, required) — Summary
- `body` (string, optional) — Long-form content
- `project` (string, optional) — Project slug
- `tags` (string[], optional) — Tags
- `status` (string, optional) — `draft`, `active`, `archived`, `deprecated`, `canonical`
- `purpose` (string, optional) — LifeOS-aligned purpose: `WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`

**Example (from agent):**
```json
{
  "method": "save_entry",
  "params": {
    "title": "Clean Architecture Rules",
    "type": "skill",
    "purpose": "KNOWLEDGE",
    "summary": "Clean Architecture rules for the project",
    "content": "1. Domain depends on nothing\n2. App depends on domain\n3. Adapters depend on app",
    "project": "myapp",
    "tags": ["architecture", "clean-code"]
  }
}
```

**Response:**
```json
{
  "id": "clean-architecture-rules",
  "status": "created"
}
```

---

### `search_entries`

Full-text search with filters.

**Parameters:**
- `query` (string, required) — Search terms
- `type` (string, optional) — Filter by type
- `project` (string, optional) — Filter by project
- `tags` (string[], optional) — Filter by tags
- `purpose` (string, optional) — Filter by purpose (`WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`)
- `status` (string, optional) — Filter by status
- `include_archived` (bool, optional) — Include archived (default: false)
- `limit` (int, optional) — Max results
- `vector` (bool, optional) — Use cosine similarity search (default: false)

---

### `get_entry`

Retrieve an entry by ID or slug.

**Parameters:**
- `id` (string, required) — Entry ID or slug

---

### `save_artifact`

Save a large filesystem-backed artifact.

**Parameters:**
- `title` (string, required) — Title
- `type` (string, required) — Artifact type
- `content` (string, required) — Full content
- `project` (string, optional) — Project
- `tags` (string[], optional) — Tags

**How it works:** content is written to `~/.skillvault/objects/YYYY/MM/<slug>.<ext>` with a SHA256 hash. Metadata (title, type, slug, date) goes to SQLite. Ideal for long AI outputs, PDF analyses, reports, etc.

---

### `save_result`

Save an AI prompt result as a vault entry.

**Parameters:**
- `name` (string, required) — Result name (generates slug)
- `content` (string, required) — Result content
- `type` (string, optional) — Entry type (default: `ai_output`)
- `category` (string, optional) — Optional category
- `tags` (string[], optional) — Tags
- `project` (string, optional) — Project slug
- `source_prompt_id` (string, optional) — ID of the prompt entry that generated this result
- `model` (string, optional) — Model that generated the result

---

### `get_context`

Compile an agent-ready context pack.

**Parameters:**
- `mode` (string, required) — `profile`, `project`, `workflow`, `skill`, `planning`, `session_recall`, `full_brief`
- `project` (string, required) — Project
- `max_chars` (int, optional) — Max characters (default: 10000)
- `include` (string[], optional) — Filter specific sections

**Response:** structured text with prioritized sections. Designed for direct injection into the agent's prompt.

---

### `compose_series`

Get ordered entries in a series.

**Parameters:**
- `series_id` (string, required) — Series ID

---

### `render_workflow`

Get workflow steps as an ordered checklist.

**Parameters:**
- `workflow_id` (string, required) — Workflow ID

---

### `session_wrap`

Create a session entry with decisions, pending items, and learnings.

**Parameters:**
- `project` (string, required) — Project slug
- `summary` (string, required) — Session summary
- `decisions` (string[], optional) — Decisions made
- `pending` (string[], optional) — Pending items
- `learnings` (string[], optional) — Learnings

**Example (from agent):**
```json
{
  "method": "session_wrap",
  "params": {
    "project": "myapp",
    "summary": "Implemented the auth module",
    "decisions": ["JWT with refresh tokens", "SQLite for sessions"],
    "pending": ["Add rate limiting", "Document endpoints"],
    "learnings": ["JWT middleware must go before the CORS handler"]
  }
}
```

---

### `archive_entry`

Archive an entry (changes status to `archived`).

**Parameters:**
- `id` (string, required) — Entry ID to archive

---

### `list_projects`

List all projects with their status.

**Parameters:** none.

---

### `save_entry_ref`

Create or update a graph edge between two entries.

**Parameters:**
- `source_id` (string, required) — Source entry ID
- `target_id` (string, required) — Target entry ID
- `relation_type` (string, required) — Type: `references`, `supersedes`, `related_to`, `part_of`, `derived_from`, `implements`, `uses`, `extends`, `handoff_of`, `generated_from`, `depends_on`
- `label` (string, optional) — Descriptive label

**Example:**
```json
{
  "method": "save_entry_ref",
  "params": {
    "source_id": "clean-architecture-rules",
    "target_id": "hexagonal-architecture-guide",
    "relation_type": "related_to",
    "label": "Both are architectural patterns"
  }
}
```

**Note:** `depends_on`, `part_of`, and `supersedes` have cycle detection — an edge that creates a cycle is rejected.

---

### `list_entry_refs`

List graph edges with optional filters.

**Parameters:**
- `source_id` (string, optional) — Filter by source
- `target_id` (string, optional) — Filter by target
- `relation_type` (string, optional) — Filter by relation type

---

### `get_entry_graph`

Traverse the graph from a starting entry, returning connected nodes and edges.

**Parameters:**
- `entry_id` (string, required) — Starting entry ID
- `depth` (int, optional) — Max depth (default: 3, max: 10)
- `direction` (string, optional) — `outgoing`, `incoming`, or `both` (default: both)

**Response:**
```json
{
  "root_entry": "clean-architecture-rules",
  "nodes": [{"id": "...", "title": "...", "type": "..."}],
  "edges": [{"source_id": "...", "target_id": "...", "ref_type": "..."}],
  "node_count": 5,
  "edge_count": 4
}
```

---

### `search_by_tags`

Search entries by tags using intersection (all) or union (any).

**Parameters:**
- `tags` (string[], required) — Tags to match
- `match` (string, optional) — `all` (intersection, default) or `any` (union)
- `type` (string, optional) — Filter by entry type
- `project` (string, optional) — Filter by project
- `limit` (int, optional) — Max results (default: 20)

**Example (from agent):**
```json
{
  "method": "search_by_tags",
  "params": {
    "tags": ["tdd", "go"],
    "match": "all"
  }
}
```

---

### `get_context_bundle`

Get a structured project context bundle in a single call.

**Parameters:**
- `project` (string, optional) — Project slug

**Response:** structured JSON with project info, entries grouped by type, and artifact references.

Use this as the first call when an agent starts working on a known project.

---

### `compare_entries`

Compute a line-based LCS unified diff between two entries.

**Parameters:**
- `id1` (string, required) — First entry ID
- `id2` (string, required) — Second entry ID

---

### `run_workflow`

Execute a workflow with structured step inputs and return per-step results.

**Parameters:**
- `workflow` (string, required) — Workflow slug or ID
- `steps` (object, optional) — Map of step index (int) → input text to inject into each step

**Response:** JSON array of per-step results with rendered prompts and their outputs. Steps without `entry_slug` are skipped (checklist-only steps).

---

### `route_scenario`

Resolve a free-text scenario to a matching workflow or skill.

**Parameters:**
- `scenario` (string, required) — Scenario text to route

**Response:** JSON with `scenario`, `target` (slug), `type` (workflow or skill), `description`, and `workflow` details. Returns an error if no match is found.

---

## Typical Agent Flow

```
1. Session start → get_context_bundle(project=myapp)
   → Full bundle: project + entries grouped by type + artifacts

2. During session → save_entry(...) or save_artifact(...)
   → Save skills, decisions, long outputs

3. Targeted search → search_by_tags(tags=["go","tdd"], match="all")
   → Find exact entries by tags

4. Scenario routing → route_scenario(scenario="research")
   → Resolve what workflow or skill handles the scenario

5. Execute workflow → run_workflow(workflow="research-workflow", steps={...})
   → Run the pipeline with structured inputs

6. Session close → session_wrap(project, summary, decisions, pending, learnings)
   → Persist session state for the next one
```

## Editor-specific Configuration

### OpenCode

```json
// opencode.json
{
  "mcpServers": {
    "skillvault": {
      "command": "/home/user/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

### Claude Code (VS Code extension)

```json
// claude_desktop_config.json
{
  "mcpServers": {
    "skillvault": {
      "command": "/home/user/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

### Cline / Continue

Same configuration — point to the same binary with `args: ["mcp"]`.
