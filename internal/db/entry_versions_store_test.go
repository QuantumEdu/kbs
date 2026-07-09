package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupEntryVersionStore(t *testing.T) (*sqliteEntryVersionStore, *sqliteEntryStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	versionStore := &sqliteEntryVersionStore{db: db}
	entryStore := &sqliteEntryStore{db: db}
	cleanup := func() { db.Close() }
	return versionStore, entryStore, cleanup
}

func TestSaveAndListVersions(t *testing.T) {
	vStore, eStore, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create an entry first.
	entry := domain.Entry{
		ID:           "test-entry",
		Title:        "Original Title",
		Slug:         "test-entry",
		Type:         domain.EntryTypeSkill,
		Summary:      "Original summary",
		BodyOptional: "Original body",
		Status:       domain.StatusActive,
	}
	if err := eStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	// After first save (new entry), no versions should exist.
	versions, err := vStore.ListVersions(ctx, "test-entry")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions for new entry, got %d", len(versions))
	}

	// Update the entry to trigger archival.
	entry.Title = "Updated Title"
	entry.Summary = "Updated summary"
	if err := eStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("update Save failed: %v", err)
	}

	// Now one version should exist.
	versions, err = vStore.ListVersions(ctx, "test-entry")
	if err != nil {
		t.Fatalf("ListVersions after update failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version after update, got %d", len(versions))
	}
	v := versions[0]
	if v.VersionNumber != 1 {
		t.Errorf("version number = %d, want 1", v.VersionNumber)
	}
	if v.Title != "Original Title" {
		t.Errorf("archived title = %q, want 'Original Title'", v.Title)
	}
	if v.Summary != "Original summary" {
		t.Errorf("archived summary = %q, want 'Original summary'", v.Summary)
	}
	if v.BodyOptional != "Original body" {
		t.Errorf("archived body = %q, want 'Original body'", v.BodyOptional)
	}
	if v.EntryID != "test-entry" {
		t.Errorf("entry_id = %q, want 'test-entry'", v.EntryID)
	}
	if v.VersionID == "" {
		t.Error("version_id should not be empty")
	}

	// Update again to get version 2.
	entry.Title = "Title v3"
	if err := eStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("third Save failed: %v", err)
	}

	versions, err = vStore.ListVersions(ctx, "test-entry")
	if err != nil {
		t.Fatalf("ListVersions after third save failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	// Versions should be descending.
	if versions[0].VersionNumber != 2 {
		t.Errorf("first version should be 2 (descending), got %d", versions[0].VersionNumber)
	}
	if versions[1].VersionNumber != 1 {
		t.Errorf("second version should be 1, got %d", versions[1].VersionNumber)
	}
}

func TestGetVersion(t *testing.T) {
	vStore, eStore, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create an entry and update it once to create a version.
	entry := domain.Entry{
		ID:           "get-test",
		Title:        "v1 Title",
		Slug:         "get-test",
		Type:         domain.EntryTypeSkill,
		Summary:      "v1 summary",
		BodyOptional: "v1 body",
		Status:       domain.StatusActive,
	}
	if err := eStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	entry.Title = "v2 Title"
	if err := eStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	// Retrieve version 1.
	v, err := vStore.GetVersion(ctx, "get-test", 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if v.Title != "v1 Title" {
		t.Errorf("restored title = %q, want 'v1 Title'", v.Title)
	}

	// Non-existent version should error.
	_, err = vStore.GetVersion(ctx, "get-test", 99)
	if err == nil {
		t.Fatal("expected error for non-existent version")
	}

	// Non-existent entry should error.
	_, err = vStore.GetVersion(ctx, "nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for non-existent entry")
	}
}

func TestListVersionsEmptyResults(t *testing.T) {
	vStore, _, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	// Non-existent entry returns empty list.
	versions, err := vStore.ListVersions(ctx, "no-such-entry")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions for non-existent entry, got %d", len(versions))
	}
}

func TestArchiveOnlyOnContentChange(t *testing.T) {
	vStore, eStore, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:           "same-entry",
		Title:        "Title",
		Slug:         "same-entry",
		Type:         domain.EntryTypeSkill,
		Summary:      "Summary",
		BodyOptional: "Body",
		Status:       domain.StatusActive,
	}
	if err := eStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	// Save again with same content (only tags different).
	if err := eStore.Save(ctx, entry, []string{"new-tag"}); err != nil {
		t.Fatalf("second Save (same content) failed: %v", err)
	}

	versions, err := vStore.ListVersions(ctx, "same-entry")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions when content unchanged, got %d", len(versions))
	}
}

func TestVersionAutoIncrement(t *testing.T) {
	vStore, eStore, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{
		ID:           "inc-entry",
		Title:        "v1",
		Slug:         "inc-entry",
		Type:         domain.EntryTypeSkill,
		Summary:      "s1",
		BodyOptional: "b1",
		Status:       domain.StatusActive,
	}
	if err := eStore.Save(ctx, entry, nil); err != nil {
		t.Fatalf("save v1 failed: %v", err)
	}

	for i := 2; i <= 5; i++ {
		entry.Title = fmt.Sprintf("v%d", i)
		if err := eStore.Save(ctx, entry, nil); err != nil {
			t.Fatalf("save v%d failed: %v", i, err)
		}
	}

	versions, err := vStore.ListVersions(ctx, "inc-entry")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	// 4 updates → 4 versions (versions 1-4). v5 is current.
	if len(versions) != 4 {
		t.Fatalf("expected 4 versions, got %d", len(versions))
	}
	for _, v := range versions {
		if v.VersionNumber < 1 || v.VersionNumber > 4 {
			t.Errorf("unexpected version number %d", v.VersionNumber)
		}
	}
}

func TestListVersionsExcludesOtherEntries(t *testing.T) {
	vStore, eStore, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	// Entry A: 2 versions.
	a := domain.Entry{ID: "entry-a", Title: "A v1", Slug: "entry-a", Type: domain.EntryTypeSkill, BodyOptional: "A", Status: domain.StatusActive}
	eStore.Save(ctx, a, nil)
	a.Title = "A v2"
	eStore.Save(ctx, a, nil)

	// Entry B: 1 version.
	b := domain.Entry{ID: "entry-b", Title: "B v1", Slug: "entry-b", Type: domain.EntryTypeSkill, BodyOptional: "B", Status: domain.StatusActive}
	eStore.Save(ctx, b, nil)
	b.Title = "B v2"
	eStore.Save(ctx, b, nil)

	// List versions for entry A only.
	versions, err := vStore.ListVersions(ctx, "entry-a")
	if err != nil {
		t.Fatalf("ListVersions for entry-a failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version for entry-a, got %d", len(versions))
	}
	if versions[0].EntryID != "entry-a" {
		t.Errorf("wrong entry_id: %q", versions[0].EntryID)
	}
}
