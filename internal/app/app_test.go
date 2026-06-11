package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupAppServices(t *testing.T) (*db.Store, *EntryService, *SeriesService, *WorkflowService, *VaultExportService, *VaultImportService, *ContextService, func()) {
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

	entrySvc := NewEntryService(store.Entries)
	seriesSvc := NewSeriesService(store.Series, store.Entries)
	workflowSvc := NewWorkflowService(store.Entries, store.Workflows)
	exportSvc := NewVaultExportService(store.ImportExport)
	importSvc := NewVaultImportService(store.ImportExport)
	contextSvc := NewContextService(store.Entries, store.Projects, store.Series)

	cleanup := func() { sqlDB.Close() }
	return store, entrySvc, seriesSvc, workflowSvc, exportSvc, importSvc, contextSvc, cleanup
}

func TestEntryServiceUpsertNormalizesTags(t *testing.T) {
	_, svc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "test-entry", Name: "Test", Type: domain.EntryTypeSkill, Content: "Test", Active: true}
	err := svc.UpsertEntry(ctx, entry, []string{" Go ", "CLI", "", "cli-tool"}, nil)
	if err != nil {
		t.Fatalf("UpsertEntry failed: %v", err)
	}

	result, err := svc.GetEntry(ctx, "test-entry", false)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if len(result.Tags) != 3 {
		t.Errorf("expected 3 tags (go, cli, cli-tool), got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestEntryServiceRejectsInvalidType(t *testing.T) {
	_, svc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "bad", Name: "Bad", Type: domain.EntryType("invalid"), Content: "bad", Active: true}
	err := svc.UpsertEntry(ctx, entry, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid entry type")
	}
}

func TestEntryServiceArchive(t *testing.T) {
	_, svc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "e1", Name: "E1", Type: domain.EntryTypeNote, Content: "C1", Active: true}
	svc.UpsertEntry(ctx, entry, nil, nil)
	svc.ArchiveEntry(ctx, "e1")

	_, err := svc.GetEntry(ctx, "e1", false)
	if err == nil {
		t.Fatal("expected archived error")
	}

	result, err := svc.GetEntry(ctx, "e1", true)
	if err != nil {
		t.Fatalf("GetEntry with include_archived failed: %v", err)
	}
	if result.Entry.ID != "e1" {
		t.Errorf("ID = %q, want 'e1'", result.Entry.ID)
	}
}

func TestSeriesServiceReplaceRejectsCrossProject(t *testing.T) {
	store, _, svc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	// Create projects
	store.Projects.UpsertProject(ctx, domain.Project{ID: "proj-a", Name: "A", Active: true})
	store.Projects.UpsertProject(ctx, domain.Project{ID: "proj-b", Name: "B", Active: true})

	// Create entries in different projects
	projA := "proj-a"
	projB := "proj-b"
	store.Entries.UpsertEntry(ctx, domain.Entry{ID: "ea", Name: "EA", Type: domain.EntryTypeSkill, Content: "CA", ProjectID: &projA, Active: true}, nil, nil)
	store.Entries.UpsertEntry(ctx, domain.Entry{ID: "eb", Name: "EB", Type: domain.EntryTypeSkill, Content: "CB", ProjectID: &projB, Active: true}, nil, nil)

	// Create series in proj-a
	store.Series.UpsertSeries(ctx, domain.Series{ID: "s1", Name: "S1", ProjectID: &projA, Active: true})

	// Try to add entry from proj-b to series in proj-a — should fail
	err := svc.ReplaceSeriesEntries(ctx, "s1", []domain.SeriesEntryInput{
		{EntryID: "eb"},
	})
	if err == nil {
		t.Fatal("expected scope validation error")
	}
}

func TestSeriesServiceReplaceAcceptsGlobalEntry(t *testing.T) {
	store, _, svc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projA := "proj-a"
	store.Projects.UpsertProject(ctx, domain.Project{ID: projA, Name: "A", Active: true})
	store.Entries.UpsertEntry(ctx, domain.Entry{ID: "eg", Name: "EG", Type: domain.EntryTypeSkill, Content: "CG", Active: true}, nil, nil)
	store.Series.UpsertSeries(ctx, domain.Series{ID: "s1", Name: "S1", ProjectID: &projA, Active: true})

	err := svc.ReplaceSeriesEntries(ctx, "s1", []domain.SeriesEntryInput{
		{EntryID: "eg"},
	})
	if err != nil {
		t.Fatalf("global entry should be accepted in project series: %v", err)
	}
}

func TestWorkflowServiceRun(t *testing.T) {
	store, _, _, svc, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	// Create workflow entry with steps
	entry := domain.Entry{ID: "wf1", Name: "WF1", Type: domain.EntryTypeWorkflow, Content: "test", Active: true}
	store.Entries.UpsertEntry(ctx, entry, nil, nil)
	store.Workflows.UpsertWorkflowSteps(ctx, "wf1", []domain.WorkflowStep{
		{StepNum: 1, Role: domain.WorkflowRoleSystem, Content: "You are {{role}}"},
		{StepNum: 2, Role: domain.WorkflowRoleUser, Content: "Today is {{date}}"},
	})

	result, err := svc.RunWorkflow(ctx, "wf1", map[string]string{"role": "tester"})
	if err != nil {
		t.Fatalf("RunWorkflow failed: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Role != domain.WorkflowRoleSystem {
		t.Errorf("step 0 role = %q, want 'system'", result.Steps[0].Role)
	}
	if result.Steps[0].Content != "You are tester" {
		t.Errorf("step 0 content = %q, want 'You are tester'", result.Steps[0].Content)
	}
	// Check date was injected
	if result.Steps[1].Content == "Today is {{date}}" {
		t.Error("date was not resolved")
	}
}

func TestWorkflowServiceRejectsNonWorkflow(t *testing.T) {
	store, _, _, svc, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	store.Entries.UpsertEntry(ctx, domain.Entry{ID: "sk1", Name: "Skill", Type: domain.EntryTypeSkill, Content: "skill", Active: true}, nil, nil)

	_, err := svc.RunWorkflow(ctx, "sk1", nil)
	if err == nil {
		t.Fatal("expected error for non-workflow entry")
	}
}

func TestExportImportRoundTripApp(t *testing.T) {
	_, entrySvc, _, _, exportSvc, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "e1", Name: "E1", Type: domain.EntryTypeSkill, Content: "C1", Active: true}
	entrySvc.UpsertEntry(ctx, entry, []string{"tag1"}, nil)

	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "export.json")

	if err := exportSvc.Export(ctx, exportPath); err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatal("export file not created")
	}

	// Import into fresh DB
	sqlDB2, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB2 failed: %v", err)
	}
	defer sqlDB2.Close()
	if err := db.RunMigrations(sqlDB2); err != nil {
		t.Fatalf("RunMigrations2 failed: %v", err)
	}
	store2 := db.NewStore(sqlDB2)
	importSvc2 := NewVaultImportService(store2.ImportExport)
	if err := importSvc2.Import(ctx, exportPath); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	result, err := NewEntryService(store2.Entries).GetEntry(ctx, "e1", false)
	if err != nil {
		t.Fatalf("GetEntry after import failed: %v", err)
	}
	if result.Entry.ID != "e1" {
		t.Errorf("ID = %q, want 'e1'", result.Entry.ID)
	}
}

func TestContextService(t *testing.T) {
	store, _, _, _, _, _, svc, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projID := "vitacare"
	store.Projects.UpsertProject(ctx, domain.Project{ID: projID, Name: "VitaCare", Active: true})
	store.Entries.UpsertEntry(ctx, domain.Entry{ID: "e1", Name: "E1", Type: domain.EntryTypeSkill, Content: "C1", ProjectID: &projID, Active: true}, nil, nil)
	store.Series.UpsertSeries(ctx, domain.Series{ID: "s1", Name: "S1", ProjectID: &projID, Active: true})

	result, err := svc.GetContext(ctx, projID)
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if result.Project.ID != projID {
		t.Errorf("Project.ID = %q, want %q", result.Project.ID, projID)
	}
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result.Entries))
	}
	if len(result.Series) != 1 {
		t.Errorf("expected 1 series, got %d", len(result.Series))
	}
}

func TestContextServiceMissingProject(t *testing.T) {
	_, _, _, _, _, _, svc, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.GetContext(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}
