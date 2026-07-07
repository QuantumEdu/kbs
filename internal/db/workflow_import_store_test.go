package db

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupImportStore(t *testing.T) (*Store, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := NewStore(db)
	cleanup := func() { db.Close() }
	return store, cleanup
}

func fixtureValidYAML() []byte {
	return []byte(`workflow:
  name: Research Workflow
  type: research
  created: "2026-01-01"
phases:
  - id: phase-1
    name: "Literature Review"
    skill: "Search and summarize academic papers"
    description: "Find and summarize papers on a given topic"
    outputs:
      - "summary of findings"
    completion_criteria:
      - "At least 5 papers reviewed"
    depends_on: []
  - id: phase-2
    name: "Synthesis"
    skill: "Synthesize findings into coherent narrative"
    description: "Combine findings from all phases"
    outputs:
      - "synthesized narrative"
    completion_criteria:
      - "Narrative coherent"
    depends_on:
      - "phase-1"
`)
}

func fixtureMinimalYAML() []byte {
	return []byte(`workflow:
  name: Simple
phases:
  - id: p1
    name: "Step One"
    description: "First step"
`)
}

func TestImportWorkflowWithEntries_ValidYAML(t *testing.T) {
	store, cleanup := setupImportStore(t)
	defer cleanup()
	ctx := context.Background()

	yamlData := fixtureValidYAML()
	wf, slugs, err := store.ImportWorkflowWithEntries(ctx, yamlData, nil)
	if err != nil {
		t.Fatalf("ImportWorkflowWithEntries failed: %v", err)
	}

	if wf == nil {
		t.Fatal("expected workflow, got nil")
	}
	if wf.Name != "Research Workflow" {
		t.Errorf("expected workflow name 'Research Workflow', got %q", wf.Name)
	}
	if wf.Description != "Auto-imported from YAML" {
		t.Errorf("expected description 'Auto-imported from YAML', got %q", wf.Description)
	}

	if len(slugs) != 2 {
		t.Fatalf("expected 2 entry slugs, got %d", len(slugs))
	}

	// Verify entries exist by listing all skill entries
	entries, err := store.Entries.List(ctx, domain.EntryFilter{
		IncludeArchived: false,
	})
	if err != nil {
		t.Fatalf("List entries failed: %v", err)
	}

	importedCount := 0
	for _, e := range entries {
		if e.Entry.Type == domain.EntryTypeSkill {
			for _, s := range slugs {
				if e.Entry.Slug == s {
					importedCount++
					break
				}
			}
		}
	}
	if importedCount != 2 {
		t.Errorf("expected 2 imported skill entries, found %d", importedCount)
	}

	// Verify workflow steps
	steps, err := store.Workflows.GetSteps(ctx, wf.ID)
	if err != nil {
		t.Fatalf("GetSteps failed: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].OrderIndex != 1 {
		t.Errorf("expected step 1 order_index 1, got %d", steps[0].OrderIndex)
	}
	if steps[1].OrderIndex != 2 {
		t.Errorf("expected step 2 order_index 2, got %d", steps[1].OrderIndex)
	}
	if steps[0].EntrySlug == "" {
		t.Error("expected step 1 to have entry_slug")
	}
	if steps[1].EntrySlug == "" {
		t.Error("expected step 2 to have entry_slug")
	}

	// Verify step entry_slugs match returned slugs
	stepSlugs := map[string]bool{}
	for _, s := range steps {
		stepSlugs[s.EntrySlug] = true
	}
	for _, s := range slugs {
		if !stepSlugs[s] {
			t.Errorf("returned slug %q not found in steps", s)
		}
	}

	// Verify workflow is retrievable
	retrieved, err := store.Workflows.Get(ctx, wf.ID)
	if err != nil {
		t.Fatalf("Get workflow failed: %v", err)
	}
	if retrieved.Name != "Research Workflow" {
		t.Errorf("expected workflow name 'Research Workflow', got %q", retrieved.Name)
	}
}

func TestImportWorkflowWithEntries_InvalidYAML(t *testing.T) {
	store, cleanup := setupImportStore(t)
	defer cleanup()
	ctx := context.Background()

	_, _, err := store.ImportWorkflowWithEntries(ctx, []byte("not: valid: yaml: ["), nil)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}

	// Verify no entries were created (list by filter)
	entries, err := store.Entries.List(ctx, domain.EntryFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("List entries failed: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("expected 0 entries after failed import, got %d", len(entries))
	}

	// Verify no workflows were created
	wfs, err := store.Workflows.List(ctx, true)
	if err != nil {
		t.Fatalf("List workflows failed: %v", err)
	}
	if len(wfs) > 0 {
		t.Errorf("expected 0 workflows after failed import, got %d", len(wfs))
	}
}

func TestImportWorkflowWithEntries_EmptyPhases(t *testing.T) {
	store, cleanup := setupImportStore(t)
	defer cleanup()
	ctx := context.Background()

	yamlData := []byte(`workflow:
  name: Empty Workflow
  type: test
phases: []
`)
	_, _, err := store.ImportWorkflowWithEntries(ctx, yamlData, nil)
	if err == nil {
		t.Fatal("expected error for empty phases, got nil")
	}
}

func TestImportWorkflowWithEntries_SlugCollision(t *testing.T) {
	store, cleanup := setupImportStore(t)
	defer cleanup()
	ctx := context.Background()

	// First import
	yamlData := fixtureValidYAML()
	_, _, err := store.ImportWorkflowWithEntries(ctx, yamlData, nil)
	if err != nil {
		t.Fatalf("first import failed: %v", err)
	}

	// Second import with same name should trigger slug collision handling
	wf2, _, err := store.ImportWorkflowWithEntries(ctx, yamlData, nil)
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}

	// The second workflow should have a different slug due to collision
	if wf2.Slug == "research-workflow" {
		t.Error("expected second workflow to have a different slug due to collision")
	}
}

func TestImportWorkflowWithEntries_MinimalYAML(t *testing.T) {
	store, cleanup := setupImportStore(t)
	defer cleanup()
	ctx := context.Background()

	wf, slugs, err := store.ImportWorkflowWithEntries(ctx, fixtureMinimalYAML(), nil)
	if err != nil {
		t.Fatalf("ImportWorkflowWithEntries failed: %v", err)
	}
	if wf.Name != "Simple" {
		t.Errorf("expected 'Simple', got %q", wf.Name)
	}
	if len(slugs) != 1 {
		t.Fatalf("expected 1 slug, got %d", len(slugs))
	}

	// Verify entry exists by slug via list
	entries, err := store.Entries.List(ctx, domain.EntryFilter{IncludeArchived: false})
	if err != nil {
		t.Fatalf("List entries failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Entry.Slug == slugs[0] {
			found = true
			if e.Entry.Type != domain.EntryTypeSkill {
				t.Errorf("expected skill type, got %q", e.Entry.Type)
			}
			break
		}
	}
	if !found {
		t.Errorf("entry with slug %q not found", slugs[0])
	}
}

func TestImportWorkflowWithEntries_YAMLParsing(t *testing.T) {
	store, cleanup := setupImportStore(t)
	defer cleanup()
	ctx := context.Background()

	wf, slugs, err := store.ImportWorkflowWithEntries(ctx, fixtureValidYAML(), nil)
	if err != nil {
		t.Fatalf("ImportWorkflowWithEntries failed: %v", err)
	}

	// Verify the phase entry body contains expected YAML fields
	entries, err := store.Entries.List(ctx, domain.EntryFilter{IncludeArchived: false})
	if err != nil {
		t.Fatalf("List entries failed: %v", err)
	}

	_ = wf
	for _, s := range slugs {
		found := false
		for _, e := range entries {
			if e.Entry.Slug == s {
				found = true
				body := e.Entry.BodyOptional
				if body == "" {
					t.Errorf("entry %q has empty body", s)
					continue
				}
				// Verify body contains expected keys
				for _, key := range []string{"name:", "outputs:"} {
					if !strings.Contains(body, key) {
						t.Errorf("entry %q body missing key %q", s, key)
					}
				}
				break
			}
		}
		if !found {
			t.Errorf("slug %q not found in entries", s)
		}
	}
}

func TestImportWorkflowWithEntries_WithProject(t *testing.T) {
	store, cleanup := setupImportStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create a project first
	projID := "test-proj"
	proj := domain.Project{
		ID:     projID,
		Name:   "Test Project",
		Slug:   "test-project",
		Status: domain.StatusActive,
	}
	if err := store.Projects.Save(ctx, proj); err != nil {
		t.Fatalf("Save project failed: %v", err)
	}

	wf, slugs, err := store.ImportWorkflowWithEntries(ctx, fixtureMinimalYAML(), &projID)
	if err != nil {
		t.Fatalf("ImportWorkflowWithEntries failed: %v", err)
	}

	_ = wf
	if len(slugs) != 1 {
		t.Fatalf("expected 1 slug, got %d", len(slugs))
	}

	// Verify the entry has the correct project_id
	entries, err := store.Entries.List(ctx, domain.EntryFilter{
		IncludeArchived: false,
		ProjectID:       &projID,
	})
	if err != nil {
		t.Fatalf("List entries failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Entry.Slug == slugs[0] {
			found = true
			if e.Entry.ProjectID == nil || *e.Entry.ProjectID != projID {
				t.Errorf("expected project_id %q, got %v", projID, e.Entry.ProjectID)
			}
			break
		}
	}
	if !found {
		t.Errorf("entry with slug %q not found in project %q", slugs[0], projID)
	}
}

func TestBuildPhaseSkillYAML(t *testing.T) {
	phase := PhaseYAML{
		ID:                 "phase-1",
		Name:               "Test Phase",
		Skill:              "test-skill",
		Description:        "A test phase",
		Outputs:            []string{"out1", "out2"},
		CompletionCriteria: []string{"criteria1"},
		DependsOn:          []string{"phase-0"},
	}

	body, err := buildPhaseSkillYAML(phase)
	if err != nil {
		t.Fatalf("buildPhaseSkillYAML failed: %v", err)
	}

	expectedKeys := []string{
		"name: Test Phase",
		"skill: test-skill",
		"description: A test phase",
		"outputs:",
		"  - out1",
		"  - out2",
		"completion_criteria:",
		"  - criteria1",
		"depends_on:",
		"  - phase-0",
	}

	for _, key := range expectedKeys {
		if !strings.Contains(body, key) {
			t.Errorf("expected body to contain %q, got:\n%s", key, body)
		}
	}
}

func TestSlugifyForImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Literature Review", "literature-review"},
		{"Synthesis & Analysis", "synthesis-analysis"},
		{"  Spaces  ", "spaces"},
		{"!!!Special!!!", "special"},
		{"", "imported-entry"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got := slugifyForImport(tt.input)
			if got != tt.expected {
				t.Errorf("slugifyForImport(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
