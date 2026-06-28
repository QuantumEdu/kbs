package app

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/db"

	_ "modernc.org/sqlite"
)

func setupVersionServices(t *testing.T) (*db.Store, *EntryVersionService, *EntryService, func()) {
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
	versionSvc := NewEntryVersionService(store.EntryVersions, store.Entries)

	cleanup := func() { sqlDB.Close() }
	return store, versionSvc, entrySvc, cleanup
}

func TestListVersionsDescendingOrder(t *testing.T) {
	_, versionSvc, entrySvc, cleanup := setupVersionServices(t)
	defer cleanup()
	ctx := context.Background()

	// Create an entry.
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Original Title",
		Type:    "skill",
		Summary: "Original summary",
		Body:    "Original body",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}
	entryID := result.Entry.Entry.ID

	// Update to trigger auto-archive.
	entry := result.Entry.Entry
	entry.Title = "Updated Title"
	entry.Summary = "Updated summary"
	entry.BodyOptional = "Updated body"
	tags := make([]string, len(result.Entry.Tags))
	for i, tag := range result.Entry.Tags {
		tags[i] = tag.Name
	}
	if err := entrySvc.Save(ctx, entry, tags); err != nil {
		t.Fatalf("Save (update) failed: %v", err)
	}

	// Update again for a second version.
	entry.Title = "Final Title"
	entry.Summary = "Final summary"
	entry.BodyOptional = "Final body"
	if err := entrySvc.Save(ctx, entry, tags); err != nil {
		t.Fatalf("Save (second update) failed: %v", err)
	}

	// List versions — should be in descending order (newest first).
	versions, err := versionSvc.ListVersions(ctx, entryID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}

	// Descending order: highest version number first.
	if versions[0].VersionNumber <= versions[1].VersionNumber {
		t.Errorf("versions should be sorted descending: v%d before v%d",
			versions[0].VersionNumber, versions[1].VersionNumber)
	}

	// Latest version should be the one BEFORE the current ("Updated Title").
	if versions[0].Title != "Updated Title" {
		t.Errorf("first version Title = %q, want 'Updated Title'", versions[0].Title)
	}
	if versions[1].Title != "Original Title" {
		t.Errorf("second version Title = %q, want 'Original Title'", versions[1].Title)
	}
}

func TestRestoreVersionCreatesNewVersionWithPreRestoreContent(t *testing.T) {
	_, versionSvc, entrySvc, cleanup := setupVersionServices(t)
	defer cleanup()
	ctx := context.Background()

	// Create an entry.
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "First State",
		Type:    "skill",
		Summary: "First summary",
		Body:    "First body",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}
	entryID := result.Entry.Entry.ID

	// Update — this auto-archives "First State" as version 1.
	entry := result.Entry.Entry
	entry.Title = "Second State"
	entry.Summary = "Second summary"
	entry.BodyOptional = "Second body"
	tags := make([]string, len(result.Entry.Tags))
	for i, tag := range result.Entry.Tags {
		tags[i] = tag.Name
	}
	if err := entrySvc.Save(ctx, entry, tags); err != nil {
		t.Fatalf("Save (update) failed: %v", err)
	}

	// Restore to version 1 ("First State").
	restored, err := versionSvc.RestoreVersion(ctx, entryID, 1)
	if err != nil {
		t.Fatalf("RestoreVersion failed: %v", err)
	}
	if restored.Title != "First State" {
		t.Errorf("restored Title = %q, want 'First State'", restored.Title)
	}
	if restored.Summary != "First summary" {
		t.Errorf("restored Summary = %q, want 'First summary'", restored.Summary)
	}
	if restored.BodyOptional != "First body" {
		t.Errorf("restored Body = %q, want 'First body'", restored.BodyOptional)
	}

	// Verify versions: the pre-restore state ("Second State") is now version 2.
	versions, err := versionSvc.ListVersions(ctx, entryID)
	if err != nil {
		t.Fatalf("ListVersions after restore failed: %v", err)
	}
	if len(versions) < 2 {
		t.Fatalf("expected at least 2 versions after restore, got %d", len(versions))
	}
	if versions[0].Title != "Second State" {
		t.Errorf("latest version Title = %q, want 'Second State' (pre-restore)", versions[0].Title)
	}
	if versions[1].Title != "First State" {
		t.Errorf("version 1 Title = %q, want 'First State'", versions[1].Title)
	}
}

func TestRestoreNonexistentVersionReturnsError(t *testing.T) {
	_, versionSvc, entrySvc, cleanup := setupVersionServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Only State",
		Type:    "skill",
		Summary: "Only summary",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	// Restoring a version that doesn't exist should error.
	_, err = versionSvc.RestoreVersion(ctx, result.Entry.Entry.ID, 99)
	if err == nil {
		t.Fatal("expected error restoring nonexistent version")
	}
}

func TestRestoreNonexistentEntryReturnsError(t *testing.T) {
	_, versionSvc, _, cleanup := setupVersionServices(t)
	defer cleanup()
	ctx := context.Background()

	_, err := versionSvc.RestoreVersion(ctx, "nonexistent-entry", 1)
	if err == nil {
		t.Fatal("expected error restoring version for nonexistent entry")
	}
}

func TestListVersionsEmptyForNoVersions(t *testing.T) {
	_, versionSvc, entrySvc, cleanup := setupVersionServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "No-Version Entry",
		Type:    "skill",
		Summary: "No versions",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	versions, err := versionSvc.ListVersions(ctx, result.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}
