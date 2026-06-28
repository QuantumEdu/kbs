//go:build tui

package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/domain"
)

// stubEntryStore implements db.EntryStore for testing.
type stubEntryStore struct {
	entries map[string]domain.EntryResult
}

func (s *stubEntryStore) Save(ctx context.Context, entry domain.Entry, tags []string) error { return nil }
func (s *stubEntryStore) Get(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error) {
	r, ok := s.entries[id]
	if !ok {
		return domain.EntryResult{}, &domain.NotFoundError{Resource: "entry", ID: id}
	}
	return r, nil
}
func (s *stubEntryStore) Search(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	// Return all entries regardless of query for test determinism.
	var results []domain.EntrySearchResult
	for _, v := range s.entries {
		results = append(results, domain.EntrySearchResult{
			Entry: v.Entry,
			Tags:  v.Tags,
		})
	}
	return results, nil
}
func (s *stubEntryStore) SearchByTags(ctx context.Context, tags []string, matchAll bool, typePtr, projectPtr *string, limit int) ([]domain.EntrySearchResult, error) {
	return nil, nil
}
func (s *stubEntryStore) Archive(ctx context.Context, id string) error { return nil }
func (s *stubEntryStore) List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	return nil, nil
}

// stubProjectStore implements db.ProjectStore for testing.
type stubProjectStore struct{}

func (s *stubProjectStore) Save(ctx context.Context, p domain.Project) error           { return nil }
func (s *stubProjectStore) Get(ctx context.Context, id string) (domain.Project, error) { return domain.Project{}, nil }
func (s *stubProjectStore) List(ctx context.Context, incArch bool) ([]domain.Project, error) {
	return nil, nil
}
func (s *stubProjectStore) Archive(ctx context.Context, id string) error { return nil }

// stubArtifactStore implements db.ArtifactStore for testing.
type stubArtifactStore struct{}

func (s *stubArtifactStore) Save(ctx context.Context, a domain.Artifact) error          { return nil }
func (s *stubArtifactStore) Get(ctx context.Context, id string) (domain.Artifact, error) { return domain.Artifact{}, nil }
func (s *stubArtifactStore) List(ctx context.Context, projectID *string) ([]domain.Artifact, error) {
	return nil, nil
}

func setupTestSvc() *app.EntryService {
	entries := map[string]domain.EntryResult{
		"alpha": {
			Entry: domain.Entry{
				ID:      "alpha",
				Title:   "Alpha Entry",
				Type:    domain.EntryTypePrompt,
				Summary: "First test entry",
				Status:  domain.StatusActive,
			},
			Tags: []domain.Tag{{ID: "t1", Name: "test", Slug: "test"}},
		},
		"beta": {
			Entry: domain.Entry{
				ID:      "beta",
				Title:   "Beta Entry",
				Type:    domain.EntryTypeSkill,
				Summary: "Second test entry",
				Status:  domain.StatusDraft,
			},
			Tags: []domain.Tag{{ID: "t2", Name: "demo", Slug: "demo"}},
		},
	}
	return app.NewEntryService(
		&stubEntryStore{entries: entries},
		&stubProjectStore{},
		&stubArtifactStore{},
	)
}

func TestModelInitTriggersSearch(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a search command")
	}

	// Execute the command to get results.
	msg := cmd()
	results, ok := msg.(searchResultsMsg)
	if !ok {
		t.Fatalf("expected searchResultsMsg, got %T", msg)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 search results, got %d", len(results))
	}
}

func TestModelUpdateSearchResults(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	// Simulate search results arriving.
	results := searchResultsMsg{
		{Entry: domain.Entry{ID: "x", Title: "X", Type: domain.EntryTypePrompt, Status: domain.StatusActive}},
		{Entry: domain.Entry{ID: "y", Title: "Y", Type: domain.EntryTypeSkill, Status: domain.StatusDraft}},
	}

	newModel, cmd := m.Update(results)
	if cmd != nil {
		t.Fatalf("unexpected command after search results: %v", cmd)
	}

	nm := newModel.(*model)
	if len(nm.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(nm.entries))
	}
	if nm.loading {
		t.Fatal("expected loading to be false after results")
	}
	if nm.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", nm.cursor)
	}
}

func TestModelCursorNavigation(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	// Seed with search results.
	m.entries = []domain.EntrySearchResult{
		{Entry: domain.Entry{ID: "a", Title: "A", Type: domain.EntryTypePrompt, Status: domain.StatusActive}},
		{Entry: domain.Entry{ID: "b", Title: "B", Type: domain.EntryTypeSkill, Status: domain.StatusDraft}},
		{Entry: domain.Entry{ID: "c", Title: "C", Type: domain.EntryTypeDecision, Status: domain.StatusCanonical}},
	}
	m.cursor = 0
	m.loading = false
	m.width = 80
	m.height = 24

	// Move down twice.
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ := m.Update(msg)
	if nm := newModel.(*model); nm.cursor != 1 {
		t.Fatalf("after first 'j': expected cursor 1, got %d", nm.cursor)
	}

	newModel, _ = newModel.(*model).Update(msg)
	if nm := newModel.(*model); nm.cursor != 2 {
		t.Fatalf("after second 'j': expected cursor 2, got %d", nm.cursor)
	}

	// Should not go past end.
	newModel, _ = newModel.(*model).Update(msg)
	if nm := newModel.(*model); nm.cursor != 2 {
		t.Fatalf("cursor should stay at 2 (end), got %d", nm.cursor)
	}

	// Move up.
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ = newModel.(*model).Update(up)
	if nm := newModel.(*model); nm.cursor != 1 {
		t.Fatalf("after 'k': expected cursor 1, got %d", nm.cursor)
	}

	// Should not go before start.
	newModel, _ = newModel.(*model).Update(up)
	newModel, _ = newModel.(*model).Update(up)
	if nm := newModel.(*model); nm.cursor != 0 {
		t.Fatalf("cursor should stay at 0, got %d", nm.cursor)
	}
}

func TestModelEntrySelectionNavigatesToDetail(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	m.entries = []domain.EntrySearchResult{
		{Entry: domain.Entry{ID: "alpha", Title: "Alpha", Type: domain.EntryTypePrompt, Status: domain.StatusActive}},
	}
	m.cursor = 0
	m.loading = false
	m.width = 80
	m.height = 24

	// Press Enter to select entry.
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := m.Update(enter)
	if cmd == nil {
		t.Fatal("expected a command for fetching entry detail after Enter")
	}

	// Execute the command to get the detail message.
	msg := cmd()
	detail, ok := msg.(entryDetailMsg)
	if !ok {
		t.Fatalf("expected entryDetailMsg, got %T", msg)
	}
	if detail.err != nil {
		t.Fatalf("unexpected error: %v", detail.err)
	}

	// Apply the detail message to switch to detail view.
	newModel, _ = newModel.(*model).Update(msg)
	m2 := newModel.(*model)
	if m2.state != stateDetail {
		t.Fatalf("expected stateDetail after selection, got %v", m2.state)
	}
	if m2.detail == nil {
		t.Fatal("expected detail to be set")
	}
}

func TestModelEscFromDetailToBrowse(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	// Put model in detail state.
	m.state = stateDetail
	m.detail = &app.GetEntryResult{
		Entry: domain.EntryResult{
			Entry: domain.Entry{ID: "x", Title: "X", Type: domain.EntryTypePrompt, Status: domain.StatusActive},
		},
	}
	m.width = 80
	m.height = 24

	// Press Esc to go back.
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ := m.Update(esc)
	nm := newModel.(*model)
	if nm.state != stateBrowse {
		t.Fatalf("expected stateBrowse after Esc, got %v", nm.state)
	}
	if nm.detail != nil {
		t.Fatal("expected detail to be cleared after Esc")
	}
}

func TestModelQuitFromBrowse(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)
	m.state = stateBrowse
	m.width = 80
	m.height = 24

	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(q)
	if cmd == nil {
		t.Fatal("expected quit command on 'q'")
	}
	// tea.Quit returns a quit Msg when executed.
	quitMsg := cmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", quitMsg)
	}
}

func TestModelViewRendersBrowse(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	m.entries = []domain.EntrySearchResult{
		{Entry: domain.Entry{ID: "a", Title: "Alpha", Type: domain.EntryTypePrompt, Status: domain.StatusActive}},
	}
	m.cursor = 0
	m.loading = false
	m.width = 80
	m.height = 24

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty browse view")
	}
	if !contains(view, "SkillVault") {
		t.Error("browse view should contain 'SkillVault' title")
	}
	if !contains(view, "Alpha") {
		t.Error("browse view should contain entry title 'Alpha'")
	}
	if !contains(view, "prompt") {
		t.Error("browse view should contain entry type 'prompt'")
	}
}

func TestModelViewRendersDetail(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	m.state = stateDetail
	m.detail = &app.GetEntryResult{
		Entry: domain.EntryResult{
			Entry: domain.Entry{ID: "alpha", Title: "Alpha Entry", Type: domain.EntryTypePrompt, Summary: "Test summary", BodyOptional: "Body content", Status: domain.StatusActive},
			Tags:  []domain.Tag{{ID: "t1", Name: "test", Slug: "test"}},
		},
	}
	m.loading = false
	m.width = 80
	m.height = 24

	view := m.View()
	if !contains(view, "Alpha Entry") {
		t.Error("detail view should contain entry title")
	}
	if !contains(view, "alpha") {
		t.Error("detail view should contain entry ID")
	}
	if !contains(view, "Test summary") {
		t.Error("detail view should contain summary")
	}
	if !contains(view, "Body content") {
		t.Error("detail view should contain body")
	}
	if !contains(view, "test") {
		t.Error("detail view should contain tags")
	}
	if !contains(view, "← esc/q back to list") {
		t.Error("detail view should contain back hint")
	}
}

func TestModelWindowResize(t *testing.T) {
	svc := setupTestSvc()
	m := NewModel(svc)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, _ := m.Update(msg)

	nm := newModel.(*model)
	if nm.width != 120 {
		t.Fatalf("expected width 120, got %d", nm.width)
	}
	if nm.height != 40 {
		t.Fatalf("expected height 40, got %d", nm.height)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
