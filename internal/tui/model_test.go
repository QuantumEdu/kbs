//go:build tui

package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/domain"
)

type stubEntryStore struct {
	entries map[string]domain.EntryResult
	order   []string
}

func (s *stubEntryStore) Save(ctx context.Context, entry domain.Entry, tags []string) error {
	return nil
}

func (s *stubEntryStore) Get(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error) {
	result, ok := s.entries[id]
	if !ok {
		return domain.EntryResult{}, &domain.NotFoundError{Resource: "entry", ID: id}
	}
	if !includeArchived && result.Entry.Status == domain.StatusArchived {
		return domain.EntryResult{}, &domain.NotFoundError{Resource: "entry", ID: id}
	}
	return result, nil
}

func (s *stubEntryStore) Search(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	results := make([]domain.EntrySearchResult, 0)
	query := strings.ToLower(strings.TrimSpace(q.Query))
	for _, id := range s.order {
		result := s.entries[id]
		if !q.IncludeArchived && result.Entry.Status == domain.StatusArchived {
			continue
		}
		if q.ProjectID != nil {
			if result.Entry.ProjectID == nil || *result.Entry.ProjectID != *q.ProjectID {
				continue
			}
		}
		if query != "" {
			haystack := strings.ToLower(result.Entry.Title + " " + result.Entry.Summary + " " + result.Entry.BodyOptional)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		results = append(results, domain.EntrySearchResult{Entry: result.Entry, Tags: result.Tags})
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	return results, nil
}

func (s *stubEntryStore) SearchByTags(ctx context.Context, tags []string, matchAll bool, typePtr, projectPtr *string, limit int) ([]domain.EntrySearchResult, error) {
	return nil, nil
}

func (s *stubEntryStore) Archive(ctx context.Context, id string) error {
	result, ok := s.entries[id]
	if !ok {
		return &domain.NotFoundError{Resource: "entry", ID: id}
	}
	result.Entry.Status = domain.StatusArchived
	s.entries[id] = result
	return nil
}

func (s *stubEntryStore) List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	items := make([]domain.EntryListResult, 0)
	for _, id := range s.order {
		result := s.entries[id]
		if !filter.IncludeArchived && result.Entry.Status == domain.StatusArchived {
			continue
		}
		if filter.ProjectID != nil {
			if result.Entry.ProjectID == nil || *result.Entry.ProjectID != *filter.ProjectID {
				continue
			}
		}
		if filter.Type != nil && string(result.Entry.Type) != *filter.Type {
			continue
		}
		items = append(items, domain.EntryListResult{Entry: result.Entry, Tags: result.Tags})
	}
	return items, nil
}

type stubProjectStore struct {
	projects []domain.Project
	index    map[string]domain.Project
}

func (s *stubProjectStore) Save(ctx context.Context, p domain.Project) error { return nil }
func (s *stubProjectStore) Get(ctx context.Context, id string) (domain.Project, error) {
	project, ok := s.index[id]
	if !ok {
		return domain.Project{}, &domain.NotFoundError{Resource: "project", ID: id}
	}
	return project, nil
}
func (s *stubProjectStore) List(ctx context.Context, includeArchived bool) ([]domain.Project, error) {
	if includeArchived {
		return s.projects, nil
	}
	active := make([]domain.Project, 0, len(s.projects))
	for _, project := range s.projects {
		if project.Status != domain.StatusArchived {
			active = append(active, project)
		}
	}
	return active, nil
}
func (s *stubProjectStore) Archive(ctx context.Context, id string) error { return nil }

type stubArtifactStore struct{}

func (s *stubArtifactStore) Save(ctx context.Context, a domain.Artifact) error { return nil }
func (s *stubArtifactStore) Get(ctx context.Context, id string) (domain.Artifact, error) {
	return domain.Artifact{}, &domain.NotFoundError{Resource: "artifact", ID: id}
}
func (s *stubArtifactStore) List(ctx context.Context, projectID *string) ([]domain.Artifact, error) {
	return nil, nil
}

type stubWorkflowStore struct{}

func (s *stubWorkflowStore) Save(ctx context.Context, w domain.Workflow, steps []domain.WorkflowStep) error {
	return nil
}
func (s *stubWorkflowStore) Get(ctx context.Context, id string) (domain.Workflow, error) {
	return domain.Workflow{}, &domain.NotFoundError{Resource: "workflow", ID: id}
}
func (s *stubWorkflowStore) GetSteps(ctx context.Context, workflowID string) ([]domain.WorkflowStep, error) {
	return nil, nil
}
func (s *stubWorkflowStore) Render(ctx context.Context, id string) ([]domain.WorkflowStep, error) {
	return nil, nil
}
func (s *stubWorkflowStore) List(ctx context.Context, includeArchived bool) ([]domain.Workflow, error) {
	return nil, nil
}

type stubSeriesStore struct{}

func (s *stubSeriesStore) Save(ctx context.Context, series domain.Series) error { return nil }
func (s *stubSeriesStore) Get(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error) {
	return domain.SeriesResult{}, &domain.NotFoundError{Resource: "series", ID: id}
}
func (s *stubSeriesStore) Compose(ctx context.Context, seriesID string) ([]domain.Entry, error) {
	return nil, nil
}
func (s *stubSeriesStore) List(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error) {
	return nil, nil
}
func (s *stubSeriesStore) ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntry) error {
	return nil
}

func setupTestModel() *model {
	projectA := domain.Project{ID: "codex", Name: "Codex", Description: "CLI intelligence work", Status: domain.StatusActive}
	projectB := domain.Project{ID: "atlas", Name: "Atlas", Description: "Release prep", Status: domain.StatusActive}

	entryProject := func(projectID string) *string {
		return &projectID
	}

	entryStore := &stubEntryStore{
		order: []string{"pending-codex", "done-codex", "entry-codex", "decision-codex", "session-codex", "pending-atlas", "entry-atlas", "session-atlas"},
		entries: map[string]domain.EntryResult{
			"pending-codex":  {Entry: domain.Entry{ID: "pending-codex", Title: "Review pending rollout", Type: domain.EntryTypePending, Summary: "Review pending rollout", BodyOptional: "Keep output concise", Status: domain.StatusActive, ProjectID: entryProject("codex")}},
			"done-codex":     {Entry: domain.Entry{ID: "done-codex", Title: "Shipped rollout notes", Type: domain.EntryTypePending, Summary: "Shipped rollout notes", BodyOptional: "Archived after release", Status: domain.StatusArchived, ProjectID: entryProject("codex")}},
			"entry-codex":    {Entry: domain.Entry{ID: "entry-codex", Title: "Intent-first CLI", Type: domain.EntryTypeSkill, Summary: "Natural command routing", BodyOptional: "search help routing", Status: domain.StatusActive, ProjectID: entryProject("codex")}},
			"decision-codex": {Entry: domain.Entry{ID: "decision-codex", Title: "Use pending in context", Type: domain.EntryTypeDecision, Summary: "Surface deferred work", Status: domain.StatusActive, ProjectID: entryProject("codex")}},
			"session-codex":  {Entry: domain.Entry{ID: "session-codex", Title: "Recent codex session", Type: domain.EntryTypeSession, Summary: "Queued TUI work", Status: domain.StatusActive, ProjectID: entryProject("codex")}},
			"pending-atlas":  {Entry: domain.Entry{ID: "pending-atlas", Title: "Finalize release notes", Type: domain.EntryTypePending, Summary: "Finalize release notes", BodyOptional: "Ship Friday", Status: domain.StatusActive, ProjectID: entryProject("atlas")}},
			"entry-atlas":    {Entry: domain.Entry{ID: "entry-atlas", Title: "Deploy checklist", Type: domain.EntryTypeReference, Summary: "Deployment steps", BodyOptional: "deploy staging prod", Status: domain.StatusActive, ProjectID: entryProject("atlas")}},
			"session-atlas":  {Entry: domain.Entry{ID: "session-atlas", Title: "Atlas wrap", Type: domain.EntryTypeSession, Summary: "Release prep recap", Status: domain.StatusActive, ProjectID: entryProject("atlas")}},
		},
	}

	projectStore := &stubProjectStore{
		projects: []domain.Project{projectA, projectB},
		index: map[string]domain.Project{
			projectA.ID: projectA,
			projectB.ID: projectB,
		},
	}

	artifactStore := &stubArtifactStore{}
	entrySvc := app.NewEntryService(entryStore, projectStore, artifactStore)
	projectSvc := app.NewProjectService(projectStore)
	contextSvc := app.NewContextService(entryStore, projectStore, &stubSeriesStore{}, &stubWorkflowStore{}, artifactStore, entrySvc)
	return NewModel(entrySvc, projectSvc, contextSvc)
}

func applyDashboardLoad(t *testing.T, m *model, cmd tea.Cmd) *model {
	t.Helper()
	msg := cmd()
	loaded, ok := msg.(dashboardLoadedMsg)
	if !ok {
		t.Fatalf("expected dashboardLoadedMsg, got %T", msg)
	}
	updated, _ := m.Update(loaded)
	return updated.(*model)
}

func TestModelInitLoadsDashboard(t *testing.T) {
	m := setupTestModel()
	m.width = 140
	m.height = 40

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a dashboard load command")
	}

	m = applyDashboardLoad(t, m, cmd)
	if len(m.projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(m.projects))
	}
	if m.selectedProjectID() != "codex" {
		t.Fatalf("selected project = %q, want codex", m.selectedProjectID())
	}
	if len(m.pending) != 1 || m.pending[0].Entry.ID != "pending-codex" {
		t.Fatalf("unexpected pending list: %+v", m.pending)
	}
	if stats := m.pendingStatsFor("codex"); stats.Open != 1 || stats.Done != 1 {
		t.Fatalf("unexpected codex stats: %+v", stats)
	}
	if len(m.results) == 0 {
		t.Fatal("expected search results for selected project")
	}
	if !strings.Contains(m.contextPreview, "## Active Pending") {
		t.Fatalf("expected pending section in context preview, got: %s", m.contextPreview)
	}
}

func TestModelProjectNavigationReloadsDashboard(t *testing.T) {
	m := setupTestModel()
	m.width = 140
	m.height = 40
	m = applyDashboardLoad(t, m, m.Init())

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd == nil {
		t.Fatal("expected reload command when changing project")
	}
	m = applyDashboardLoad(t, updated.(*model), cmd)

	if m.selectedProjectID() != "atlas" {
		t.Fatalf("selected project = %q, want atlas", m.selectedProjectID())
	}
	if len(m.pending) != 1 || m.pending[0].Entry.ID != "pending-atlas" {
		t.Fatalf("unexpected pending for atlas: %+v", m.pending)
	}
	if !strings.Contains(m.contextPreview, "Finalize release notes") {
		t.Fatalf("expected atlas context preview, got: %s", m.contextPreview)
	}
}

func TestModelSearchFiltersSelectedProjectEntries(t *testing.T) {
	m := setupTestModel()
	m.width = 140
	m.height = 40
	m = applyDashboardLoad(t, m, m.Init())

	m.searchInput.Focus()
	m.searchInput.SetValue("routing")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected reload command after search enter")
	}
	m = applyDashboardLoad(t, updated.(*model), cmd)

	if m.searchQuery != "routing" {
		t.Fatalf("search query = %q, want routing", m.searchQuery)
	}
	if len(m.results) != 1 || m.results[0].Entry.ID != "entry-codex" {
		t.Fatalf("unexpected filtered results: %+v", m.results)
	}
}

func TestModelPendingEnterLoadsDetail(t *testing.T) {
	m := setupTestModel()
	m.width = 140
	m.height = 40
	m = applyDashboardLoad(t, m, m.Init())
	m.focus = focusPending

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected detail command from pending pane")
	}
	msg := cmd()
	detail, ok := msg.(entryDetailMsg)
	if !ok {
		t.Fatalf("expected entryDetailMsg, got %T", msg)
	}
	if detail.err != nil {
		t.Fatalf("unexpected detail error: %v", detail.err)
	}
	updated, _ = updated.(*model).Update(detail)
	m = updated.(*model)

	if m.state != stateDetail {
		t.Fatalf("state = %v, want detail", m.state)
	}
	if m.detail == nil || m.detail.Entry.Entry.ID != "pending-codex" {
		t.Fatalf("unexpected detail: %+v", m.detail)
	}
}

func TestModelResolvePendingWithConfirmation(t *testing.T) {
	m := setupTestModel()
	m.width = 140
	m.height = 40
	m = applyDashboardLoad(t, m, m.Init())
	m.focus = focusPending

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("did not expect command on confirmation prompt")
	}
	m = updated.(*model)
	if !m.confirmResolve {
		t.Fatal("expected confirmResolve to be enabled")
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected resolve command after confirmation")
	}
	msg := cmd()
	resolved, ok := msg.(resolvePendingMsg)
	if !ok {
		t.Fatalf("expected resolvePendingMsg, got %T", msg)
	}
	if resolved.err != nil {
		t.Fatalf("unexpected resolve error: %v", resolved.err)
	}
	updated, _ = updated.(*model).Update(resolved)
	m = updated.(*model)

	if m.confirmResolve {
		t.Fatal("expected confirmation mode to close after resolving")
	}
	if len(m.pending) != 0 {
		t.Fatalf("expected pending list to be empty after resolve, got %+v", m.pending)
	}
	if !strings.Contains(m.statusMessage, "Marked pending item done") {
		t.Fatalf("unexpected status message: %q", m.statusMessage)
	}
	if stats := m.pendingStatsFor("codex"); stats.Open != 0 || stats.Done != 2 {
		t.Fatalf("unexpected codex stats after resolve: %+v", stats)
	}
	if strings.Contains(m.contextPreview, "Review pending rollout") {
		t.Fatalf("resolved pending item should not remain in context preview: %s", m.contextPreview)
	}
}

func TestDashboardViewShowsAllSurfaces(t *testing.T) {
	m := setupTestModel()
	m.width = 140
	m.height = 40
	m = applyDashboardLoad(t, m, m.Init())

	view := m.View()
	for _, expected := range []string{"Projects", "Pending", "Browse", "Context Preview", "SkillVault TUI"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected %q in view, got: %s", expected, view)
		}
	}
	for _, expected := range []string{"1 open | 1/2 done", "Pending: 1 open | 1/2 done"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected %q in view, got: %s", expected, view)
		}
	}
}
