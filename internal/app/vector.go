package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/diff"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/vector"
)

// VectorService provides vector-based search, auto-embedding, and entry comparison.
type VectorService struct {
	entryStore  db.EntryStore
	vectorStore db.VectorStore
	vectors     *vector.GloveVectors
}

// NewVectorService creates a new VectorService backed by the given stores.
func NewVectorService(entryStore db.EntryStore, vectorStore db.VectorStore) *VectorService {
	return &VectorService{entryStore: entryStore, vectorStore: vectorStore}
}

// SetGlove injects GloVe vectors for embedding and search operations.
// Must be called before SearchVectors, EnsureEmbedded, or ReindexAll.
func (s *VectorService) SetGlove(glove *vector.GloveVectors) {
	s.vectors = glove
}

// IsLoaded returns true if GloVe vectors have been loaded and are ready.
func (s *VectorService) IsLoaded() bool {
	return s.vectors != nil && s.vectors.Loaded()
}

// SearchVectors embeds the query text, performs cosine similarity search
// against all stored embeddings, and returns ranked entry results.
func (s *VectorService) SearchVectors(ctx context.Context, query string, limit int) ([]domain.EntrySearchResult, error) {
	if !s.IsLoaded() {
		return nil, fmt.Errorf("vector model not loaded; run setup-vectors first")
	}

	queryVec, err := vector.Embed(query, s.vectors)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if queryVec == nil {
		return nil, nil // no vector matches found — return empty, not an error
	}

	similar, err := s.vectorStore.SearchSimilar(ctx, queryVec, limit)
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}

	results := make([]domain.EntrySearchResult, 0, len(similar))
	for _, sim := range similar {
		entry, err := s.entryStore.Get(ctx, sim.EntryID, false)
		if err != nil {
			// Skip entries that cannot be fetched (archived, deleted, etc.)
			continue
		}
		results = append(results, domain.EntrySearchResult{
			Entry: entry.Entry,
			Tags:  entry.Tags,
		})
	}
	return results, nil
}

// EnsureEmbedded generates and saves an embedding for the given entry if one
// does not already exist. Silently skips when GloVe is not loaded or when
// the entry text produces no valid tokens.
func (s *VectorService) EnsureEmbedded(ctx context.Context, entryID string) error {
	if !s.IsLoaded() {
		return nil // silently skip — GloVe not configured
	}

	// Check if embedding already exists.
	if _, err := s.vectorStore.GetEmbedding(ctx, entryID); err == nil {
		return nil
	}

	// Fetch the entry to build its embedding text.
	e, err := s.entryStore.Get(ctx, entryID, true)
	if err != nil {
		return nil // silently skip — entry not found
	}

	text := buildEntryText(e)
	emb, err := vector.Embed(text, s.vectors)
	if err != nil {
		return fmt.Errorf("embed entry %q: %w", entryID, err)
	}
	if emb == nil {
		return nil // no valid tokens — skip
	}

	data := vector.Serialize(emb)
	return s.vectorStore.SaveEmbedding(ctx, entryID, data, s.vectors.Dims(), "glove")
}

// ReindexAll iterates over every entry in the vault and generates embeddings
// for any that are missing. Returns the count of newly embedded entries.
func (s *VectorService) ReindexAll(ctx context.Context) (int, error) {
	if !s.IsLoaded() {
		return 0, fmt.Errorf("vector model not loaded")
	}

	entries, err := s.entryStore.List(ctx, domain.EntryFilter{IncludeArchived: true})
	if err != nil {
		return 0, fmt.Errorf("list entries: %w", err)
	}

	count := 0
	for _, e := range entries {
		if _, err := s.vectorStore.GetEmbedding(ctx, e.Entry.ID); err == nil {
			continue // already embedded
		}

		text := buildEntryText(domain.EntryResult{Entry: e.Entry, Tags: e.Tags})
		emb, err := vector.Embed(text, s.vectors)
		if err != nil || emb == nil {
			continue
		}

		data := vector.Serialize(emb)
		if err := s.vectorStore.SaveEmbedding(ctx, e.Entry.ID, data, s.vectors.Dims(), "glove"); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// CompareEntries fetches two entries by ID, builds their text representation,
// computes a line-based LCS unified diff, and returns it as a string.
func (s *VectorService) CompareEntries(ctx context.Context, id1, id2 string) (string, error) {
	e1, err := s.entryStore.Get(ctx, id1, false)
	if err != nil {
		return "", fmt.Errorf("entry %q not found: %w", id1, err)
	}
	e2, err := s.entryStore.Get(ctx, id2, false)
	if err != nil {
		return "", fmt.Errorf("entry %q not found: %w", id2, err)
	}

	oldText := buildEntryText(e1)
	newText := buildEntryText(e2)

	lines := diff.UnifiedDiff(oldText, newText)
	return diff.FormatUnifiedDiff(lines), nil
}

// buildEntryText assembles text from an entry's title, summary, and optional body.
func buildEntryText(e domain.EntryResult) string {
	text := e.Entry.Title + "\n" + e.Entry.Summary
	if e.Entry.BodyOptional != "" {
		text += "\n" + e.Entry.BodyOptional
	}
	return text
}
