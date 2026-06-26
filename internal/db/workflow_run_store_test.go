package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupRunStore(t *testing.T) (EntryStore, WorkflowStore, WorkflowRunStore, func()) {
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
	runStore := &sqliteWorkflowRunStore{db: db}
	cleanup := func() { db.Close() }
	return entryStore, wfStore, runStore, cleanup
}

func TestCreateRunSuccess(t *testing.T) {
	estore, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create prerequisite entry and workflow
	entry := domain.Entry{ID: "entry-abc", Title: "Entry ABC", Slug: "entry-abc", Type: domain.EntryTypePrompt, BodyOptional: "body", Status: domain.StatusActive}
	if err := estore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save entry failed: %v", err)
	}

	w := domain.Workflow{ID: "wf-abc", Name: "Test WF", Slug: "test-wf", Status: domain.StatusActive}
	steps := []domain.WorkflowStep{
		{ID: "s1", WorkflowID: "wf-abc", OrderIndex: 1, Title: "Step 1", Instruction: "First", EntrySlug: "entry-abc"},
		{ID: "s2", WorkflowID: "wf-abc", OrderIndex: 2, Title: "Step 2", Instruction: "Second"},
	}
	if err := wstore.Save(ctx, w, steps); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	// Get steps to know their real IDs
	gotSteps, err := wstore.GetSteps(ctx, "wf-abc")
	if err != nil {
		t.Fatalf("GetSteps failed: %v", err)
	}

	run := domain.WorkflowRun{
		ID:         "run-001",
		WorkflowID: "wf-abc",
		Input:      "hello world",
		Status:     domain.RunStatusPending,
	}
	runSteps := []domain.WorkflowRunStep{
		{ID: "rst-001", RunID: "run-001", StepID: parseStepID(t, gotSteps[0].ID), EntryID: "entry-abc", Status: domain.RunStatusPending},
		{ID: "rst-002", RunID: "run-001", StepID: parseStepID(t, gotSteps[1].ID), EntryID: "entry-abc", Status: domain.RunStatusPending},
	}

	if err := rstore.CreateRun(ctx, run, runSteps); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Verify run was created
	gotRun, gotSteps2, err := rstore.GetRun(ctx, "run-001")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if gotRun.ID != "run-001" {
		t.Errorf("run ID = %q, want %q", gotRun.ID, "run-001")
	}
	if gotRun.WorkflowID != "wf-abc" {
		t.Errorf("workflow_id = %q, want %q", gotRun.WorkflowID, "wf-abc")
	}
	if gotRun.Input != "hello world" {
		t.Errorf("input = %q, want %q", gotRun.Input, "hello world")
	}
	if gotRun.Status != domain.RunStatusPending {
		t.Errorf("status = %q, want %q", gotRun.Status, domain.RunStatusPending)
	}

	if len(gotSteps2) != 2 {
		t.Fatalf("expected 2 run steps, got %d", len(gotSteps2))
	}
	if gotSteps2[0].ID != "rst-001" {
		t.Errorf("step[0] ID = %q, want %q", gotSteps2[0].ID, "rst-001")
	}
	if gotSteps2[0].Status != domain.RunStatusPending {
		t.Errorf("step[0] status = %q, want pending", gotSteps2[0].Status)
	}
}

func TestCreateRunEmptySteps(t *testing.T) {
	_, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	w := domain.Workflow{ID: "wf-empty", Name: "Empty WF", Slug: "empty-wf", Status: domain.StatusActive}
	if err := wstore.Save(ctx, w, nil); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	run := domain.WorkflowRun{
		ID:         "run-empty",
		WorkflowID: "wf-empty",
		Status:     domain.RunStatusPending,
	}
	if err := rstore.CreateRun(ctx, run, nil); err != nil {
		t.Fatalf("CreateRun with empty steps failed: %v", err)
	}

	gotRun, gotSteps, err := rstore.GetRun(ctx, "run-empty")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if gotRun.ID != "run-empty" {
		t.Errorf("run ID = %q, want 'run-empty'", gotRun.ID)
	}
	if len(gotSteps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(gotSteps))
	}
}

func TestCreateRunDuplicateID(t *testing.T) {
	_, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	w := domain.Workflow{ID: "wf-dup", Name: "Dup WF", Slug: "dup-wf", Status: domain.StatusActive}
	if err := wstore.Save(ctx, w, nil); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	run := domain.WorkflowRun{ID: "run-dup", WorkflowID: "wf-dup", Status: domain.RunStatusPending}
	if err := rstore.CreateRun(ctx, run, nil); err != nil {
		t.Fatalf("first CreateRun failed: %v", err)
	}

	// Duplicate should fail
	run2 := domain.WorkflowRun{ID: "run-dup", WorkflowID: "wf-dup", Status: domain.RunStatusPending}
	if err := rstore.CreateRun(ctx, run2, nil); err == nil {
		t.Fatal("expected error on duplicate run ID, got nil")
	}
}

func TestGetRunNotFound(t *testing.T) {
	_, _, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	_, _, err := rstore.GetRun(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run, got nil")
	}
}

func TestListRuns(t *testing.T) {
	_, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	w := domain.Workflow{ID: "wf-list", Name: "List WF", Slug: "list-wf", Status: domain.StatusActive}
	if err := wstore.Save(ctx, w, nil); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	for i := 1; i <= 3; i++ {
		id := "run-" + string(rune('0'+i))
		run := domain.WorkflowRun{ID: id, WorkflowID: "wf-list", Status: domain.RunStatusPending}
		if err := rstore.CreateRun(ctx, run, nil); err != nil {
			t.Fatalf("CreateRun %s failed: %v", id, err)
		}
	}

	runs, err := rstore.ListRuns(ctx, "wf-list", 10)
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(runs))
	}

	// Test limit
	runs2, err := rstore.ListRuns(ctx, "wf-list", 2)
	if err != nil {
		t.Fatalf("ListRuns with limit failed: %v", err)
	}
	if len(runs2) != 2 {
		t.Errorf("expected 2 runs with limit, got %d", len(runs2))
	}
}

func TestUpdateStepStatus(t *testing.T) {
	estore, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "entry-upd", Title: "Entry", Slug: "entry-upd", Type: domain.EntryTypePrompt, BodyOptional: "body", Status: domain.StatusActive}
	if err := estore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save entry failed: %v", err)
	}

	w := domain.Workflow{ID: "wf-upd", Name: "Update WF", Slug: "update-wf", Status: domain.StatusActive}
	steps := []domain.WorkflowStep{
		{ID: "s1", WorkflowID: "wf-upd", OrderIndex: 1, Title: "S1", Instruction: "Step", EntrySlug: "entry-upd"},
	}
	if err := wstore.Save(ctx, w, steps); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	gotSteps, _ := wstore.GetSteps(ctx, "wf-upd")

	run := domain.WorkflowRun{ID: "run-upd", WorkflowID: "wf-upd", Status: domain.RunStatusPending}
	runSteps := []domain.WorkflowRunStep{
		{ID: "rst-upd", RunID: "run-upd", StepID: parseStepID(t, gotSteps[0].ID), EntryID: "entry-upd", Status: domain.RunStatusPending},
	}
	if err := rstore.CreateRun(ctx, run, runSteps); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Update to running
	if err := rstore.UpdateStepStatus(ctx, "rst-upd", domain.RunStatusRunning, ""); err != nil {
		t.Fatalf("UpdateStepStatus to running failed: %v", err)
	}

	// Update to completed with output
	if err := rstore.UpdateStepStatus(ctx, "rst-upd", domain.RunStatusCompleted, "result data"); err != nil {
		t.Fatalf("UpdateStepStatus to completed failed: %v", err)
	}

	// Verify status in run
	_, gotRunSteps, err := rstore.GetRun(ctx, "run-upd")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if len(gotRunSteps) != 1 {
		t.Fatalf("expected 1 run step, got %d", len(gotRunSteps))
	}
	if gotRunSteps[0].Status != domain.RunStatusCompleted {
		t.Errorf("step status = %q, want %q", gotRunSteps[0].Status, domain.RunStatusCompleted)
	}
	if gotRunSteps[0].Output != "result data" {
		t.Errorf("step output = %q, want %q", gotRunSteps[0].Output, "result data")
	}
	if gotRunSteps[0].FinishedAt == nil {
		t.Error("FinishedAt should be set after completion")
	}
}

func TestUpdateStepStatusFailed(t *testing.T) {
	estore, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "entry-fail", Title: "Entry", Slug: "entry-fail", Type: domain.EntryTypePrompt, BodyOptional: "body", Status: domain.StatusActive}
	if err := estore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save entry failed: %v", err)
	}

	w := domain.Workflow{ID: "wf-fail", Name: "Fail WF", Slug: "fail-wf", Status: domain.StatusActive}
	steps := []domain.WorkflowStep{
		{ID: "s1", WorkflowID: "wf-fail", OrderIndex: 1, Title: "S1", Instruction: "Step", EntrySlug: "entry-fail"},
	}
	if err := wstore.Save(ctx, w, steps); err != nil {
		t.Fatalf("Save workflow failed: %v", err)
	}

	gotSteps, _ := wstore.GetSteps(ctx, "wf-fail")

	run := domain.WorkflowRun{ID: "run-fail", WorkflowID: "wf-fail", Status: domain.RunStatusPending}
	runSteps := []domain.WorkflowRunStep{
		{ID: "rst-fail", RunID: "run-fail", StepID: parseStepID(t, gotSteps[0].ID), EntryID: "entry-fail", Status: domain.RunStatusPending},
	}
	if err := rstore.CreateRun(ctx, run, runSteps); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Update to failed
	if err := rstore.UpdateStepStatus(ctx, "rst-fail", domain.RunStatusFailed, "error: something broke"); err != nil {
		t.Fatalf("UpdateStepStatus to failed failed: %v", err)
	}

	_, gotRunSteps, err := rstore.GetRun(ctx, "run-fail")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if gotRunSteps[0].Status != domain.RunStatusFailed {
		t.Errorf("step status = %q, want %q", gotRunSteps[0].Status, domain.RunStatusFailed)
	}
	if gotRunSteps[0].Output != "error: something broke" {
		t.Errorf("step output = %q, want 'error: something broke'", gotRunSteps[0].Output)
	}
}

func parseStepID(t *testing.T, idStr string) int64 {
	t.Helper()
	id := int64(0)
	for _, c := range idStr {
		if c >= '0' && c <= '9' {
			id = id*10 + int64(c-'0')
		}
	}
	return id
}
