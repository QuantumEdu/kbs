package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// EntryService orchestrates entry CRUD operations.
type EntryService struct {
	store db.EntryStore
}

// NewEntryService creates a new EntryService.
func NewEntryService(store db.EntryStore) *EntryService {
	return &EntryService{store: store}
}

// UpsertEntry normalizes tags, validates the entry, and delegates to the store.
func (s *EntryService) UpsertEntry(ctx context.Context, entry domain.Entry, tags []string, steps []domain.WorkflowStep) error {
	if err := domain.ValidateEntryType(string(entry.Type)); err != nil {
		return fmt.Errorf("validate entry: %w", err)
	}
	tags = domain.NormalizeTags(tags)
	return s.store.UpsertEntry(ctx, entry, tags, steps)
}

// GetEntry retrieves an entry with optional archived inclusion.
func (s *EntryService) GetEntry(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error) {
	return s.store.GetEntry(ctx, id, includeArchived)
}

// SearchEntries performs FTS5 search via the search store.
func (s *EntryService) SearchEntries(ctx context.Context, searchStore db.SearchStore, q domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	if err := domain.ValidateSearchQuery(q); err != nil {
		return nil, fmt.Errorf("validate search: %w", err)
	}
	return searchStore.SearchEntries(ctx, q)
}

// ListEntries returns entries matching the filter.
func (s *EntryService) ListEntries(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	return s.store.ListEntries(ctx, filter)
}

// ArchiveEntry soft-deletes an entry.
func (s *EntryService) ArchiveEntry(ctx context.Context, id string) error {
	return s.store.ArchiveEntry(ctx, id)
}
