# SkillVault Qu@ntum

**Local-first knowledge operating system for developers and AI agents.**

Store, search, and retrieve prompts, skills, workflows, decisions, project memory, session summaries, and long AI outputs — all from one portable Go binary.

```
   _____ _ _    _  __     __          _          _
  / ____| (_)  | | \ \   / /__ _ __ | |__   ___| |
 | (___ | | |  | |  \ \ / / _ \ '_ \| '_ \ / _ \ |
  \___ \| | |  | |   \ V /  __/ | | | |_) |  __/ |
  ____) |_|_|  | |    \_/ \___|_| |_|_.__/ \___|_|
 |_____/       |_|
```

**Codename:** Qu@ntum  
**Status:** v3 — Workflow bridge + LifeOS taxonomy + workflow analytics + entry versioning + skill pack export
**Binary size:** ~7 MB  
**Dependencies:** Zero frameworks. Only `modernc.org/sqlite`.  
**Language:** Go 1.26+

---

## Docs

| Guide | Description |
|-------|-------------|
| [`docs/quickstart.md`](docs/quickstart.md) | Install, init, and first 6 steps in 5 minutes |
| [`docs/commands.md`](docs/commands.md) | Full CLI reference — all commands with flags |
| [`docs/vars.md`](docs/vars.md) | Variable detection, frontmatter, and injection guide |
| [`docs/mcp.md`](docs/mcp.md) | MCP server setup for Claude Code / OpenCode |
| [`docs/tutorial.md`](docs/tutorial.md) | Real-world workflow: project → skills → context → session |
| [`docs/architecture.md`](docs/architecture.md) | Clean Architecture deep-dive, data flows, design decisions |

---

## What SkillVault Does

SkillVault is a local knowledge and workflow layer for humans and AI agents:

- Store reusable prompts, skills, references, decisions, session summaries, and workflow notes.
- Classify entries by **type** and by LifeOS-aligned **purpose** (`WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`, `OBSERVABILITY`).
- Import workflow-builder YAML into SkillVault workflows and phase-skill entries.
- Route natural scenarios to the right workflow or skill with `skillvault route <scenario>`.
- Run workflows from the CLI or via MCP with structured JSON-RPC-compatible output.
- Serve agent tools over MCP (`run_workflow`, `route_scenario`, search, context, graph, artifacts, and more).

---

## Pain Problem

Working with AI coding agents creates repeated friction:

**Prompts and skills** scatter across chats, files, tools, and GitHub repos.  
**Agents get overloaded** if every skill is installed globally.  
**Long AI outputs** (PDF analyses, specs, reports) are valuable but pollute context if saved into prompts.  
**Project decisions** are forgotten across sessions.  
**Workflows miss steps** when there's no reusable checklist.  
**Context** is either too little or too much — never the right amount.

SkillVault solves this by becoming a **local source of truth** for everything your agents need, with retrieval designed for both humans and AI.

---

## Innovation: The Qu@ntum Context Layer

Most vaults just store things. SkillVault Qu@ntum **delivers the right context** when agents need it.

- **7 context modes** — profile, project, workflow, skill, planning, session_recall, full_brief
- **Priority-based compilation** — user feedback > project state > decisions > workflows > sessions > artifact summaries > references
- **Token-aware truncation** — respects `max_chars`, drops lowest-priority sections first
- **Output**: clean, structured text ready for agent injection:

```
# CONTEXT PACK

## Scope
Project: MyApp
Mode: planning

## User Preferences
- Prefer practical architecture
- Use TDD for core domain behavior

## Active Decisions
- SQLite for local storage
- No cloud sync in v1

## Suggested Next Action
Generate implementation plan from spec.
```

---

## Installation

```bash
# Prerequisites: Go 1.26+
git clone --recurse-submodules https://github.com/QuantumEdu/kbs
cd kbs

# Build (single binary, no CGO)
go build -o ~/tools/skillvault ./cmd/skillvault

# Initialize the vault
skillvault init

# Verify
skillvault version
```

That's it. One binary. No daemon, no database server, no frameworks.

> **Self-update**: Use `skillvault update` to rebuild and reinstall from source.
> Configure the source repo with `SKILLVAULT_REPO` env var and target path with
> `SKILLVAULT_INSTALL_PATH`. See `skillvault update --help` for details.

---

## Quickstart

```bash
# Create a project
skillvault add-project --name "MyApp" --description "My application"

# Save a reusable skill
skillvault add-entry \
  --title "Clean Architecture Review" \
  --type skill \
  --purpose KNOWLEDGE \
  --summary "Checklist for reviewing clean architecture compliance" \
  --project myapp \
  --tags "architecture,review"

# Search your vault
skillvault search "architecture"

# Filter by LifeOS-style purpose
skillvault search "architecture" --purpose KNOWLEDGE

# Save a long AI output as an artifact (stored on disk, indexed in DB)
skillvault save-artifact \
  --title "PDF Analysis - Security Audit" \
  --type pdf_analysis \
  --content "$(cat /tmp/long-analysis.md)" \
  --project myapp \
  --tags "security,audit"

# Get compact context for your agent
skillvault get-context --mode planning --project myapp --max-chars 5000

# Wrap up a session with decisions
skillvault session-wrap \
  --project myapp \
  --summary "Reviewed auth middleware" \
  --decisions "Use JWT,not sessions" \
  --pending "Add refresh token rotation"
```

### Workflow bridge quick path

```bash
# Import workflow-builder YAML into SkillVault
skillvault import-workflow --file .agent/skills/research/workflow.yaml --project myapp

# Add a routing entry that maps scenarios to a workflow or skill
skillvault add-entry \
  --title "Research route" \
  --type routing \
  --purpose WORK \
  --summary "Route research scenarios" \
  --body $'research:\n  workflow: research-workflow' \
  --tags workflow-route

# Resolve what should handle a scenario
skillvault route research
skillvault route research --json

# Execute a workflow pipeline from the CLI
skillvault run research-workflow input.md --save output.md
```

---

## Architecture

### Core Principle

```
DB decides. Disk remembers. Qu@ntum delivers.
```

- **DB decides**: what exists, what type, status, relations, how to find it (SQLite + FTS5).
- **Disk remembers**: long artifacts, AI outputs, specs, reports stored as files in `objects/YYYY/MM/`.
- **Qu@ntum delivers**: compact, filtered, priority-sorted context for agents.

### Package Map

```
cmd/skillvault/
├── internal/cli/         # 25+ CLI commands (stdlib, no Cobra)
├── internal/mcp/         # 24 MCP tools over stdio JSON-RPC 2.0
├── internal/api/         # HTTP API (local only)
├── internal/app/         # Use cases: save, search, context, session, refs, memory, pipeline
├── internal/domain/      # Pure entities + validators
├── internal/db/          # SQLite + FTS5 stores + embedded migrations
├── internal/files/       # Artifact filesystem (objects/YYYY/MM/)
├── internal/context/     # Qu@ntum context compiler (7 modes)
├── internal/security/    # Secret scanner (4 regex patterns)
├── internal/vars/        # Variable detection + injection + frontmatter parser
├── internal/export/      # Import/Export with conflict resolution
└── internal/api/         # HTTP REST server
```

### Storage Layout

```
~/.skillvault/
├── vault.db              # SQLite + FTS5 (metadata, search, relations)
├── objects/              # Long artifact files
│   └── YYYY/MM/
│       └── <slug>.<ext>  # Content-addressed by SHA256
├── exports/              # JSON exports
└── cache/                # Temporary cache
```

**Rule**: Content goes to DB when small and frequently retrieved. Content goes to filesystem when long, or a final document, or an AI output worth preserving.

---

## CLI Commands

| Command | Description | Example |
|---------|-------------|---------|
| `init` | Create vault directories + DB | `skillvault init` |
| `add-entry` | Save a reusable entry | `skillvault add-entry --title "..." --type skill --summary "..."` |
| `search` | FTS5 search with filters | `skillvault search "auth" --type skill --project myapp` |
| `get` | Get entry by ID or slug | `skillvault get clean-architecture-review` |
| `save-artifact` | Save a long file-backed artifact | `skillvault save-artifact --title "..." --type pdf_analysis --file report.md` |
| `save-result` | Save an AI result as a vault entry | `skillvault save-result --name "result" --content "..."` |
| `get-context` | Compile Qu@ntum context pack | `skillvault get-context --mode planning --project myapp` |
| `add-project` | Create a project | `skillvault add-project --name "MyApp" --description "..."` |
| `list-projects` | List all projects | `skillvault list-projects` |
| `archive` | Archive an entry | `skillvault archive clean-architecture-review` |
| `add-workflow` | Create a workflow (JSON file) | `skillvault add-workflow workflow.json` |
| `import-workflow` | Import workflow-builder YAML | `skillvault import-workflow --file workflow.yaml --project myapp` |
| `render-workflow` | Render workflow as checklist | `skillvault render-workflow spec-plan-task` |
| `route` | Resolve scenario → workflow or skill | `skillvault route research --json` |
| `run` | Execute a workflow pipeline step by step | `skillvault run research-article article.md --save output.md` |
| `session-wrap` | Save session with decisions | `skillvault session-wrap --project myapp --summary "..." --decisions "d1,d2"` |
| `graph` | Visualize entry graph | `skillvault graph --entry e1 --format mermaid` |
| `ref` | Manage graph edges (alias) | `skillvault ref add e1 e2 depends_on` |
| `entry ref add/list/remove` | Manage graph edges | `skillvault entry ref add e1 e2 depends_on` |
| `entry history` | Show version history for an entry | `skillvault entry history clean-architecture-review` |
| `entry restore` | Restore an entry to a previous version | `skillvault entry restore clean-architecture-review --version 2` |
| `memory index/reindex/list-external` | Index pi-memory.md files | `skillvault memory index --path ~/memory --project myapp` |
| `export` | Export vault to JSON or skill pack (`.svpack`) | `skillvault export vault.json [--pack --author ...]` |
| `import` | Import vault from JSON or skill pack | `skillvault import vault.json [--pack --prefix ns/]` |
| `version` | Show vault version | `skillvault version` |
| `compare-entries` | Vector similarity between two entries | `skillvault compare-entries e1 e2` |
| `stats` | Show vault statistics and entry counts | `skillvault stats [--workflow-runs] [--json]` |
| `update` | Rebuild and reinstall binary from source | `skillvault update [--repo <path>] [--install-path <path>]` |
| `secrets` | Run q-secrets (optional encrypted secret manager) | `skillvault secrets init`, `skillvault secrets add pi KEY=val` |

---

## q-secrets integration (optional)

SkillVault can run [q-secrets](https://github.com/QuantumEdu/q-secrets) as a subprogram for local encrypted secret management.

### What is q-secrets?

q-secrets is a local encrypted secret manager that stores secrets in a SQLite database with keyring-backed master key encryption. It supports `init`, `add`, `get`, `run` (inject secrets into a child process), `list`, `delete`, `rotate`, `export`, and `import` commands.

### Installation

q-secrets is included as a **git submodule** in the kbs repository. After cloning kbs, initialize the submodule:

```bash
# Clone kbs with submodules
git clone --recurse-submodules https://github.com/QuantumEdu/kbs

# Or, if you already cloned without --recurse-submodules:
git submodule update --init --recursive

# Install both skillvault and q-secrets
make install-all

# Or install via kbs (recommended)
skillvault init --with-secrets

# Or install q-secrets separately
make install-q-secrets
# OR
skillvault secrets install
```

### Usage

```bash
# Initialize q-secrets (creates ~/.q-secrets/secrets.db and generates keys)
skillvault secrets init

# Add secrets to a profile
skillvault secrets add pi KEY=val GITHUB_TOKEN=ghp_...

# List profiles
skillvault secrets list

# Retrieve a secret
skillvault secrets get pi KEY

# Run a command with secrets as environment variables
skillvault secrets run pi -- opencode

# All q-secrets commands are available
skillvault secrets version
skillvault secrets rotate pi
skillvault secrets export pi
```

### Binary resolution order

The `secrets` command resolves the q-secrets binary by checking, in order:

1. `Q_SECRETS_BIN` environment variable (full path override)
2. Same directory as the `skillvault` executable
3. `q-secrets/q-secrets` relative to the kbs repository root (dev mode)
4. `q-secrets` in `PATH`

If the binary is not found, run `make install-q-secrets` or set `Q_SECRETS_BIN`.

### Updates

`skillvault update` will automatically rebuild and reinstall q-secrets if the
`q-secrets/` directory exists in the kbs repository. Set `SKIP_Q_SECRETS=1` to
skip the q-secrets rebuild.

---

## Agent Telemetry System

Local-first observability for CLI LLM coding agents. Collects, redacts, and persists
tool calls, token usage, quality signals, and agent lifecycle events to SQLite.

### Components

- **`telemetryd`** — Unix socket daemon that validates, redacts, hashes, and stores events
- **`telemetryctl`** — CLI to query runs, events, and daemon status
- **`telemetrywrap`** — CLI wrapper that infers telemetry events from any command
- **OpenCode plugin** — Native `EventEmitter` emitting 9 canonical event types

### Quick Start

```bash
# Build the binaries
go build -o ~/tools/telemetryd ./cmd/telemetryd/
go build -o ~/tools/telemetryctl ./cmd/telemetryctl/

# Start the daemon
telemetryd &

# Check daemon status
telemetryctl status

# List runs
telemetryctl run list
telemetryctl run recent
```

### Security

- SHA-256 arg hashing with 32-byte random salt (`~/.telemetry/salt`, 0600 perms)
- Built-in regex redaction: OpenAI keys, Bearer tokens, auth headers, `--api-key` flags
- Base64 entropy scanning → `scanned-warning` flag
- Zero plaintext secrets in logs, DB, or CLI output

For full documentation, see [`docs/telemetry.md`](docs/telemetry.md).

---

## MCP Tools (24)

For AI agents (Claude Code, OpenCode, etc.):

| Tool | Description |
|------|-------------|
| `save_entry` | Save a prompt, skill, decision, feedback, session — anything reusable |
| `search_entries` | FTS5 search with filters by type, project, tags, status, purpose |
| `get_entry` | Retrieve entry by ID with artifact reference |
| `save_artifact` | Save long AI output as file-backed artifact with metadata |
| `save_result` | Save an AI prompt result as a vault entry |
| `get_context` | Compile agent-ready context pack (7 modes) |
| `compose_series` | Get ordered entries in a series |
| `render_workflow` | Get workflow steps as ordered checklist |
| `session_wrap` | Create session entry with decisions, pending items, learnings |
| `archive_entry` | Set entry status to archived |
| `list_projects` | List all projects with status |
| `save_entry_ref` | Create/update a graph edge between two entries (with cycle detection) |
| `list_entry_refs` | List graph edges with filters |
| `get_entry_graph` | Traverse entry graph from a starting entry |
| `compare_entries` | Compare semantic/vector similarity between entries |
| `search_by_tags` | Search entries by tag intersection (all) or union (any) |
| `get_context_bundle` | Get structured project context bundle with entries grouped by type |
| `run_workflow` | Run a workflow with structured step inputs and JSON results |
| `route_scenario` | Resolve a scenario to a workflow or skill route |
| `get_stats` | Return vault statistics including workflow run analytics |
| `list_workflow_runs` | List workflow runs with optional workflow filter and step progress |
| `get_run` | Get a single workflow run with step details |
| `list_entry_versions` | List version history for an entry (descending by version number) |
| `restore_entry_version` | Restore an entry to a previous version by version number |

### MCP Setup (Claude Code / OpenCode)

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "skillvault": {
      "command": "/path/to/skillvault",
      "args": ["mcp"]
    }
  }
}
```

Create a symlink for agent-only access:

```bash
ln -sf ~/tools/skillvault ~/tools/mcp
# Now agents can call "mcp" directly
```

---

## Entity Model

### Entry Types (12)

| Type | Purpose |
|------|---------|
| `prompt` | Reusable prompt template |
| `skill` | Structured skill/pattern |
| `workflow_note` | Note about a workflow |
| `reference` | Reference document |
| `user` | User preference |
| `feedback` | User feedback/decision |
| `project_state` | Project snapshot |
| `session` | Session summary |
| `decision` | Architectual decision |
| `artifact_summary` | Summary of a stored artifact |
| `handoff` | Session handoff document |
| `routing` | Scenario → workflow/skill routing rules |

### Purpose Taxonomy

Purpose is orthogonal to entry type. Use it to organize memory by why it exists, not just what shape it has.

| Purpose | Use for |
|---------|---------|
| `WORK` | Active projects, workflows, tasks, deliverables |
| `KNOWLEDGE` | Concepts, references, reusable technical facts |
| `LEARNING` | Lessons, skill development, retrospectives |
| `RELATIONSHIP` | People, organizations, stakeholder context |
| `STATE` | Current state snapshots, project status, handoffs |
| `OBSERVABILITY` | Logs, metrics, monitoring dashboards, workflow analytics |

Examples:

```bash
skillvault add-entry --title "ISO checklist" --type reference --purpose KNOWLEDGE --summary "..."
skillvault search "ISO" --purpose KNOWLEDGE
```

### Status Model

| Status | Meaning |
|--------|---------|
| `draft` | Not ready for agent use |
| `active` | Available for normal retrieval |
| `archived` | Searchable but excluded from context packs |
| `deprecated` | Kept for history, not recommended |
| `canonical` | Preferred version |

### Relations (EntryLink — 11 types)

Entries can be explicitly related via a directed graph with cycle detection:

```
references   → links to supporting content
supersedes   → newer version replaces older (cycle-prone)
related_to   → loosely connected
part_of      → compositional relationship (cycle-prone)
derived_from → source → derived relationship
implements   → implements a spec/decision
uses         → source invokes target (agent uses workflow)
extends      → source specializes target
handoff_of   → handoff entry refers to session work
generated_from → entry derives from another
depends_on   → source depends on target (cycle-prone)
```

**Cycle detection**: `depends_on`, `part_of`, and `supersedes` validate against
transitive cycles using `WITH RECURSIVE` CTE before insertion. Max depth: 10.

**Traversal**: `GetEntryGraph` supports multi-direction (`outgoing|incoming|both`)
traversal with configurable depth. CLI: `skillvault graph --entry <id>`.

**Memory index**: Shadow entries from pi-memory-md are automatically linked via
`external_ref`. Wikilinks (`[[target]]`) in markdown bodies become `related_to` edges.

---

## Security

SkillVault detects and blocks secrets before they enter the vault:

| Pattern | Example |
|---------|---------|
| OpenAI keys | `sk-...` (20+ chars) |
| Private keys | `-----BEGIN PRIVATE KEY-----` |
| GitHub tokens | `ghp_...` |
| Slack tokens | `xoxb-...`, `xoxa-...` |

On detection: content is **rejected** or **redacted** with a warning. No secrets stored.

---

## User Flows

### 1. Save a PDF analysis from an AI agent

```
User → "Guarda este análisis en SkillVault como artefacto del proyecto Forense Digital"
Agent → calls save_artifact(title, type=pdf_analysis, content, project)
SkillVault → stores file in objects/2026/06/forense-analisis.md
           → creates artifact metadata + entry in DB
           → indexes summary + tags in FTS5
```

### 2. Agent retrieves planning context

```
Agent → calls get_context(project="MyApp", mode="planning", max_chars=8000)
SkillVault → compiles: user preferences + project state + decisions + workflows
           → truncates lowest-priority sections to fit 8000 chars
           → returns structured text for agent injection
```

### 3. Session wrap for project continuity

```
User → "Cierra sesión, guarda lo que decidimos"
Agent → calls session_wrap(project, summary, decisions[...], pending[...], learnings[...])
SkillVault → creates session entry + optional artifact
           → links to project
           → available for next get_context call
```

---

## Comparison

| Feature | File system | GitHub Gists | Obsidian | SkillVault |
|---------|-------------|-------------|----------|------------|
| Agent-facing MCP | ❌ | ❌ | ❌ | ✅ |
| FTS5 search | ❌ | ❌ | ❌ | ✅ |
| Context compilation | ❌ | ❌ | ❌ | ✅ (Qu@ntum) |
| Hybrid DB+disk | ❌ | ❌ | ❌ | ✅ |
| Secret detection | ❌ | ❌ | ❌ | ✅ |
| Workflow checklists | ❌ | ❌ | ❌ | ✅ |
| Workflow pipelines | ❌ | ❌ | ❌ | ✅ (v3) |
| Single 10MB binary | ❌ | ❌ | ❌ | ✅ |
| Zero cloud dependency | ✅ | ❌ | ❌ | ✅ |
| Portable (any OS) | ✅ | ❌ | ❌ | ✅ |
| Entry relations | ❌ | ❌ | ❌ | ✅ |

---

## Testing

```bash
# Run all 750+ tests
go test ./...

# With coverage
go test -cover ./...

# Integration tests use in-memory SQLite (no filesystem needed)
```

Test pyramid:
- **Unit tests**: domain validation, security patterns, variable detection
- **Integration tests**: SQLite stores, artifact filesystem, MCP tools
- **Acceptance tests**: full end-to-end flows (AC1-AC10)

---

## Dependencies

- **Go 1.26+** — standard library for CLI, HTTP, JSON, file I/O, crypto, embed
- **`modernc.org/sqlite`** — pure Go SQLite driver (no CGO)
- **`telemetryd`** — Unix socket daemon for agent observability (included in repo)
- **`telemetryctl`** — CLI for run queries and daemon status (included in repo)

**Zero frameworks.** No Cobra, no Fiber, no ORM, no Gin, no Echo.

---

## Project Status

| Phase | Status |
|-------|--------|
| v1-alpha (SQLite vault) | ✅ Archived |
| v2 Qu@ntum (Hybrid + Context) | ✅ Archived |
| v3 Qu@ntum (Workflow Pipelines) | ✅ Active |
| v3 Service hardening (auth, shutdown, MCP tools) | ✅ Active |
| Cloud sync (S3 + GitHub transports) | ✅ Active |
| TUI (Bubble Tea, build-tag gated) | ✅ Active |
| Vector search (GloVe, pure Go) | ✅ Active |
| Entry diff / compare-entries | ✅ Active |
| Entry versioning (history, restore, diff API) | ✅ Active |
| Entry skill pack export | ✅ Active |
| HTTP auth layer | ✅ Active |
| Graceful shutdown | ✅ Active |
| save_result MCP tool | ✅ Active |
| Workflow-builder YAML import | ✅ Active |
| Scenario routing (`route`, `route_scenario`) | ✅ Active |
| LifeOS purpose taxonomy | ✅ Active |
| Structured MCP workflow runs (`run_workflow`) | ✅ Active |
| Workflow run analytics (`get_stats`, `list_workflow_runs`, `get_run`) | ✅ Active |
| Agent telemetry daemon (`telemetryd`) | ✅ Active |
| Agent telemetry CLI (`telemetryctl`) | ✅ Active |
| Agent telemetry plugins + wrapper | ✅ Active |
| Quality signal detectors (loop, stall, streak, token) | ✅ Active |

---

## License

MIT
