# CLI Reference — 34 commands

All entries use **slugs** as identifiers. A slug is the title in kebab-case: `"Clean Architecture Review"` → `clean-architecture-review`.

---

## `init`

Initialize the vault: creates directories and the SQLite database.

```bash
skillvault init
```

Idempotent: if `~/.skillvault/` already exists, it only ensures subdirectories are present.

---

## `add-entry`

Save a reusable entry (prompt, skill, decision, feedback, routing rule, etc.).

```bash
skillvault add-entry \
  --title "CSS Grid Layout" \
  --type skill \
  --purpose KNOWLEDGE \
  --summary "Quick CSS Grid guide" \
  --body "grid-template-columns: repeat(...)" \
  --project web \
  --tags "css,frontend" \
  --status active
```

| Flag | Required | Description |
|------|----------|-------------|
| `--title` | ✅ | Entry title (generates slug automatically) |
| `--type` | ❌ | Type (default: `reference`): `prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary`, `handoff`, `routing` |
| `--purpose` | ❌ | LifeOS-aligned purpose: `WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`, `OBSERVABILITY` |
| `--summary` | ✅ | Short summary (indexed in FTS5) |
| `--body` | ❌ | Long-form body (optional if you only want the summary) |
| `--project` | ❌ | Slug of the project |
| `--tags` | ❌ | Comma-separated tags |
| `--status` | ❌ | `draft`, `active` (default), `archived`, `deprecated`, `canonical` |

The vault rejects an entry if it detects secrets (API keys, tokens, private keys).

### Purpose Taxonomy

Purpose is orthogonal to entry type — classify memory by why it exists, not just what shape it has.

| Purpose | Use for |
|---------|---------|
| `WORK` | Active projects, workflows, tasks, deliverables |
| `KNOWLEDGE` | Concepts, references, reusable technical facts |
| `LEARNING` | Lessons, skill development, retrospectives |
| `RELATIONSHIP` | People, organizations, stakeholder context |
| `STATE` | Current state snapshots, project status, handoffs |
| `OBSERVABILITY` | Logs, metrics, monitoring dashboards, workflow analytics |

---

## `search`

Full-text search with filters.

```bash
skillvault search "grid css" --type skill --purpose KNOWLEDGE --project web --status active --limit 10
```

| Flag | Description |
|------|-------------|
| `--type` | Filter by entry type |
| `--purpose` | Filter by purpose (`WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`, `OBSERVABILITY`) |
| `--project` | Filter by project |
| `--tag` | Filter by individual tag |
| `--include-archived` | Include archived entries |
| `--limit` | Max results (default: 20) |
| `--vector` | Use cosine similarity search (requires loaded vectors) |

Search uses FTS5 with `porter unicode61` tokenizer. It searches title, summary, content, and tags.

`--vector` enables semantic search with GloVe vectors (if configured via `SKILLVAULT_GLOVE_PATH`).

---

## `get`

Retrieve an entry by ID or slug.

```bash
skillvault get css-grid-layout
skillvault get skill:css-grid-layout        # with type prefix
```

Returns:

```json
{
  "id": "css-grid-layout",
  "type": "skill",
  "title": "CSS Grid Layout",
  "status": "active",
  "summary": "Quick CSS Grid guide",
  "created_at": "2026-06-20T..."
}
```

---

## `save-artifact`

Save a large artifact backed by the filesystem.

```bash
skillvault save-artifact \
  --title "Audit Report" \
  --type pdf_analysis \
  --content "$(cat report.md)" \
  --project myapp \
  --tags "security"
```

| Flag | Required | Description |
|------|----------|-------------|
| `--title` | ✅ | Artifact title |
| `--type` | ✅ | Type: `pdf_analysis`, `spec_doc`, `architecture_doc`, `research_note`, `prompt_response`, `ai_output`, `raw_document`, `report`, `log_analysis`, `generated_code` |
| `--content` | ✅ | Content (stored in `~/.skillvault/objects/YYYY/MM/`) |
| `--project` | ❌ | Associated project |
| `--tags` | ❌ | Tags |

Content is saved to disk with SHA256 hash. Metadata (title, type, tags, file slug) goes to SQLite.

---

## `get-context`

Compile a context pack for AI agents.

```bash
skillvault get-context \
  --mode planning \
  --project myapp \
  --max-chars 8000
```

| Flag | Required | Description |
|------|----------|-------------|
| `--mode` | ✅ | Mode: `profile`, `project`, `workflow`, `skill`, `planning`, `session_recall`, `full_brief` |
| `--project` | ✅ | Project |
| `--max-chars` | ❌ | Max characters (default: 12000) |
| `--include` | ❌ | Filter specific sections (comma-separated) |
| `--query` | ❌ | Additional query to filter entries |

### Context Modes

| Mode | Includes |
|------|----------|
| `profile` | User feedback + user-type entries |
| `project` | Active project state + decisions + artifact summaries |
| `workflow` | Workflows + their steps |
| `skill` | Active skills + prompts |
| `planning` | profile + project + workflow combined |
| `session_recall` | Last 10 sessions |
| `full_brief` | All sections |

If content exceeds `max_chars`, lowest-priority sections are truncated first.

---

## `add-project`

Create a project.

```bash
skillvault add-project \
  --name "MyApp" \
  --description "Example web application"
```

Projects group entries, artifacts, sessions, and workflows.

---

## `list-projects`

List all projects.

```bash
skillvault list-projects
```

---

## `archive`

Archive an entry (soft delete: changes status to `archived`).

```bash
skillvault archive css-grid-layout
```

Archived entries remain searchable but are excluded from context packs.

---

## `add-workflow`

Create a workflow from a JSON file.

```bash
skillvault add-workflow workflow.json
```

JSON format:

```json
{
  "id": "spec-plan-task",
  "name": "Spec → Plan → Task",
  "steps": [
    {"order": 1, "description": "Write spec", "type": "prompt"},
    {"order": 2, "description": "Review with team"},
    {"order": 3, "description": "Create tasks"}
  ]
}
```

---

## `import-workflow`

Import a workflow-builder YAML file into SkillVault workflows and phase-skill entries.

```bash
skillvault import-workflow --file .agent/skills/research/workflow.yaml
skillvault import-workflow --file workflow.yaml --project myapp
```

| Flag | Required | Description |
|------|----------|-------------|
| `--file` | ✅ | Path to workflow-builder YAML file |
| `--project` | ❌ | Project slug or ID for scoped import |

This command reads the workflow-builder YAML format (phases → skills → steps), creates workflow entries and phase-skill entries, and links them. Use this as the bridge between agent workflow definitions and SkillVault workflows.

---

## `render-workflow`

Render a workflow as a checklist.

```bash
skillvault render-workflow spec-plan-task
```

Output:

```
- [ ] Write spec
- [ ] Review with team
- [ ] Create tasks
```

---

## `route`

Resolve a natural-language scenario to its matching workflow or skill.

```bash
skillvault route research
skillvault route onboarding --json
```

| Flag | Required | Description |
|------|----------|-------------|
| `<scenario>` | ✅ | Scenario text to route (positional) |
| `--json` | ❌ | Output result as JSON |

Output (default):

```
Route: research → research-workflow (workflow)
  Description: Research and analyze a topic
  Workflow: research-workflow (wf-abc123)
  Steps:
    1. Define scope [REQUIRED]
    2. Gather sources
    3. Synthesize findings
```

JSON output includes full metadata: `scenario`, `target`, `type` (workflow/skill), `description`, and `workflow` details.

Routing works by matching the scenario text against entries of type `routing` and `workflow_note` that contain YAML route maps.

---

## `run`

Execute a workflow as a step-by-step pipeline.

```bash
skillvault run <workflow-slug> <input-file> [--save output.md]
skillvault run research-article article.md --save result.md
skillvault run research-article -                  # read input from stdin
```

| Flag | Required | Description |
|------|----------|-------------|
| `--save` | ❌ | Save final output to a file |

The pipeline executes each step that has `entry_slug` configured:
1. Resolves the linked entry and verifies it's active
2. Injects `{{input}}`, `{{previous_output}}`, `{{final_output}}` into the content
3. Prints the rendered prompt to stdout
4. Reads the agent's response from stdin
5. Passes the result to the next step as `{{previous_output}}`

Steps without `entry_slug` are skipped (renderable checklists).

---

## `session-wrap`

Create a session entry with decisions, pending items, and learnings.

```bash
skillvault session-wrap \
  --project myapp \
  --summary "Sprint planning" \
  --decisions "Migrate to SQLite,use FTS5" \
  --pending "Benchmark queries" \
  --learnings "FTS5 needs explicit tokenizer"
```

Comma-separated parameters. Optionally links an artifact.

---

## `graph`

Visualize the entry relationship graph.

```bash
skillvault graph --entry clean-architecture-review --depth 3 --format mermaid
skillvault graph --entry clean-architecture-review --format json
skillvault graph --entry clean-architecture-review --format dot
```

| Flag | Required | Description |
|------|----------|-------------|
| `--entry` | ✅ | Root entry ID |
| `--depth` | ❌ | Traversal depth (default: 3, max: 10) |
| `--format` | ❌ | `mermaid`, `json`, or `dot` (default: mermaid) |
| `--direction` | ❌ | `outgoing`, `incoming`, or `both` (default: both) |

The `mermaid` format generates `graph TD` which renders natively on GitHub.

---

## `entry ref`

Manage graph edges between entries (entry_links).

```bash
# Add a relation
skillvault entry ref add <source> <target> <type> --label "optional"

# List relations
skillvault entry ref list [--source <id>] [--target <id>] [--type <rel>]

# Remove a relation
skillvault entry ref remove <source> <target> <type>
```

Relation types: `references`, `supersedes`, `related_to`, `part_of`, `derived_from`, `implements`, `uses`, `extends`, `handoff_of`, `generated_from`, `depends_on`.

`depends_on`, `part_of`, and `supersedes` have cycle detection.

> 💡 The `ref` subcommand can also be invoked directly as `skillvault ref ...` (alias).

---

## `memory index` / `memory reindex` / `memory list-external`

Index pi-memory (.md) files as shadow entries in the vault.

```bash
# Index a memory directory
skillvault memory index --path ~/memory --project myapp [--wikilinks]

# Reindex (alias)
skillvault memory reindex --path ~/memory --project myapp

# List indexed external entries
skillvault memory list-external --project myapp
```

| Flag | Required | Description |
|------|----------|-------------|
| `--path` | ✅ | Directory with .md files |
| `--project` | ✅ | Target project |
| `--wikilinks` | ❌ | Parse `[[wikilinks]]` and create entry_refs |

Supports YAML frontmatter (description, tags, created, updated). Files removed from the directory are auto-archived (orphan cleanup).

---

## `compare-entries`

Compute a unified diff between two entries.

```bash
skillvault compare-entries <entry-id-1> <entry-id-2>
```

Shows a line-based LCS diff of both entries' full text representations (title + summary + body + tags).

---

## `export`

Export the entire vault to a JSON file.

```bash
skillvault export backup.json
skillvault export --output export.json   # explicit
```

| Flag | Required | Description |
|------|----------|-------------|
| `--output` | ❌ | Output path (default: `skillvault-export.json`) |

Includes all entry types, projects, workflows, series, tags, entry_links, and artifact metadata.

---

## `import`

Import a vault from a JSON file.

```bash
skillvault import backup.json
```

Resolves slug conflicts automatically (appends numeric suffix to duplicates).

---

## `save-result`

Save an AI prompt result as a vault entry.

```bash
skillvault save-result --name "My result" --content "Model response..." [flags]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | ✅ | Result name (generates slug) |
| `--content` | ✅ | Result content |
| `--type` | ❌ | Type (default: `ai_output`) |
| `--category` | ❌ | Optional category |
| `--tags` | ❌ | Comma-separated tags |
| `--project` | ❌ | Project slug |
| `--source-prompt` | ❌ | ID of the prompt entry that generated this result |
| `--model` | ❌ | Model that generated the result |

**MCP equivalent:** `save_result` (available in MCP mode).

---

## `version`

Show vault version.

```bash
skillvault version
# SkillVault v3
```

---

## `stats`

Show vault statistics and entry counts. Three output variants.

```bash
# Default: human-readable summary (entry counts, projects, totals)
skillvault stats

# Include workflow run analytics (total runs, run history summary)
skillvault stats --workflow-runs

# Machine-readable JSON output (includes all stats including workflow runs)
skillvault stats --json
```

| Flag | Required | Description |
|------|----------|-------------|
| `--workflow-runs` | ❌ | Include workflow run analytics in output |
| `--json` | ❌ | Output as JSON |

Default output shows summary counts: total entries, entry type breakdown, active/archived/draft status counts, project count, artifact count, and workflow count.

With `--workflow-runs`, the output appends a workflow run summary: total runs, runs per workflow, and recent run history.

With `--json`, all statistics are printed as a single JSON object (workflow run analytics are included automatically — no separate flag needed).

**MCP equivalent:** `get_stats`, `list_workflow_runs`, `get_run` (see [`docs/mcp.md`](mcp.md)).

---

## `http`

Start the HTTP REST API server.

```bash
skillvault http
# Serves on http://127.0.0.1:7438
```

| Flag | Required | Description |
|------|----------|-------------|
| `--api-key` | ❌ | API key for HTTP Basic authentication |

Endpoints: health, entries CRUD, artifacts, context, projects, sessions, workflows, export/import. See [`docs/quickstart.md`](quickstart.md) or [`docs/architecture.md`](architecture.md) for details.
