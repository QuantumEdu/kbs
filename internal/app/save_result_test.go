package app

import (
	"context"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/domain"
)

func TestSavePromptResultFieldMapping(t *testing.T) {
	store, entrySvc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	// Create the project so FK constraint passes
	store.Projects.UpsertProject(ctx, domain.Project{ID: "kbs", Name: "KBS", Active: true})

	svc := NewSavePromptResultService(entrySvc)

	input := SavePromptResultInput{
		Name:           "Architecture Review",
		Content:        "FastAPI architecture decision",
		Type:           "prompt",
		Category:       "architecture",
		Tags:           []string{"go", "cli"},
		ProjectID:      "kbs",
		SourcePromptID: "prd-fastapi",
		Model:          "claude-3",
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if output.EntryID == "" {
		t.Error("expected non-empty entry ID")
	}
	if output.Name != input.Name {
		t.Errorf("Name = %q, want %q", output.Name, input.Name)
	}
	if output.Type != "prompt" {
		t.Errorf("Type = %q, want 'prompt'", output.Type)
	}
	if output.ProjectID != "kbs" {
		t.Errorf("ProjectID = %q, want 'kbs'", output.ProjectID)
	}

	// Verify the stored entry
	result, err := entrySvc.GetEntry(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Entry.Name != "Architecture Review" {
		t.Errorf("stored Name = %q, want 'Architecture Review'", result.Entry.Name)
	}
	if result.Entry.Content != "FastAPI architecture decision" {
		t.Errorf("stored Content = %q, want 'FastAPI architecture decision'", result.Entry.Content)
	}
	if result.Entry.Type != domain.EntryTypePrompt {
		t.Errorf("stored Type = %q, want 'prompt'", result.Entry.Type)
	}
	if result.Entry.Description != "architecture" {
		t.Errorf("stored Description = %q, want 'architecture'", result.Entry.Description)
	}
	if result.Entry.ProjectID == nil || *result.Entry.ProjectID != "kbs" {
		t.Errorf("stored ProjectID = %v, want 'kbs'", result.Entry.ProjectID)
	}
	if result.Entry.Active != true {
		t.Error("stored Active should be true")
	}

	// Verify tags were stored and normalized
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(result.Tags), result.Tags)
	}
	if result.Tags[0] != "cli" || result.Tags[1] != "go" {
		t.Errorf("Tags = %v, want [cli go]", result.Tags)
	}

	// Verify vars contains model and source_prompt_id
	if result.Entry.Vars == "" {
		t.Error("Vars should contain model JSON")
	}
	if !strings.Contains(result.Entry.Vars, "claude-3") {
		t.Errorf("Vars %q should contain 'claude-3'", result.Entry.Vars)
	}
	if !strings.Contains(result.Entry.Vars, "prd-fastapi") {
		t.Errorf("Vars %q should contain 'prd-fastapi'", result.Entry.Vars)
	}
}

func TestSavePromptResultDefaults(t *testing.T) {
	_, entrySvc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(entrySvc)

	// Only name and content — all defaults
	input := SavePromptResultInput{
		Name:    "Minimal",
		Content: "Just content",
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := entrySvc.GetEntry(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Entry.Type != domain.EntryTypeNote {
		t.Errorf("default Type = %q, want 'note'", result.Entry.Type)
	}
	if result.Entry.ProjectID != nil {
		t.Errorf("default ProjectID should be nil (global), got %v", *result.Entry.ProjectID)
	}
	if len(result.Tags) != 0 {
		t.Errorf("default Tags should be empty, got %v", result.Tags)
	}
	if result.Entry.Vars != "" {
		t.Errorf("default Vars should be empty, got %q", result.Entry.Vars)
	}
	if result.Entry.Description != "" {
		t.Errorf("default Description should be empty, got %q", result.Entry.Description)
	}
}

func TestSavePromptResultValidation(t *testing.T) {
	_, entrySvc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(entrySvc)

	tests := []struct {
		name    string
		input   SavePromptResultInput
		wantErr string
	}{
		{
			name:    "empty name",
			input:   SavePromptResultInput{Name: "", Content: "valid"},
			wantErr: "name is required",
		},
		{
			name:    "empty content",
			input:   SavePromptResultInput{Name: "valid", Content: ""},
			wantErr: "content is required",
		},
		{
			name: "invalid type",
			input: SavePromptResultInput{
				Name:    "test",
				Content: "test",
				Type:    "invalid_type",
			},
			wantErr: "invalid entry type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Save(ctx, tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSavePromptResultTagNormalization(t *testing.T) {
	_, entrySvc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(entrySvc)

	input := SavePromptResultInput{
		Name:    "Tag Test",
		Content: "testing tag normalization",
		Tags:    []string{" Go ", "CLI", "go", "", "cli-tool"},
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := entrySvc.GetEntry(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if len(result.Tags) != 3 {
		t.Errorf("expected 3 tags (go, cli, cli-tool), got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestSavePromptResultModelOnly(t *testing.T) {
	_, entrySvc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(entrySvc)

	input := SavePromptResultInput{
		Name:    "Model Only",
		Content: "test",
		Model:   "gpt-4",
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := entrySvc.GetEntry(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if !strings.Contains(result.Entry.Vars, "gpt-4") {
		t.Errorf("Vars %q should contain 'gpt-4'", result.Entry.Vars)
	}
	if strings.Contains(result.Entry.Vars, "source_prompt_id") {
		t.Errorf("Vars %q should NOT contain source_prompt_id", result.Entry.Vars)
	}
}

func TestSavePromptResultSourceOnly(t *testing.T) {
	_, entrySvc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(entrySvc)

	input := SavePromptResultInput{
		Name:           "Source Only",
		Content:        "test",
		SourcePromptID: "prompt-123",
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := entrySvc.GetEntry(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if !strings.Contains(result.Entry.Vars, "prompt-123") {
		t.Errorf("Vars %q should contain 'prompt-123'", result.Entry.Vars)
	}
}
