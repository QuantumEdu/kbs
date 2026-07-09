package app

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupPackExportService(t *testing.T) (*VaultPackExportService, *db.Store, func()) {
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
	exportSvc := NewVaultPackExportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts, store.Workflows)
	cleanup := func() { sqlDB.Close() }
	return exportSvc, store, cleanup
}

func TestPackExportRoundTrip(t *testing.T) {
	exportSvc, store, cleanup := setupPackExportService(t)
	defer cleanup()
	ctx := context.Background()

	// ... (importSvc created inline below)

	// Seed some data.
	e := domain.Entry{
		ID: "test-entry", Title: "Test Entry", Slug: "test-entry",
		Type: domain.EntryTypeSkill, Summary: "A test", BodyOptional: "Body",
		Status: domain.StatusActive,
	}
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	// Export to a pack file.
	packPath := t.TempDir() + "/test.svpack"
	if err := exportSvc.ExportPack(ctx, ExportPackInput{
		Author:      "test-author",
		Version:     "1.0.0",
		Description: "Test pack",
		OutputPath:  packPath,
	}); err != nil {
		t.Fatalf("ExportPack failed: %v", err)
	}

	// Verify the file is valid JSON with pack metadata.
	data, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatalf("Read file: %v", err)
	}
	if !strings.Contains(string(data), `"pack"`) {
		t.Error("expected pack key in export")
	}
	if !strings.Contains(string(data), `"data"`) {
		t.Error("expected data key in export")
	}

	// Import it back in a new DB.
	sqlDB2, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB2 failed: %v", err)
	}
	defer sqlDB2.Close()
	if err := db.RunMigrations(sqlDB2); err != nil {
		t.Fatalf("RunMigrations2 failed: %v", err)
	}
	store2 := db.NewStore(sqlDB2)
	importSvc2 := NewVaultImportService(store2.ImportExport, store2.Entries, store2.Projects, store2.Artifacts)

	if err := importSvc2.Import(ctx, packPath); err != nil {
		t.Fatalf("Import pack failed: %v", err)
	}

	// Verify the entry was imported.
	result, err := store2.Entries.Get(ctx, "test-entry", false)
	if err != nil {
		t.Fatalf("Get imported entry: %v", err)
	}
	if result.Entry.Title != "Test Entry" {
		t.Errorf("imported title = %q, want 'Test Entry'", result.Entry.Title)
	}
}

func TestPackImportWithPrefix(t *testing.T) {
	exportSvc, store, cleanup := setupPackExportService(t)
	defer cleanup()
	ctx := context.Background()

	// Seed data.
	e := domain.Entry{
		ID: "pref-entry", Title: "Prefix Entry", Slug: "pref-entry",
		Type: domain.EntryTypeSkill, Summary: "Test", BodyOptional: "Body",
		Status: domain.StatusActive,
	}
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	// Export.
	packPath := t.TempDir() + "/pref.svpack"
	if err := exportSvc.ExportPack(ctx, ExportPackInput{
		Author: "a", Version: "1", OutputPath: packPath,
	}); err != nil {
		t.Fatalf("ExportPack: %v", err)
	}

	// Import with prefix in new DB.
	sqlDB2, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB2: %v", err)
	}
	defer sqlDB2.Close()
	if err := db.RunMigrations(sqlDB2); err != nil {
		t.Fatalf("RunMigrations2: %v", err)
	}
	store2 := db.NewStore(sqlDB2)
	importSvc2 := NewVaultImportService(store2.ImportExport, store2.Entries, store2.Projects, store2.Artifacts)

	if err := importSvc2.ImportWithPrefix(ctx, packPath, "ns/"); err != nil {
		t.Fatalf("ImportWithPrefix: %v", err)
	}

	// The entry should be at the prefixed ID.
	result, err := store2.Entries.Get(ctx, "ns/pref-entry", false)
	if err != nil {
		t.Fatalf("Get prefixed entry: %v", err)
	}
	if result.Entry.Title != "Prefix Entry" {
		t.Errorf("title = %q, want 'Prefix Entry'", result.Entry.Title)
	}

	// The original slug is unchanged, so Get by unprefixed string still
	// works (it matches the slug). Verify the ID is correctly prefixed.
	if result.Entry.ID != "ns/pref-entry" {
		t.Errorf("expected prefixed ID 'ns/pref-entry', got %q", result.Entry.ID)
	}
}

func TestBareImportBackwardCompatible(t *testing.T) {
	_, store, cleanup := setupPackExportService(t)
	defer cleanup()
	ctx := context.Background()

	// Seed data and do a bare export (standard VaultExport).
	e := domain.Entry{
		ID: "bare-entry", Title: "Bare", Slug: "bare-entry",
		Type: domain.EntryTypePrompt, Summary: "B", BodyOptional: "B",
		Status: domain.StatusActive,
	}
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Export bare (using VaultExportService).
	exportSvcBare := NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	barePath := t.TempDir() + "/bare.json"
	if err := exportSvcBare.Export(ctx, barePath); err != nil {
		t.Fatalf("Bare export: %v", err)
	}

	// Import bare in new DB — should work without --pack flag.
	sqlDB2, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB2: %v", err)
	}
	defer sqlDB2.Close()
	if err := db.RunMigrations(sqlDB2); err != nil {
		t.Fatalf("RunMigrations2: %v", err)
	}
	store2 := db.NewStore(sqlDB2)
	importSvc2 := NewVaultImportService(store2.ImportExport, store2.Entries, store2.Projects, store2.Artifacts)

	if err := importSvc2.Import(ctx, barePath); err != nil {
		t.Fatalf("Bare import: %v", err)
	}

	result, err := store2.Entries.Get(ctx, "bare-entry", false)
	if err != nil {
		t.Fatalf("Get after bare import: %v", err)
	}
	if result.Entry.Title != "Bare" {
		t.Errorf("title = %q, want 'Bare'", result.Entry.Title)
	}
}

func TestImportPackWithEmptyPrefix(t *testing.T) {
	exportSvc, store, cleanup := setupPackExportService(t)
	defer cleanup()
	ctx := context.Background()

	e := domain.Entry{
		ID: "no-prefix", Title: "No Prefix", Slug: "no-prefix",
		Type: domain.EntryTypeSkill, Summary: "NP", BodyOptional: "NP",
		Status: domain.StatusActive,
	}
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("Save: %v", err)
	}
	packPath := t.TempDir() + "/np.svpack"
	if err := exportSvc.ExportPack(ctx, ExportPackInput{Author: "a", Version: "1", OutputPath: packPath}); err != nil {
		t.Fatalf("ExportPack: %v", err)
	}

	sqlDB2, _ := db.OpenDB(":memory:")
	defer sqlDB2.Close()
	db.RunMigrations(sqlDB2)
	store2 := db.NewStore(sqlDB2)
	importSvc2 := NewVaultImportService(store2.ImportExport, store2.Entries, store2.Projects, store2.Artifacts)

	// Empty prefix should import as-is.
	if err := importSvc2.ImportWithPrefix(ctx, packPath, ""); err != nil {
		t.Fatalf("ImportWithPrefix empty: %v", err)
	}
	result, err := store2.Entries.Get(ctx, "no-prefix", false)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Entry.Title != "No Prefix" {
		t.Errorf("title = %q", result.Entry.Title)
	}
}

func TestPackExportJSONValid(t *testing.T) {
	_, store, cleanup := setupPackExportService(t)
	defer cleanup()
	ctx := context.Background()

	exportSvcBare := NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	packOutput, err := exportSvcBare.ExportJSON(ctx)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	// Verify it's valid VaultExport JSON.
	var ve domain.VaultExport
	if err := json.Unmarshal(packOutput, &ve); err != nil {
		t.Fatalf("unmarshal export JSON: %v", err)
	}
}
