package domain

import "testing"

func TestEntryTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant EntryType
		expected string
	}{
		{"prompt", EntryTypePrompt, "prompt"},
		{"skill", EntryTypeSkill, "skill"},
		{"workflow_note", EntryTypeWorkflowNote, "workflow_note"},
		{"reference", EntryTypeReference, "reference"},
		{"user", EntryTypeUser, "user"},
		{"feedback", EntryTypeFeedback, "feedback"},
		{"project_state", EntryTypeProjectState, "project_state"},
		{"session", EntryTypeSession, "session"},
		{"decision", EntryTypeDecision, "decision"},
		{"artifact_summary", EntryTypeArtifactSummary, "artifact_summary"},
		{"handoff", EntryTypeHandoff, "handoff"},
		{"routing", EntryTypeRouting, "routing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("EntryType %s = %q, want %q", tt.name, string(tt.constant), tt.expected)
			}
		})
	}
}

func TestEntryStruct(t *testing.T) {
	e := Entry{
		ID:           "prd-fastapi",
		Title:        "FastAPI PRD",
		Slug:         "fastapi-prd",
		Type:         EntryTypeSkill,
		Summary:      "PRD document",
		BodyOptional: "Design FastAPI backend",
		Status:       StatusActive,
		ProjectID:    nil,
	}

	if e.ID != "prd-fastapi" {
		t.Errorf("ID = %q, want %q", e.ID, "prd-fastapi")
	}
	if e.Title != "FastAPI PRD" {
		t.Errorf("Title = %q, want %q", e.Title, "FastAPI PRD")
	}
	if e.Type != EntryTypeSkill {
		t.Errorf("Type = %q, want %q", e.Type, EntryTypeSkill)
	}
	if e.BodyOptional != "Design FastAPI backend" {
		t.Errorf("BodyOptional = %q, want %q", e.BodyOptional, "Design FastAPI backend")
	}
	if e.ProjectID != nil {
		t.Errorf("ProjectID should be nil for global entry")
	}
	if e.Status != StatusActive {
		t.Errorf("Status = %q, want %q", e.Status, StatusActive)
	}
}

func TestValidEntryTypes(t *testing.T) {
	valid := map[EntryType]bool{
		EntryTypePrompt:          true,
		EntryTypeSkill:           true,
		EntryTypeWorkflowNote:    true,
		EntryTypeReference:       true,
		EntryTypeUser:            true,
		EntryTypeFeedback:        true,
		EntryTypeProjectState:    true,
		EntryTypeSession:         true,
		EntryTypeDecision:        true,
		EntryTypeArtifactSummary: true,
		EntryTypeHandoff:         true,
		EntryTypeRouting:         true,
	}
	if len(valid) != 12 {
		t.Errorf("Expected 12 valid entry types, got %d", len(valid))
	}
	for et := range valid {
		if !et.IsValid() {
			t.Errorf("EntryType %q should be valid but IsValid() returned false", et)
		}
	}
	invalid := EntryType("invalid")
	if invalid.IsValid() {
		t.Errorf("EntryType %q should be invalid but IsValid() returned true", invalid)
	}
}

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant Status
		expected string
	}{
		{"draft", StatusDraft, "draft"},
		{"active", StatusActive, "active"},
		{"archived", StatusArchived, "archived"},
		{"deprecated", StatusDeprecated, "deprecated"},
		{"canonical", StatusCanonical, "canonical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("Status %s = %q, want %q", tt.name, string(tt.constant), tt.expected)
			}
		})
	}
}

func TestValidStatuses(t *testing.T) {
	valid := []Status{StatusDraft, StatusActive, StatusArchived, StatusDeprecated, StatusCanonical}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("Status %q should be valid", s)
		}
	}
	if Status("invalid").IsValid() {
		t.Error("Status 'invalid' should not be valid")
	}
}

func TestStatusTransitions(t *testing.T) {
	all := []Status{StatusDraft, StatusActive, StatusArchived, StatusDeprecated, StatusCanonical}
	for _, s := range all {
		if !s.IsValid() {
			t.Errorf("Status %q should be valid", s)
		}
	}
}
