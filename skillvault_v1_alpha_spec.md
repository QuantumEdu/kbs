# SkillVault v1-alpha — Specification

**Codename:** SkillVault-Hermes  
**Product name:** SkillVault  
**Version:** v1-alpha  
**Spec status:** Contract draft ready for agent planning/tasks  
**Primary user:** local developer / power user working with multiple coding agents  
**Main purpose:** local knowledge retrieval, prompt/skill/workflow vault, and agent context delivery layer.

---

## 0. Executive Summary

SkillVault is a local-first vault for organizing prompts, reusable skills, workflows, decisions, project memory, session summaries, and long AI-generated outputs. It exposes a CLI for the user and a small MCP interface for agents.

The system uses a hybrid storage model:

- **SQLite + FTS5** stores metadata, summaries, tags, relations, status, projects, workflows, and searchable indexes.
- **Filesystem objects** store long Markdown/JSON/TXT artifacts such as PDF analyses, generated specs, long AI responses, reports, session outputs, or reusable documents.
- **Hermes context layer** compiles compact context packs for agents through `get_context`, so agents retrieve only what they need instead of loading the whole vault.

SkillVault began as a way to organize prompts, skills, and workflows without overloading agents. In this v1-alpha it expands into a portable agent memory and knowledge retrieval layer, while remaining simple, local, inspectable, and non-cloud by default.

---

## 1. Problem Statement

Working with coding agents creates repeated friction:

1. Prompts and skills become scattered across chats, files, tools, and GitHub repositories.
2. Agents get overloaded if every skill is installed globally.
3. Important AI outputs are often long and valuable, but saving them directly into prompts, chats, or memory pollutes future context.
4. Project decisions are forgotten across sessions.
5. Agent workflows can miss steps when there is no reusable checklist or workflow object.
6. Context is usually either too little or too much.
7. Memory inside one vendor or one chat is not portable.

SkillVault solves this by becoming a local source of truth for skills, prompts, workflows, memory, knowledge, and artifacts, with retrieval designed for both humans and agents.

---

## 2. Product Goals

### G1. Organize reusable AI operating assets
Store and search prompts, skills, workflows, references, decisions, and project state.

### G2. Avoid overloading agents
Agents should not need all skills installed at once. They can search SkillVault and retrieve only the needed skill, workflow, or context.

### G3. Preserve long AI outputs safely
Long outputs, PDF analyses, reports, specs, dictamen-style content, and generated documents should be stored as files in the vault, while the DB stores metadata and summaries.

### G4. Provide compact agent context
`get_context` must compile a concise context pack from DB metadata, selected memories, decisions, workflows, and artifacts.

### G5. Support workflows
Users should be able to define workflow steps so agents do not omit important actions.

### G6. Support project continuity
Projects must have active/archived status, decisions, sessions, entries, and artifacts.

### G7. Remain local-first and simple
No cloud sync, no daemon, no vector DB, no GUI, no TUI, and no multi-user auth in v1-alpha.

---

## 3. Non-Goals for v1-alpha

The following are explicitly out of scope:

- Cloud synchronization.
- Obsidian integration.
- Raycast integration.
- TUI.
- Web UI.
- Vector database.
- Embeddings.
- Background daemon.
- Multi-user support.
- Remote API with auth.
- Automatic execution of workflows.
- Agent marketplace.
- Automatic capture of every chat response.
- Full AGENTS.md hook implementation for every agent.
- PDF parsing engine. The system stores analysis outputs and artifact references, but does not need to parse PDFs itself in v1-alpha.

---

## 4. Core Concept

SkillVault has three conceptual layers:

```text
SkillVault Core       = stores and indexes entries, projects, workflows, artifacts.
Hermes Context Layer  = compiles and delivers context packs to agents.
Interfaces            = CLI for user, MCP tools for agents.
```

### Guiding rule

```text
DB decides. Disk remembers. Hermes delivers.
```

- **DB decides:** what exists, what type it is, whether it is active, how it relates, how it should be found.
- **Disk remembers:** long artifacts, generated outputs, specs, analyses, reports.
- **Hermes delivers:** compact, filtered, agent-ready context.

---

## 5. Storage Strategy

### 5.1 Hybrid Storage

SkillVault MUST use a hybrid model:

```text
~/.skillvault/
├── vault.db
├── objects/
│   └── YYYY/MM/<artifact-slug>.<md|json|txt>
├── exports/
├── snapshots/
└── cache/
```

### 5.2 SQLite responsibilities

SQLite stores:

- IDs.
- Titles.
- Types.
- Status.
- Summaries.
- Tags.
- Project links.
- Series links.
- Workflow metadata.
- Artifact references.
- Content hashes.
- Created/updated timestamps.
- Search indexes through FTS5.

### 5.3 Filesystem responsibilities

Filesystem stores:

- Long AI outputs.
- PDF analysis results.
- Specs.
- Reports.
- Session transcripts or extended summaries.
- Large prompts or generated artifacts.
- JSON exports.
- Markdown documents.

### 5.4 Rule for DB vs file

Content MAY be stored directly in DB only when it is small and frequently retrieved.

Content MUST be stored as an artifact file when:

- It is long.
- It is a final document.
- It is an AI output worth preserving.
- It is a PDF analysis.
- It is a generated spec/report.
- It may overload context if retrieved by default.

---

## 6. Entity Model

### 6.1 Entry

The core reusable unit.

```text
Entry
- id
- title
- slug
- type
- summary
- body_optional
- status
- project_id optional
- artifact_id optional
- created_at
- updated_at
```

#### Entry types

Required v1-alpha types:

```text
prompt
skill
workflow_note
reference
user
feedback
project_state
session
decision
artifact_summary
```

Optional future types:

```text
playbook
pattern
checklist
spec
plan
task
external_pointer
```

### 6.2 Project

```text
Project
- id
- name
- slug
- description
- status: active | archived
- created_at
- updated_at
```

Projects group entries, decisions, sessions, workflows, and artifacts.

### 6.3 Artifact

```text
Artifact
- id
- title
- slug
- type
- file_path
- mime_type
- summary
- content_hash
- size_bytes
- project_id optional
- source_entry_id optional
- created_at
- updated_at
```

Artifact types:

```text
markdown
json
txt
html
pdf_reference
ai_output
pdf_analysis
spec
report
session_output
```

### 6.4 Workflow

```text
Workflow
- id
- name
- slug
- description
- status: active | archived | draft
- created_at
- updated_at
```

### 6.5 WorkflowStep

```text
WorkflowStep
- id
- workflow_id
- order_index
- title
- instruction
- required: boolean
- expected_output optional
```

### 6.6 Series

A series groups ordered entries, for example a learning path, reusable prompt chain, or architecture checklist.

```text
Series
- id
- name
- slug
- description
- status
```

```text
SeriesEntry
- series_id
- entry_id
- order_index
```

### 6.7 Tag

Tags allow retrieval by topic.

```text
Tag
- id
- name
- slug
```

### 6.8 EntryLink

Explicit relationship between entries.

```text
EntryLink
- from_entry_id
- to_entry_id
- relation_type
```

Relation types:

```text
references
supersedes
related_to
part_of
derived_from
implements
```

---

## 7. Status Model

Entries, projects, workflows, and series MUST support status.

Required statuses:

```text
draft
active
archived
deprecated
canonical
```

Meaning:

- **draft:** not ready for agent use.
- **active:** available for normal retrieval and context.
- **archived:** searchable but excluded from normal context packs.
- **deprecated:** kept for history, not recommended.
- **canonical:** preferred version.

`get_context` MUST exclude archived/deprecated content by default unless explicitly requested.

---

## 8. Hermes Context Layer

### 8.1 Purpose

Hermes compiles relevant context for agents without overloading them.

It is not a chat memory dump. It is a filtered, compact, structured context pack.

### 8.2 `get_context` modes

Required modes:

```text
profile
project
workflow
skill
planning
session_recall
full_brief
```

### 8.3 Context pack structure

`get_context` MUST return structured text suitable for agent injection.

Example format:

```text
# CONTEXT PACK

## Scope
Project: SkillVault
Mode: planning

## User Preferences
- Prefer practical architecture, not overengineering.
- Use spec -> plan -> tasks when building software.

## Project State
- SkillVault stores prompts, skills, workflows, memory, and artifacts.
- SQLite indexes metadata; disk stores long artifacts.

## Active Decisions
- Use DB + filesystem hybrid storage.
- Use get_context as the agent-facing retrieval layer.
- No vector DB in v1-alpha.

## Relevant Workflows
- spec -> plan -> tasks
- session_wrap -> decisions -> pending -> next context

## Suggested Next Action
Generate implementation plan and tasks from this spec.
```

### 8.4 Context constraints

`get_context` MUST support a max token or max character limit.

Required input fields:

```json
{
  "mode": "project",
  "project": "SkillVault",
  "query": "optional search text",
  "include": ["profile", "decisions", "workflows", "recent_sessions"],
  "exclude_archived": true,
  "max_chars": 12000
}
```

### 8.5 Context priority order

When compiling context, use this priority:

1. User feedback/preferences.
2. Active project state.
3. Canonical decisions.
4. Relevant workflow.
5. Recent sessions.
6. Artifact summaries.
7. References.
8. Archived index lines only when requested.

---

## 9. CLI Requirements

Binary name:

```text
skillvault
```

### 9.1 Required commands

```bash
skillvault init
skillvault add-entry
skillvault search
skillvault get
skillvault save-artifact
skillvault get-context
skillvault add-project
skillvault list-projects
skillvault archive
skillvault add-workflow
skillvault render-workflow
skillvault session-wrap
skillvault export
skillvault import
```

### 9.2 Command behavior

#### `skillvault init`
Creates:

```text
~/.skillvault/vault.db
~/.skillvault/objects/
~/.skillvault/exports/
~/.skillvault/cache/
```

#### `skillvault add-entry`
Adds a small or medium entry.

Required options:

```bash
--title
--type
--summary
```

Optional:

```bash
--body
--project
--tags
--status
```

#### `skillvault save-artifact`
Saves a long file-backed artifact and creates DB metadata.

Required options:

```bash
--title
--type
--file
```

Optional:

```bash
--project
--summary
--tags
--source
```

#### `skillvault get-context`
Compiles a context pack.

Examples:

```bash
skillvault get-context --mode profile
skillvault get-context --project skillvault --mode planning
skillvault get-context --workflow spec-plan-task
```

#### `skillvault session-wrap`
Creates a session entry and optionally artifact.

Required output:

- Decisions.
- Pending items.
- Useful context for continuation.
- Linked project.

#### `skillvault archive`
Changes status to archived.

Must not delete data.

#### `skillvault export`
Exports DB data and optional artifacts manifest.

#### `skillvault import`
Imports exported data.

---

## 10. MCP Tool Requirements

v1-alpha must expose a small MCP surface. Do not exceed what agents need.

### Required MCP tools

```text
save_entry
search_entries
get_entry
save_artifact
get_context
compose_series
render_workflow
session_wrap
archive_entry
list_projects
```

### 10.1 `save_entry`

Purpose: save prompt, skill, decision, feedback, reference, project state, or session summary.

Must reject obvious secrets.

Input:

```json
{
  "title": "string",
  "type": "prompt|skill|reference|user|feedback|project_state|session|decision|artifact_summary",
  "summary": "string",
  "body": "string optional",
  "project": "string optional",
  "tags": ["string"],
  "status": "draft|active|archived|deprecated|canonical"
}
```

### 10.2 `search_entries`

Purpose: search DB using FTS5 and filters.

Input:

```json
{
  "query": "string",
  "type": "string optional",
  "project": "string optional",
  "tags": ["string"],
  "include_archived": false,
  "limit": 10
}
```

### 10.3 `get_entry`

Purpose: retrieve one entry by ID or slug.

Must include artifact reference if linked.

### 10.4 `save_artifact`

Purpose: save long AI output or file-backed artifact.

Input:

```json
{
  "title": "string",
  "type": "ai_output|pdf_analysis|spec|report|session_output|markdown|json|txt",
  "content": "string optional",
  "file_path": "string optional",
  "summary": "string",
  "project": "string optional",
  "tags": ["string"]
}
```

At least one of `content` or `file_path` must be provided.

### 10.5 `get_context`

Purpose: compile agent-ready context.

Input:

```json
{
  "mode": "profile|project|workflow|skill|planning|session_recall|full_brief",
  "project": "string optional",
  "query": "string optional",
  "workflow": "string optional",
  "include": ["profile", "decisions", "workflows", "recent_sessions", "artifact_summaries", "references"],
  "exclude_archived": true,
  "max_chars": 12000
}
```

### 10.6 `compose_series`

Purpose: return ordered entries in a series.

### 10.7 `render_workflow`

Purpose: return workflow steps as agent instructions/checklist.

### 10.8 `session_wrap`

Purpose: save a compact session summary.

Input:

```json
{
  "project": "string optional",
  "summary": "string",
  "decisions": ["string"],
  "pending": ["string"],
  "learnings": ["string"],
  "artifacts": ["artifact id optional"]
}
```

### 10.9 `archive_entry`

Purpose: set entry status to archived.

### 10.10 `list_projects`

Purpose: list projects and statuses.

---

## 11. Natural Language Save Policy

The system should not automatically save every AI output.

The user or agent should explicitly indicate persistence using commands such as:

```text
Guarda esto en SkillVault.
Vault it.
Archiva esta respuesta como artefacto del proyecto X.
Guarda este análisis de PDF como output largo con resumen y tags.
Cierra sesión y guarda decisiones, pendientes y contexto para continuar.
```

### Save decision rule

```text
Small reusable knowledge -> entry.
Long output/final document/PDF analysis/spec/report -> artifact file + DB metadata.
Temporary content -> cache or do not save.
```

---

## 12. Security Requirements

### 12.1 Secret detection

The system MUST reject saving content that appears to contain obvious secrets.

Minimum patterns:

```regex
sk-[A-Za-z0-9_-]{20,}
-----BEGIN (RSA |EC |OPENSSH |)?PRIVATE KEY-----
ghp_[A-Za-z0-9_]{20,}
xox[baprs]-[A-Za-z0-9-]{20,}
```

### 12.2 Redaction behavior

When secret-like content is detected:

- Do not save the secret value.
- Return a warning.
- Allow saving a redacted note if useful.

### 12.3 Local-first

No network calls in v1-alpha unless explicitly added by the user outside core.

### 12.4 Destructive operations

Archive is preferred over delete.

Hard delete is out of scope for v1-alpha unless implemented behind explicit confirmation.

---

## 13. Import / Export

### 13.1 Export

Must export:

- Projects.
- Entries.
- Workflows.
- Workflow steps.
- Series.
- Tags.
- Artifact metadata.
- Artifact manifest.

Artifact files may be optionally copied into export bundle in a later version. v1-alpha may export paths and hashes only.

### 13.2 Import

Must import valid SkillVault JSON.

On duplicate slug:

- Do not overwrite silently.
- Create conflict suffix or report conflict.

---

## 14. Search Requirements

Search must use SQLite FTS5 for entry body, summary, title, and artifact summaries.

Required filters:

- type.
- project.
- tag.
- status.
- include archived.
- limit.

Search result must return:

```text
id
title
type
summary
project
status
tags
artifact_ref optional
```

---

## 15. Workflow Requirements

Workflows are not executable automation in v1-alpha.

They are renderable instruction checklists for humans and agents.

Example workflow: `spec-plan-task`

```text
1. Read source spec.
2. Identify scope and non-scope.
3. Confirm entities and interfaces.
4. Generate implementation plan.
5. Generate tasks.
6. Validate tasks against acceptance criteria.
```

`render_workflow` must output clear, ordered steps.

---

## 16. Architecture Requirements

Language target: Go is recommended based on the prior project direction.

Architecture style:

- Clean Architecture basic.
- Hexagonal architecture light.
- SOLID practical, not ceremonial.
- TDD for core domain/application behavior.

Recommended structure:

```text
cmd/skillvault/
internal/cli/
internal/mcp/
internal/app/
internal/domain/
internal/db/
internal/files/
internal/search/
internal/context/
internal/security/
internal/export/
```

### Layer responsibilities

#### `domain`
Pure entities and rules.

No SQLite, no filesystem, no CLI.

#### `app`
Use cases.

Examples:

```text
SaveEntry
SearchEntries
GetEntry
SaveArtifact
GetContext
RenderWorkflow
SessionWrap
ArchiveEntry
ExportVault
ImportVault
```

#### `db`
SQLite repositories and migrations.

#### `files`
Filesystem artifact writer/reader.

#### `search`
FTS5 search abstraction.

#### `context`
Hermes context compiler.

#### `security`
Secret scanner/redactor.

#### `cli`
Command interface.

#### `mcp`
Agent-facing tools.

---

## 17. Design Patterns

Use only minimal necessary patterns:

1. **Repository pattern** for persistence boundaries.
2. **Use case / service pattern** for application operations.
3. **Strategy pattern** only if context compilation modes become complex.
4. **Adapter pattern** for CLI and MCP interfaces.

Avoid:

- Excessive factories.
- Abstract interfaces for everything.
- Event sourcing.
- CQRS.
- Plugin systems.
- Premature vector retrieval.

---

## 18. TDD Requirements

Core behavior must be test-first or test-covered.

### Required test groups

#### Domain tests

- Entry type validation.
- Status validation.
- Artifact classification.
- Project archive behavior.

#### Security tests

- Reject API key pattern.
- Reject private key pattern.
- Allow safe content.

#### Storage tests

- Save entry.
- Save artifact metadata.
- Link entry to artifact.
- Archive entry.

#### Search tests

- Search by keyword.
- Filter by type.
- Filter by project.
- Exclude archived by default.

#### Context tests

- `get_context(profile)` includes user and feedback entries.
- `get_context(project)` includes active project state and decisions.
- Archived entries excluded by default.
- Max size limit respected.

#### Import/export tests

- Export produces valid JSON.
- Import restores entries/projects/workflows.
- Duplicate slugs handled safely.

#### Workflow tests

- Render ordered workflow steps.
- Compose series in order.

---

## 19. Acceptance Criteria

### AC1. Initialize vault
Given no existing vault, when user runs `skillvault init`, then required folders and SQLite database are created.

### AC2. Save and search entry
Given an entry is saved, when searching by title/body/tag, then it is returned with metadata.

### AC3. Save long artifact
Given a long PDF analysis is saved, then content is stored as a file and DB stores metadata, summary, hash, and file path.

### AC4. Context generation
Given profile, feedback, project decisions, and workflow entries exist, when `get_context --project X --mode planning` is called, then a compact context pack is returned.

### AC5. Archived content behavior
Given an entry is archived, normal search/context excludes it unless `include_archived` is true.

### AC6. Secret protection
Given content includes a secret-like pattern, saving is rejected or redacted.

### AC7. Workflow rendering
Given a workflow has steps, `render_workflow` returns ordered checklist instructions.

### AC8. Session wrap
Given a session summary with decisions and pending items, `session_wrap` creates a session entry linked to the project.

### AC9. Import/export
Given a vault has entries/projects/workflows, export and import preserve them.

### AC10. MCP agent use
Given an agent calls MCP `get_context`, it receives the same context pack as CLI `get-context`.

---

## 20. MVP Implementation Sequence

Recommended build sequence:

1. Project skeleton.
2. Domain entities.
3. SQLite migrations.
4. Repository implementations.
5. Secret scanner.
6. Save/search/get entry use cases.
7. Artifact file storage.
8. Project support.
9. Workflow support.
10. Hermes `get_context` compiler.
11. CLI commands.
12. MCP tools.
13. Import/export.
14. Tests and acceptance verification.

---

## 21. Migration from Original SkillVault Concept

Original concept:

```text
Organize prompts, skills, and workflows for personal use.
```

v1-alpha evolved concept:

```text
Organize prompts, skills, workflows, memory, decisions, project state, sessions, and long AI outputs for both user and agents.
```

Migration principles:

- Existing skills/prompts become `entries`.
- Existing workflows become `workflows` + `workflow_steps`.
- Long generated results become `artifacts`.
- Project decisions become `decision` entries.
- Personal preferences become `feedback` entries.
- Project memory becomes `project_state` entries.
- Agent context is delivered through `get_context`.

---

## 22. Compatibility with Agentic Memory Pattern

SkillVault v1-alpha covers the essential agentic memory pattern:

| Agentic memory concept | SkillVault equivalent |
|---|---|
| always-on memory | `get_context(profile)` |
| project memory | `project_state`, `decision`, `session` entries |
| semantic knowledge | `reference` entries and artifact summaries |
| long outputs | file-backed artifacts |
| write-note skill | `save_entry` and `save_artifact` |
| session wrap | `session_wrap` |
| context injection | MCP/CLI `get_context` |
| archived projects | status model |
| grep/search | SQLite FTS5 |

SkillVault should not copy a separate `~/.aios` structure as the source of truth. It may optionally export compatible Markdown later.

---

## 23. Example User Workflows

### 23.1 Save a PDF analysis

User says:

```text
Guarda este análisis de PDF en SkillVault como artefacto del proyecto Forense Digital.
```

Expected system action:

1. Store long analysis as Markdown file under `objects/YYYY/MM/`.
2. Create artifact metadata.
3. Create or update an `artifact_summary` entry.
4. Link to project.
5. Add tags.

### 23.2 Agent retrieves planning context

Agent calls:

```text
get_context(project="SkillVault", mode="planning")
```

Expected output:

- User preferences.
- Project scope.
- Current decisions.
- Relevant workflows.
- Recent session summaries.
- Next action suggestion.

### 23.3 User retrieves a skill only when needed

User asks:

```text
Busca una skill para revisar arquitectura limpia ligera.
```

Expected system action:

1. Search entries type `skill`.
2. Return matches.
3. User or agent selects one.
4. Agent applies selected skill without installing all skills globally.

---

## 24. Agent Instructions for Plan/Tasks Generation

When an implementation agent receives this spec, it must:

1. Treat this document as the product contract.
2. Generate a plan before tasks.
3. Keep v1-alpha scope strict.
4. Do not add cloud, GUI, embeddings, daemon, or sync.
5. Use TDD for domain, app, search, security, context, and import/export.
6. Prefer simple Go implementation with SQLite FTS5.
7. Implement CLI first, MCP second.
8. Keep artifacts on disk and metadata in DB.
9. Ensure `get_context` is central and tested.
10. Avoid overengineering.

---

## 25. Suggested Plan Prompt

Use this prompt with an agent after loading the spec:

```text
Read the SkillVault v1-alpha spec as the product contract.
Generate an implementation plan only. Do not write code yet.
Keep the scope limited to v1-alpha. Use Go, SQLite FTS5, CLI-first, MCP-second, TDD, clean architecture basic, and hybrid DB/filesystem storage.
Explicitly include milestones, architecture, data model, test strategy, and risks.
Do not add cloud sync, GUI, TUI, vector DB, embeddings, daemon, or multi-user features.
```

---

## 26. Suggested Tasks Prompt

Use this prompt after the agent generates and you approve the plan:

```text
Using the approved SkillVault v1-alpha implementation plan, generate implementation tasks.
Tasks must be small, ordered, testable, and traceable to the spec acceptance criteria.
Include TDD tasks before implementation tasks where appropriate.
Do not implement features outside v1-alpha.
```

---

## 27. Final Contract

SkillVault v1-alpha is not a full agentic operating system. It is a local-first knowledge retrieval and context vault for agents.

It succeeds if:

1. The user can store and find prompts, skills, workflows, decisions, sessions, and artifacts.
2. Long AI outputs are preserved without polluting the DB or default context.
3. Agents can request compact context through `get_context`.
4. Workflows can be rendered so agents do not omit steps.
5. The system remains simple, portable, testable, and local-first.

