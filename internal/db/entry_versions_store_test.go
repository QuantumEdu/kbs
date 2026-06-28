package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupEntryVersionStore(t *testing.T) (EntryVersionStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := &sqliteEntryVersionStore{db: db}
	cleanup := func() { db.Close() }
	return store, cleanup
}

// seedEntry inserts a minimal entry so FK constraints pass for version tests.
func seedEntry(t *testing.T, store EntryVersionStore, entryID, title string) {
	t.Helper()
	// We need raw db access to insert an entry. The store is *sqliteEntryVersionStore.
	s := store.(*sqliteEntryVersionStore)
	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO entries (id, title, slug, type, status) VALUES (?, ?, ?, 'skill', 'active')
	`, entryID, title, title+"-slug")
	if err != nil {
		t.Fatalf("seed entry failed: %v", err)
	}
}

func TestSaveVersion(t *testing.T) {
	store, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	seedEntry(t, store, "entry-1", "Entry One")

	v := domain.EntryVersion{
		VersionID:     "v1-abc",
		EntryID:       "entry-1",
		VersionNumber: 1,
		Title:         "First Title",
		Summary:       "First summary",
		BodyOptional:  "First body",
	}

	if err := store.SaveVersion(ctx, v); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Retrieve it
	got, err := store.GetVersion(ctx, "entry-1", 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if got.VersionID != "v1-abc" {
		t.Errorf("VersionID = %q, want 'v1-abc'", got.VersionID)
	}
	if got.EntryID != "entry-1" {
		t.Errorf("EntryID = %q, want 'entry-1'", got.EntryID)
	}
	if got.VersionNumber != 1 {
		t.Errorf("VersionNumber = %d, want 1", got.VersionNumber)
	}
	if got.Title != "First Title" {
		t.Errorf("Title = %q, want 'First Title'", got.Title)
	}
	if got.Summary != "First summary" {
		t.Errorf("Summary = %q, want 'First summary'", got.Summary)
	}
	if got.BodyOptional != "First body" {
		t.Errorf("BodyOptional = %q, want 'First body'", got.BodyOptional)
	}
}

func TestListVersionsDescending(t *testing.T) {
	store, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	seedEntry(t, store, "entry-1", "Entry One")

	versions := []domain.EntryVersion{
		{VersionID: "v1", EntryID: "entry-1", VersionNumber: 1, Title: "V1", Summary: "S1", BodyOptional: "B1"},
		{VersionID: "v2", EntryID: "entry-1", VersionNumber: 2, Title: "V2", Summary: "S2", BodyOptional: "B2"},
		{VersionID: "v3", EntryID: "entry-1", VersionNumber: 3, Title: "V3", Summary: "S3", BodyOptional: "B3"},
	}

	for _, v := range versions {
		if err := store.SaveVersion(ctx, v); err != nil {
			t.Fatalf("SaveVersion %d failed: %v", v.VersionNumber, err)
		}
	}

	results, err := store.ListVersions(ctx, "entry-1")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(results))
	}

	// Must be descending by version_number
	if results[0].VersionNumber != 3 {
		t.Errorf("first version = %d, want 3", results[0].VersionNumber)
	}
	if results[1].VersionNumber != 2 {
		t.Errorf("second version = %d, want 2", results[1].VersionNumber)
	}
	if results[2].VersionNumber != 1 {
		t.Errorf("third version = %d, want 1", results[2].VersionNumber)
	}
}

func TestGetVersionByNumber(t *testing.T) {
	store, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	seedEntry(t, store, "entry-a", "Entry A")

	v1 := domain.EntryVersion{VersionID: "va1", EntryID: "entry-a", VersionNumber: 1, Title: "A1", Summary: "SA1", BodyOptional: "BA1"}
	v2 := domain.EntryVersion{VersionID: "va2", EntryID: "entry-a", VersionNumber: 2, Title: "A2", Summary: "SA2", BodyOptional: "BA2"}

	if err := store.SaveVersion(ctx, v1); err != nil {
		t.Fatalf("SaveVersion 1 failed: %v", err)
	}
	if err := store.SaveVersion(ctx, v2); err != nil {
		t.Fatalf("SaveVersion 2 failed: %v", err)
	}

	got, err := store.GetVersion(ctx, "entry-a", 2)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if got.Title != "A2" {
		t.Errorf("Title = %q, want 'A2'", got.Title)
	}

	got, err = store.GetVersion(ctx, "entry-a", 1)
	if err != nil {
		t.Fatalf("GetVersion 1 failed: %v", err)
	}
	if got.Title != "A1" {
		t.Errorf("Title = %q, want 'A1'", got.Title)
	}
}

func TestListVersionsEmpty(t *testing.T) {
	store, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	results, err := store.ListVersions(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 versions, got %d", len(results))
	}
}

func TestGetVersionNotFound(t *testing.T) {
	store, cleanup := setupEntryVersionStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.GetVersion(ctx, "nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}
