package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/quantum-6/skillvault/internal/line"
)

type dummyExec struct{}

func (d dummyExec) Run(ctx context.Context, phase, prompt string) (string, error) {
	return `{"status":"ok","summary":"dummy"}`, nil
}

func testEngine(t *testing.T) (*line.Engine, *line.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_tui.db")
	store, err := line.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	engine := line.NewEngine(store, dummyExec{}, "")
	return engine, store
}

func TestTUIModelInitAndWindowSize(t *testing.T) {
	t.Parallel()
	engine, _ := testEngine(t)
	m := NewModel(engine)

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil Init cmd (tick)")
	}

	// Send window size message
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)
	if model.width != 120 || model.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", model.width, model.height)
	}

	view := model.View()
	if !strings.Contains(view, "LINE FACTORY") {
		t.Fatalf("view missing header: %s", view)
	}
}

func TestTUIModalTransitions(t *testing.T) {
	t.Parallel()
	engine, store := testEngine(t)

	// Create a project
	ctx := context.Background()
	_ = store.CreateProject(ctx, line.Project{
		ID:       "proj1",
		Name:     "Project One",
		RepoPath: "/path/one",
	})

	m := NewModel(engine)

	// 1. Press 'n' -> switch to ModeNewJob
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)
	if model.mode != ModeNewJob {
		t.Fatalf("expected ModeNewJob, got %v", model.mode)
	}
	view := model.View()
	if !strings.Contains(view, "Start New Job on") {
		t.Fatalf("view missing new job modal: %s", view)
	}

	// 2. Press 'esc' -> return to ModeMain
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.mode != ModeMain {
		t.Fatalf("expected ModeMain after esc, got %v", model.mode)
	}

	// 3. Press 'p' -> switch to ModeSwitchProject
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(Model)
	if model.mode != ModeSwitchProject {
		t.Fatalf("expected ModeSwitchProject, got %v", model.mode)
	}
	view = model.View()
	if !strings.Contains(view, "Select Active Project") {
		t.Fatalf("view missing project switch modal: %s", view)
	}

	// 4. Press 'esc' -> return to ModeMain
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.mode != ModeMain {
		t.Fatalf("expected ModeMain after esc, got %v", model.mode)
	}
}

func TestTUIContinueNeedsHuman(t *testing.T) {
	t.Parallel()
	engine, store := testEngine(t)
	ctx := context.Background()

	// Seed a job in needs_human
	job := line.Job{
		ID:       "job-test-1",
		IssueURL: "https://github.com/o/r/issues/42",
		RepoPath: "/path/repo",
		Phase:    line.PhasePlan,
		Status:   line.StatusNeedsHuman,
		Question: "Confirm architecture?",
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	m := NewModel(engine)
	m.reload(ctx)

	// Press 'c' to open continue modal
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model := updated.(Model)
	if model.mode != ModeContinue {
		t.Fatalf("expected ModeContinue, got %v", model.mode)
	}
	view := model.View()
	if !strings.Contains(view, "Continue Job") {
		t.Fatalf("view missing continue modal: %s", view)
	}
}
