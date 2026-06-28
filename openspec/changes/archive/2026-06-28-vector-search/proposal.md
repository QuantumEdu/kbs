# Proposal: Vector Search + Entry Diff

## Intent

SkillVault search is keyword-only (FTS5). "auth" won't match "login." Users need semantic search for conceptual queries. Also, comparing related entries requires manual side-by-side review — no built-in diff tool. `get_context_bundle` already exists (Capability 12, REQ-MCP-13), no work needed.

## Scope

### In Scope
- **GloVe vector search**: pure-Go embedding from pre-trained GloVe 300d vectors, stored as BLOB in `entry_embeddings`, auto-embedded on `Save()`, brute-force cosine similarity via `--vector` flag
- **CLI setup/reindex**: `setup-vectors <glove-file>` loads model, `reindex-embeddings` batch-embeds existing entries
- **Entry diff**: `compare_entries` MCP tool + `compare-entries` CLI, line-based LCS unified diff, pure Go

### Out of Scope
- `get_context_bundle` (already implemented, REQ-MCP-13)
- Replacing FTS5, ONNX runtime, external embedding servers, word-level diff, approximate nearest neighbor indices, cloud sync

## Capabilities

### New Capabilities
- `vector-search`: GloVe 300d embedding generation, BLOB storage in `entry_embeddings` table, cosine similarity brute-force search, auto-embed on save, coexists with FTS5 (user picks per query)
- `entry-diff`: line-based LCS unified diff between two entry bodies, pure Go, available via MCP tool `compare_entries` and CLI `compare-entries`

### Modified Capabilities
- `mcp-tools` (Capability 12): add `compare_entries(from_id, to_id)` tool; extend `search_entries` with optional `vector: bool` param
- `cli-commands` (Capability 11): add `setup-vectors`, `reindex-embeddings`, `compare-entries` commands; add `--vector` flag to `search`
- `search-fts5` (Capability 14): entry store gains `SearchVector()` method; schema gains `entry_embeddings` table (migration v005)

## Approach

**Vector search**: New `internal/vector/` package (glove.go, tokenizer.go, embedding.go, similarity.go). GloVe file loaded at startup as `map[string][]float32`. Tokenizer splits, lowercases, averages word vectors → 300d `[]float32` serialized to BLOB (~1.2KB/entry). `Save()` triggers `syncEmbedding()`. Search flow: query embedding → brute-force cosine over all entries → rank + return. `--vector` flag switches search path in MCP/CLI.

**Entry diff**: LCS-based line diff in `internal/vector/diff.go` (or `internal/diff/`). Fetches two entries by ID, computes unified diff, returns entries + diff hunks + shared/unique tags.

**Architecture**: Both additive to existing search/store layers. No new dependencies. Pure Go, CGO-free, local-first.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/vector/` | **New** | GloVe loader, tokenizer, embedding, similarity, diff |
| `internal/db/schema.sql` + `migrations/005_` | Modified/New | `entry_embeddings` table (BLOB) |
| `internal/db/entries_store.go` | Modified | Save syncs embedding; `SearchVector()` added |
| `internal/db/store.go` | Modified | New `VectorStore` interface |
| `internal/app/entries.go` | Modified | Vector search + embedding sync in service |
| `internal/mcp/tools.go` | Modified | `compare_entries` tool; vector param on `search_entries` |
| `internal/cli/commands.go` | Modified | New commands + `--vector` flag |
| `cmd/skillvault/main.go` | Modified | Wire vector services, config, startup |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| 400MB GloVe RAM load (~1-1.5GB expanded) | Medium | Lazy-load only words in vault; int8 quantization later |
| OOV words → zero vectors degrade quality | Medium | Accept Phase 1 limit; subword fallback later |
| Brute-force cosine O(N) at 100k+ entries | Low | Accept at current scale; FAISS-style index later |
| Schema migration on existing vaults | Low | NULL-tolerant; `reindex-embeddings` backfills |

## Rollback Plan

- `entry_embeddings` table is additive — drop it to fully revert
- `--vector` flag defaults false; removing vector code leaves FTS5 intact
- Diff tool is standalone; no DB changes, no rollback risk

## Dependencies

- User-provided GloVe file (e.g., `glove.6B.300d.txt` from Stanford NLP)
- No new Go dependencies (pure Go, stdlib only)

## Success Criteria

- [ ] `search --vector "semantic concepts"` returns conceptually related entries not matched by FTS5
- [ ] `Save()` auto-generates embedding; embedding persisted as BLOB
- [ ] `setup-vectors <path>` loads model; search errors gracefully if not loaded
- [ ] `compare-entries <id1> <id2>` outputs unified diff matching `diff -u` format
- [ ] `reindex-embeddings` embeds all existing entries without data loss
- [ ] FTS5 search unchanged when `--vector` not used
