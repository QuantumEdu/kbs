# Delta for SkillVault

## ADDED Requirements

### Requirement: Code Integrity

The system MUST maintain integrity across build, runtime, and schema.

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-CI-01 | `go.mod` MUST use `go 1.24` — code compiles with standard Go 1.24, no 1.26-specific features | MUST |
| REQ-CI-02 | MCP server MUST shut down within 5s of SIGTERM, draining active calls before exit | MUST |
| REQ-CI-03 | HTTP server MUST drain connections on `Shutdown(ctx)` with deadline | MUST |
| REQ-CI-04 | FTS5 MUST work without CGO: defensive import `_ "modernc.org/sqlite/lib/fts5"` | MUST |
| REQ-CI-05 | Schema MUST be consistent between migration output and `schema.sql` — no drift | MUST |

#### Scenario: MCP graceful shutdown
- GIVEN active tool calls, WHEN SIGTERM received, THEN in-flight calls complete, exit code 0 within 5s.

#### Scenario: HTTP active-connection drain
- GIVEN active connections, WHEN shutdown triggered, THEN no new requests accepted, existing drained, clean exit.

#### Scenario: FTS5 without CGO
- GIVEN `CGO_ENABLED=0`, WHEN built and search invoked, THEN FTS5 queries execute correctly.

### Requirement: Tag Query Support

Inner DB layer for all/any tag matching via `entry_tags` junction table. Supports Capability 7 (Tag Entity) and 14 (Search FTS5).

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-TQR-01 | `match_all`: entry MUST have ALL specified tags (intersection) | MUST |
| REQ-TQR-02 | `match_any`: entry MUST have at least one tag (union) | MUST |
| REQ-TQR-03 | Input tags MUST be normalized (lowercase, trim, deduplicate), consistent with REQ-TAG-03/05 | MUST |

#### Scenario: Intersection
- GIVEN entries tagged `["go","cli"]`, `["go"]`, `["cli"]`, WHEN `match="all"` with `["go","cli"]`, THEN only dual-tagged entry returned.

#### Scenario: Union
- GIVEN same entries, WHEN `match="any"` with `["go","cli"]`, THEN all three returned.

## MODIFIED Requirements

### Requirement: MCP Tools (12)

(Previously: 10 MCP tools; added `search_by_tags` and `get_context_bundle`)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MCP-01 | 12 MCP tools: `save_entry`, `search_entries`, `get_entry`, `save_artifact`, `get_context`, `compose_series`, `render_workflow`, `session_wrap`, `archive_entry`, `list_projects`, `search_by_tags`, `get_context_bundle` | MUST |
| REQ-MCP-02 | `save_entry`: `title`, `type`, `summary`, `body`(opt), `project`(opt), `tags`, `status`; rejects secrets | MUST |
| REQ-MCP-03 | `search_entries`: `query`, `type`(opt), `project`(opt), `tags`, `include_archived`(default false), `limit`(default 10) | MUST |
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

#### Scenario: Core MCP search, context, save
- GIVEN `search_entries` with filters, WHEN results match, THEN entries returned with full metadata.
- GIVEN `get_context(mode=project, project=myapp)`, WHEN compiled, THEN returns compact context pack matching CLI output.
- GIVEN `save_entry` with API key content, WHEN secret detected, THEN save rejected with warning.

#### Scenario: search_by_tags all match
- GIVEN entries tagged `["tdd","go"]` and `["tdd"]`, WHEN `search_by_tags(tags=["tdd","go"], match="all")`, THEN only dual-tagged entry returned.

#### Scenario: get_context_bundle grouped output
- GIVEN project X with 2 decisions, 1 session, 1 artifact, WHEN `get_context_bundle(project="X")`, THEN response contains project object, `decisions`(2), `sessions`(1), and artifact refs.
