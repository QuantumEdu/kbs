package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupPackExportServices(t *testing.T) (*VaultPackExportService, *VaultImportService, *EntryService, *ProjectService, *db.Store, func()) {
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
	projectSvc := NewProjectService(store.Projects)
	packExportSvc := NewVaultPackExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	importSvc := NewVaultImportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts)

	cleanup := func() { sqlDB.Close() }
	return packExportSvc, importSvc, entrySvc, projectSvc, store, cleanup
}

func TestPackExportRoundTrip(t *testing.T) {
	packSvc, _, entrySvc, projectSvc, _, cleanup := setupPackExportServices(t)
	defer cleanup()
	ctx := context.Background()

	// Seed data.
	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Pack Entry",
		Type:    "skill",
		Summary: "A packable entry",
		Project: "testproj",
		Tags:    []string{"pack-test"},
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	// Export pack.
	tmpDir := t.TempDir()
	packPath := filepath.Join(tmpDir, "test-pack.svpack")
	if err := packSvc.ExportPack(ctx, PackExportInput{
		Pack:        "Test Pack",
		Author:      "tester",
		Version:     "1.0",
		Description: "A test pack",
		OutputPath:  packPath,
	}); err != nil {
		t.Fatalf("ExportPack failed: %v", err)
	}

	// Verify pack file exists and is valid JSON with pack wrapper.
	raw, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatalf("Read pack file failed: %v", err)
	}
	var pack domain.VaultPackExport
	if err := json.Unmarshal(raw, &pack); err != nil {
		t.Fatalf("Parse pack JSON failed: %v", err)
	}
	if pack.Pack.PackID == "" {
		t.Error("pack.pack_id is empty")
	}
	if pack.Pack.Author != "tester" {
		t.Errorf("pack.author = %q, want 'tester'", pack.Pack.Author)
	}
	if pack.Pack.Version != "1.0" {
		t.Errorf("pack.version = %q, want '1.0'", pack.Pack.Version)
	}

	// Round-trip: import into a fresh DB with prefix.
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
	if err := importSvc2.Import(ctx, packPath, "ns/"); err != nil {
		t.Fatalf("Import with prefix failed: %v", err)
	}

	// Verify imported entry has prefixed ID.
	prefixedID := "ns/" + result.Entry.Entry.ID
	entrySvc2 := NewEntryService(store2.Entries, store2.Projects, store2.Artifacts)
	got, err := entrySvc2.GetEntry(ctx, prefixedID)
	if err != nil {
		t.Fatalf("GetEntry after prefixed import failed: %v (looking for %q)", err, prefixedID)
	}
	if got.Entry.Entry.ID != prefixedID {
		t.Errorf("imported entry ID = %q, want %q", got.Entry.Entry.ID, prefixedID)
	}
	if got.Entry.Entry.Title != "Pack Entry" {
		t.Errorf("imported entry title = %q, want 'Pack Entry'", got.Entry.Entry.Title)
	}

	// Verify all imported entities have the prefix in their IDs.
	entries, err := entrySvc2.List(ctx, domain.EntryFilter{})
	if err != nil {
		t.Fatalf("List entries after import failed: %v", err)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Entry.ID, "ns/") {
			t.Errorf("imported entry %q should have 'ns/' prefix", e.Entry.ID)
		}
	}
}

func TestEmptyPrefixImportsAsIs(t *testing.T) {
	packSvc, _, entrySvc, projectSvc, _, cleanup := setupPackExportServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "No Prefix Entry",
		Type:    "skill",
		Summary: "No prefix",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	tmpDir := t.TempDir()
	packPath := filepath.Join(tmpDir, "noprefix.svpack")
	if err := packSvc.ExportPack(ctx, PackExportInput{
		Pack:       "No Prefix Pack",
		Author:     "tester",
		Version:    "1.0",
		OutputPath: packPath,
	}); err != nil {
		t.Fatalf("ExportPack failed: %v", err)
	}

	// Import into fresh DB with empty prefix.
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
	if err := importSvc2.Import(ctx, packPath, ""); err != nil {
		t.Fatalf("Import with empty prefix failed: %v", err)
	}

	entrySvc2 := NewEntryService(store2.Entries, store2.Projects, store2.Artifacts)
	got, err := entrySvc2.GetEntry(ctx, result.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry after empty-prefix import failed: %v", err)
	}
	if got.Entry.Entry.ID != result.Entry.Entry.ID {
		t.Errorf("imported entry ID = %q, want %q", got.Entry.Entry.ID, result.Entry.Entry.ID)
	}
}

func TestBareExportStillWorks(t *testing.T) {
	packSvc, _, entrySvc, projectSvc, store, cleanup := setupPackExportServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Bare Export Entry",
		Type:    "skill",
		Summary: "Bare export test",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	// Bare export (existing path, backward compat).
	exportSvc := NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	tmpDir := t.TempDir()
	barePath := filepath.Join(tmpDir, "bare-export.json")
	if err := exportSvc.ExportVault(ctx, ExportVaultInput{OutputPath: barePath, IncludeArtifacts: true}); err != nil {
		t.Fatalf("Bare ExportVault failed: %v", err)
	}

	// Verify bare export has no "pack" key.
	raw, err := os.ReadFile(barePath)
	if err != nil {
		t.Fatalf("Read bare export failed: %v", err)
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("Parse bare JSON failed: %v", err)
	}
	if _, hasPack := rawMap["pack"]; hasPack {
		t.Error("bare export should NOT have a 'pack' key")
	}
	if _, hasSchema := rawMap["schema_version"]; !hasSchema {
		t.Error("bare export should have 'schema_version'")
	}

	// Import bare export into fresh DB.
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
	if err := importSvc2.Import(ctx, barePath, ""); err != nil {
		t.Fatalf("Bare import failed: %v", err)
	}

	entrySvc2 := NewEntryService(store2.Entries, store2.Projects, store2.Artifacts)
	got, err := entrySvc2.GetEntry(ctx, result.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry after bare import failed: %v", err)
	}
	if got.Entry.Entry.Title != "Bare Export Entry" {
		t.Errorf("imported entry title = %q, want 'Bare Export Entry'", got.Entry.Entry.Title)
	}
	_ = packSvc // suppress unused warning
}

func TestPackDetectionOnImport(t *testing.T) {
	// Create a pack JSON and a bare JSON manually to verify detection.
	ctx := context.Background()

	pack := domain.VaultPackExport{
		Pack: domain.PackMetadata{
			PackID:     "test-pack-id",
			Author:     "test",
			Version:    "1.0",
			ExportedAt: "2026-06-28T00:00:00Z",
			Source:     "test",
		},
		Data: domain.VaultExport{
			SchemaVersion: 2,
			AppVersion:    "v3",
			ExportedAt:    "2026-06-28T00:00:00Z",
			Source:        "test",
		},
	}

	tmpDir := t.TempDir()

	// Write pack JSON.
	packBytes, _ := json.MarshalIndent(pack, "", "  ")
	packPath := filepath.Join(tmpDir, "detect-pack.svpack")
	os.WriteFile(packPath, packBytes, 0644)

	// Write bare JSON (no pack key).
	bare := domain.VaultExport{
		SchemaVersion: 2,
		AppVersion:    "v3",
		ExportedAt:    "2026-06-28T00:00:00Z",
		Source:        "test",
	}
	bareBytes, _ := json.MarshalIndent(bare, "", "  ")
	barePath := filepath.Join(tmpDir, "detect-bare.json")
	os.WriteFile(barePath, bareBytes, 0644)

	// Verify pack detection via Import.
	for _, tc := range []struct {
		name   string
		path   string
		prefix string
	}{
		{"pack detected", packPath, ""},
		{"bare still works", barePath, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sqlDB, err := db.OpenDB(":memory:")
			if err != nil {
				t.Fatalf("OpenDB failed: %v", err)
			}
			defer sqlDB.Close()
			if err := db.RunMigrations(sqlDB); err != nil {
				t.Fatalf("RunMigrations failed: %v", err)
			}
			store := db.NewStore(sqlDB)
			importSvc := NewVaultImportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts)
			if err := importSvc.Import(ctx, tc.path, tc.prefix); err != nil {
				t.Fatalf("Import failed: %v", err)
			}
		})
	}
}

func TestPackImportWithPrefixMultipleEntities(t *testing.T) {
	packSvc, _, entrySvc, projectSvc, _, cleanup := setupPackExportServices(t)
	defer cleanup()
	ctx := context.Background()

	// Seed multiple entries.
	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "multi-proj"})
	r1, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Entry One", Type: "skill", Summary: "First",
		Project: "multi-proj", Tags: []string{"multi"},
	})
	if err != nil {
		t.Fatalf("SaveEntry 1 failed: %v", err)
	}
	r2, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Entry Two", Type: "reference", Summary: "Second",
		Project: "multi-proj", Tags: []string{"multi"},
	})
	if err != nil {
		t.Fatalf("SaveEntry 2 failed: %v", err)
	}

	// Export pack.
	tmpDir := t.TempDir()
	packPath := filepath.Join(tmpDir, "multi-pack.svpack")
	if err := packSvc.ExportPack(ctx, PackExportInput{
		Pack: "Multi Pack", Author: "tester", Version: "1.0",
		OutputPath: packPath,
	}); err != nil {
		t.Fatalf("ExportPack failed: %v", err)
	}

	// Import with prefix.
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
	if err := importSvc2.Import(ctx, packPath, "p/"); err != nil {
		t.Fatalf("Import with prefix failed: %v", err)
	}

	entrySvc2 := NewEntryService(store2.Entries, store2.Projects, store2.Artifacts)

	// Both entries should have prefixed IDs.
	entries, err := entrySvc2.List(ctx, domain.EntryFilter{})
	if err != nil {
		t.Fatalf("List entries after import failed: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Entry.ID, "p/") {
			t.Errorf("imported entry %q should start with 'p/'", e.Entry.ID)
		}
	}
	_ = r1
	_ = r2
}
