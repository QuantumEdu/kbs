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

func TestWorkflowStepsUpsertAndGet(t *testing.T) {
	estore, wstore, cleanup := setupWorkflowStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create workflow entry
	entry := domain.Entry{ID: "wf1", Name: "WF1", Type: domain.EntryTypeWorkflow, Content: "test", Active: true}
	if err := estore.UpsertEntry(ctx, entry, nil, nil); err != nil {
		t.Fatalf("UpsertEntry failed: %v", err)
	}

	// Upsert steps
	steps := []domain.WorkflowStep{
		{StepNum: 1, Role: domain.WorkflowRoleSystem, Content: "System prompt", Label: "Setup"},
		{StepNum: 2, Role: domain.WorkflowRoleUser, Content: "User input", Label: "Input"},
		{StepNum: 3, Role: domain.WorkflowRoleAssistant, Content: "Response", Label: "Output"},
	}
	if err := wstore.UpsertWorkflowSteps(ctx, "wf1", steps); err != nil {
		t.Fatalf("UpsertWorkflowSteps failed: %v", err)
	}

	// Get steps
	got, err := wstore.GetWorkflowSteps(ctx, "wf1")
	if err != nil {
		t.Fatalf("GetWorkflowSteps failed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(got))
	}
	if got[0].Role != domain.WorkflowRoleSystem {
		t.Errorf("step 0 role = %q, want 'system'", got[0].Role)
	}
	if got[1].Role != domain.WorkflowRoleUser {
		t.Errorf("step 1 role = %q, want 'user'", got[1].Role)
	}
	if got[2].Role != domain.WorkflowRoleAssistant {
		t.Errorf("step 2 role = %q, want 'assistant'", got[2].Role)
	}
}

func TestWorkflowStepsReplacePreservesOrder(t *testing.T) {
	estore, wstore, cleanup := setupWorkflowStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "wf1", Name: "WF1", Type: domain.EntryTypeWorkflow, Content: "test", Active: true}
	estore.UpsertEntry(ctx, entry, nil, nil)

	// First set of steps
	wstore.UpsertWorkflowSteps(ctx, "wf1", []domain.WorkflowStep{
		{StepNum: 1, Role: domain.WorkflowRoleSystem, Content: "Old"},
	})
	// Replace
	wstore.UpsertWorkflowSteps(ctx, "wf1", []domain.WorkflowStep{
		{StepNum: 1, Role: domain.WorkflowRoleUser, Content: "New"},
	})

	got, err := wstore.GetWorkflowSteps(ctx, "wf1")
	if err != nil {
		t.Fatalf("GetWorkflowSteps failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got))
	}
	if got[0].Content != "New" {
		t.Errorf("Content = %q, want 'New'", got[0].Content)
	}
	if got[0].Role != domain.WorkflowRoleUser {
		t.Errorf("Role = %q, want 'user'", got[0].Role)
	}
}
