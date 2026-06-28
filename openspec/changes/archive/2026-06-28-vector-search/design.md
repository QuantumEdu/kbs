# Design: Vector Search + Entry Diff

## Technical Approach

Additive change — new `internal/vector/` package for GloVe embedding + cosine search, new `internal/diff/` for LCS-based unified diff, new `entry_embeddings` table (migration 005), and `VectorService` in `app/`. FTS5 remains default; `--vector` flag or `vector: true` param switches to cosine path. Auto-embedding on `Save()` is transparent when GloVe is loaded.

```
CLI (cmd/) ──→ cli/ ──→ app/ ──→ domain/ + db/
           │                  │
           │                  ├── EntryService ──→ auto-embed on Save()
           │                  │
           │                  ├── VectorService (new)
           │                  │     ├── GloveVectors (sync.Once)
           │                  │     ├── Embed() / Search()
           │                  │     └── CompareEntries()
           │                  │
           │                  └── db/
           │                       ├── sqliteVectorStore (new)
           │                       └── 005_vector_search.sql

MCP ──→ mcp/tools.go ──→ app/
```

## Architecture Decisions

| Decision | Choice | Tradeoff | Rationale |
|----------|--------|----------|-----------|
| Embedding runtime | Pure Go GloVe loader | 400MB+ RAM vs zero deps | Matches local-first / CGO-free mandate. Accept Phase 1 RAM cost. |
| Search algorithm | Brute-force cosine in Go | O(N) vs ANN index | N < 10k at current scale. ANN index is premature optimization. |
| Auto-embed trigger | On `Save()` via `EntryService` | Every save may block vs batch-only | Spec requires REQ-VS-04 auto-embed. Lazy-load glove means first save is fast if not configured. |
| Diff algorithm | LCS line-based | Suboptimal for prose vs zero deps | Myers diff adds 60+ lines of table-building; LCS is simpler and adequate for markdown/spec entries. |
| GloVe loading | `sync.Once` lazy-load with explicit `setup-vectors` | Separate init step vs auto-find | User must provide file; explicit config prevents silent 1GB RAM spike. |
| Config storage | Env var `SKILLVAULT_GLOVE_PATH` | No config file vs existing config.yaml | Avoids new dependency for YAML config parsing. Existing codebase has no config file infrastructure. |
| Diff package location | `internal/diff/` (separate from vector) | Two new packages vs one | Diff is independent of vector search. Clean separation of concerns. |

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/vector/glove.go` | Create | `GloveVectors` struct, `LoadGlove()` — parse text file, `map[string][]float32` |
| `internal/vector/tokenizer.go` | Create | `Tokenize()` — lowercase, regexp split, stop-word filter |
| `internal/vector/embedding.go` | Create | `Embed()` — word → avg → normalize; `Serialize/Deserialize` for BLOB |
| `internal/vector/similarity.go` | Create | `Cosine()`, `Search()` — brute-force ranked by score |
| `internal/diff/diff.go` | Create | `UnifiedDiff()`, `DiffLine` struct, `FormatUnifiedDiff()` |
| `internal/db/migrations/005_vector_search.sql` | Create | `entry_embeddings` table (entry_id, embedding BLOB, dims, model) |
| `internal/db/schema.sql` | Modify | Add `entry_embeddings` table definition |
| `internal/db/vector_store.go` | Create | `VectorStore` interface + `sqliteVectorStore` impl |
| `internal/db/store.go` | Modify | Add `Embeddings VectorStore` field to `Store` struct; add `VectorStore` interface |
| `internal/app/vector.go` | Create | `VectorService` with `EnsureEmbedded()`, `Search()`, `CompareEntries()` |
| `internal/app/entries.go` | Modify | `EntryService` gains `vector *VectorService` field; auto-embed after `Save()` |
| `internal/domain/filters.go` | Modify | Add `Vector bool` field to `SearchQuery` |
| `internal/mcp/tools.go` | Modify | `search_entries` gains `vector` param; new `compare_entries` tool + handler |
| `internal/cli/commands.go` | Modify | `SearchFlags` gains `Vector` bool; new `ParseSearchFlags` includes `--vector` |
| `cmd/skillvault/main.go` | Modify | Wire `VectorService`, handle `setup-vectors`, `reindex-embeddings`, `compare-entries` CLI |
| `go.mod` | Modify | No new dependencies (verified) |

## Interfaces / Contracts

```go
// VectorStore — db layer
type VectorStore interface {
    SaveEmbedding(ctx, entryID string, embedding []byte) error
    GetEmbedding(ctx, entryID string) ([]byte, error)
    GetAllEmbeddings(ctx) (map[string][]byte, error) // for brute-force search
}

// VectorService — app layer
type VectorService struct {
    vectors      *vector.GloveVectors
    vectorStore  db.VectorStore
    entryStore   db.EntryStore
    loaded       bool // true after lazy init
}

func (s *VectorService) Search(query string, limit int) ([]SearchResult, error)
func (s *VectorService) EnsureEmbedded(entryID string) error
func (s *VectorService) CompareEntries(id1, id2 string) (*DiffResult, error)
```

`SearchQuery` gains `Vector bool` field. `EntryService` gains `vector *VectorService` field (nil-safe — if nil, auto-embed is skipped).

## Sequence: Vector Search

```
CLI: search --vector "machine learning"
  │
  ├─ cli.ParseSearchFlags() → SearchFlags{Vector: true}
  ├─ entrySvc.SearchEntries(query, filters) // Vector bool in SearchQuery
  │    ├─ vector.Embed("machine learning", gloveVectors) → []float32{300d}
  │    ├─ vectorStore.GetAllEmbeddings() → map[entryID][]byte
  │    ├─ deserialize each BLOB → map[entryID][]float32
  │    ├─ vector.Search(queryEmb, candidates, limit) → []Score
  │    └─ fetch entries by ranked IDs → []EntrySearchResult
  └─ print results
```

## Sequence: SaveEntry with Auto-Embed

```
MCP: save_entry(title="JWT Guide", body="...")
  │
  ├─ entrySvc.SaveEntry() → slug, insert entry
  ├─ if entrySvc.vector != nil && entrySvc.vector.vectors != nil:
  │    └─ vector.EnsureEmbedded(entryID)
  │         ├─ getEntry(body)
  │         ├─ vector.Embed(title + " " + summary + " " + body, gloveVectors)
  │         ├─ vector.Serialize(embedding) → BLOB
  │         └─ vectorStore.SaveEmbedding(entryID, BLOB)
  └─ return result
```

## CLI Command Spec

| Command | Flags | Description |
|---------|-------|-------------|
| `setup-vectors <path>` | positional path | Load GloVe file, save path in process memory (env var) |
| `reindex-embeddings` | — | Batch embed all entries without embeddings; report count |
| `compare-entries <id1> <id2>` | positional IDs | Fetch both entries, compute LCS unified diff, print |
| `search <query> --vector` | `--vector` added | Switch to cosine similarity path |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| GloVe not loaded + `--vector` | Error: "vector model not loaded; run setup-vectors first" |
| GloVe not loaded + `Save()` | Skip embedding silently — entry saved normally |
| Query with 100% OOV words | Return nil embedding → "no vector matches found" |
| `reindex-embeddings` with no glove | Error: "vector model not loaded" |
| `compare-entries` on missing IDs | Error: "entry X not found" |
| Identical entries in diff | Return "No changes" |
| Corrupt BLOB in `entry_embeddings` | Skip entry in search, log warning |

## Testing Strategy

| Layer | What | How |
|-------|------|-----|
| Unit: `internal/vector/` | Tokenizer, embedding math, cosine, serialize | Table-driven Go tests, small GloVe fixture (5-word file) |
| Unit: `internal/diff/` | LCS diff, unified format output | Golden-file tests with known diffs |
| Integration: `internal/db/` | BLOB save/load, migration 005 | In-memory SQLite, `RunMigrations()` |
| Integration: `internal/app/` | VectorService search flow, auto-embed | Mock GloVe (pre-built small `map`), in-memory DB |
| Integration: `internal/mcp/` | `compare_entries` handler, `search_entries` vector param | Registry tests with service mocks |
| E2E: `cmd/` | Full `setup-vectors → add-entry → search --vector → compare-entries` | Acceptance tests (existing pattern in `app/acceptance_test.go`) |

## Migration / Rollout

- Migration 005 is additive (`CREATE TABLE IF NOT EXISTS`). No data loss on existing vaults.
- `entry_embeddings` rows are created on first `Save()` or `reindex-embeddings`. Missing embeddings do not break search — they are simply excluded.
- Rollback: drop `entry_embeddings` table, remove `Vector bool` from `SearchQuery`. FTS5 and all other features are untouched.
- GloVe path is ephemeral (env var / CLI flag). No config file migration needed.

## Open Questions

None — all design decisions resolved.
