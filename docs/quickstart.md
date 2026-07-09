# Quickstart — 5 minutes

## Installation

```bash
# Prerequisites: Go 1.26+
git clone https://github.com/QuantumEdu/kbs
cd kbs

# Build (single binary, no CGO, ~7 MB)
go build -ldflags="-s -w" -o ~/tools/skillvault ./cmd/skillvault

# Add ~/tools to your PATH if it's not there
export PATH="$HOME/tools:$PATH"
```

## Initialize the vault

```bash
skillvault init
```

Creates `~/.skillvault/` with:

```
~/.skillvault/
├── vault.db       # SQLite + FTS5
├── objects/       # Long artifacts (files)
├── exports/       # JSON backups
└── cache/         # Temporary cache
```

## First steps

### 1. Create a project

```bash
skillvault add-project \
  --name "MyApp" \
  --description "My first SkillVault app"
```

### 2. Save a reusable skill

```bash
skillvault add-entry \
  --title "Code Review Checklist" \
  --type skill \
  --purpose KNOWLEDGE \
  --summary "Checklist for reviewing PRs" \
  --content "1. Does it work?\n2. Does it have tests?\n3. Does it handle errors?" \
  --project myapp \
  --tags "review,pr"
```

### 3. Search

```bash
skillvault search "review" --type skill --project myapp
```

Filter by LifeOS-aligned purpose:

```bash
skillvault search "review" --purpose KNOWLEDGE
```

### 4. Save a large artifact (filesystem-backed)

```bash
skillvault save-artifact \
  --title "Security Analysis" \
  --type pdf_analysis \
  --content "$(cat /tmp/security-report.md)" \
  --project myapp \
  --tags "security"
```

### 5. Get context for your agent

```bash
skillvault get-context \
  --mode planning \
  --project myapp \
  --max-chars 5000
```

Returns structured text like:

```
## Scope
Project: MyApp
Mode: planning

## Active Decisions
...

## Suggested Next Action
...
```

### 6. Wrap up a session with decisions

```bash
skillvault session-wrap \
  --project myapp \
  --summary "Reviewed auth middleware" \
  --decisions "Use JWT,no sessions" \
  --pending "Add refresh token rotation"
```

### 7. Versioning: track and restore entry history

Every time you update an entry's content, a version snapshot is automatically saved. You can browse the history and restore any previous version.

```bash
# Save an initial entry
skillvault add-entry \
  --title "Architecture Notes" \
  --type reference \
  --purpose KNOWLEDGE \
  --summary "Initial architecture decisions" \
  --project myapp

# Later, update the entry with new decisions
skillvault add-entry \
  --title "Architecture Notes" \
  --type reference \
  --purpose KNOWLEDGE \
  --summary "Revised architecture after review" \
  --project myapp

# This updates the same entry (matched by slug). Check the version history:
skillvault entry history architecture-notes

# Output:
# Version history for entry architecture-notes:
#   #  | Title                | Saved At
#   ---+----------------------+-------------------
#   2  | Architecture Notes   | 2026-07-01
#   1  | Architecture Notes   | 2026-06-28

# Restore to version 1 if you change your mind:
skillvault entry restore architecture-notes --version 1

# The current state is preserved as a new version before restoring.
```

### 8. Execute a workflow pipeline

```bash
# Create a workflow with entry_slug in steps
skillvault add-workflow pipeline.json

# Run it: each step renders the prompt, waits for agent input
skillvault run research-article article.md --save result.md
```

Each pipeline step:
1. Takes the linked entry and injects `{{input}}` and `{{previous_output}}`
2. Prints the prompt to stdout
3. Waits for the agent to respond via stdin
4. Passes the response to the next step

## Workflow bridge

Import workflow-builder YAML directly into SkillVault:

```bash
# Import a workflow-builder YAML file
skillvault import-workflow --file .agent/skills/research/workflow.yaml --project myapp

# Add a routing entry that maps scenarios to workflows
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

# Run the workflow from the CLI
skillvault run research-workflow input.md --save output.md
```

## Purpose taxonomy

Purpose is orthogonal to entry type. Use it to organize memory by why it exists:

| Purpose | Use for |
|---------|---------|
| `WORK` | Active projects, workflows, tasks, deliverables |
| `KNOWLEDGE` | Concepts, references, reusable technical facts |
| `LEARNING` | Lessons, skill development, retrospectives |
| `RELATIONSHIP` | People, organizations, stakeholder context |
| `STATE` | Current state snapshots, project status, handoffs |
| `OBSERVABILITY` | Logs, metrics, monitoring dashboards, workflow analytics |

```bash
skillvault add-entry --title "ISO checklist" --type reference --purpose KNOWLEDGE --summary "..."
skillvault search "ISO" --purpose KNOWLEDGE
```

## What next?

- [`docs/commands.md`](commands.md) — full command reference
- [`docs/mcp.md`](mcp.md) — MCP setup for Claude Code / OpenCode
- [`docs/tutorial.md`](tutorial.md) — complete tutorial with a real workflow
