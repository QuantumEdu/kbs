package db

import (
	"context"
	"database/sql"

	"github.com/quantum-6/skillvault/internal/domain"
)

// EntryStore defines persistence operations for entries.
type EntryStore interface {
	UpsertEntry(ctx context.Context, entry domain.Entry, tags []string, steps []domain.WorkflowStep) error
	GetEntry(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error)
	ListEntries(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error)
	ArchiveEntry(ctx context.Context, id string) error
}

// ProjectStore defines persistence operations for projects.
type ProjectStore interface {
	UpsertProject(ctx context.Context, p domain.Project) error
	ListProjects(ctx context.Context, includeArchived bool) ([]domain.Project, error)
}

// SeriesStore defines persistence operations for series.
type SeriesStore interface {
	UpsertSeries(ctx context.Context, s domain.Series) error
	GetSeries(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error)
	ListSeries(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error)
	ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntryInput) error
}

// WorkflowStore defines persistence operations for workflow steps.
type WorkflowStore interface {
	UpsertWorkflowSteps(ctx context.Context, entryID string, steps []domain.WorkflowStep) error
	GetWorkflowSteps(ctx context.Context, entryID string) ([]domain.WorkflowStep, error)
}

// SearchStore defines FTS5 search operations.
type SearchStore interface {
	SearchEntries(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error)
	RebuildFTS(ctx context.Context) error
}

// ImportExportStore defines vault import/export operations.
type ImportExportStore interface {
	ExportAll(ctx context.Context) (domain.VaultExport, error)
	ImportAll(ctx context.Context, data domain.VaultExport) error
}

// Store is the top-level data access layer composing all sub-stores.
// App services should depend on individual interfaces, not Store directly.
type Store struct {
	Entries      EntryStore
	Projects     ProjectStore
	Series       SeriesStore
	Workflows    WorkflowStore
	Search       SearchStore
	ImportExport ImportExportStore

	db *sql.DB
}

// NewStore creates a new Store with all sub-store implementations.
func NewStore(db *sql.DB) *Store {
	return &Store{
		Entries:      &sqliteEntryStore{db: db},
		Projects:     &sqliteProjectStore{db: db},
		Series:       &sqliteSeriesStore{db: db},
		Workflows:    &sqliteWorkflowStore{db: db},
		Search:       &sqliteSearchStore{db: db},
		ImportExport: &sqliteImportExportStore{db: db},
		db:           db,
	}
}

// DB returns the underlying database connection.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Store implementations — each lives in its own file.
// struct definitions are here so NewStore can instantiate them.
type sqliteEntryStore struct{ db *sql.DB }
type sqliteProjectStore struct{ db *sql.DB }
type sqliteSeriesStore struct{ db *sql.DB }
type sqliteWorkflowStore struct{ db *sql.DB }
type sqliteSearchStore struct{ db *sql.DB }
type sqliteImportExportStore struct{ db *sql.DB }
