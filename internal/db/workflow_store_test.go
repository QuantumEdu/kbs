package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupWorkflowStore(t *testing.T) (EntryStore, WorkflowStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	entryStore := &sqliteEntryStore{db: db}
	wfStore := &sqliteWorkflowStore{db: db}
	cleanup := func() { db.Close() }
	return entryStore, wfStore, cleanup
}

func TestSaveAndGetWorkflowWithSteps(t *testing.T) {
	estore, wstore, cleanup := setupWorkflowStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "wf1", Title: "WF1", Slug: "wf1", Type: domain.EntryTypeWorkflowNote, BodyOptional: "test", Status: domain.StatusActive}
	if err := estore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save entry failed: %v", err)
	}

	w := domain.Workflow{
		ID:          "wf1",
		Name:        "Test Workflow",
		Slug:        "test-workflow",
		Description: "A test workflow",
		Status:      domain.StatusActive,
	}
	steps := []domain.WorkflowStep{
		{ID: "step-1", WorkflowID: "wf1", OrderIndex: 1, Title: "Step 1", Instruction: "First step", Required: true},
		{ID: "step-2", WorkflowID: "wf1", OrderIndex: 2, Title: "Step 2", Instruction: "Second step", Required: true},
		{ID: "step-3", WorkflowID: "wf1", OrderIndex: 3, Title: "Step 3", Instruction: "Third step", Required: false},
	}

	if err := wstore.Save(ctx, w, steps); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	got, err := wstore.Get(ctx, "wf1")
	if err != nil {
		t.Fatalf("Get workflow failed: %v", err)
	}
	if got.Name != "Test Workflow" {
		t.Errorf("Name = %q, want 'Test Workflow'", got.Name)
	}
	if got.Status != domain.StatusActive {
		t.Errorf("Status = %q, want 'active'", got.Status)
	}

	gotSteps, err := wstore.GetSteps(ctx, "wf1")
	if err != nil {
		t.Fatalf("GetSteps failed: %v", err)
	}
	if len(gotSteps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(gotSteps))
	}
	if gotSteps[0].Instruction != "First step" {
		t.Errorf("step 0 instruction = %q, want 'First step'", gotSteps[0].Instruction)
	}
	if gotSteps[1].Instruction != "Second step" {
		t.Errorf("step 1 instruction = %q, want 'Second step'", gotSteps[1].Instruction)
	}
	if !gotSteps[0].Required {
		t.Error("step 0 should be required")
	}
	if gotSteps[2].Required {
		t.Error("step 2 should not be required")
	}
}

func TestRenderWorkflow(t *testing.T) {
	_, wstore, cleanup := setupWorkflowStore(t)
	defer cleanup()
	ctx := context.Background()

	w := domain.Workflow{ID: "w1", Name: "W1", Slug: "w1", Status: domain.StatusActive}
	steps := []domain.WorkflowStep{
		{ID: "s1", WorkflowID: "w1", OrderIndex: 1, Title: "S1", Instruction: "Do this"},
	}
	if err := wstore.Save(ctx, w, steps); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	rendered, err := wstore.Render(ctx, "w1")
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if len(rendered) != 1 {
		t.Fatalf("expected 1 step, got %d", len(rendered))
	}
	if rendered[0].Instruction != "Do this" {
		t.Errorf("Instruction = %q, want 'Do this'", rendered[0].Instruction)
	}
}

func TestListWorkflows(t *testing.T) {
	_, wstore, cleanup := setupWorkflowStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, w := range []struct {
		w domain.Workflow
		s []domain.WorkflowStep
	}{
		{domain.Workflow{ID: "w1", Name: "W1", Slug: "w1", Status: domain.StatusActive}, nil},
		{domain.Workflow{ID: "w2", Name: "W2", Slug: "w2", Status: domain.StatusActive}, nil},
		{domain.Workflow{ID: "w3", Name: "W3", Slug: "w3", Status: domain.StatusArchived}, nil},
	} {
		if err := wstore.Save(ctx, w.w, w.s); err != nil {
			t.Fatalf("Save workflow %s failed: %v", w.w.ID, err)
		}
	}

	results, err := wstore.List(ctx, false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 active workflows, got %d", len(results))
	}

	results, err = wstore.List(ctx, true)
	if err != nil {
		t.Fatalf("List with include_archived failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 workflows with include_archived, got %d", len(results))
	}
}

func TestSaveWorkflowReplacesSteps(t *testing.T) {
	_, wstore, cleanup := setupWorkflowStore(t)
	defer cleanup()
	ctx := context.Background()

	w := domain.Workflow{ID: "w1", Name: "W1", Slug: "w1", Status: domain.StatusActive}
	if err := wstore.Save(ctx, w, []domain.WorkflowStep{
		{ID: "s1", WorkflowID: "w1", OrderIndex: 1, Title: "Old", Instruction: "Old step"},
	}); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	if err := wstore.Save(ctx, w, []domain.WorkflowStep{
		{ID: "s2", WorkflowID: "w1", OrderIndex: 1, Title: "New", Instruction: "New step"},
	}); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	got, err := wstore.GetSteps(ctx, "w1")
	if err != nil {
		t.Fatalf("GetSteps failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got))
	}
	if got[0].Instruction != "New step" {
		t.Errorf("Instruction = %q, want 'New step'", got[0].Instruction)
	}
}
