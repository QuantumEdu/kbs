# SkillVault v3 — Delta Specs

> All capabilities are **ADDED** (v2 supersedes v1-alpha with new architecture; v3 adds pipeline execution).
> Source: `skillvault_v1_alpha_spec.md` (sections 4–20, authoritative).
> v1-alpha delta specs archived at `sdd/skillvault-v1-alpha/spec` (Engram #58).
> v3 delta specs archived at `openspec/changes/archive/2026-06-26-skillvault-v3-workflow-pipelines/`.

---

## Capability 1: Hybrid Storage Model

See spec §5 (§5.1–§5.4).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-HYB-01 | Vault root at `~/.skillvault/` with subdirectories `objects/`, `exports/`, `snapshots/`, `cache/` | MUST |
| REQ-HYB-02 | SQLite (`vault.db`) stores IDs, titles, types, status, summaries, tags, project/series/workflow links, artifact references, content hashes, timestamps, and FTS5 indexes | MUST |
| REQ-HYB-03 | Filesystem under `objects/YYYY/MM/<artifact-slug>.<ext>` stores long AI outputs, PDF analyses, specs, reports, session transcripts, large prompts, JSON exports, Markdown documents | MUST |
| REQ-HYB-04 | Content stored in DB directly only when small and frequently retrieved | SHOULD |
| REQ-HYB-05 | Content stored as artifact file when: long, final document, AI output, PDF analysis, generated spec/report, or may overload context | MUST |
| REQ-HYB-06 | `objects/` directory uses year/month subdirectories for filesystem organization | MUST |
| REQ-HYB-07 | The vault MUST remain local-first by default. Cloud sync via pluggable transports is OPTIONAL and user-initiated — no daemon, background sync, or automatic network calls. | MUST |

**Scenarios**:
- GIVEN no vault exists, WHEN `skillvault init` runs, THEN `~/.skillvault/vault.db` is created alongside `objects/`, `exports/`, and `cache/` subdirectories — no network calls.
- GIVEN running vault, WHEN no `sync` command issued, THEN zero network calls occur.
- GIVEN a long PDF analysis is saved, WHEN `save_artifact` is called, THEN content is stored as a file under `objects/YYYY/MM/` and DB stores metadata, summary, content hash, and file path.
- GIVEN small frequently retrieved content, WHEN `add-entry` with inline body is called, THEN body stored in DB directly without creating an artifact file.

---

## Capability 2: Entry Entity + 10 Types

See spec §6.1.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-ENT-01 | Entry fields: `id`, `title`, `slug`, `type`, `summary`, `body` (optional), `status`, `project_id` (nullable), `artifact_id` (nullable), `created_at`, `updated_at` | MUST |
| REQ-ENT-02 | Required entry types: `prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary` | MUST |
| REQ-ENT-03 | Entry type validation rejects unknown types | MUST |
| REQ-ENT-04 | Slug auto-generated from title if not provided, unique per type | MUST |
| REQ-ENT-05 | Entry may have zero or more tags via join table | MUST |
| REQ-ENT-06 | Entry may link to at most one artifact | SHOULD |

**Scenarios**:
- GIVEN valid entry data with type `decision`, WHEN `save_entry` MCP tool or `add-entry` CLI command is called, THEN entry is persisted with auto-generated slug, timestamps, and status defaulting to `draft`.
- GIVEN entry with invalid type `invalid_type`, WHEN `save_entry` is attempted, THEN entry is rejected with validation error.
- GIVEN entry with type `artifact_summary`, WHEN saved, THEN entry references an existing artifact by `artifact_id`.

---

## Capability 3: Project Entity

See spec §6.2.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-PRJ-01 | Project fields: `id`, `name`, `slug`, `description`, `status` (active | archived), `created_at`, `updated_at` | MUST |
| REQ-PRJ-02 | Projects group entries, decisions, sessions, workflows, and artifacts | MUST |
| REQ-PRJ-03 | Project status defaults to `active` on creation | MUST |
| REQ-PRJ-04 | `list-projects` CLI and `list_projects` MCP tool return all active projects; optional flag includes archived | MUST |
| REQ-PRJ-05 | Archiving a project does not cascade-archive its entries | MUST |

**Scenarios**:
- GIVEN an active project with 5 entries, WHEN `list-projects` is called, THEN project appears in output.
- GIVEN project is archived, WHEN `list-projects` is called without `--include-archived`, THEN project is excluded.
- GIVEN project is created, WHEN `add-project` CLI command is called, THEN project is persisted with slug, description, and active status.

---

## Capability 4: Artifact Entity + File-Backed Storage

See spec §6.3 and §5.3.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-ART-01 | Artifact fields: `id`, `title`, `slug`, `type`, `file_path`, `mime_type`, `summary`, `content_hash`, `size_bytes`, `project_id` (nullable), `source_entry_id` (nullable), `created_at`, `updated_at` | MUST |
| REQ-ART-02 | Artifact types: `markdown`, `json`, `txt`, `html`, `pdf_reference`, `ai_output`, `pdf_analysis`, `spec`, `report`, `session_output` | MUST |
| REQ-ART-03 | File path stored as relative path under `objects/` directory | MUST |
| REQ-ART-04 | Content hash (SHA-256) computed on save; used for deduplication and integrity | MUST |
| REQ-ART-05 | MIME type auto-detected from file extension or content | MUST |
| REQ-ART-06 | Size in bytes tracked in artifact metadata | MUST |
| REQ-ART-07 | Artifact may be linked to a source entry via `source_entry_id` | SHOULD |
| REQ-ART-08 | At least one of `content` or `file_path` must be provided when saving an artifact | MUST |

**Scenarios**:
- GIVEN AI output content with title and type `ai_output`, WHEN `save_artifact` MCP tool is called, THEN content is written to `objects/YYYY/MM/<slug>.md`, artifact metadata is stored in DB with hash, size, and path.
- GIVEN artifact is saved for project X, WHEN `search` is called with project filter X, THEN artifact metadata appears in results.
- GIVEN artifact save request without content or file_path, WHEN `save_artifact` is called, THEN request is rejected with validation error.

---

## Capability 5: Workflow + WorkflowStep Entities

See spec §6.4–§6.5.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-WKF-01 | Workflow fields: `id`, `name`, `slug`, `description`, `status` (active | archived | draft), `created_at`, `updated_at` | MUST |
| REQ-WKF-02 | WorkflowStep fields: `id`, `workflow_id`, `order_index`, `title`, `instruction`, `required` (boolean), `expected_output` (optional) | MUST |
| REQ-WKF-03 | Steps ordered by `order_index` ascending, sequential from 1 | MUST |
| REQ-WKF-04 | A workflow must have at least one step to be considered usable | SHOULD |
| REQ-WKF-05 | Workflows are primarily renderable instruction checklists. When steps have `entry_slug` set, they MAY be executed as sequential pipeline steps via `skillvault run`. | MUST |
| REQ-WKF-06 | Each `workflow_step` MAY include an `entry_slug` referencing a specific entry. When `entry_slug` is NULL, the step is renderable-only. | MUST |

**Scenarios**:
- GIVEN a workflow with 3 steps, WHEN `render_workflow` is called, THEN steps are returned in order with title, instruction, and required flag.
- GIVEN a new workflow is created via `add-workflow` CLI command, WHEN workflow steps are added, THEN each step has sequential ordering, title, and instruction.
- GIVEN a step marked `required: true`, WHEN the workflow is rendered, THEN the required step is clearly indicated in output.
- GIVEN workflow with step 1 (`entry_slug` set) and step 2 (`entry_slug` null), WHEN `render-workflow` renders the workflow, THEN both steps appear in order with full instructions. WHEN `skillvault run` executes, THEN only step 1 executes; step 2 is skipped.
- GIVEN a workflow with 3 steps all having `entry_slug` set to valid entries, WHEN `skillvault run` is invoked, THEN steps execute sequentially with system variable substitution.

---

## Capability 6: Series + SeriesEntry Entities

See spec §6.6.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-SER-01 | Series fields: `id`, `name`, `slug`, `description`, `status` | MUST |
| REQ-SER-02 | SeriesEntry fields: `series_id`, `entry_id`, `order_index` | MUST |
| REQ-SER-03 | Series groups ordered entries (e.g., learning path, prompt chain, architecture checklist) | MUST |
| REQ-SER-04 | `compose_series` MCP tool returns ordered entries with their metadata | MUST |
| REQ-SER-05 | An entry may belong to multiple series | SHOULD |
| REQ-SER-06 | Series status supports the same 5-status model | MUST |

**Scenarios**:
- GIVEN a series with 4 entries in order, WHEN `compose_series` MCP tool is called with series slug, THEN entries returned in correct order with metadata.
- GIVEN an entry belongs to two different series, WHEN either series is composed, THEN entry appears in both ordered results.
- GIVEN series is archived, WHEN normal search is performed, THEN series entries are excluded from default context.

---

## Capability 7: Tag Entity

See spec §6.7.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-TAG-01 | Tag fields: `id`, `name`, `slug` | MUST |
| REQ-TAG-02 | Tags shared across entries — one tag can appear on many entries | MUST |
| REQ-TAG-03 | Tag slug normalized: lowercase, trimmed, spaces to dashes | MUST |
| REQ-TAG-04 | Empty tag names rejected | MUST |
| REQ-TAG-05 | Duplicate tags on same entry deduplicated | MUST |

**Scenarios**:
- GIVEN entry with tags `["Go", "go", "cli-tools"]`, WHEN saved, THEN normalized tags are `["go", "cli-tools"]` (deduplicated, lowercased).
- GIVEN entry saved with tag `"  "`, WHEN validation runs, THEN empty tag is rejected.
- GIVEN entry with tag `"TDD"` exists, WHEN another entry is saved with tag `"tdd"`, THEN both entries share the same normalized tag `"tdd"`.

---

## Capability 8: EntryLink Entity + Relation Types

See spec §6.8.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-LNK-01 | EntryLink fields: `from_entry_id`, `to_entry_id`, `relation_type` | MUST |
| REQ-LNK-02 | Relation types: `references`, `supersedes`, `related_to`, `part_of`, `derived_from`, `implements` | MUST |
| REQ-LNK-03 | EntryLink creates a directed relationship between two entries | MUST |
| REQ-LNK-04 | Invalid relation type is rejected | MUST |
| REQ-LNK-05 | Self-referencing links (from == to) are rejected | MUST |

**Scenarios**:
- GIVEN entry A and entry B exist, WHEN link with type `references` is created from A to B, THEN querying entry A shows B as a reference.
- GIVEN entry A supersedes entry B, WHEN search returns both, THEN the `supersedes` relationship is available in metadata.
- GIVEN link with invalid type `invalid_rel`, WHEN creation is attempted, THEN link is rejected with validation error.

---

## Capability 9: Multi-Status Model

See spec §7.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-STA-01 | Required statuses: `draft`, `active`, `archived`, `deprecated`, `canonical` | MUST |
| REQ-STA-02 | Status semantics: `draft` (not ready), `active` (normal retrieval), `archived` (excluded from context), `deprecated` (not recommended), `canonical` (preferred version) | MUST |
| REQ-STA-03 | `get_context` MUST exclude `archived` and `deprecated` content by default | MUST |
| REQ-STA-04 | `include_archived` query parameter re-enables visibility of archived content | MUST |
| REQ-STA-05 | Entry, project, workflow, and series all support the status model | MUST |
| REQ-STA-06 | Default status for new entries is `draft` | MUST |
| REQ-STA-07 | Archive is a status change, not a delete — no data loss | MUST |

**Scenarios**:
- GIVEN entry has status `archived`, WHEN `search_entries` is called without `include_archived`, THEN entry is excluded from results.
- GIVEN entry has status `canonical`, WHEN context is compiled, THEN canonical entries are prioritized per context priority rules.
- GIVEN entry has status `deprecated`, WHEN `get_context` is called, THEN deprecated entry is excluded from default context pack.

---

## Capability 10: Hermes Context Layer (7 Modes)

See spec §8 (§8.1–§8.5).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-HRM-01 | Compile agent context packs via `get_context` | MUST |
| REQ-HRM-02 | 7 context modes: `profile`, `project`, `workflow`, `skill`, `planning`, `session_recall`, `full_brief` | MUST |
| REQ-HRM-03 | Context pack structure includes: Scope, User Preferences, Project State, Active Decisions, Relevant Workflows, Suggested Next Action | MUST |
| REQ-HRM-04 | Input fields: `mode`, `project` (optional), `query` (optional), `workflow` (optional), `include` (array), `exclude_archived` (bool), `max_chars` (int, default 12000) | MUST |
| REQ-HRM-05 | Context priority order: (1) user feedback/preferences, (2) active project state, (3) canonical decisions, (4) relevant workflow, (5) recent sessions, (6) artifact summaries, (7) references, (8) archived index lines only when requested | MUST |
| REQ-HRM-06 | Context pack respects `max_chars` limit; truncates lowest priority sections first | MUST |
| REQ-HRM-07 | `profile` mode returns user preferences and feedback entries | MUST |
| REQ-HRM-08 | `project` mode returns project decisions, state, and session summaries for a specific project | MUST |
| REQ-HRM-09 | `planning` mode combines profile, project state, decisions, and workflows | MUST |
| REQ-HRM-10 | `full_brief` mode returns all available context (subject to max_chars) | MUST |
| REQ-HRM-11 | Archived and deprecated content excluded by default in all modes | MUST |

**Scenarios**:
- GIVEN profile entry with user preferences exists, WHEN `get_context(mode=profile)` is called, THEN context pack contains user preferences and feedback entries, with mode scope clearly labeled.
- GIVEN project X has 3 decisions and 2 session entries, WHEN `get_context(mode=project, project=X)` is called, THEN context pack includes project state, decisions (prioritized as canonical first), and recent session summaries, respecting `max_chars`.
- GIVEN `get_context` is called with `max_chars=500`, WHEN context compilation exceeds limit, THEN lowest-priority sections are truncated until under limit.

---

## Capability 11: CLI Commands (20)

See spec §9 (§9.1–§9.2).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-CLI-01 | Binary name: `skillvault` | MUST |
| REQ-CLI-02 | Required commands: `init`, `add-entry`, `search`, `get`, `save-artifact`, `save-result`, `get-context`, `add-project`, `list-projects`, `archive`, `add-workflow`, `render-workflow`, `run`, `session-wrap`, `export`, `import`, `sync`, `tui`, `version`, `compare-entries`, `setup-vectors`, `reindex-embeddings` | MUST |
| REQ-CLI-03 | `init` creates `~/.skillvault/vault.db`, `objects/`, `exports/`, `cache/` | MUST |
| REQ-CLI-04 | `add-entry` accepts `--title`, `--type`, `--summary` (required), `--body`, `--project`, `--tags`, `--status`, `--purpose` (optional) | MUST |
| REQ-CLI-05 | `save-artifact` accepts `--title`, `--type`, `--file` (required), `--project`, `--summary`, `--tags`, `--source` (optional) | MUST |
| REQ-CLI-06 | `get-context` accepts `--mode`, `--project`, `--workflow`, `--include`, `--max-chars` | MUST |
| REQ-CLI-07 | `session-wrap` creates session entry with decisions, pending items, linked project, and optionally an artifact | MUST |
| REQ-CLI-08 | `archive` changes entry status to `archived`; does not delete data | MUST |
| REQ-CLI-09 | `export` exports DB data and optional artifact metadata manifest | MUST |
| REQ-CLI-10 | `import` imports valid SkillVault JSON; conflict handling on duplicate slugs | MUST |
| REQ-CLI-11 | `search` supports `--query`, `--type`, `--project`, `--tags`, `--include-archived`, `--limit`, `--vector`, `--purpose` | MUST |
| REQ-CLI-12 | `run` executes workflows: `skillvault run <workflow> <file> [--save output.md]`. Input from file or stdin (`-`). Output to stdout or `--save` path. Sequential pipeline execution. Pre-flight validates entry existence and status. | MUST |

**Scenarios**:
- GIVEN no vault exists, WHEN `skillvault init` runs, THEN `vault.db` plus `objects/`, `exports/`, `cache/` directories are created under `~/.skillvault/`.
- GIVEN entry exists with status `active`, WHEN `skillvault archive --id <id>` runs, THEN entry status changes to `archived`; data is preserved.
- GIVEN `skillvault session-wrap --project X --summary "completed auth" --decisions "use JWT" --pending "add refresh"` runs, THEN a session entry is created linked to project X with decisions and pending items.
- GIVEN file `article.md` exists, WHEN `skillvault run research_article article.md` executes, THEN final composed output is printed to stdout.
- GIVEN input piped via stdin, WHEN `echo "test" | skillvault run my_workflow - --save out.md` runs successfully, THEN `out.md` contains `{{final_output}}` content.
- GIVEN workflow `missing_wf` does not exist, WHEN `skillvault run missing_wf file.md` is invoked, THEN command exits with error indicating workflow not found.
- GIVEN vault configured, WHEN `skillvault sync push` runs, THEN snapshot uploaded via transport.
- GIVEN non-tui build, WHEN `skillvault tui` runs, THEN rebuild message printed to stderr.
- GIVEN GloVe loaded, WHEN `skillvault search "machine learning" --vector`, THEN vector search executes instead of FTS5.
- GIVEN a running vault, WHEN `skillvault add-entry --title "Review" --type reference --purpose LEARNING`, THEN entry persisted with purpose LEARNING.
- GIVEN a running vault, WHEN `skillvault add-entry --title "Go Patterns" --type reference` without `--purpose`, THEN entry persisted with empty purpose — no error.
- GIVEN a running vault, WHEN `skillvault add-entry --title "Bad" --type reference --purpose INVALID`, THEN command exits with validation error indicating unrecognized purpose value.
- GIVEN entries with purposes WORK, KNOWLEDGE, and empty, WHEN `skillvault search --purpose KNOWLEDGE`, THEN only KNOWLEDGE entries returned.
- GIVEN entries with various purposes, WHEN `skillvault search --query "patterns"` without `--purpose`, THEN all matching entries returned regardless of purpose.

---

## Capability 12: MCP Tools (19)

See spec §10 (§10.1–§10.12).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MCP-01 | 19 MCP tools: `save_entry`, `search_entries`, `get_entry`, `save_artifact`, `get_context`, `compose_series`, `render_workflow`, `session_wrap`, `archive_entry`, `list_projects`, `search_by_tags`, `get_context_bundle`, `save_entry_ref`, `list_entry_refs`, `get_entry_graph`, `compare_entries`, `save_result`, `run_workflow`, `route_scenario` | MUST |
| REQ-MCP-02 | `save_entry`: `title`, `type`, `summary`, `body`(opt), `project`(opt), `tags`, `status`, `purpose`(opt); rejects secrets | MUST |
| REQ-MCP-03 | `search_entries`: `query`, `type`(opt), `project`(opt), `tags`, `purpose`(opt), `include_archived`(default false), `limit`(default 10), `vector`(opt bool, default false) | MUST |
| REQ-MCP-04 | `get_entry`: returns entry by ID/slug with artifact ref if linked | MUST |
| REQ-MCP-05 | `save_artifact`: `title`, `type`, `content`(opt), `file_path`(opt), `summary`, `project`(opt), `tags`; at least one of content/file_path required | MUST |
| REQ-MCP-06 | `get_context`: `mode`, `project`(opt), `query`(opt), `workflow`(opt), `include`, `exclude_archived`, `max_chars` | MUST |
| REQ-MCP-07 | `compose_series`: returns ordered entries in a series | MUST |
| REQ-MCP-08 | `render_workflow`: returns steps as agent instructions/checklist | MUST |
| REQ-MCP-09 | `session_wrap`: `project`(opt), `summary`, `decisions`, `pending`, `learnings`, `artifacts` | MUST |
| REQ-MCP-10 | `archive_entry`: sets status to `archived` | MUST |
| REQ-MCP-11 | `list_projects`: lists projects and statuses | MUST |
| REQ-MCP-12 | `search_by_tags`: `tags`(array, req), `match`(`all`/`any`, default `all`), `type`(opt), `project`(opt), `limit`(default 20). Returns id, title, type, summary, project, status, tags. Uses REQ-TQR-01/02. | MUST |
| REQ-MCP-13 | `get_context_bundle`: `project`(opt). Returns structured JSON — project info, entries grouped by type, artifact refs. Cross-refs Hermes Context Layer (Capability 10). | MUST |
| REQ-MCP-18 | `run_workflow` MCP tool: delegates to `RunPipelineStructured`. Input: `workflow` (slug or ID, required), `steps` (map of step index → input text, required). Output: structured run result with `run_id`, `workflow_id`, `workflow_slug`, `status`, `steps` array (each with `step_index`, `status`, `output`, `error`), `started_at`, `finished_at`. All values JSON-RPC-compatible. | MUST |
| REQ-MCP-19 | `route_scenario` MCP tool: wraps `EntryService.RouteScenario`. Input: `scenario` (string, required). Output: matched workflow (ID, slug, name, steps) and skill/entry metadata as JSON. Empty scenario rejected with validation error. No-match returns meaningful error. | MUST |

**Scenarios**:
- GIVEN `search_entries` with filters, WHEN results match, THEN entries returned with full metadata.
- GIVEN `get_context(mode=project, project=myapp)`, WHEN compiled, THEN returns compact context pack matching CLI output.
- GIVEN `save_entry` with API key content, WHEN secret detected, THEN save rejected with warning.
- GIVEN entries tagged `["tdd","go"]` and `["tdd"]`, WHEN `search_by_tags(tags=["tdd","go"], match="all")`, THEN only dual-tagged entry returned.
- GIVEN project X with 2 decisions, 1 session, 1 artifact, WHEN `get_context_bundle(project="X")`, THEN response contains project object, `decisions`(2), `sessions`(1), and artifact refs.
- GIVEN GloVe loaded, WHEN `search_entries` called with `vector: true` and `query: "authentication"`, THEN results ranked by cosine similarity instead of FTS5.
- GIVEN valid entry payload, WHEN `save_entry` is called with `purpose: "LEARNING"`, THEN entry persisted with purpose LEARNING.
- GIVEN valid entry payload with no `purpose` field, WHEN `save_entry` is called, THEN entry persisted with empty purpose — no error.
- GIVEN entries with purposes WORK and KNOWLEDGE, WHEN `search_entries` is called with `purpose: "WORK"`, THEN only WORK entries returned.
- GIVEN workflow "research" with 2 valid steps, WHEN `run_workflow(workflow: "research", steps: {1: "topic: Go", 2: ""})` is called, THEN response includes `status: "completed"` and both steps with outputs.
- GIVEN step references a missing entry, WHEN `run_workflow` is called, THEN pre-flight validation rejects before any execution.
- GIVEN routing entry associates scenario "write spec" with workflow "spec-plan-task", WHEN `route_scenario(scenario: "write spec")` is called, THEN response includes workflow slug "spec-plan-task" and related metadata.
- GIVEN no routing entry matches "unknown task", WHEN `route_scenario(scenario: "unknown task")` is called, THEN response is error indicating no workflow matched.

---

## Capability 13: Secret Detection

See spec §12 (§12.1–§12.4).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-SEC-01 | Reject saving content with obvious secrets on `save_entry` and `save_artifact` | MUST |
| REQ-SEC-02 | Minimum regex patterns: OpenAI key (`sk-[A-Za-z0-9_-]{20,}`), private key (`-----BEGIN (RSA |EC |OPENSSH |)?PRIVATE KEY-----`), GitHub PAT (`ghp_[A-Za-z0-9_]{20,}`), Slack token (`xox[baprs]-[A-Za-z0-9-]{20,}`) | MUST |
| REQ-SEC-03 | On secret detection: do NOT save secret value; return warning to caller | MUST |
| REQ-SEC-04 | Allow saving a redacted note if the user chooses | SHOULD |
| REQ-SEC-05 | The system MUST NOT make network calls except during explicit `sync push` or `sync pull`. All other operations remain local-only. | MUST |
| REQ-SEC-06 | Archive is preferred over hard delete | MUST |
| REQ-SEC-07 | Hard delete requires explicit confirmation if implemented | SHOULD |

**Scenarios**:
- GIVEN content contains `sk-proj-AbCdEf1234567890123456789012345678901234567890`, WHEN `save_entry` or `save_artifact` is called, THEN save is rejected with a secret-detected warning.
- GIVEN content contains `-----BEGIN RSA PRIVATE KEY-----`, WHEN validation runs, THEN secret scanner detects the pattern and rejects the save.
- GIVEN safe content without any secret pattern, WHEN validation runs, THEN save proceeds normally.
- GIVEN any operation other than sync (search, get-context, save-entry, etc.), WHEN invoked, THEN only local SQLite/filesystem accessed.
- GIVEN configured transport, WHEN `sync push` executes, THEN network calls transfer snapshot.

---

## Capability 14: Search (FTS5 + Vector)

See spec §14.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-SRC-01 | Search uses SQLite FTS5 (default) OR brute-force cosine similarity over GloVe embeddings (when `vector` flag/param true) | MUST |
| REQ-SRC-02 | Required filters: `type`, `project`, `tag`, `status`, `include_archived`, `limit` | MUST |
| REQ-SRC-03 | Search result fields: `id`, `title`, `type`, `summary`, `project`, `status`, `tags`, `artifact_ref` (optional) | MUST |
| REQ-SRC-04 | Archived entries excluded by default | MUST |
| REQ-SRC-05 | Partial and fuzzy matching through FTS5 tokenizer (porter unicode61) | MUST |

**Scenarios**:
- GIVEN vault has entry titled "FastAPI PRD" with body describing FastAPI architecture, WHEN `search --query "fastapi"` is called, THEN entry appears in results with all required fields.
- GIVEN vault has 3 entries for project "SkillVault" and 2 for "Vitacare", WHEN `search --project SkillVault` is called, THEN only SkillVault entries are returned.
- GIVEN archived entry matches search query, WHEN `search --query "archived-term"` is called without `--include-archived`, THEN archived entry is excluded.

---

## Capability 15: Workflow Rendering

See spec §15.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-WFR-01 | Workflows are primarily renderable instruction checklists. When a step has `entry_slug` set referencing a valid active entry, that step MAY be executed as part of a sequential pipeline via `skillvault run`. | MUST |
| REQ-WFR-02 | `render_workflow` MCP tool and `render-workflow` CLI command output clear, ordered steps | MUST |
| REQ-WFR-03 | Each rendered step includes: order_index, title, instruction, required flag, and expected_output (if set) | MUST |
| REQ-WFR-04 | Example workflow `spec-plan-task` has 6 ordered steps | SHOULD |

**Scenarios**:
- GIVEN workflow "spec-plan-task" has 6 steps, WHEN `render_workflow` is called with workflow slug, THEN steps 1–6 are returned in sequential order with full instructions and required flags.
- GIVEN a workflow step has `expected_output` set, WHEN rendered, THEN the expected output is included in the step instruction output.
- GIVEN workflow has status `draft`, WHEN context compilation runs, THEN draft workflows are only included if explicitly requested.
- GIVEN workflow step has `entry_slug: summarize` referencing an active entry, WHEN `skillvault run` executes that step, THEN the referenced entry body is composed with system variables and executed in order.

---

## Capability 16: Import/Export

See spec §13 (§13.1–§13.2).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-IEX-01 | Export includes: projects, entries, workflows, workflow steps, series, tags, artifact metadata, artifact manifest | MUST |
| REQ-IEX-02 | Export format: valid JSON with schema version metadata | MUST |
| REQ-IEX-03 | Artifact files export: v2 exports paths and hashes; actual file copy is optional | MUST |
| REQ-IEX-04 | Import accepts valid SkillVault JSON; validates schema version | MUST |
| REQ-IEX-05 | On duplicate slug during import: do NOT overwrite silently; create conflict suffix or report conflict | MUST |
| REQ-IEX-06 | Import runs in a transaction; schema validation before any write | SHOULD |
| REQ-IEX-07 | Export/import available via CLI commands only (not MCP tools) | MUST |

**Scenarios**:
- GIVEN vault has 2 projects, 5 entries, 2 workflows, 1 series, and 3 artifacts, WHEN `skillvault export vault.json` runs, THEN output JSON contains all records with schema version and export timestamp.
- GIVEN export JSON has an entry with slug "my-entry", WHEN imported into vault that already has "my-entry", THEN conflict is reported and entry gets a conflict suffix rather than silent overwrite.
- GIVEN import file has invalid schema version, WHEN import runs, THEN import is rejected with validation error before any writes.

---

## Capability 17: Session Wrap

See spec §10.8.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-SWR-01 | `session_wrap` creates a session-type entry with summary, decisions, pending items, and learnings | MUST |
| REQ-SWR-02 | `session_wrap` accepts: `project` (optional), `summary`, `decisions` (array), `pending` (array), `learnings` (array), `artifacts` (optional array) | MUST |
| REQ-SWR-03 | Session entry links to specified project | MUST |
| REQ-SWR-04 | Session may optionally link one or more artifacts produced during the session | SHOULD |
| REQ-SWR-05 | `session-wrap` CLI command has same semantics as `session_wrap` MCP tool | MUST |

**Scenarios**:
- GIVEN session with summary "implemented auth", decisions `["use JWT"]`, pending `["add refresh tokens"]`, WHEN `session_wrap` is called, THEN a session-type entry is created with all data, linked to project if specified.
- GIVEN session produced artifact "spec.md", WHEN `session_wrap` includes artifact reference, THEN session entry links to artifact.
- GIVEN `session-wrap` CLI command runs with `--project myapp --summary "fixed bug"`, THEN output confirms session entry created and linked to project myapp.

---

## Capability 18: Code Integrity

The system MUST maintain integrity across build, runtime, and schema.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-CI-01 | `go.mod` MUST use `go 1.24` — code compiles with standard Go 1.24, no 1.26-specific features | MUST |
| REQ-CI-02 | MCP server MUST shut down within 5s of SIGTERM, draining active calls before exit | MUST |
| REQ-CI-03 | HTTP server MUST drain connections on `Shutdown(ctx)` with deadline | MUST |
| REQ-CI-04 | FTS5 MUST work without CGO: defensive import `_ "modernc.org/sqlite/lib/fts5"` | MUST |
| REQ-CI-05 | Schema MUST be consistent between migration output and `schema.sql` — no drift | MUST |
| REQ-CI-06 | Application version MUST be `v3` (upgraded from `v2-quantum`). Import/export schema version SHALL remain `2` (additive change only). v2 exports MUST import into v3 without data loss. | MUST |

**Scenarios**:
- GIVEN active tool calls, WHEN SIGTERM received, THEN in-flight calls complete, exit code 0 within 5s.
- GIVEN active connections, WHEN shutdown triggered, THEN no new requests accepted, existing drained, clean exit.
- GIVEN `CGO_ENABLED=0`, WHEN built and search invoked, THEN FTS5 queries execute correctly.
- GIVEN no prior runs exist, WHEN `skillvault version` is executed, THEN output is `v3`.
- GIVEN a v2 export file with schema version 2, WHEN imported into v3, THEN import succeeds without data loss, AND new `runs`/`run_steps` fields are absent (null/empty).

---

## Capability 19: Tag Query Support

Inner DB layer for all/any tag matching via `entry_tags` junction table. Supports Capability 7 (Tag Entity) and 14 (Search FTS5).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-TQR-01 | `match_all`: entry MUST have ALL specified tags (intersection) | MUST |
| REQ-TQR-02 | `match_any`: entry MUST have at least one tag (union) | MUST |
| REQ-TQR-03 | Input tags MUST be normalized (lowercase, trim, deduplicate), consistent with REQ-TAG-03/05 | MUST |

**Scenarios**:
- GIVEN entries tagged `["go","cli"]`, `["go"]`, `["cli"]`, WHEN `match="all"` with `["go","cli"]`, THEN only dual-tagged entry returned.
- GIVEN same entries, WHEN `match="any"` with `["go","cli"]`, THEN all three returned.

---

## Capability 20: Pipeline Execution Engine

See delta spec `skillvault-v3-workflow-pipelines`.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-RUN-01 | A `runs` table SHALL track each execution with `id`, `workflow_id`, `input`, `output`, `status` (pending|running|completed|failed), `started_at`, and `finished_at`. | MUST |
| REQ-RUN-02 | A `run_steps` table SHALL track each step with `id`, `run_id`, `step_id`, `entry_id`, `input`, `output`, `status`, `started_at`, and `finished_at`. | MUST |
| REQ-RUN-03 | `WorkflowRunService` MUST execute steps in `order_index` ascending order. Each step's output SHALL become `{{previous_output}}` for the next. Execution MUST halt on step failure. | MUST |
| REQ-RUN-04 | The system SHALL support three system variables: `{{input}}` (initial file/stdin content), `{{previous_output}}` (last completed step output, truncated at 32K), and `{{final_output}}` (all completed step outputs concatenated). | MUST |
| REQ-RUN-05 | Pre-flight validation SHALL reject execution BEFORE any step runs if a referenced entry slug does not exist or references an archived entry. | MUST |
| REQ-RUN-06 | `{{previous_output}}` SHALL be truncated at 32K characters with a warning emitted on truncation. | MUST |
| REQ-RUN-07 | The service SHALL NOT call LLMs; it composes prompts from entry bodies with system variable substitution and records step results. | MUST |

**Scenarios**:
- GIVEN a workflow with 3 steps, WHEN `skillvault run` executes, THEN a `run` record is created with status `pending`, AND 3 `run_step` records are created with status `pending`, AND statuses transition to `running` then `completed` (or `failed` on error).
- GIVEN step 2 of 3 fails, WHEN execution halts, THEN run status is `failed`, AND step 1 status is `completed`, step 2 status is `failed`, AND step 3 status remains `pending`.
- GIVEN initial input "Hello World", WHEN step 1 entry body contains "Process: {{input}}", THEN step executes with "Process: Hello World", AND step output is stored for next step's `{{previous_output}}`.
- GIVEN step output exceeds 32K characters, WHEN next step substitutes `{{previous_output}}`, THEN variable is truncated at 32K, AND a truncation warning is emitted.
- GIVEN workflow step references entry slug `extract_wisdom` that does not exist, WHEN `run` is invoked, THEN execution is rejected BEFORE any step runs, AND no `run` or `run_step` records are created.
- GIVEN workflow step references an archived entry, WHEN `run` is invoked, THEN execution is rejected with a validation error indicating the entry is not active.

---

## Capability 21: Cloud Sync

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-SYNC-01 | The system MUST define a pluggable `Transport` interface (`Push`/`Pull` byte streams) in `internal/sync/`. Implementations: S3-compatible (minio-go v7, supporting AWS S3, MinIO, Cloudflare R2, Backblaze B2) and GitHub Releases. Gzip MUST compress payload before wire transfer; decompress on pull. Sync service SHALL reuse `ExportAll()`/`ImportAll()` unchanged. Snapshot semantics: last-write-wins via timestamp comparison. | MUST |
| REQ-SYNC-02 | The CLI MUST support `skillvault sync push` and `skillvault sync pull`. Credentials MUST load from env vars (`AWS_ACCESS_KEY_ID`, `GITHUB_TOKEN`, etc.) and `~/.skillvault/config.yaml`. Transport logs MUST sanitize credentials. | MUST |

**Scenarios**:
- GIVEN vault with entries and configured S3 credentials, WHEN `sync push --transport s3` then `sync pull --transport s3`, THEN all entries, projects, workflows, and tags are preserved identically.
- GIVEN `--dry-run` flag, WHEN `sync push` runs, THEN timestamp diff and payload size shown, no transfer.
- GIVEN invalid credentials, WHEN push executes, THEN error reported with sanitized logs (no raw keys).
- GIVEN configured transport, WHEN `sync push` runs, THEN vault exported, compressed, uploaded as dated snapshot.
- GIVEN remote newer than local, WHEN `sync pull` runs, THEN downloaded, decompressed, imported.

---

## Capability 22: TUI (Build-Tag Gated)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-TUI-01 | All `internal/tui/` files MUST be gated by `//go:build tui`. CLI SHALL register `tui` via build-tag-gated `main_tui.go`. Without tag, `main_notui.go` SHALL print: "TUI not available. Rebuild with: go build -tags tui ./cmd/skillvault" to stderr, exit 1. TUI MUST provide browse (entry list), FTS5 search, and entry detail views. TUI SHALL be read-only — no creation, editing, or mutation. A `make build-tui` target SHOULD build with `-tags tui`. Default binary (no tag) SHOULD grow <500KB over baseline. | MUST |

**Scenarios**:
- GIVEN `go build -tags tui`, WHEN `skillvault tui` runs, THEN Bubble Tea TUI launches with browse/search/detail views.
- GIVEN build without tag, WHEN `skillvault tui` runs, THEN stderr prints rebuild message, exit code 1.
- GIVEN TUI launched with populated vault, WHEN user browses or searches, THEN entries display title/type/status.
- GIVEN user selects entry, WHEN detail view opens, THEN metadata/tags/body shown without edit capability.
- GIVEN project root, WHEN `make build-tui` runs, THEN binary includes TUI and passes existing tests.

## Capability 23: Vector Search (GloVe 300d)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-VS-01 | `setup-vectors <glove-file>` loads GloVe 300d file into `map[string][]float32` | MUST |
| REQ-VS-02 | Tokenizer lowercases, splits on whitespace, filters non-alpha; OOV words → zero vector | MUST |
| REQ-VS-03 | `entry_embeddings` table: `entry_id` TEXT PK, `embedding` BLOB, `dims` INT, `model` TEXT | MUST |
| REQ-VS-04 | `Save()` auto-embeds Title + Summary + Body; persists `[]float32` as BLOB | MUST |
| REQ-VS-05 | Vector search: query → embedding → brute-force cosine similarity over all entries → ranked | MUST |
| REQ-VS-06 | `--vector` flag (CLI) / `vector: bool` param (MCP) switches to vector path; default is FTS5 | MUST |
| REQ-VS-07 | `reindex-embeddings` batch-embeds all existing entries; no data loss | MUST |

**Scenarios**:
- GIVEN entries "JWT auth" and "login flow" with GloVe loaded, WHEN `search --query "authentication" --vector`, THEN both ranked by cosine similarity.
- GIVEN GloVe loaded, WHEN `save_entry` saves "OAuth2 Guide", THEN embedding BLOB persists in `entry_embeddings`.
- GIVEN no GloVe loaded, WHEN `search --vector` runs, THEN error: "vector model not loaded; run setup-vectors first".
- GIVEN vault has 3 unembedded entries, WHEN `reindex-embeddings` runs, THEN all 3 get embeddings; existing entries unchanged.

---

## Capability 24: Entry Diff

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-DIFF-01 | Line-based LCS unified diff between two entry bodies, pure Go, no deps | MUST |
| REQ-DIFF-02 | CLI `compare-entries <id1> <id2>` prints unified diff | MUST |
| REQ-DIFF-03 | MCP `compare_entries(from_id, to_id)` returns entries + diff hunks | MUST |
| REQ-DIFF-04 | Diff output approximates `diff -u` format with context lines | SHOULD |

**Scenarios**:
- GIVEN entry A body "line 1\nline 2\nline 3", entry B body "line 1\nline 2 edited\nline 3", WHEN `compare-entries <idA> <idB>`, THEN unified diff shows line 2 change with context.
- GIVEN entry A exists, WHEN `compare-entries <idA> <idA>`, THEN diff shows no changes.
- GIVEN entry ID "nonexistent" missing, WHEN `compare-entries <valid-id> nonexistent`, THEN error: entry not found.

---

## Capability 25: Entry Purpose Taxonomy

The `purpose` field classifies entries by LifeOS purpose, orthogonal to entry type. Five values represent the v7.6 purpose model (minus OBSERVABILITY, deferred). Empty purpose is backward-compatible.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-PUR-01 | The system SHALL support five purpose values: `WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`. | MUST |
| REQ-PUR-02 | Empty purpose (`""`) SHALL be valid and represent "unset" — backward-compatible default for all existing entries and calls without purpose. | MUST |
| REQ-PUR-03 | The system SHALL reject any purpose value not in the allowed set with a validation error naming the invalid value. | MUST |
| REQ-PUR-04 | `search` CLI command and `search_entries` MCP tool SHALL accept an optional `--purpose` / `purpose` filter that returns only entries matching the given purpose value. | MUST |
| REQ-PUR-05 | `add-entry` CLI SHALL accept optional `--purpose` flag accepting one of the five values. | MUST |
| REQ-PUR-06 | `save_entry` MCP tool SHALL accept an optional `purpose` parameter accepting one of the five values or empty. | MUST |
| REQ-PUR-07 | Export SHALL include the `purpose` field for each entry. Import SHALL restore it — full round-trip fidelity. | MUST |
| REQ-PUR-08 | Purpose SHALL be stored as a `TEXT` column in the entries table, defaulting to empty string (`""`), added via migration 007. | MUST |

**Scenarios**:
- GIVEN a vault with no prior purpose usage, WHEN `save_entry` is called with `purpose: "KNOWLEDGE"` or `add-entry --purpose KNOWLEDGE`, THEN the entry is persisted with `purpose = "KNOWLEDGE"`.
- GIVEN a valid entry payload, WHEN `save_entry` is called with `purpose: "INVALID_VALUE"`, THEN the save is rejected with a validation error indicating "INVALID_VALUE" is not a recognized purpose.
- GIVEN a valid entry payload with no `purpose` field (or empty string), WHEN the entry is saved, THEN the entry is persisted with `purpose = ""` — no error, backward-compatible behavior.
- GIVEN 3 entries: one WORK, one KNOWLEDGE, one with empty purpose, WHEN `search_entries` is called with `purpose: "WORK"`, THEN only the WORK entry is returned.
- GIVEN entries with various purposes including empty, WHEN `search_entries` is called without a `purpose` parameter, THEN all entries are returned regardless of their purpose value.
- GIVEN a running vault, WHEN `skillvault add-entry --title "Review" --type reference --purpose LEARNING`, THEN the entry is persisted with purpose LEARNING.
- GIVEN an export containing entries with purposes WORK, KNOWLEDGE, and empty, WHEN the export is imported into a fresh vault, THEN all entries retain their original purpose values.
- GIVEN entries with purposes WORK, KNOWLEDGE, LEARNING, and an empty-purpose entry, WHEN `skillvault search --purpose WORK`, THEN only WORK entries appear in results.

---

## Capability 26: Workflow Run Bridge

Provides structured pipeline execution (`RunPipelineStructured`) that accepts step inputs as structured arguments (not stdin/file) and returns a JSON-shaped run result. Exposed as the `run_workflow` MCP tool. Additive — existing CLI `run` (`RunPipeline`) is unchanged.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-RBR-01 | `RunPipelineStructured(workflowRef, steps)` SHALL accept a workflow reference (slug or ID) and a map of step index → input text. It SHALL execute steps sequentially in `order_index` order, substituting system variables identically to `RunPipeline`. | MUST |
| REQ-RBR-02 | The return value SHALL be a structured result containing: `run_id`, `workflow_id`, `workflow_slug`, `status` (completed\|failed), `steps` (array of {step_index, status, output, error}), `started_at`, `finished_at`. | MUST |
| REQ-RBR-03 | If a step fails, execution SHALL halt. The run status SHALL be `failed`, the failing step's entry in `steps` SHALL include `status: "failed"` and an `error` field, and unexecuted steps SHALL have `status: "pending"`. | MUST |
| REQ-RBR-04 | `RunPipelineStructured` SHALL create `runs` and `run_steps` records in the database identically to `RunPipeline`. | MUST |
| REQ-RBR-05 | Pre-flight validation SHALL reject execution if any referenced entry slug does not exist or is archived, BEFORE any step executes. | MUST |
| REQ-RBR-06 | The existing `RunPipeline` method and CLI `run` command SHALL remain unchanged — stdin/file input path is preserved. | MUST |
| REQ-RBR-07 | `{{previous_output}}` truncation at 32K SHALL apply to structured runs identically to existing pipeline behavior. | MUST |
| REQ-RBR-08 | The system SHALL NOT execute steps in parallel; execution is strictly sequential. | MUST |

**Scenarios**:
- GIVEN a workflow "research_article" with 2 steps both referencing valid active entries, WHEN `RunPipelineStructured("research_article", {1: "Analyze: REST vs GraphQL", 2: ""})` is called, THEN a run record is created with status `completed`, AND `steps` array shows both steps with status `completed` and their respective outputs.
- GIVEN step 1 succeeds, step 2 fails, WHEN structured run executes, THEN run status is `failed`, step 1 shows `completed` with output, step 2 shows `failed` with error, and any step 3 shows `pending`.
- GIVEN a workflow step references entry slug "nonexistent_entry" that does not exist, WHEN `RunPipelineStructured` is invoked, THEN execution is rejected before any step runs, AND no run/run_step records are created.
- GIVEN a workflow and input file, WHEN `skillvault run my_workflow input.md` is invoked, THEN the existing `RunPipeline` path executes via stdin/file — behavior is identical to before structured run was added.
- GIVEN a valid workflow slug and step inputs, WHEN the `run_workflow` MCP tool is called, THEN it delegates to `RunPipelineStructured` and returns the structured result as JSON-RPC response.

---

## Capability 27: MCP Route Tool

Expose the existing `RouteScenario` capability (from PR #16) as an MCP tool. The `route_scenario` MCP tool accepts a scenario string and returns the matched workflow, skill, or entry information — enabling agent-driven scenario-to-workflow resolution.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MRT-01 | The system SHALL expose a `route_scenario` MCP tool that wraps `EntryService.RouteScenario`. | MUST |
| REQ-MRT-02 | Input: `scenario` (string, required) — the scenario description to resolve. | MUST |
| REQ-MRT-03 | Output SHALL include the matched workflow (ID, slug, name, steps) and any matched skill/entry (ID, slug, title, type), encoded as a JSON object. | MUST |
| REQ-MRT-04 | The tool SHALL return a meaningful error message when no workflow or skill matches the scenario string. | MUST |
| REQ-MRT-05 | The tool SHALL return a validation error when `scenario` is empty or missing. | MUST |
| REQ-MRT-06 | Args and results SHALL be JSON-RPC-compatible — all values serializable as JSON without custom types. | MUST |

**Scenarios**:
- GIVEN a workflow "spec-plan-task" is associated with a routing entry tagged for "spec writing", WHEN `route_scenario` is called with `scenario: "write a specification"`, THEN the response includes the matched workflow ID, slug "spec-plan-task", steps, and related skill/entry metadata.
- GIVEN no routing entries match the scenario, WHEN `route_scenario` is called with `scenario: "do something unknown"`, THEN the response is an error indicating no workflow or skill matched the scenario.
- GIVEN an empty string scenario, WHEN `route_scenario` is called with `scenario: ""`, THEN the call is rejected with a validation error: "scenario is required".
- GIVEN a valid scenario that matches a workflow, WHEN `route_scenario` returns, THEN the result is a JSON object with string/array/number values only — no Go-specific types, function references, or channels.

---

## Coverage Summary

| Capability | Requirements | Scenarios |
|-----------|-------------|-----------|
| Hybrid Storage Model | 7 | 4 |
| Entry Entity + 10 Types | 6 | 3 |
| Project Entity | 5 | 3 |
| Artifact Entity + File-Backed Storage | 8 | 3 |
| Workflow + WorkflowStep Entities | 6 | 5 |
| Series + SeriesEntry Entities | 6 | 3 |
| Tag Entity | 5 | 3 |
| EntryLink + Relation Types | 5 | 3 |
| Multi-Status Model | 7 | 3 |
| Hermes Context Layer (7 Modes) | 11 | 3 |
| CLI Commands | 12 | 13 |
| MCP Tools | 15 | 14 |
| Secret Detection | 7 | 5 |
| Search (FTS5 + Vector) | 5 | 3 |
| Workflow Rendering | 4 | 4 |
| Import/Export | 7 | 3 |
| Session Wrap | 5 | 3 |
| Code Integrity | 6 | 5 |
| Tag Query Support | 3 | 2 |
| Pipeline Execution Engine | 7 | 6 |
| Cloud Sync | 2 | 5 |
| TUI (Build-Tag Gated) | 1 | 5 |
| Vector Search (GloVe 300d) | 7 | 4 |
| Entry Diff | 4 | 3 |
| Entry Purpose Taxonomy | 8 | 8 |
| Workflow Run Bridge | 8 | 5 |
| MCP Route Tool | 6 | 4 |
| **Total** | **168** | **117** |
**Edge cases**: secret detection rejection, duplicate slug import conflict, archived exclusion in context and search, empty tag rejection, self-referencing link rejection, missing content/file_path on artifact save, max_chars truncation, TUI rebuild message on non-tagged build, sanitized credential logging on sync errors.
**Error states**: invalid entry type, invalid relation type, invalid schema version on import, secret detected warning, missing required fields on CLI, unknown sync subcommand, TUI startup with no terminal (TTY check).
