# Delta for SkillVault — vector-search

## ADDED Requirements

### Capability 21: Vector Search (GloVe 300d)

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

### Capability 22: Entry Diff

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

## MODIFIED Requirements

### REQ-MCP-01 (Capability 12: MCP Tools)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MCP-01 | 16 MCP tools: `save_entry`, `search_entries`, `get_entry`, `save_artifact`, `get_context`, `compose_series`, `render_workflow`, `session_wrap`, `archive_entry`, `list_projects`, `search_by_tags`, `get_context_bundle`, `save_entry_ref`, `list_entry_refs`, `get_entry_graph`, **`compare_entries`** | MUST |
(Previously: 15 tools; added compare_entries)

### REQ-MCP-03 (search_entries tool)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-MCP-03 | `search_entries`: `query`, `type`(opt), `project`(opt), `tags`, `include_archived`(default false), `limit`(default 10), `vector`(opt bool, default false) | MUST |
(Previously: no vector parameter)

**Scenario**:
- GIVEN GloVe loaded, WHEN `search_entries` called with `vector: true` and `query: "authentication"`, THEN results ranked by cosine similarity instead of FTS5.

### REQ-CLI-02 (Capability 11: CLI Commands)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-CLI-02 | Required commands: `init`, `add-entry`, `search`, `get`, `save-artifact`, `get-context`, `add-project`, `list-projects`, `archive`, `add-workflow`, `render-workflow`, `session-wrap`, `export`, `import`, `run`, **`setup-vectors`**, **`reindex-embeddings`**, **`compare-entries`** | MUST |
(Previously: 15 commands; added 3)

### REQ-CLI-11 (search command flags)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-CLI-11 | `search` supports `--query`, `--type`, `--project`, `--tags`, `--include-archived`, `--limit`, `--vector` | MUST |
(Previously: no --vector flag)

**Scenario**:
- GIVEN GloVe loaded, WHEN `skillvault search "machine learning" --vector`, THEN vector search executes instead of FTS5.

### REQ-SRC-01 (Capability 14: Search)

| ID | Requirement | Strength |
|----|-------------|----------|
| REQ-SRC-01 | Search uses SQLite FTS5 (default) OR brute-force cosine similarity over GloVe embeddings (when `vector` flag/param true) | MUST |
(Previously: FTS5 only; now dual-mode with vector path)

---
