package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupImportExportStore(t *testing.T) (EntryStore, ImportExportStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	entryStore := &sqliteEntryStore{db: db}
	ieStore := &sqliteImportExportStore{db: db}
	cleanup := func() { db.Close() }
	return entryStore, ieStore, cleanup
}

func TestExportRoundTrip(t *testing.T) {
	estore, iestore, cleanup := setupImportExportStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create data
	estore.UpsertEntry(ctx, domain.Entry{ID: "e1", Name: "Entry 1", Type: domain.EntryTypeSkill, Content: "Content 1", Active: true}, []string{"tag1"}, nil)
	estore.UpsertEntry(ctx, domain.Entry{ID: "e2", Name: "Entry 2", Type: domain.EntryTypePrompt, Content: "Content 2", Active: false}, []string{"tag2"}, nil)

	// Export
	exported, err := iestore.ExportAll(ctx)
	if err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}

	if exported.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", exported.SchemaVersion)
	}
	if exported.AppVersion != "v1-alpha" {
		t.Errorf("AppVersion = %q, want 'v1-alpha'", exported.AppVersion)
	}
	if exported.ExportedAt == "" {
		t.Error("ExportedAt should not be empty")
	}
	if exported.Source != "skillvault" {
		t.Errorf("Source = %q, want 'skillvault'", exported.Source)
	}

	// Entries should be exported
	if len(exported.Data.Entries) != 2 {
		t.Fatalf("expected 2 exported entries, got %d", len(exported.Data.Entries))
	}
	if len(exported.Data.EntryTags) != 2 {
		t.Errorf("expected 2 exported tags, got %d", len(exported.Data.EntryTags))
	}

	// Import back into fresh DB
	db2, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB2 failed: %v", err)
	}
	defer db2.Close()
	if err := RunMigrations(db2); err != nil {
		t.Fatalf("RunMigrations2 failed: %v", err)
	}

	ieStore2 := &sqliteImportExportStore{db: db2}
	if err := ieStore2.ImportAll(ctx, exported); err != nil {
		t.Fatalf("ImportAll failed: %v", err)
	}

	// Verify re-export matches
	reexported, err := ieStore2.ExportAll(ctx)
	if err != nil {
		t.Fatalf("Re-ExportAll failed: %v", err)
	}
	if len(reexported.Data.Entries) != 2 {
		t.Errorf("round-trip: expected 2 entries, got %d", len(reexported.Data.Entries))
	}
}

func TestImportRejectsMissingVersion(t *testing.T) {
	_, iestore, cleanup := setupImportExportStore(t)
	defer cleanup()
	ctx := context.Background()

	err := iestore.ImportAll(ctx, domain.VaultExport{})
	if err == nil {
		t.Fatal("expected error for missing schema_version")
	}
}

func TestImportRejectsHigherVersion(t *testing.T) {
	_, iestore, cleanup := setupImportExportStore(t)
	defer cleanup()
	ctx := context.Background()

	err := iestore.ImportAll(ctx, domain.VaultExport{SchemaVersion: 99})
	if err == nil {
		t.Fatal("expected error for schema_version > 1")
	}
}
