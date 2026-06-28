package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/diff"
	"github.com/quantum-6/skillvault/internal/domain"
)

// VectorService provides vector-based search and entry comparison.
type VectorService struct {
	entryStore db.EntryStore
}

// NewVectorService creates a new VectorService backed by the given entry store.
func NewVectorService(entryStore db.EntryStore) *VectorService {
	return &VectorService{entryStore: entryStore}
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

// buildEntryText assembles the diffable text from an entry's title, summary, and optional body.
func buildEntryText(e domain.EntryResult) string {
	text := e.Entry.Title + "\n" + e.Entry.Summary
	if e.Entry.BodyOptional != "" {
		text += "\n" + e.Entry.BodyOptional
	}
	return text
}
