package db

import (
	"context"
	"strings"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupEntryStore(t *testing.T) (EntryStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := &sqliteEntryStore{db: db}
	cleanup := func() { db.Close() }
	return store, cleanup
}

func TestUpsertEntryCreate(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:      "prd-fastapi",
		Name:    "FastAPI PRD",
		Type:    domain.EntryTypeSkill,
		Content: "Design the FastAPI backend architecture",
		Active:  true,
	}
	tags := []string{"go", "api", "backend"}

	err := store.UpsertEntry(ctx, entry, tags, nil)
	if err != nil {
		t.Fatalf("UpsertEntry failed: %v", err)
	}

	// Verify entry can be retrieved
	result, err := store.GetEntry(ctx, "prd-fastapi", false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Entry.ID != "prd-fastapi" {
		t.Errorf("Entry.ID = %q, want 'prd-fastapi'", result.Entry.ID)
	}
	if result.Entry.Name != "FastAPI PRD" {
		t.Errorf("Entry.Name = %q, want 'FastAPI PRD'", result.Entry.Name)
	}
	if result.Entry.Type != domain.EntryTypeSkill {
		t.Errorf("Entry.Type = %q, want 'skill'", result.Entry.Type)
	}
	if !result.Entry.Active {
		t.Errorf("Entry.Active should be true")
	}
	if len(result.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestUpsertEntryUpdate(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create
	entry := domain.Entry{
		ID:      "prd-fastapi",
		Name:    "FastAPI PRD",
		Type:    domain.EntryTypeSkill,
		Content: "Original content",
		Active:  true,
	}
	if err := store.UpsertEntry(ctx, entry, []string{"go"}, nil); err != nil {
		t.Fatalf("initial UpsertEntry failed: %v", err)
	}

	// Update
	entry.Name = "FastAPI PRD v2"
	entry.Content = "Updated content"
	if err := store.UpsertEntry(ctx, entry, []string{"go", "api"}, nil); err != nil {
		t.Fatalf("update UpsertEntry failed: %v", err)
	}

	result, err := store.GetEntry(ctx, "prd-fastapi", false)
	if err != nil {
		t.Fatalf("GetEntry after update failed: %v", err)
	}
	if result.Entry.Name != "FastAPI PRD v2" {
		t.Errorf("Name = %q, want 'FastAPI PRD v2'", result.Entry.Name)
	}
	if result.Entry.Content != "Updated content" {
		t.Errorf("Content = %q, want 'Updated content'", result.Entry.Content)
	}
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestUpsertEntryWithWorkflowSteps(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:      "my-workflow",
		Name:    "Test Workflow",
		Type:    domain.EntryTypeWorkflow,
		Content: "Run tests",
		Active:  true,
	}
	steps := []domain.WorkflowStep{
		{StepNum: 1, Role: domain.WorkflowRoleSystem, Content: "You are a tester"},
		{StepNum: 2, Role: domain.WorkflowRoleUser, Content: "Run tests"},
	}

	err := store.UpsertEntry(ctx, entry, nil, steps)
	if err != nil {
		t.Fatalf("UpsertEntry with steps failed: %v", err)
	}

	result, err := store.GetEntry(ctx, "my-workflow", false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Role != domain.WorkflowRoleSystem {
		t.Errorf("step 0 role = %q, want 'system'", result.Steps[0].Role)
	}
	if result.Steps[1].Content != "Run tests" {
		t.Errorf("step 1 content = %q, want 'Run tests'", result.Steps[1].Content)
	}
}

func TestGetEntryArchived(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:      "old-entry",
		Name:    "Old Entry",
		Type:    domain.EntryTypeNote,
		Content: "Old content",
		Active:  true,
	}
	if err := store.UpsertEntry(ctx, entry, nil, nil); err != nil {
		t.Fatalf("UpsertEntry failed: %v", err)
	}

	// Archive it
	if err := store.ArchiveEntry(ctx, "old-entry"); err != nil {
		t.Fatalf("ArchiveEntry failed: %v", err)
	}

	// Get without include_archived should return error
	_, err := store.GetEntry(ctx, "old-entry", false)
	if err == nil {
		t.Fatal("expected error for archived entry without include_archived")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("error should mention 'archived', got: %v", err)
	}

	// Get with include_archived should succeed
	result, err := store.GetEntry(ctx, "old-entry", true)
	if err != nil {
		t.Fatalf("GetEntry with include_archived failed: %v", err)
	}
	if !result.Entry.Active {
		// Active should be false in the result (archived)
		if result.Entry.ID != "old-entry" {
			t.Errorf("ID = %q, want 'old-entry'", result.Entry.ID)
		}
	}
}

func TestListEntries(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create entries
	entries := []domain.Entry{
		{ID: "e1", Name: "E1", Type: domain.EntryTypeSkill, Content: "C1", Active: true},
		{ID: "e2", Name: "E2", Type: domain.EntryTypePrompt, Content: "C2", Active: true},
		{ID: "e3", Name: "E3", Type: domain.EntryTypeSkill, Content: "C3", Active: true},
	}
	for _, e := range entries {
		if err := store.UpsertEntry(ctx, e, nil, nil); err != nil {
			t.Fatalf("UpsertEntry %s failed: %v", e.ID, err)
		}
	}

	// List all active
	results, err := store.ListEntries(ctx, domain.EntryFilter{})
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 entries, got %d", len(results))
	}

	// List by type
	skillType := "skill"
	results, err = store.ListEntries(ctx, domain.EntryFilter{Type: &skillType})
	if err != nil {
		t.Fatalf("ListEntries by type failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 skill entries, got %d", len(results))
	}

	// Archive e1 then list — should be excluded
	if err := store.ArchiveEntry(ctx, "e1"); err != nil {
		t.Fatalf("ArchiveEntry failed: %v", err)
	}
	results, err = store.ListEntries(ctx, domain.EntryFilter{})
	if err != nil {
		t.Fatalf("ListEntries after archive failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 active entries after archive, got %d", len(results))
	}

	// List with include_archived
	results, err = store.ListEntries(ctx, domain.EntryFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListEntries with include_archived failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 entries with include_archived, got %d", len(results))
	}
}

func TestUpsertEntryRemovesOldTags(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:      "test-entry",
		Name:    "Test",
		Type:    domain.EntryTypeSkill,
		Content: "Test",
		Active:  true,
	}

	// First upsert with tags A, B
	if err := store.UpsertEntry(ctx, entry, []string{"a", "b"}, nil); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	// Second upsert with tags B, C — A should be removed, B+C kept
	if err := store.UpsertEntry(ctx, entry, []string{"b", "c"}, nil); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	result, err := store.GetEntry(ctx, "test-entry", false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(result.Tags), result.Tags)
	}
	// Check tags b and c are present
	hasB := false
	hasC := false
	for _, tag := range result.Tags {
		if tag == "b" {
			hasB = true
		}
		if tag == "c" {
			hasC = true
		}
	}
	if !hasB {
		t.Error("tag 'b' should be present")
	}
	if !hasC {
		t.Error("tag 'c' should be present")
	}
}
