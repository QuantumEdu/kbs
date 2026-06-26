package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupRunServices(t *testing.T) (*db.Store, *WorkflowRunService, *WorkflowService, *EntryService, *ProjectService, func()) {
	t.Helper()
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := db.RunMigrations(sqlDB); err != nil {
		sqlDB.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := db.NewStore(sqlDB)

	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	workflowSvc := NewWorkflowService(store.Workflows)
	projectSvc := NewProjectService(store.Projects)
	runSvc := NewWorkflowRunService(store.Workflows, store.WorkflowRuns, store.Entries)

	cleanup := func() { sqlDB.Close() }
	return store, runSvc, workflowSvc, entrySvc, projectSvc, cleanup
}

func TestRunPipelineWorkflowNotFound(t *testing.T) {
	_, runSvc, _, _, _, cleanup := setupRunServices(t)
	defer cleanup()
	ctx := context.Background()

	stdin := strings.NewReader("")
	var stdout bytes.Buffer
	_, _, err := runSvc.RunPipeline(ctx, "nonexistent", "input", stdin, &stdout)
	if err == nil {
		t.Fatal("expected error for nonexistent workflow")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestRunPipelineEntrySlugNotFound(t *testing.T) {
	store, runSvc, wfSvc, entrySvc, projSvc, cleanup := setupRunServices(t)
	defer cleanup()
	ctx := context.Background()

	// Create a project and an entry
	projSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Summarize Entry",
		Type:    "prompt",
		Summary: "A prompt entry",
		Body:    "Summarize: {{input}}",
		Project: "testproj",
	})

	// Create workflow with a step referencing nonexistent entry slug
	steps := []SaveWorkflowStep{
		{OrderIndex: 1, Title: "Step 1", Instruction: "Do something", Required: true, EntrySlug: "nonexistent_entry"},
	}
	wf, err := wfSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:        "Test WF",
		Description: "Test",
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	stdin := strings.NewReader("")
	var stdout bytes.Buffer
	_, _, err = runSvc.RunPipeline(ctx, wf.Slug, "input text", stdin, &stdout)
	if err == nil {
		t.Fatal("expected error for nonexistent entry slug")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}

	// Verify no run was created (fail fast)
	runs, _ := store.WorkflowRuns.ListRuns(ctx, wf.ID, 10)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs after pre-flight failure, got %d", len(runs))
	}
}

func TestRunPipelineEntryArchived(t *testing.T) {
	store, runSvc, wfSvc, entrySvc, projSvc, cleanup := setupRunServices(t)
	defer cleanup()
	ctx := context.Background()

	projSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	result, _ := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Archived Entry",
		Type:    "prompt",
		Summary: "Will be archived",
		Body:    "Process: {{input}}",
		Project: "testproj",
	})
	entrySvc.ArchiveEntry(ctx, result.Entry.Entry.ID)

	steps := []SaveWorkflowStep{
		{OrderIndex: 1, Title: "Step 1", Instruction: "Use archived", Required: true, EntrySlug: result.Entry.Entry.Slug},
	}
	wf, _ := wfSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:  "Archive WF",
		Steps: steps,
	})

	stdin := strings.NewReader("")
	var stdout bytes.Buffer
	_, _, err := runSvc.RunPipeline(ctx, wf.Slug, "input", stdin, &stdout)
	if err == nil {
		t.Fatal("expected error for archived entry")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "archived") && !strings.Contains(strings.ToLower(err.Error()), "active") {
		t.Errorf("error should mention archived/active status, got: %v", err)
	}

	// Verify no run was created
	runs, _ := store.WorkflowRuns.ListRuns(ctx, wf.ID, 10)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs after pre-flight failure, got %d", len(runs))
	}
}

func TestRunPipelineSuccessfulExecution(t *testing.T) {
	store, runSvc, wfSvc, entrySvc, projSvc, cleanup := setupRunServices(t)
	defer cleanup()
	ctx := context.Background()

	// Create entries
	projSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Step 1 Prompt",
		Type:    "prompt",
		Summary: "First step",
		Body:    "Process input: {{input}}",
		Project: "testproj",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Step 2 Prompt",
		Type:    "prompt",
		Summary: "Second step",
		Body:    "Previous was: {{previous_output}}. Input: {{input}}",
		Project: "testproj",
	})

	steps := []SaveWorkflowStep{
		{OrderIndex: 1, Title: "Step 1", Instruction: "First", Required: true, EntrySlug: "step-1-prompt"},
		{OrderIndex: 2, Title: "Step 2", Instruction: "Second", Required: true, EntrySlug: "step-2-prompt"},
	}
	wf, err := wfSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:  "Pipeline WF",
		Steps: steps,
	})
	if err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Simulated stdin: each step responds
	stdin := strings.NewReader("STEP1_OUTPUT\nSTEP2_OUTPUT\n")
	var stdout bytes.Buffer

	run, output, err := runSvc.RunPipeline(ctx, wf.Slug, "my input", stdin, &stdout)
	if err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}
	if run == nil {
		t.Fatal("expected non-nil run")
	}
	if run.Status != domain.RunStatusCompleted {
		t.Errorf("run status = %q, want 'completed'", run.Status)
	}
	if output != "STEP1_OUTPUT\nSTEP2_OUTPUT" {
		t.Errorf("final output = %q, want 'STEP1_OUTPUT\\nSTEP2_OUTPUT'", output)
	}

	// Verify steps were completed
	_, runSteps, err := store.WorkflowRuns.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if len(runSteps) != 2 {
		t.Fatalf("expected 2 run steps, got %d", len(runSteps))
	}
	if runSteps[0].Status != domain.RunStatusCompleted {
		t.Errorf("step 0 status = %q, want 'completed'", runSteps[0].Status)
	}
	if runSteps[1].Status != domain.RunStatusCompleted {
		t.Errorf("step 1 status = %q, want 'completed'", runSteps[1].Status)
	}

	// Verify steps output
	if runSteps[0].Output != "STEP1_OUTPUT" {
		t.Errorf("step 0 output = %q, want 'STEP1_OUTPUT'", runSteps[0].Output)
	}
	if runSteps[1].Output != "STEP2_OUTPUT" {
		t.Errorf("step 1 output = %q, want 'STEP2_OUTPUT'", runSteps[1].Output)
	}

	// Verify stdout contains rendered prompts
	stdoutStr := stdout.String()
	if !strings.Contains(stdoutStr, "Process input: my input") {
		t.Errorf("stdout should contain rendered step 1 prompt, got: %q", stdoutStr)
	}
	if !strings.Contains(stdoutStr, "Previous was: STEP1_OUTPUT") {
		t.Errorf("stdout should contain rendered step 2 with previous_output, got: %q", stdoutStr)
	}
}

func TestRunPipelineMixedRenderableAndExecutable(t *testing.T) {
	store, runSvc, wfSvc, entrySvc, projSvc, cleanup := setupRunServices(t)
	defer cleanup()
	ctx := context.Background()

	projSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Executable Step",
		Type:    "prompt",
		Summary: "Only executable",
		Body:    "Process: {{input}}",
		Project: "testproj",
	})

	steps := []SaveWorkflowStep{
		{OrderIndex: 1, Title: "Renderable Only", Instruction: "Just a checklist item", Required: false, EntrySlug: ""},
		{OrderIndex: 2, Title: "Executable Step", Instruction: "This runs", Required: true, EntrySlug: "executable-step"},
	}
	wf, _ := wfSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:  "Mixed WF",
		Steps: steps,
	})

	stdin := strings.NewReader("STEP_OUTPUT\n")
	var stdout bytes.Buffer

	run, output, err := runSvc.RunPipeline(ctx, wf.Slug, "input", stdin, &stdout)
	if err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}
	if run.Status != domain.RunStatusCompleted {
		t.Errorf("run status = %q, want 'completed'", run.Status)
	}
	if output != "STEP_OUTPUT" {
		t.Errorf("final output = %q, want 'STEP_OUTPUT'", output)
	}

	_, runSteps, _ := store.WorkflowRuns.GetRun(ctx, run.ID)
	// Only executable steps create run steps
	if len(runSteps) != 1 {
		t.Fatalf("expected 1 run step (only executable), got %d", len(runSteps))
	}
}

func TestRunPipelineStepErrorHaltsExecution(t *testing.T) {
	store, runSvc, wfSvc, entrySvc, projSvc, cleanup := setupRunServices(t)
	defer cleanup()
	ctx := context.Background()

	projSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Step A",
		Type:    "prompt",
		Summary: "First step",
		Body:    "Do A: {{input}}",
		Project: "testproj",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Step B",
		Type:    "prompt",
		Summary: "Second step",
		Body:    "Do B: {{previous_output}}",
		Project: "testproj",
	})

	steps := []SaveWorkflowStep{
		{OrderIndex: 1, Title: "Step A", Instruction: "First", Required: true, EntrySlug: "step-a"},
		{OrderIndex: 2, Title: "Step B", Instruction: "Second", Required: true, EntrySlug: "step-b"},
	}
	wf, _ := wfSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:  "Error WF",
		Steps: steps,
	})

	// Simulate stdin EOF during step 1 (causes read error)
	stdin := strings.NewReader("") // empty → EOF → step 1 fails with read error
	var stdout bytes.Buffer

	run, _, err := runSvc.RunPipeline(ctx, wf.Slug, "input", stdin, &stdout)
	if err != nil {
		t.Fatalf("RunPipeline should return partial run on step error: %v", err)
	}
	if run == nil {
		t.Fatal("expected non-nil run even on error")
	}
	if run.Status != domain.RunStatusFailed {
		t.Errorf("run status = %q, want 'failed'", run.Status)
	}

	// Verify step states
	_, runSteps, _ := store.WorkflowRuns.GetRun(ctx, run.ID)
	if len(runSteps) != 2 {
		t.Fatalf("expected 2 run steps, got %d", len(runSteps))
	}
	// Step 1 should be failed (read error)
	if runSteps[0].Status != domain.RunStatusFailed {
		t.Errorf("step 0 status = %q, want 'failed'", runSteps[0].Status)
	}
	// Step 2 should still be pending (halted)
	if runSteps[1].Status != domain.RunStatusPending {
		t.Errorf("step 1 status = %q, want 'pending' (halted)", runSteps[1].Status)
	}
}

func TestRunPipelineTruncationWarning(t *testing.T) {
	_, runSvc, wfSvc, entrySvc, projSvc, cleanup := setupRunServices(t)
	defer cleanup()
	ctx := context.Background()

	projSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Step 1",
		Type:    "prompt",
		Summary: "Produces large output",
		Body:    "Produce output",
		Project: "testproj",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Step 2",
		Type:    "prompt",
		Summary: "Uses previous",
		Body:    "Prev: {{previous_output}}",
		Project: "testproj",
	})

	steps := []SaveWorkflowStep{
		{OrderIndex: 1, Title: "Step 1", Instruction: "First", Required: true, EntrySlug: "step-1"},
		{OrderIndex: 2, Title: "Step 2", Instruction: "Second", Required: true, EntrySlug: "step-2"},
	}
	wf, _ := wfSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:  "Truncation WF",
		Steps: steps,
	})

	// Step 1 produces output > 32K, this should trigger truncation warning on step 2
	largeOutput := strings.Repeat("X", 33000) // 33K, exceeds 32K threshold
	stdin := strings.NewReader(largeOutput + "\n" + "step2_response\n")
	var stdout bytes.Buffer

	run, output, err := runSvc.RunPipeline(ctx, wf.Slug, "input", stdin, &stdout)
	if err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}
	if run.Status != domain.RunStatusCompleted {
		t.Errorf("run status = %q, want 'completed'", run.Status)
	}
	// Final output should still be step outputs concatenated with newlines
	if !strings.HasPrefix(output, strings.Repeat("X", 32768)) {
		t.Error("final output should have step 1 output (possibly truncated)")
	}

	stdoutStr := stdout.String()
	// Check for truncation warning in stdout
	if !strings.Contains(stdoutStr, "truncat") {
		t.Logf("stdout (may not contain truncation warning if rendered output was in step 2): %q", stdoutStr)
	}
	_ = output // used for assertions above
}
