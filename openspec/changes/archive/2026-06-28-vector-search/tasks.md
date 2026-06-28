# Tasks: Vector Search + Entry Diff

## Review Workload Forecast

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

| Unit | Goal | PR | Lines | Base |
|------|------|-----|-------|------|
| 1 | Entry diff engine + CLI + MCP | 1 | ~340 | feature/vector-search |
| 2 | GloVe loader, tokenizer, embedding | 2 | ~350 | PR 1 branch |
| 3 | Cosine similarity, schema, vector store | 3 | ~288 | PR 2 branch |
| 4 | VectorService, auto-embed, wiring | 4 | ~385 | PR 3 branch |

Total: ~1360 lines.

## Phase 1: Entry Diff (PR 1)

- [x] 1.1 `internal/diff/diff.go` — `UnifiedDiff()` LCS, `DiffLine`, `FormatUnifiedDiff()`
- [x] 1.2 `internal/diff/diff_test.go` — golden-file: identity, edit, insert, delete
- [x] 1.3 `internal/cli/commands.go` — `compare-entries` cmd + `ParseCompareEntriesFlags`
- [x] 1.4 `internal/mcp/tools.go` — `compare_entries` schema + handler
- [x] 1.5 `internal/app/vector.go` — `VectorService` + `CompareEntries()` stub
- [x] 1.6 `cmd/skillvault/main.go` — wire `compare-entries` CLI + MCP

## Phase 2: Vector Math (PR 2)

- [x] 2.1 `internal/vector/glove.go` — `LoadGlove(path)` → `map[string][]float32`
- [x] 2.2 `internal/vector/glove_test.go` — 5-word fixture, dims + values
- [x] 2.3 `internal/vector/tokenizer.go` — `Tokenize()` lowercase, whitespace, non-alpha
- [x] 2.4 `internal/vector/tokenizer_test.go` — table-driven, punctuation, OOV
- [x] 2.5 `internal/vector/embedding.go` — `Embed()` avg → `[]float32`, `Serialize/Deserialize`
- [x] 2.6 `internal/vector/embedding_test.go` — 300d output, serde roundtrip

## Phase 3: Similarity + DB (PR 3)

- [x] 3.1 `internal/vector/similarity.go` — `Cosine()`, `Search()` brute-force ranked
- [x] 3.2 `internal/vector/similarity_test.go` — identity=1.0, orthogonal≈0, ranking
- [x] 3.3 `internal/db/migrations/005_vector_search.sql` — `entry_embeddings` table
- [x] 3.4 `internal/db/schema.sql` — append `entry_embeddings` definition
- [x] 3.5 `internal/db/vector_store.go` — `VectorStore` interface + `sqliteVectorStore`
- [x] 3.6 `internal/db/vector_store_test.go` — BLOB save/load, GetAll, empty
- [x] 3.7 `internal/db/store.go` — `VectorStore` interface + `Embeddings` field on `Store`
- [x] 3.8 `internal/domain/filters.go` — `Vector bool` on `SearchQuery`

## Phase 4: Integration (PR 4)

- [x] 4.1 `internal/app/vector.go` — `Search()`, `EnsureEmbedded()`, `ReindexEmbeddings()`, `SetupVectors()`
- [x] 4.2 `internal/app/entries.go` — auto-embed on `Save()` (nil-safe)
- [x] 4.3 `internal/mcp/tools.go` — `vector` param on `search_entries`
- [x] 4.4 `internal/cli/commands.go` — `--vector` flag, `setup-vectors`, `reindex-embeddings`
- [x] 4.5 `cmd/skillvault/main.go` — wire new CLI commands + vector search, pass `VectorService` to MCP

## Verification

- [x] V.1 Unit tests: `internal/vector/*`, `internal/diff/*`, `internal/db/vector_store*`
- [x] V.2 Existing tests: `app_test.go`, `acceptance_test.go` — no regressions
- [x] V.3 `setup-vectors <glove>` → `search --vector "MCP"` → ranked results
- [x] V.4 `compare-entries <idA> <idB>` → unified diff; identity → "No changes"
- [x] V.5 `reindex-embeddings` → all embedded, no data loss, FTS5 unchanged
