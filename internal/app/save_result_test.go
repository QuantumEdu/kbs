package app

import (
	"context"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupResultTest(t *testing.T) (*db.Store, *EntryService, func()) {
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
	cleanup := func() { sqlDB.Close() }
	return store, entrySvc, cleanup
}

func TestSavePromptResultFieldMapping(t *testing.T) {
	store, entrySvc, cleanup := setupResultTest(t)
	defer cleanup()
	ctx := context.Background()

	store.Projects.Save(ctx, domain.Project{ID: "kbs", Name: "KBS", Slug: "kbs", Status: domain.StatusActive})

	svc := NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)

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

	result, err := entrySvc.Get(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Entry.Title != "Architecture Review" {
		t.Errorf("stored Title = %q, want 'Architecture Review'", result.Entry.Title)
	}
	if result.Entry.BodyOptional != "FastAPI architecture decision" {
		t.Errorf("stored BodyOptional = %q, want 'FastAPI architecture decision'", result.Entry.BodyOptional)
	}
	if result.Entry.Type != domain.EntryTypePrompt {
		t.Errorf("stored Type = %q, want 'prompt'", result.Entry.Type)
	}
	if result.Entry.Summary != "architecture" {
		t.Errorf("stored Summary = %q, want 'architecture'", result.Entry.Summary)
	}
	if result.Entry.ProjectID == nil || *result.Entry.ProjectID != "kbs" {
		t.Errorf("stored ProjectID = %v, want 'kbs'", result.Entry.ProjectID)
	}
	if result.Entry.Status != domain.StatusActive {
		t.Errorf("stored Status = %q, want 'active'", result.Entry.Status)
	}

	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestSavePromptResultDefaults(t *testing.T) {
	store, entrySvc, cleanup := setupResultTest(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)

	input := SavePromptResultInput{
		Name:    "Minimal",
		Content: "Just content",
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := entrySvc.Get(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Entry.Type != domain.EntryTypePrompt {
		t.Errorf("default Type = %q, want 'prompt'", result.Entry.Type)
	}
	if result.Entry.ProjectID != nil {
		t.Errorf("default ProjectID should be nil (global), got %v", *result.Entry.ProjectID)
	}
	if len(result.Tags) != 0 {
		t.Errorf("default Tags should be empty, got %v", result.Tags)
	}
	if result.Entry.Summary != "" {
		t.Errorf("default Summary should be empty, got %q", result.Entry.Summary)
	}
}

func TestSavePromptResultValidation(t *testing.T) {
	store, _, cleanup := setupResultTest(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)

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
			name:    "invalid type",
			input:   SavePromptResultInput{Name: "test", Content: "test", Type: "invalid_type"},
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
	store, entrySvc, cleanup := setupResultTest(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)

	input := SavePromptResultInput{
		Name:    "Tag Test",
		Content: "testing tag normalization",
		Tags:    []string{" Go ", "CLI", "go", "", "cli-tool"},
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := entrySvc.Get(ctx, output.EntryID, false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if len(result.Tags) != 3 {
		t.Errorf("expected 3 tags (go, cli, cli-tool), got %d: %v", len(result.Tags), result.Tags)
	}
}
