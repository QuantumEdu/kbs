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

func TestSaveEntryCreate(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:           "prd-fastapi",
		Title:        "FastAPI PRD",
		Slug:         "fastapi-prd",
		Type:         domain.EntryTypeSkill,
		Summary:      "PRD document",
		BodyOptional: "Design the FastAPI backend architecture",
		Status:       domain.StatusActive,
	}
	tags := []string{"go", "api", "backend"}

	err := store.Save(ctx, entry, tags)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := store.Get(ctx, "prd-fastapi", false)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result.Entry.ID != "prd-fastapi" {
		t.Errorf("Entry.ID = %q, want 'prd-fastapi'", result.Entry.ID)
	}
	if result.Entry.Title != "FastAPI PRD" {
		t.Errorf("Entry.Title = %q, want 'FastAPI PRD'", result.Entry.Title)
	}
	if result.Entry.Type != domain.EntryTypeSkill {
		t.Errorf("Entry.Type = %q, want 'skill'", result.Entry.Type)
	}
	if result.Entry.Status != domain.StatusActive {
		t.Errorf("Entry.Status = %q, want 'active'", result.Entry.Status)
	}
	if len(result.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestSaveEntryUpdate(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:           "prd-fastapi",
		Title:        "FastAPI PRD",
		Slug:         "fastapi-prd",
		Type:         domain.EntryTypeSkill,
		BodyOptional: "Original content",
		Status:       domain.StatusActive,
	}
	if err := store.Save(ctx, entry, []string{"go"}); err != nil {
		t.Fatalf("initial Save failed: %v", err)
	}

	entry.Title = "FastAPI PRD v2"
	entry.BodyOptional = "Updated content"
	if err := store.Save(ctx, entry, []string{"go", "api"}); err != nil {
		t.Fatalf("update Save failed: %v", err)
	}

	result, err := store.Get(ctx, "prd-fastapi", false)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if result.Entry.Title != "FastAPI PRD v2" {
		t.Errorf("Title = %q, want 'FastAPI PRD v2'", result.Entry.Title)
	}
	if result.Entry.BodyOptional != "Updated content" {
		t.Errorf("BodyOptional = %q, want 'Updated content'", result.Entry.BodyOptional)
	}
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestGetEntryArchived(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:           "old-entry",
		Title:        "Old Entry",
		Slug:         "old-entry",
		Type:         domain.EntryTypeSession,
		BodyOptional: "Old content",
		Status:       domain.StatusActive,
	}
	if err := store.Save(ctx, entry, nil); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Archive(ctx, "old-entry"); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	_, err := store.Get(ctx, "old-entry", false)
	if err == nil {
		t.Fatal("expected error for archived entry without include_archived")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("error should mention 'archived', got: %v", err)
	}

	result, err := store.Get(ctx, "old-entry", true)
	if err != nil {
		t.Fatalf("Get with include_archived failed: %v", err)
	}
	if result.Entry.Status != domain.StatusArchived {
		t.Errorf("Status = %q, want 'archived'", result.Entry.Status)
	}
}

func TestListEntries(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entries := []domain.Entry{
		{ID: "e1", Title: "E1", Slug: "e1", Type: domain.EntryTypeSkill, BodyOptional: "C1", Status: domain.StatusActive},
		{ID: "e2", Title: "E2", Slug: "e2", Type: domain.EntryTypePrompt, BodyOptional: "C2", Status: domain.StatusActive},
		{ID: "e3", Title: "E3", Slug: "e3", Type: domain.EntryTypeSkill, BodyOptional: "C3", Status: domain.StatusActive},
	}
	for _, e := range entries {
		if err := store.Save(ctx, e, nil); err != nil {
			t.Fatalf("Save %s failed: %v", e.ID, err)
		}
	}

	results, err := store.List(ctx, domain.EntryFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 entries, got %d", len(results))
	}

	skillType := "skill"
	results, err = store.List(ctx, domain.EntryFilter{Type: &skillType})
	if err != nil {
		t.Fatalf("List by type failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 skill entries, got %d", len(results))
	}

	if err := store.Archive(ctx, "e1"); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	results, err = store.List(ctx, domain.EntryFilter{})
	if err != nil {
		t.Fatalf("List after archive failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 active entries after archive, got %d", len(results))
	}

	results, err = store.List(ctx, domain.EntryFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("List with include_archived failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 entries with include_archived, got %d", len(results))
	}
}

func TestSaveEntryRemovesOldTags(t *testing.T) {
	store, cleanup := setupEntryStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:           "test-entry",
		Title:        "Test",
		Slug:         "test",
		Type:         domain.EntryTypeSkill,
		BodyOptional: "Test",
		Status:       domain.StatusActive,
	}

	if err := store.Save(ctx, entry, []string{"a", "b"}); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	if err := store.Save(ctx, entry, []string{"b", "c"}); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	result, err := store.Get(ctx, "test-entry", false)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(result.Tags), result.Tags)
	}
	hasB := false
	hasC := false
	for _, tag := range result.Tags {
		if tag.Name == "b" {
			hasB = true
		}
		if tag.Name == "c" {
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
