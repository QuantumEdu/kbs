package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type SaveSeriesInput struct {
	Name        string
	Description string
	Status      string
	EntryIDs    []string
}

type SeriesService struct {
	seriesStore db.SeriesStore
	entryStore  db.EntryStore
}

func NewSeriesService(seriesStore db.SeriesStore, entryStore db.EntryStore) *SeriesService {
	return &SeriesService{seriesStore: seriesStore, entryStore: entryStore}
}

func (s *SeriesService) Save(ctx context.Context, series domain.Series) error {
	return s.seriesStore.Save(ctx, series)
}

func (s *SeriesService) SaveSeries(ctx context.Context, input SaveSeriesInput) (*domain.Series, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	status := domain.StatusActive
	if input.Status != "" {
		if err := domain.ValidateStatus(input.Status); err != nil {
			return nil, fmt.Errorf("validate status: %w", err)
		}
		status = domain.Status(input.Status)
	}

	ser := domain.Series{
		ID:          generateSeriesID(),
		Name:        input.Name,
		Slug:        slugify(input.Name),
		Description: input.Description,
		Status:      status,
	}

	if err := s.seriesStore.Save(ctx, ser); err != nil {
		return nil, fmt.Errorf("save series: %w", err)
	}

	if len(input.EntryIDs) > 0 {
		entries := make([]domain.SeriesEntry, 0, len(input.EntryIDs))
		for i, eid := range input.EntryIDs {
			entry, err := s.entryStore.Get(ctx, eid, true)
			if err != nil {
				return nil, fmt.Errorf("entry %q at index %d: %w", eid, i, err)
			}
			if err := domain.ValidateSeriesScope(nil, entry.Entry.ProjectID); err != nil {
				return nil, fmt.Errorf("scope validation for entry %s: %w", eid, err)
			}
			entries = append(entries, domain.SeriesEntry{
				SeriesID:   ser.ID,
				EntryID:    eid,
				OrderIndex: i + 1,
			})
		}
		if err := s.seriesStore.ReplaceSeriesEntries(ctx, ser.ID, entries); err != nil {
			return nil, fmt.Errorf("set series entries: %w", err)
		}
	}

	return &ser, nil
}

func (s *SeriesService) Compose(ctx context.Context, seriesID string) ([]domain.Entry, error) {
	return s.seriesStore.Compose(ctx, seriesID)
}

func (s *SeriesService) ComposeSeries(ctx context.Context, id string) ([]domain.Entry, error) {
	return s.seriesStore.Compose(ctx, id)
}

func (s *SeriesService) Get(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error) {
	return s.seriesStore.Get(ctx, id, includeArchived)
}

func (s *SeriesService) List(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error) {
	return s.seriesStore.List(ctx, filter)
}

func (s *SeriesService) ListSeries(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error) {
	return s.seriesStore.List(ctx, filter)
}

func (s *SeriesService) ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntry) error {
	for _, se := range entries {
		entry, err := s.entryStore.Get(ctx, se.EntryID, true)
		if err != nil {
			return fmt.Errorf("get entry %s: %w", se.EntryID, err)
		}
		if err := domain.ValidateSeriesScope(nil, entry.Entry.ProjectID); err != nil {
			return fmt.Errorf("scope validation for entry %s: %w", se.EntryID, err)
		}
	}
	return s.seriesStore.ReplaceSeriesEntries(ctx, seriesID, entries)
}
