package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quantum-6/skillvault/internal/vector"
)

// SimilarityResult holds the result of a vector similarity search.
type SimilarityResult struct {
	EntryID string
	Score   float64
}

type sqliteVectorStore struct {
	db *sql.DB
}

// SaveEmbedding inserts or replaces an embedding for the given entry.
func (s *sqliteVectorStore) SaveEmbedding(ctx context.Context, entryID string, embedding []byte, dims int, model string) error {
	if model == "" {
		model = "glove"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO entry_embeddings (entry_id, embedding, dims, model, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		entryID, embedding, dims, model)
	if err != nil {
		return fmt.Errorf("save embedding for %q: %w", entryID, err)
	}
	return nil
}

// GetEmbedding retrieves the embedding BLOB for the given entry.
func (s *sqliteVectorStore) GetEmbedding(ctx context.Context, entryID string) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT embedding FROM entry_embeddings WHERE entry_id = ?", entryID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("embedding for entry %q: %w", entryID, err)
	}
	if err != nil {
		return nil, fmt.Errorf("get embedding for %q: %w", entryID, err)
	}
	return data, nil
}

// SearchSimilar loads all embeddings from the database, deserializes them,
// computes cosine similarity against queryVec, and returns results sorted by
// score descending. Corrupt BLOBs are silently skipped.
func (s *sqliteVectorStore) SearchSimilar(ctx context.Context, queryVec []float32, limit int) ([]SimilarityResult, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT entry_id, embedding FROM entry_embeddings")
	if err != nil {
		return nil, fmt.Errorf("query embeddings: %w", err)
	}
	defer rows.Close()

	candidates := make(map[string][]float32)
	for rows.Next() {
		var entryID string
		var data []byte
		if err := rows.Scan(&entryID, &data); err != nil {
			return nil, fmt.Errorf("scan embedding row: %w", err)
		}
		vec, err := vector.Deserialize(data)
		if err != nil {
			// Corrupt BLOB – skip entry per design error handling.
			continue
		}
		if len(vec) > 0 {
			candidates[entryID] = vec
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embeddings: %w", err)
	}

	scored := vector.Search(queryVec, candidates, limit)

	results := make([]SimilarityResult, len(scored))
	for i, s := range scored {
		results[i] = SimilarityResult{EntryID: s.EntryID, Score: s.Score}
	}
	return results, nil
}

// DeleteEmbedding removes the embedding for the given entry.
func (s *sqliteVectorStore) DeleteEmbedding(ctx context.Context, entryID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM entry_embeddings WHERE entry_id = ?", entryID)
	if err != nil {
		return fmt.Errorf("delete embedding for %q: %w", entryID, err)
	}
	return nil
}
