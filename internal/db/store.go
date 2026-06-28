package db

import (
	"context"
	"database/sql"

	"github.com/quantum-6/skillvault/internal/domain"
)

type EntryStore interface {
	Save(ctx context.Context, entry domain.Entry, tags []string) error
	Get(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error)
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error)
	SearchByTags(ctx context.Context, tags []string, matchAll bool, typePtr, projectPtr *string, limit int) ([]domain.EntrySearchResult, error)
	Archive(ctx context.Context, id string) error
	List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error)
}

type ArtifactStore interface {
	Save(ctx context.Context, a domain.Artifact) error
	Get(ctx context.Context, id string) (domain.Artifact, error)
	List(ctx context.Context, projectID *string) ([]domain.Artifact, error)
}

type WorkflowStore interface {
	Save(ctx context.Context, w domain.Workflow, steps []domain.WorkflowStep) error
	Get(ctx context.Context, id string) (domain.Workflow, error)
	GetSteps(ctx context.Context, workflowID string) ([]domain.WorkflowStep, error)
	Render(ctx context.Context, id string) ([]domain.WorkflowStep, error)
	List(ctx context.Context, includeArchived bool) ([]domain.Workflow, error)
}

type WorkflowRunStore interface {
	CreateRun(ctx context.Context, run domain.WorkflowRun, steps []domain.WorkflowRunStep) error
	GetRun(ctx context.Context, id string) (domain.WorkflowRun, []domain.WorkflowRunStep, error)
	ListRuns(ctx context.Context, workflowID string, limit int) ([]domain.WorkflowRun, error)
	UpdateStepStatus(ctx context.Context, stepID string, status domain.RunStatus, output string) error
	UpdateRunStatus(ctx context.Context, runID string, status domain.RunStatus, output string) error
}

type SeriesStore interface {
	Save(ctx context.Context, s domain.Series) error
	Get(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error)
	Compose(ctx context.Context, seriesID string) ([]domain.Entry, error)
	List(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error)
	ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntry) error
}

type TagStore interface {
	Save(ctx context.Context, tag domain.Tag) error
	List(ctx context.Context) ([]domain.Tag, error)
	Search(ctx context.Context, query string) ([]domain.Tag, error)
}

type EntryLinkStore interface {
	Save(ctx context.Context, link domain.EntryLink) error
	GetLinks(ctx context.Context, entryID string) ([]domain.EntryLink, error)
	GetLinksByType(ctx context.Context, entryID string, relationType string) ([]domain.EntryLink, error)
	ListRefs(ctx context.Context, filter EntryLinkFilter) ([]domain.EntryLink, error)
	RemoveRef(ctx context.Context, fromEntryID, toEntryID, relationType string) error
	ReachableRefs(ctx context.Context, entryID string, refType string, maxDepth int) ([]EntryLinkNode, error)
	GetEntryGraph(ctx context.Context, entryID string, refTypes []string, direction string, maxDepth int) ([]EntryLinkNode, []domain.EntryLink, error)
}

type EntryLinkFilter struct {
	SourceID        *string
	TargetID        *string
	RefType         *string
	Active          *bool
	IncludeArchived bool
}

type EntryLinkNode struct {
	EntryID string
	Depth   int
}

type ProjectStore interface {
	Save(ctx context.Context, p domain.Project) error
	Get(ctx context.Context, id string) (domain.Project, error)
	List(ctx context.Context, includeArchived bool) ([]domain.Project, error)
	Archive(ctx context.Context, id string) error
}

type SearchStore interface {
	RebuildFTS(ctx context.Context) error
}

type ImportExportStore interface {
	ExportAll(ctx context.Context) (domain.VaultExport, error)
	ImportAll(ctx context.Context, data domain.VaultExport) error
}

type VectorStore interface {
	SaveEmbedding(ctx context.Context, entryID string, embedding []byte, dims int, model string) error
	GetEmbedding(ctx context.Context, entryID string) ([]byte, error)
	SearchSimilar(ctx context.Context, queryVec []float32, limit int) ([]SimilarityResult, error)
	DeleteEmbedding(ctx context.Context, entryID string) error
}

type Store struct {
	Entries      EntryStore
	Artifacts    ArtifactStore
	Workflows    WorkflowStore
	WorkflowRuns WorkflowRunStore
	Series       SeriesStore
	Tags         TagStore
	EntryLinks   EntryLinkStore
	Projects     ProjectStore
	Search       SearchStore
	ImportExport ImportExportStore
	Embeddings   VectorStore

	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		Entries:      &sqliteEntryStore{db: db},
		Artifacts:    &sqliteArtifactStore{db: db},
		Workflows:    &sqliteWorkflowStore{db: db},
		WorkflowRuns: &sqliteWorkflowRunStore{db: db},
		Series:       &sqliteSeriesStore{db: db},
		Tags:         &sqliteTagStore{db: db},
		EntryLinks:   &sqliteEntryLinkStore{db: db},
		Projects:     &sqliteProjectStore{db: db},
		Search:       &sqliteSearchStore{db: db},
		ImportExport: &sqliteImportExportStore{db: db},
		Embeddings:   &sqliteVectorStore{db: db},
		db:           db,
	}
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

type sqliteEntryStore struct{ db *sql.DB }
type sqliteArtifactStore struct{ db *sql.DB }
type sqliteWorkflowStore struct{ db *sql.DB }
type sqliteWorkflowRunStore struct{ db *sql.DB }
type sqliteSeriesStore struct{ db *sql.DB }
type sqliteTagStore struct{ db *sql.DB }
type sqliteEntryLinkStore struct{ db *sql.DB }
type sqliteProjectStore struct{ db *sql.DB }
type sqliteSearchStore struct{ db *sql.DB }
type sqliteImportExportStore struct{ db *sql.DB }

var _ = sql.NullString{}
