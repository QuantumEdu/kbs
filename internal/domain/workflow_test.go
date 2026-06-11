package domain

import "testing"

func TestWorkflowRoleConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant WorkflowRole
		expected string
	}{
		{"system", WorkflowRoleSystem, "system"},
		{"user", WorkflowRoleUser, "user"},
		{"assistant", WorkflowRoleAssistant, "assistant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("WorkflowRole %s = %q, want %q", tt.name, string(tt.constant), tt.expected)
			}
		})
	}
}

func TestWorkflowStepStruct(t *testing.T) {
	ws := WorkflowStep{
		ID:       1,
		EntryID:  "my-workflow",
		StepNum:  2,
		Role:     WorkflowRoleUser,
		Content:  "Review the PRD",
		Label:    "Review Step",
	}

	if ws.ID != 1 {
		t.Errorf("ID = %d, want 1", ws.ID)
	}
	if ws.EntryID != "my-workflow" {
		t.Errorf("EntryID = %q, want %q", ws.EntryID, "my-workflow")
	}
	if ws.StepNum != 2 {
		t.Errorf("StepNum = %d, want 2", ws.StepNum)
	}
	if ws.Role != WorkflowRoleUser {
		t.Errorf("Role = %q, want %q", ws.Role, WorkflowRoleUser)
	}
	if ws.Content != "Review the PRD" {
		t.Errorf("Content = %q, want %q", ws.Content, "Review the PRD")
	}
	if ws.Label != "Review Step" {
		t.Errorf("Label = %q, want %q", ws.Label, "Review Step")
	}
}

func TestWorkflowStepWithoutLabel(t *testing.T) {
	ws := WorkflowStep{
		EntryID: "my-workflow",
		StepNum: 1,
		Role:    WorkflowRoleSystem,
		Content: "You are a helpful assistant",
	}
	// Label is optional — empty string is valid
	if ws.Label != "" {
		t.Errorf("Label should default to empty, got %q", ws.Label)
	}
}
