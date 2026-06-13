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
