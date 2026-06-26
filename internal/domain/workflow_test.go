package domain

import "testing"

func TestWorkflowStruct(t *testing.T) {
	w := Workflow{
		ID:          "spec-plan-task",
		Name:        "Spec Plan Task",
		Slug:        "spec-plan-task",
		Description: "Full SDD workflow",
		Status:      StatusActive,
	}

	if w.ID != "spec-plan-task" {
		t.Errorf("ID = %q, want %q", w.ID, "spec-plan-task")
	}
	if w.Name != "Spec Plan Task" {
		t.Errorf("Name = %q, want %q", w.Name, "Spec Plan Task")
	}
	if w.Status != StatusActive {
		t.Errorf("Status = %q, want %q", w.Status, StatusActive)
	}
}

func TestWorkflowStepStruct(t *testing.T) {
	ws := WorkflowStep{
		ID:             "step-1",
		WorkflowID:     "spec-plan-task",
		OrderIndex:     1,
		Title:          "Read Spec",
		Instruction:    "Read the source spec document",
		Required:       true,
		ExpectedOutput: "Understanding of the spec",
	}

	if ws.ID != "step-1" {
		t.Errorf("ID = %q, want %q", ws.ID, "step-1")
	}
	if ws.WorkflowID != "spec-plan-task" {
		t.Errorf("WorkflowID = %q, want %q", ws.WorkflowID, "spec-plan-task")
	}
	if ws.OrderIndex != 1 {
		t.Errorf("OrderIndex = %d, want 1", ws.OrderIndex)
	}
	if ws.Title != "Read Spec" {
		t.Errorf("Title = %q, want %q", ws.Title, "Read Spec")
	}
	if !ws.Required {
		t.Error("Required should be true")
	}
}

func TestRunStatusValues(t *testing.T) {
	tests := []struct {
		status RunStatus
		valid  bool
	}{
		{RunStatusPending, true},
		{RunStatusRunning, true},
		{RunStatusCompleted, true},
		{RunStatusFailed, true},
		{RunStatus("invalid"), false},
		{RunStatus(""), false},
	}

	valid := map[RunStatus]bool{
		RunStatusPending:   true,
		RunStatusRunning:   true,
		RunStatusCompleted: true,
		RunStatusFailed:    true,
	}

	for _, tt := range tests {
		if valid[tt.status] != tt.valid {
			t.Errorf("RunStatus(%q) valid = %v, want %v", tt.status, valid[tt.status], tt.valid)
		}
	}
}

func TestWorkflowRunStruct(t *testing.T) {
	r := WorkflowRun{
		ID:         "run-001",
		WorkflowID: "wf-abc",
		Input:      "Initial content",
		Output:     "Final result",
		Status:     RunStatusCompleted,
	}

	if r.ID != "run-001" {
		t.Errorf("ID = %q, want %q", r.ID, "run-001")
	}
	if r.WorkflowID != "wf-abc" {
		t.Errorf("WorkflowID = %q, want %q", r.WorkflowID, "wf-abc")
	}
	if r.Input != "Initial content" {
		t.Errorf("Input = %q, want %q", r.Input, "Initial content")
	}
	if r.Output != "Final result" {
		t.Errorf("Output = %q, want %q", r.Output, "Final result")
	}
	if r.Status != RunStatusCompleted {
		t.Errorf("Status = %q, want %q", r.Status, RunStatusCompleted)
	}
	if r.FinishedAt != nil {
		t.Errorf("FinishedAt should be nil when not set")
	}
}

func TestWorkflowRunStepStruct(t *testing.T) {
	s := WorkflowRunStep{
		ID:      "rst-001",
		RunID:   "run-001",
		StepID:  42,
		EntryID: "entry-abc",
		Input:   "Composed prompt",
		Output:  "Step result",
		Status:  RunStatusRunning,
	}

	if s.ID != "rst-001" {
		t.Errorf("ID = %q, want %q", s.ID, "rst-001")
	}
	if s.RunID != "run-001" {
		t.Errorf("RunID = %q, want %q", s.RunID, "run-001")
	}
	if s.StepID != 42 {
		t.Errorf("StepID = %d, want 42", s.StepID)
	}
	if s.EntryID != "entry-abc" {
		t.Errorf("EntryID = %q, want %q", s.EntryID, "entry-abc")
	}
	if s.Input != "Composed prompt" {
		t.Errorf("Input = %q, want %q", s.Input, "Composed prompt")
	}
	if s.Output != "Step result" {
		t.Errorf("Output = %q, want %q", s.Output, "Step result")
	}
	if s.Status != RunStatusRunning {
		t.Errorf("Status = %q, want %q", s.Status, RunStatusRunning)
	}
}

func TestWorkflowStepEntrySlug(t *testing.T) {
	// Default: EntrySlug is empty for renderable-only steps
	ws := WorkflowStep{
		ID:          "step-1",
		WorkflowID:  "wf1",
		OrderIndex:  1,
		Title:       "Read Spec",
		Instruction: "Read the spec document",
	}
	if ws.EntrySlug != "" {
		t.Errorf("EntrySlug should default to empty, got %q", ws.EntrySlug)
	}

	// When set: step is executable
	ws2 := WorkflowStep{
		ID:          "step-2",
		WorkflowID:  "wf1",
		OrderIndex:  2,
		Title:       "Summarize",
		Instruction: "Summarize the findings",
		EntrySlug:   "summarize",
	}
	if ws2.EntrySlug != "summarize" {
		t.Errorf("EntrySlug = %q, want %q", ws2.EntrySlug, "summarize")
	}
}

func TestWorkflowStepOptionalOutput(t *testing.T) {
	ws := WorkflowStep{
		ID:          "step-2",
		WorkflowID:  "spec-plan-task",
		OrderIndex:  2,
		Title:       "Draft Plan",
		Instruction: "Create implementation plan",
		Required:    false,
	}

	if ws.Title != "Draft Plan" {
		t.Errorf("Title = %q, want %q", ws.Title, "Draft Plan")
	}
	if ws.Required {
		t.Error("Required should be false")
	}
	if ws.ExpectedOutput != "" {
		t.Errorf("ExpectedOutput should be empty, got %q", ws.ExpectedOutput)
	}
}
