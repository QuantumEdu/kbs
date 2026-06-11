package domain

import "testing"

func TestEntryTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant EntryType
		expected string
	}{
		{"skill", EntryTypeSkill, "skill"},
		{"agent", EntryTypeAgent, "agent"},
		{"workflow", EntryTypeWorkflow, "workflow"},
		{"prompt", EntryTypePrompt, "prompt"},
		{"context", EntryTypeContext, "context"},
		{"note", EntryTypeNote, "note"},
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
		ID:          "prd-fastapi",
		Name:        "FastAPI PRD",
		Type:        EntryTypeSkill,
		Content:     "Design FastAPI backend",
		ProjectID:   nil,
		Description: "PRD document",
		Active:      true,
	}

	if e.ID != "prd-fastapi" {
		t.Errorf("ID = %q, want %q", e.ID, "prd-fastapi")
	}
	if e.Name != "FastAPI PRD" {
		t.Errorf("Name = %q, want %q", e.Name, "FastAPI PRD")
	}
	if e.Type != EntryTypeSkill {
		t.Errorf("Type = %q, want %q", e.Type, EntryTypeSkill)
	}
	if e.Content != "Design FastAPI backend" {
		t.Errorf("Content = %q, want %q", e.Content, "Design FastAPI backend")
	}
	if e.ProjectID != nil {
		t.Errorf("ProjectID should be nil for global entry")
	}
	if !e.Active {
		t.Errorf("Active should default to true")
	}
}

func TestValidEntryTypes(t *testing.T) {
	valid := map[EntryType]bool{
		EntryTypeSkill:    true,
		EntryTypeAgent:    true,
		EntryTypeWorkflow: true,
		EntryTypePrompt:   true,
		EntryTypeContext:  true,
		EntryTypeNote:     true,
	}
	if len(valid) != 6 {
		t.Errorf("Expected 6 valid entry types, got %d", len(valid))
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
