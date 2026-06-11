package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func TestOpenDBConnection(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB(:memory:) failed: %v", err)
	}
	defer db.Close()

	var v int
	if err := db.QueryRow("SELECT 1").Scan(&v); err != nil {
		t.Fatalf("connection test failed: %v", err)
	}
	if v != 1 {
		t.Errorf("expected 1, got %d", v)
	}
}

func TestNewStoreCreation(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	s := NewStore(db)
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
	if s.Entries == nil {
		t.Error("Entries store is nil")
	}
	if s.Projects == nil {
		t.Error("Projects store is nil")
	}
	if s.Series == nil {
		t.Error("Series store is nil")
	}
	if s.Workflows == nil {
		t.Error("Workflows store is nil")
	}
	if s.Search == nil {
		t.Error("Search store is nil")
	}
	if s.ImportExport == nil {
		t.Error("ImportExport store is nil")
	}
	if s.DB() == nil {
		t.Error("DB() returns nil")
	}
}

func TestStoreInterfacesCompile(t *testing.T) {
	var _ EntryStore = (*entryStoreImpl)(nil)
	var _ ProjectStore = (*projectStoreImpl)(nil)
	var _ SeriesStore = (*seriesStoreImpl)(nil)
	var _ WorkflowStore = (*workflowStoreImpl)(nil)
	var _ SearchStore = (*searchStoreImpl)(nil)

	s := &Store{}
	if s == nil {
		t.Fatal("Store should not be nil")
	}
}

type entryStoreImpl struct{}
type projectStoreImpl struct{}
type seriesStoreImpl struct{}
type workflowStoreImpl struct{}
type searchStoreImpl struct{}

func (e *entryStoreImpl) UpsertEntry(ctx context.Context, entry domain.Entry, tags []string, steps []domain.WorkflowStep) error {
	return nil
}
func (e *entryStoreImpl) GetEntry(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error) {
	return domain.EntryResult{}, nil
}
func (e *entryStoreImpl) ListEntries(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	return nil, nil
}
func (e *entryStoreImpl) ArchiveEntry(ctx context.Context, id string) error { return nil }

func (p *projectStoreImpl) UpsertProject(ctx context.Context, proj domain.Project) error { return nil }
func (p *projectStoreImpl) ListProjects(ctx context.Context, includeArchived bool) ([]domain.Project, error) {
	return nil, nil
}

func (s *seriesStoreImpl) UpsertSeries(ctx context.Context, series domain.Series) error { return nil }
func (s *seriesStoreImpl) GetSeries(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error) {
	return domain.SeriesResult{}, nil
}
func (s *seriesStoreImpl) ListSeries(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error) {
	return nil, nil
}
func (s *seriesStoreImpl) ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntryInput) error {
	return nil
}

func (w *workflowStoreImpl) UpsertWorkflowSteps(ctx context.Context, entryID string, steps []domain.WorkflowStep) error {
	return nil
}
func (w *workflowStoreImpl) GetWorkflowSteps(ctx context.Context, entryID string) ([]domain.WorkflowStep, error) {
	return nil, nil
}

func (s *searchStoreImpl) SearchEntries(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	return nil, nil
}
func (s *searchStoreImpl) RebuildFTS(ctx context.Context) error { return nil }
