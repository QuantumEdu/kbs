package app

import (
	"context"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// ContextService provides project context (entries + series).
type ContextService struct {
	entryStore   db.EntryStore
	projectStore db.ProjectStore
	seriesStore  db.SeriesStore
}

// ProjectContext is the result of GetContext.
type ProjectContext struct {
	Project domain.Project        `json:"project"`
	Entries []domain.EntryListResult `json:"entries"`
	Series  []domain.SeriesListResult `json:"series"`
}

// NewContextService creates a new ContextService.
func NewContextService(entryStore db.EntryStore, projectStore db.ProjectStore, seriesStore db.SeriesStore) *ContextService {
	return &ContextService{
		entryStore:   entryStore,
		projectStore: projectStore,
		seriesStore:  seriesStore,
	}
}

// GetContext returns entries and series for a project.
func (s *ContextService) GetContext(ctx context.Context, projectID string) (*ProjectContext, error) {
	// Find the project
	projects, err := s.projectStore.ListProjects(ctx, true)
	if err != nil {
		return nil, err
	}

	var project *domain.Project
	for _, p := range projects {
		if p.ID == projectID {
			project = &p
			break
		}
	}
	if project == nil {
		return nil, &domain.NotFoundError{Resource: "project", ID: projectID}
	}

	// Get entries for this project (including global entries)
	entries, err := s.entryStore.ListEntries(ctx, domain.EntryFilter{})
	if err != nil {
		return nil, err
	}

	var projectEntries []domain.EntryListResult
	for _, e := range entries {
		if e.Entry.ProjectID != nil && *e.Entry.ProjectID == projectID {
			projectEntries = append(projectEntries, e)
		}
	}
	if projectEntries == nil {
		projectEntries = []domain.EntryListResult{}
	}

	// Get series for this project
	series, err := s.seriesStore.ListSeries(ctx, domain.SeriesFilter{ProjectID: &projectID})
	if err != nil {
		return nil, err
	}

	return &ProjectContext{
		Project: *project,
		Entries: projectEntries,
		Series:  series,
	}, nil
}
