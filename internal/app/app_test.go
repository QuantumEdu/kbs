package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupAppServices(t *testing.T) (*db.Store, *EntryService, *ArtifactService, *WorkflowService, *SeriesService, *ProjectService, *ContextService, *SessionService, *VaultExportService, *VaultImportService, func()) {
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
	artifactSvc := NewArtifactService(store.Artifacts, store.Entries, store.Projects)
	workflowSvc := NewWorkflowService(store.Workflows)
	seriesSvc := NewSeriesService(store.Series, store.Entries)
	projectSvc := NewProjectService(store.Projects)
	contextSvc := NewContextService(store.Entries, store.Projects, store.Series, store.Workflows, store.Artifacts, entrySvc)
	sessionSvc := NewSessionService(entrySvc, artifactSvc, projectSvc, store.Entries, store.Artifacts, store.Projects)
	exportSvc := NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	importSvc := NewVaultImportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts)

	cleanup := func() { sqlDB.Close() }
	return store, entrySvc, artifactSvc, workflowSvc, seriesSvc, projectSvc, contextSvc, sessionSvc, exportSvc, importSvc, cleanup
}

func TestEntryServiceRoutingTypeRoundTrip(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Workflow Route Map",
		Summary: "Maps scenarios to workflow slugs",
		Type:    string(domain.EntryTypeRouting),
		Body:    "scenario: new-project -> workflow: onboarding",
		Tags:    []string{"routing", "workflow-map"},
	})
	if err != nil {
		t.Fatalf("SaveEntry routing failed: %v", err)
	}
	if result.Entry.Entry.Type != domain.EntryTypeRouting {
		t.Errorf("expected type %q, got %q", domain.EntryTypeRouting, result.Entry.Entry.Type)
	}

	got, err := svc.GetEntry(ctx, result.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry routing failed: %v", err)
	}
	if got.Entry.Entry.Type != domain.EntryTypeRouting {
		t.Errorf("retrieved type mismatch: expected %q, got %q", domain.EntryTypeRouting, got.Entry.Entry.Type)
	}
	if got.Entry.Entry.Title != "Workflow Route Map" {
		t.Errorf("retrieved title mismatch: expected %q, got %q", "Workflow Route Map", got.Entry.Entry.Title)
	}
}

func TestEntryServiceUpsertNormalizesTags(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "test-entry", Title: "Test", Slug: "test", Type: domain.EntryTypeSkill, BodyOptional: "Test", Status: domain.StatusActive}
	err := svc.Save(ctx, entry, []string{" Go ", "CLI", "", "cli-tool"})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := svc.Get(ctx, "test-entry", false)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(result.Tags) != 3 {
		t.Errorf("expected 3 tags (go, cli, cli-tool), got %d: %v", len(result.Tags), result.Tags)
	}
}

func TestEntryServiceRejectsInvalidType(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "bad", Title: "Bad", Slug: "bad", Type: domain.EntryType("invalid"), BodyOptional: "bad", Status: domain.StatusActive}
	err := svc.Save(ctx, entry, nil)
	if err == nil {
		t.Fatal("expected error for invalid entry type")
	}
}

func TestEntryServiceArchive(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	entry := domain.Entry{ID: "e1", Title: "E1", Slug: "e1", Type: domain.EntryTypeSession, BodyOptional: "C1", Status: domain.StatusActive}
	svc.Save(ctx, entry, nil)
	svc.Archive(ctx, "e1")

	_, err := svc.Get(ctx, "e1", false)
	if err == nil {
		t.Fatal("expected archived error")
	}

	result, err := svc.Get(ctx, "e1", true)
	if err != nil {
		t.Fatalf("Get with include_archived failed: %v", err)
	}
	if result.Entry.ID != "e1" {
		t.Errorf("ID = %q, want 'e1'", result.Entry.ID)
	}
}

func TestSaveEntryValidatesProject(t *testing.T) {
	_, svc, _, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj", Description: "Test"})

	result, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Test Entry",
		Type:    "skill",
		Summary: "A test skill",
		Body:    "body content",
		Project: "testproj",
		Tags:    []string{"go", "test"},
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}
	if result.Entry.Entry.Title != "Test Entry" {
		t.Errorf("Title = %q, want 'Test Entry'", result.Entry.Entry.Title)
	}
	if result.Entry.Entry.ProjectID == nil || *result.Entry.Entry.ProjectID == "" {
		t.Fatal("expected non-empty ProjectID")
	}
	if len(result.Entry.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(result.Entry.Tags))
	}
}

func TestSaveEntryRejectsMissingProject(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Test",
		Type:    "skill",
		Summary: "test",
		Project: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestSaveEntryRejectsInvalidType(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Test",
		Type:    "invalid_type",
		Summary: "test",
	})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestSaveEntryRejectsInvalidStatus(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title:  "Test",
		Type:   "skill",
		Status: "invalid_status",
	})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestGetEntryWithArtifactRef(t *testing.T) {
	_, svc, artSvc, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	entryResult, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Artifact Entry",
		Type:    "skill",
		Summary: "Has artifact",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	artifact, err := artSvc.SaveArtifact(ctx, SaveArtifactInput{
		Title:   "Linked Artifact",
		Type:    "markdown",
		Content: "# Artifact Content",
		Summary: "An artifact",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveArtifact failed: %v", err)
	}

	if err := artSvc.LinkArtifactToEntry(ctx, artifact.ID, entryResult.Entry.Entry.ID); err != nil {
		t.Fatalf("LinkArtifactToEntry failed: %v", err)
	}

	result, err := svc.GetEntry(ctx, entryResult.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Entry.Entry.Title != "Artifact Entry" {
		t.Errorf("Title = %q, want 'Artifact Entry'", result.Entry.Entry.Title)
	}
	if result.Artifact == nil {
		t.Fatal("expected artifact to be linked")
	}
	if result.Artifact.Title != "Linked Artifact" {
		t.Errorf("Artifact Title = %q, want 'Linked Artifact'", result.Artifact.Title)
	}
}

func TestSearchEntriesViaFTS5(t *testing.T) {
	_, svc, _, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Go REST API",
		Type:    "skill",
		Summary: "Building REST APIs in Go",
		Body:    "Use net/http or chi router",
		Project: "testproj",
		Tags:    []string{"go", "rest"},
		Status:  "active",
	})
	svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Python Data Science",
		Type:    "skill",
		Summary: "Data science with Python",
		Body:    "Use pandas and numpy",
		Project: "testproj",
		Tags:    []string{"python", "data"},
		Status:  "active",
	})

	results, err := svc.SearchEntries(ctx, "REST", domain.SearchQuery{Limit: 10, IncludeArchived: true})
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 search result for 'REST'")
	}

	found := false
	for _, r := range results {
		if r.Entry.Title == "Go REST API" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("search 'REST' should find 'Go REST API', got: %v", results)
	}
}

func TestSearchEntriesFiltersArchivedByDefault(t *testing.T) {
	_, svc, _, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Active Entry",
		Type:    "skill",
		Summary: "Active",
		Project: "testproj",
		Status:  "active",
	})
	svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Archived Entry",
		Type:    "skill",
		Summary: "Archived",
		Project: "testproj",
		Status:  "archived",
	})

	results, err := svc.SearchEntries(ctx, "Entry", domain.SearchQuery{Limit: 10, IncludeArchived: false})
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}

	for _, r := range results {
		if r.Entry.Title == "Archived Entry" {
			t.Error("archived entry should not appear with IncludeArchived=false")
		}
	}
}

func TestArchiveEntry(t *testing.T) {
	_, svc, _, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	result, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "To Archive",
		Type:    "skill",
		Summary: "Will be archived",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	if err := svc.ArchiveEntry(ctx, result.Entry.Entry.ID); err != nil {
		t.Fatalf("ArchiveEntry failed: %v", err)
	}

	_, err = svc.GetEntry(ctx, result.Entry.Entry.ID)
	if err == nil {
		t.Fatal("expected error for archived entry (exclude_archived by default)")
	}
}

func TestExportImportRoundTripApp(t *testing.T) {
	_, entrySvc, _, _, _, projectSvc, _, _, exportSvc, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Export Test",
		Type:    "skill",
		Summary: "Testing export",
		Project: "testproj",
		Tags:    []string{"export"},
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "export.json")

	if err := exportSvc.ExportVault(ctx, ExportVaultInput{OutputPath: exportPath, IncludeArtifacts: true}); err != nil {
		t.Fatalf("ExportVault failed: %v", err)
	}
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatal("export file not created")
	}

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
	if err := importSvc2.Import(ctx, exportPath); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	got, err := NewEntryService(store2.Entries, store2.Projects, store2.Artifacts).GetEntry(ctx, result.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry after import failed: %v", err)
	}
	if got.Entry.Entry.Slug != result.Entry.Entry.Slug {
		t.Errorf("Slug = %q, want %q", got.Entry.Entry.Slug, result.Entry.Entry.Slug)
	}
}

func TestSaveArtifact(t *testing.T) {
	_, _, svc, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	artifact, err := svc.SaveArtifact(ctx, SaveArtifactInput{
		Title:   "Test Artifact",
		Type:    "markdown",
		Content: "# Hello World",
		Summary: "A test markdown artifact",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveArtifact failed: %v", err)
	}
	if artifact.Title != "Test Artifact" {
		t.Errorf("Title = %q, want 'Test Artifact'", artifact.Title)
	}
	if artifact.Type != domain.ArtifactTypeMarkdown {
		t.Errorf("Type = %q, want 'markdown'", artifact.Type)
	}
	if artifact.MimeType != "text/markdown" {
		t.Errorf("MimeType = %q, want 'text/markdown'", artifact.MimeType)
	}
}

func TestSaveArtifactValidation(t *testing.T) {
	_, _, svc, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	tests := []struct {
		name    string
		input   SaveArtifactInput
		wantErr string
	}{
		{
			name:    "empty title",
			input:   SaveArtifactInput{Title: "", Type: "markdown", Content: "x"},
			wantErr: "title is required",
		},
		{
			name:    "invalid type",
			input:   SaveArtifactInput{Title: "T", Type: "invalid", Content: "x"},
			wantErr: "invalid artifact type",
		},
		{
			name:    "no content or filepath",
			input:   SaveArtifactInput{Title: "T", Type: "markdown"},
			wantErr: "either content or file_path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.SaveArtifact(ctx, tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetArtifact(t *testing.T) {
	_, _, svc, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	saved, err := svc.SaveArtifact(ctx, SaveArtifactInput{
		Title:   "My Artifact",
		Type:    "markdown",
		Content: "content",
		Summary: "summary",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveArtifact failed: %v", err)
	}

	got, err := svc.GetArtifact(ctx, saved.ID)
	if err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}
	if got.Title != "My Artifact" {
		t.Errorf("Title = %q, want 'My Artifact'", got.Title)
	}
}

func TestLinkArtifactToEntry(t *testing.T) {
	_, entrySvc, artSvc, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	entry, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Entry for Linking",
		Type:    "skill",
		Summary: "Will link to artifact",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	artifact, err := artSvc.SaveArtifact(ctx, SaveArtifactInput{
		Title:   "Linkable Artifact",
		Type:    "json",
		Content: `{"key": "value"}`,
		Summary: "JSON artifact",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveArtifact failed: %v", err)
	}

	if err := artSvc.LinkArtifactToEntry(ctx, artifact.ID, entry.Entry.Entry.ID); err != nil {
		t.Fatalf("LinkArtifactToEntry failed: %v", err)
	}

	result, err := entrySvc.GetEntry(ctx, entry.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Artifact == nil {
		t.Fatal("expected linked artifact")
	}
	if result.Artifact.ID != artifact.ID {
		t.Errorf("Artifact ID = %q, want %q", result.Artifact.ID, artifact.ID)
	}
}

func TestSaveWorkflow(t *testing.T) {
	_, _, _, svc, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	wf, err := svc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:        "Spec Plan Task",
		Description: "Standard workflow for implementation",
		Steps: []SaveWorkflowStep{
			{OrderIndex: 1, Title: "Read Spec", Instruction: "Read the source spec document", Required: true},
			{OrderIndex: 2, Title: "Plan", Instruction: "Generate implementation plan", Required: true},
			{OrderIndex: 3, Title: "Implement", Instruction: "Write code", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	if wf.Name != "Spec Plan Task" {
		t.Errorf("Name = %q, want 'Spec Plan Task'", wf.Name)
	}
	if string(wf.Status) != "active" {
		t.Errorf("Status = %q, want 'active'", wf.Status)
	}
}

func TestRenderWorkflow(t *testing.T) {
	_, _, _, svc, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	svc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name: "Test WF",
		Steps: []SaveWorkflowStep{
			{OrderIndex: 1, Title: "Step One", Instruction: "Do first thing"},
			{OrderIndex: 2, Title: "Step Two", Instruction: "Do second thing"},
		},
	})

	wfs, err := svc.ListWorkflows(ctx, false)
	if err != nil {
		t.Fatalf("ListWorkflows failed: %v", err)
	}
	if len(wfs) == 0 {
		t.Fatal("expected at least 1 workflow")
	}

	steps, err := svc.RenderWorkflow(ctx, wfs[0].ID)
	if err != nil {
		t.Fatalf("RenderWorkflow failed: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Title != "Step One" {
		t.Errorf("Step0 Title = %q, want 'Step One'", steps[0].Title)
	}
	if steps[1].Title != "Step Two" {
		t.Errorf("Step1 Title = %q, want 'Step Two'", steps[1].Title)
	}
}

func TestSaveSeries(t *testing.T) {
	_, entrySvc, _, _, svc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	e1, _ := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Entry 1", Type: "skill", Summary: "First",
	})
	e2, _ := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Entry 2", Type: "skill", Summary: "Second",
	})

	ser, err := svc.SaveSeries(ctx, SaveSeriesInput{
		Name:        "Learning Path",
		Description: "A sequence of skills",
		EntryIDs:    []string{e1.Entry.Entry.ID, e2.Entry.Entry.ID},
	})
	if err != nil {
		t.Fatalf("SaveSeries failed: %v", err)
	}
	if ser.Name != "Learning Path" {
		t.Errorf("Name = %q, want 'Learning Path'", ser.Name)
	}
}

func TestComposeSeries(t *testing.T) {
	_, entrySvc, _, _, svc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	e1, _ := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "First", Type: "skill", Summary: "First entry",
	})
	e2, _ := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Second", Type: "skill", Summary: "Second entry",
	})

	ser, _ := svc.SaveSeries(ctx, SaveSeriesInput{
		Name:     "Ordered Series",
		EntryIDs: []string{e1.Entry.Entry.ID, e2.Entry.Entry.ID},
	})

	entries, err := svc.ComposeSeries(ctx, ser.ID)
	if err != nil {
		t.Fatalf("ComposeSeries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "First" {
		t.Errorf("Entry0 Title = %q, want 'First'", entries[0].Title)
	}
	if entries[1].Title != "Second" {
		t.Errorf("Entry1 Title = %q, want 'Second'", entries[1].Title)
	}
}

func TestSaveProject(t *testing.T) {
	_, _, _, _, _, svc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := svc.SaveProject(ctx, SaveProjectInput{
		Name:        "My Project",
		Description: "A test project",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
	if proj.Name != "My Project" {
		t.Errorf("Name = %q, want 'My Project'", proj.Name)
	}
	if string(proj.Status) != "active" {
		t.Errorf("Status = %q, want 'active'", proj.Status)
	}
}

func TestListProjects(t *testing.T) {
	_, _, _, _, _, svc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	svc.SaveProject(ctx, SaveProjectInput{Name: "Alpha", Description: "First"})
	svc.SaveProject(ctx, SaveProjectInput{Name: "Beta", Description: "Second"})

	projects, err := svc.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestArchiveProject(t *testing.T) {
	_, _, _, _, _, svc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, _ := svc.SaveProject(ctx, SaveProjectInput{Name: "To Archive"})

	if err := svc.ArchiveProject(ctx, proj.ID); err != nil {
		t.Fatalf("ArchiveProject failed: %v", err)
	}

	projects, err := svc.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	for _, p := range projects {
		if p.ID == proj.ID {
			t.Error("archived project should not appear in ListProjects")
		}
	}
}

func TestSessionWrapCreatesEntry(t *testing.T) {
	_, entrySvc, _, _, _, projectSvc, _, svc, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	output, err := svc.SessionWrap(ctx, SessionWrapInput{
		Project:   "testproj",
		Summary:   "Fixed the auth middleware bug",
		Decisions: []string{"Use JWT with refresh tokens"},
		Pending:   []string{"Add rate limiting"},
		Learnings: []string{"JWT expiry must be short"},
	})
	if err != nil {
		t.Fatalf("SessionWrap failed: %v", err)
	}
	if output.Entry == nil {
		t.Fatal("expected entry in output")
	}
	if output.Entry.Entry.Entry.Type != domain.EntryTypeSession {
		t.Errorf("Type = %q, want 'session'", output.Entry.Entry.Entry.Type)
	}
	if !strings.Contains(output.Entry.Entry.Entry.Summary, "auth") {
		t.Errorf("Summary should contain 'auth', got %q", output.Entry.Entry.Entry.Summary)
	}

	result, err := entrySvc.GetEntry(ctx, output.Entry.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if result.Entry.Entry.Type != domain.EntryTypeSession {
		t.Errorf("stored Type = %q, want 'session'", result.Entry.Entry.Type)
	}
}

func TestSessionWrapWithArtifact(t *testing.T) {
	_, _, artSvc, _, _, projectSvc, _, svc, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	a1, _ := artSvc.SaveArtifact(ctx, SaveArtifactInput{
		Title: "Session Report", Type: "markdown", Content: "# Report", Summary: "Report", Project: "testproj",
	})

	output, err := svc.SessionWrap(ctx, SessionWrapInput{
		Project:   "testproj",
		Summary:   "Session with artifact",
		Decisions: []string{"Decision A"},
		Artifacts: []string{a1.ID},
	})
	if err != nil {
		t.Fatalf("SessionWrap failed: %v", err)
	}
	if output.Artifact == nil {
		t.Fatal("expected artifact in output")
	}
	if output.Artifact.Type != domain.ArtifactTypeSessionOutput {
		t.Errorf("Artifact Type = %q, want 'session_output'", output.Artifact.Type)
	}
}

func TestGetContextModeProject(t *testing.T) {
	store, entrySvc, _, _, _, projectSvc, svc, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, _ := projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj", Description: "Test project"})

	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Use Go chi", Type: "decision", Summary: "Use chi router", Project: proj.ID,
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Feedback note", Type: "feedback", Summary: "Prefer simple architecture",
	})

	store.Projects.Save(ctx, domain.Project{ID: "otherproj", Name: "otherproj", Slug: "otherproj", Status: domain.StatusActive})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Other decision", Type: "decision", Summary: "Not in scope", Project: "otherproj",
	})

	pack, err := svc.GetContext(ctx, ContextInput{
		Mode:            "planning",
		Project:         proj.ID,
		Include:         []string{"decisions"},
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if !strings.Contains(pack.Raw, "CONTEXT PACK") {
		t.Error("expected CONTEXT PACK header")
	}
	if !strings.Contains(pack.Raw, "Use Go chi") {
		t.Error("expected decision 'Use Go chi' in context pack")
	}
	if strings.Contains(pack.Raw, "Other decision") {
		t.Error("should not include decisions from other projects")
	}
}

func TestGetContextMaxCharsLimit(t *testing.T) {
	_, entrySvc, _, _, _, projectSvc, svc, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, _ := projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	longBody := strings.Repeat("A", 500)
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Long Decision", Type: "decision", Body: longBody, Summary: "Long", Project: proj.ID,
	})

	pack, err := svc.GetContext(ctx, ContextInput{
		Mode:            "planning",
		Project:         proj.ID,
		Include:         []string{"decisions"},
		ExcludeArchived: true,
		MaxChars:        200,
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len(pack.Raw) > 250 {
		t.Errorf("expected truncated output (max 200 chars + overhead), got %d", len(pack.Raw))
	}
}

func TestGetContextExcludesArchivedByDefault(t *testing.T) {
	_, entrySvc, _, _, _, projectSvc, svc, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, _ := projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Active Decision", Type: "decision", Summary: "Active", Project: proj.ID, Status: "active",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Archived Decision", Type: "decision", Summary: "Archived", Project: proj.ID, Status: "archived",
	})

	pack, err := svc.GetContext(ctx, ContextInput{
		Mode:            "planning",
		Project:         proj.ID,
		Include:         []string{"decisions"},
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if strings.Contains(pack.Raw, "Archived") {
		t.Error("archived entries should be excluded by default")
	}
	if !strings.Contains(pack.Raw, "Active") {
		t.Error("active entries should be included")
	}
}

func TestWorkflowServiceNewAPI(t *testing.T) {
	_, _, _, svc, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	wf, err := svc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:        "Review Flow",
		Description: "Code review steps",
		Status:      "active",
		Steps: []SaveWorkflowStep{
			{Title: "Check style", Instruction: "Run linter", Required: true},
			{Title: "Run tests", Instruction: "go test ./...", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	steps, err := svc.RenderWorkflow(ctx, wf.ID)
	if err != nil {
		t.Fatalf("RenderWorkflow failed: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	list, err := svc.ListWorkflows(ctx, false)
	if err != nil {
		t.Fatalf("ListWorkflows failed: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least 1 workflow")
	}
}

func TestSeriesList(t *testing.T) {
	_, entrySvc, _, _, svc, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	e1, _ := entrySvc.SaveEntry(ctx, SaveEntryInput{Title: "E1", Type: "skill", Summary: "S1"})
	e2, _ := entrySvc.SaveEntry(ctx, SaveEntryInput{Title: "E2", Type: "skill", Summary: "S2"})

	svc.SaveSeries(ctx, SaveSeriesInput{
		Name:     "Series A",
		EntryIDs: []string{e1.Entry.Entry.ID, e2.Entry.Entry.ID},
	})
	svc.SaveSeries(ctx, SaveSeriesInput{Name: "Series B"})

	list, err := svc.ListSeries(ctx, domain.SeriesFilter{IncludeArchived: false})
	if err != nil {
		t.Fatalf("ListSeries failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 series, got %d", len(list))
	}
}

func TestImportResolvesSlugConflicts(t *testing.T) {
	_, entrySvc, _, _, _, projectSvc, _, _, _, importSvc, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Existing", Type: "skill", Summary: "Existing", Project: "testproj",
	})

	conflictData := domain.VaultExport{
		SchemaVersion: 2,
		Data: domain.VaultData{
			Entries: []domain.Entry{
				{ID: "new-id", Title: "Existing", Slug: "Existing", Type: domain.EntryTypeSkill, Status: domain.StatusActive},
			},
		},
	}

	err := importSvc.ImportVault(ctx, conflictData)
	if err != nil {
		t.Fatalf("Expected conflict to be resolved, got: %v", err)
	}

	result, err := entrySvc.GetEntry(ctx, "new-id")
	if err != nil {
		t.Fatalf("GetEntry after import failed: %v", err)
	}
	if result.Entry.Entry.Slug == "Existing" {
		t.Errorf("expected slug to be different from 'Existing' after conflict resolution, got %q", result.Entry.Entry.Slug)
	}
}

func TestSaveEntryEmptyTitle(t *testing.T) {
	_, svc, _, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.SaveEntry(ctx, SaveEntryInput{
		Title: "",
		Type:  "skill",
	})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestSaveArtifactWithFilePath(t *testing.T) {
	_, _, svc, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	artifact, err := svc.SaveArtifact(ctx, SaveArtifactInput{
		Title:    "File Artifact",
		Type:     "markdown",
		FilePath: "/tmp/artifact.md",
		Summary:  "File-backed",
		Project:  "testproj",
	})
	if err != nil {
		t.Fatalf("SaveArtifact failed: %v", err)
	}
	if artifact.FilePath != "/tmp/artifact.md" {
		t.Errorf("FilePath = %q, want '/tmp/artifact.md'", artifact.FilePath)
	}
}

func TestLinkArtifactToEntryNotFound(t *testing.T) {
	_, _, svc, _, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	err := svc.LinkArtifactToEntry(ctx, "nonexistent-art", "nonexistent-entry")
	if err == nil {
		t.Fatal("expected error for nonexistent artifact")
	}
}
