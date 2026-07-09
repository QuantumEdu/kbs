package db

import (
	"context"
	"fmt"
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

// createSeededRun is a helper that creates an entry, workflow, and runs with given counts.
func createSeededRun(t *testing.T, ctx context.Context, estore EntryStore, wstore WorkflowStore, rstore WorkflowRunStore, runID, entryID, wfID string, status domain.RunStatus, stepStatuses []domain.RunStatus) {
	t.Helper()
	gotSteps, err := wstore.GetSteps(ctx, wfID)
	if err != nil {
		t.Fatalf("GetSteps: %v", err)
	}
	var rs []domain.WorkflowRunStep
	for i, ss := range stepStatuses {
		if i >= len(gotSteps) {
			t.Fatalf("need %d workflow steps but only %d exist", len(stepStatuses), len(gotSteps))
		}
		rs = append(rs, domain.WorkflowRunStep{
			ID:      fmt.Sprintf("%s-step%d", runID, i),
			RunID:   runID,
			StepID:  parseStepID(t, gotSteps[i].ID),
			EntryID: entryID,
			Status:  ss,
		})
	}
	run := domain.WorkflowRun{ID: runID, WorkflowID: wfID}
	if err := rstore.CreateRun(ctx, run, rs); err != nil {
		t.Fatalf("CreateRun %s: %v", runID, err)
	}
	if status != domain.RunStatusPending {
		if err := rstore.UpdateRunStatus(ctx, runID, status, ""); err != nil {
			t.Fatalf("UpdateRunStatus %s: %v", runID, err)
		}
	}
}

// Task 1.1: TestGetRunStats_MixedStatuses
func TestGetRunStats_MixedStatuses(t *testing.T) {
	estore, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create entry + workflow with 2 steps.
	entry := domain.Entry{ID: "entry-mix", Title: "E", Slug: "entry-mix", Type: domain.EntryTypePrompt, BodyOptional: "body", Status: domain.StatusActive}
	if err := estore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save entry: %v", err)
	}
	wf := domain.Workflow{ID: "wf-mix", Name: "W", Slug: "wf-mix", Status: domain.StatusActive}
	wfSteps := []domain.WorkflowStep{
		{ID: "ws1", WorkflowID: "wf-mix", OrderIndex: 1, Title: "S1", Instruction: "Step", EntrySlug: "entry-mix"},
		{ID: "ws2", WorkflowID: "wf-mix", OrderIndex: 2, Title: "S2", Instruction: "Step", EntrySlug: "entry-mix"},
	}
	if err := wstore.Save(ctx, wf, wfSteps); err != nil {
		t.Fatalf("Save workflow: %v", err)
	}

	// Seed: 7 completed, 2 failed, 1 running (pending).
	for i := 1; i <= 7; i++ {
		createSeededRun(t, ctx, estore, wstore, rstore, fmt.Sprintf("run-mix-c%d", i), "entry-mix", "wf-mix", domain.RunStatusCompleted, []domain.RunStatus{domain.RunStatusCompleted})
	}
	for i := 1; i <= 2; i++ {
		createSeededRun(t, ctx, estore, wstore, rstore, fmt.Sprintf("run-mix-f%d", i), "entry-mix", "wf-mix", domain.RunStatusFailed, []domain.RunStatus{domain.RunStatusFailed})
	}
	createSeededRun(t, ctx, estore, wstore, rstore, "run-mix-p1", "entry-mix", "wf-mix", domain.RunStatusPending, []domain.RunStatus{domain.RunStatusPending})

	stats, err := rstore.GetRunStats(ctx, nil)
	if err != nil {
		t.Fatalf("GetRunStats: %v", err)
	}

	if stats.TotalRuns != 10 {
		t.Errorf("TotalRuns = %d, want 10", stats.TotalRuns)
	}
	if stats.CompletedRuns != 7 {
		t.Errorf("CompletedRuns = %d, want 7", stats.CompletedRuns)
	}
	if stats.FailedRuns != 2 {
		t.Errorf("FailedRuns = %d, want 2", stats.FailedRuns)
	}
	if stats.FailedStepCount != 2 {
		t.Errorf("FailedStepCount = %d, want 2", stats.FailedStepCount)
	}
	// Duration stats should be populated (completed + failed runs have finished_at).
	// Duration may be zero when created/completed near-instantly.
	if stats.AvgDurationSecs < 0 {
		t.Errorf("AvgDurationSecs = %f, want >= 0", stats.AvgDurationSecs)
	}
	if stats.MaxDurationSecs < 0 {
		t.Errorf("MaxDurationSecs = %f, want >= 0", stats.MaxDurationSecs)
	}
	if stats.MinDurationSecs < 0 {
		t.Errorf("MinDurationSecs = %f, want >= 0", stats.MinDurationSecs)
	}
}

// Task 1.2: TestGetRunStats_Empty
func TestGetRunStats_Empty(t *testing.T) {
	_, _, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	stats, err := rstore.GetRunStats(ctx, nil)
	if err != nil {
		t.Fatalf("GetRunStats on empty DB: %v", err)
	}

	if stats.TotalRuns != 0 {
		t.Errorf("TotalRuns = %d, want 0", stats.TotalRuns)
	}
	if stats.CompletedRuns != 0 {
		t.Errorf("CompletedRuns = %d, want 0", stats.CompletedRuns)
	}
	if stats.FailedRuns != 0 {
		t.Errorf("FailedRuns = %d, want 0", stats.FailedRuns)
	}
	if stats.FailedStepCount != 0 {
		t.Errorf("FailedStepCount = %d, want 0", stats.FailedStepCount)
	}
	if stats.AvgDurationSecs != 0 {
		t.Errorf("AvgDurationSecs = %f, want 0", stats.AvgDurationSecs)
	}
	if stats.MaxDurationSecs != 0 {
		t.Errorf("MaxDurationSecs = %f, want 0", stats.MaxDurationSecs)
	}
	if stats.MinDurationSecs != 0 {
		t.Errorf("MinDurationSecs = %f, want 0", stats.MinDurationSecs)
	}
}

// Task 1.3: TestGetRunStats_PerWorkflow
func TestGetRunStats_PerWorkflow(t *testing.T) {
	estore, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create entry.
	entry := domain.Entry{ID: "entry-pw", Title: "E", Slug: "entry-pw", Type: domain.EntryTypePrompt, BodyOptional: "body", Status: domain.StatusActive}
	if err := estore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	// Create two workflows, each with 1 step.
	for _, pair := range []struct{ wfID, wfName, wfSlug string }{ {"wf-a", "A", "wf-a"}, {"wf-b", "B", "wf-b"} } {
		wf := domain.Workflow{ID: pair.wfID, Name: pair.wfName, Slug: pair.wfSlug, Status: domain.StatusActive}
		steps := []domain.WorkflowStep{{ID: "s-" + pair.wfID, WorkflowID: pair.wfID, OrderIndex: 1, Title: "S", Instruction: "Step", EntrySlug: "entry-pw"}}
		if err := wstore.Save(ctx, wf, steps); err != nil {
			t.Fatalf("Save workflow %s: %v", pair.wfID, err)
		}
	}

	// Workflow A: 3 completed runs.
	for i := 1; i <= 3; i++ {
		createSeededRun(t, ctx, estore, wstore, rstore, fmt.Sprintf("run-pw-a%d", i), "entry-pw", "wf-a", domain.RunStatusCompleted, []domain.RunStatus{domain.RunStatusCompleted})
	}
	// Workflow B: 2 completed runs.
	for i := 1; i <= 2; i++ {
		createSeededRun(t, ctx, estore, wstore, rstore, fmt.Sprintf("run-pw-b%d", i), "entry-pw", "wf-b", domain.RunStatusCompleted, []domain.RunStatus{domain.RunStatusCompleted})
	}

	// Query per-workflow: filter A.
	wfA := "wf-a"
	statsA, err := rstore.GetRunStats(ctx, &wfA)
	if err != nil {
		t.Fatalf("GetRunStats(wf-a): %v", err)
	}
	if statsA.TotalRuns != 3 {
		t.Errorf("TotalRuns for wf-a = %d, want 3", statsA.TotalRuns)
	}
	if statsA.CompletedRuns != 3 {
		t.Errorf("CompletedRuns for wf-a = %d, want 3", statsA.CompletedRuns)
	}

	// Query per-workflow: filter B.
	wfB := "wf-b"
	statsB, err := rstore.GetRunStats(ctx, &wfB)
	if err != nil {
		t.Fatalf("GetRunStats(wf-b): %v", err)
	}
	if statsB.TotalRuns != 2 {
		t.Errorf("TotalRuns for wf-b = %d, want 2", statsB.TotalRuns)
	}
	if statsB.CompletedRuns != 2 {
		t.Errorf("CompletedRuns for wf-b = %d, want 2", statsB.CompletedRuns)
	}
}

// Task 1.4: TestListAllRuns_WithProgress
func TestListAllRuns_WithProgress(t *testing.T) {
	estore, wstore, rstore, cleanup := setupRunStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create entry + workflow with 5 steps.
	entry := domain.Entry{ID: "entry-prog", Title: "E", Slug: "entry-prog", Type: domain.EntryTypePrompt, BodyOptional: "body", Status: domain.StatusActive}
	if err := estore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save entry: %v", err)
	}
	wf := domain.Workflow{ID: "wf-prog", Name: "W", Slug: "wf-prog", Status: domain.StatusActive}
	wfSteps := make([]domain.WorkflowStep, 5)
	for i := 0; i < 5; i++ {
		wfSteps[i] = domain.WorkflowStep{ID: fmt.Sprintf("wsp%d", i), WorkflowID: "wf-prog", OrderIndex: i + 1, Title: fmt.Sprintf("S%d", i+1), Instruction: "Step", EntrySlug: "entry-prog"}
	}
	if err := wstore.Save(ctx, wf, wfSteps); err != nil {
		t.Fatalf("Save workflow: %v", err)
	}

	// Create run with 5 steps: 3 completed, 2 pending.
	stepStatuses := []domain.RunStatus{domain.RunStatusCompleted, domain.RunStatusCompleted, domain.RunStatusPending, domain.RunStatusCompleted, domain.RunStatusPending}
	createSeededRun(t, ctx, estore, wstore, rstore, "run-prog-r1", "entry-prog", "wf-prog", domain.RunStatusPending, stepStatuses)

	runs, progresses, err := rstore.ListAllRuns(ctx, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListAllRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if len(progresses) != 1 {
		t.Fatalf("expected 1 progress, got %d", len(progresses))
	}

	r := runs[0]
	p := progresses[0]

	if r.ID != "run-prog-r1" {
		t.Errorf("run ID = %q, want %q", r.ID, "run-prog-r1")
	}
	if p.RunID != "run-prog-r1" {
		t.Errorf("progress RunID = %q, want %q", p.RunID, "run-prog-r1")
	}
	if p.CompletedSteps != 3 {
		t.Errorf("CompletedSteps = %d, want 3", p.CompletedSteps)
	}
	if p.TotalSteps != 5 {
		t.Errorf("TotalSteps = %d, want 5", p.TotalSteps)
	}
}
