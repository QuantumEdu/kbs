package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// SeriesService orchestrates series operations.
type SeriesService struct {
	store       db.SeriesStore
	entryStore  db.EntryStore
}

// NewSeriesService creates a new SeriesService.
func NewSeriesService(store db.SeriesStore, entryStore db.EntryStore) *SeriesService {
	return &SeriesService{store: store, entryStore: entryStore}
}

// UpsertSeries creates or updates a series.
func (s *SeriesService) UpsertSeries(ctx context.Context, series domain.Series) error {
	return s.store.UpsertSeries(ctx, series)
}

// GetSeries retrieves a series with entries.
func (s *SeriesService) GetSeries(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error) {
	return s.store.GetSeries(ctx, id, includeArchived)
}

// ListSeries returns series matching the filter.
func (s *SeriesService) ListSeries(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error) {
	return s.store.ListSeries(ctx, filter)
}

// ReplaceSeriesEntries validates scope and replaces entries in a series.
func (s *SeriesService) ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntryInput) error {
	// Get series to check scope
	series, err := s.store.GetSeries(ctx, seriesID, true)
	if err != nil {
		return fmt.Errorf("get series: %w", err)
	}

	// Validate scope for each entry
	for _, input := range entries {
		entry, err := s.entryStore.GetEntry(ctx, input.EntryID, true)
		if err != nil {
			return fmt.Errorf("get entry %s: %w", input.EntryID, err)
		}
		if err := domain.ValidateSeriesScope(series.Series.ProjectID, entry.Entry.ProjectID); err != nil {
			return fmt.Errorf("scope validation for entry %s: %w", input.EntryID, err)
		}
	}

	return s.store.ReplaceSeriesEntries(ctx, seriesID, entries)
}
