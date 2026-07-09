package app

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupEntryVersionService(t *testing.T) (*EntryVersionService, func()) {
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
	svc := NewEntryVersionService(store.EntryVersions, store.Entries)
	cleanup := func() { sqlDB.Close() }
	return svc, cleanup
}

func TestEntryVersionServiceListVersions(t *testing.T) {
	svc, cleanup := setupEntryVersionService(t)
	defer cleanup()
	ctx := context.Background()

	// Create entry and update it twice to generate 2 versions.
	entry := domain.Entry{
		ID:           "list-test",
		Title:        "v1",
		Slug:         "list-test",
		Type:         domain.EntryTypeSkill,
		Summary:      "s1",
		BodyOptional: "b1",
		Status:       domain.StatusActive,
	}
	if err := svc.entryStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("save v1 failed: %v", err)
	}
	entry.Title = "v2"
	if err := svc.entryStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("save v2 failed: %v", err)
	}
	entry.Title = "v3"
	if err := svc.entryStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("save v3 failed: %v", err)
	}

	versions, err := svc.ListVersions(ctx, "list-test")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	// Descending order: v2 first, then v1.
	if versions[0].VersionNumber != 2 {
		t.Errorf("first version should be 2 (descending), got %d", versions[0].VersionNumber)
	}
	if versions[1].VersionNumber != 1 {
		t.Errorf("second version should be 1, got %d", versions[1].VersionNumber)
	}
	if versions[0].Title != "v2" {
		t.Errorf("v2 title = %q, want 'v2'", versions[0].Title)
	}
}

func TestEntryVersionServiceRestoreVersion(t *testing.T) {
	svc, cleanup := setupEntryVersionService(t)
	defer cleanup()
	ctx := context.Background()

	// Create entry with v1 content.
	entry := domain.Entry{
		ID:           "restore-test",
		Title:        "Original Title",
		Slug:         "restore-test",
		Type:         domain.EntryTypePrompt,
		Summary:      "Original summary",
		BodyOptional: "Original body",
		Status:       domain.StatusActive,
	}
	if err := svc.entryStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("save v1 failed: %v", err)
	}

	// Update to v2.
	entry.Title = "Updated Title"
	entry.Summary = "Updated summary"
	entry.BodyOptional = "Updated body"
	if err := svc.entryStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("save v2 failed: %v", err)
	}

	// Restore v1.
	if err := svc.RestoreVersion(ctx, "restore-test", 1); err != nil {
		t.Fatalf("RestoreVersion failed: %v", err)
	}

	// Verify current entry has v1 content.
	current, err := svc.entryStore.Get(ctx, "restore-test", false)
	if err != nil {
		t.Fatalf("Get after restore failed: %v", err)
	}
	if current.Entry.Title != "Original Title" {
		t.Errorf("after restore title = %q, want 'Original Title'", current.Entry.Title)
	}
	if current.Entry.Summary != "Original summary" {
		t.Errorf("after restore summary = %q, want 'Original summary'", current.Entry.Summary)
	}
	if current.Entry.BodyOptional != "Original body" {
		t.Errorf("after restore body = %q, want 'Original body'", current.Entry.BodyOptional)
	}

	// v2 (pre-restore state) should have been archived as version 2.
	versions, err := svc.ListVersions(ctx, "restore-test")
	if err != nil {
		t.Fatalf("ListVersions after restore failed: %v", err)
	}
	// Versions: v2 (Updated content, archived from restore), v1 (Original)
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions after restore, got %d", len(versions))
	}
}

func TestEntryVersionServiceRestoreNonexistentVersion(t *testing.T) {
	svc, cleanup := setupEntryVersionService(t)
	defer cleanup()
	ctx := context.Background()

	// Create an entry.
	entry := domain.Entry{
		ID:           "nonexist-test",
		Title:        "Only version",
		Slug:         "nonexist-test",
		Type:         domain.EntryTypeReference,
		Summary:      "summary",
		BodyOptional: "body",
		Status:       domain.StatusActive,
	}
	if err := svc.entryStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Trying to restore version 99 should fail.
	err := svc.RestoreVersion(ctx, "nonexist-test", 99)
	if err == nil {
		t.Fatal("expected error for non-existent version")
	}
}

func TestEntryVersionServiceRestoreNonexistentEntry(t *testing.T) {
	svc, cleanup := setupEntryVersionService(t)
	defer cleanup()
	ctx := context.Background()

	err := svc.RestoreVersion(ctx, "no-such-entry", 1)
	if err == nil {
		t.Fatal("expected error for non-existent entry")
	}
}

func TestEntryVersionServiceListVersionsEmptyForNewEntry(t *testing.T) {
	svc, cleanup := setupEntryVersionService(t)
	defer cleanup()
	ctx := context.Background()

	// Non-existent entry — should return empty list, not error.
	versions, err := svc.ListVersions(ctx, "no-such-entry")
	if err != nil {
		t.Fatalf("ListVersions should not error for non-existent entry: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions for non-existent entry, got %d", len(versions))
	}
}
