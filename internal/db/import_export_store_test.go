package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/version"
)

func setupImportExportStore(t *testing.T) (EntryStore, WorkflowStore, SeriesStore, ProjectStore, ArtifactStore, EntryLinkStore, TagStore, WorkflowRunStore, ImportExportStore, func()) {
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
	workflowStore := &sqliteWorkflowStore{db: db}
	seriesStore := &sqliteSeriesStore{db: db}
	projectStore := &sqliteProjectStore{db: db}
	artifactStore := &sqliteArtifactStore{db: db}
	linkStore := &sqliteEntryLinkStore{db: db}
	tagStore := &sqliteTagStore{db: db}
	rstore := &sqliteWorkflowRunStore{db: db}
	ieStore := &sqliteImportExportStore{db: db}
	cleanup := func() { db.Close() }
	return entryStore, workflowStore, seriesStore, projectStore, artifactStore, linkStore, tagStore, rstore, ieStore, cleanup
}

func TestExportRoundTrip(t *testing.T) {
	estore, wfstore, sstore, pstore, astore, linkStore, tagStore, rstore, iestore, cleanup := setupImportExportStore(t)
	defer cleanup()
	ctx := context.Background()

	pstore.Save(ctx, domain.Project{ID: "proj-1", Name: "Test Project", Slug: "test-project", Description: "A test project", Status: domain.StatusActive})
	pstore.Save(ctx, domain.Project{ID: "proj-2", Name: "Archived Project", Slug: "archived-proj", Description: "Archived", Status: domain.StatusArchived})

	estore.Save(ctx, domain.Entry{ID: "e1", Title: "Entry 1", Slug: "entry-1", Type: domain.EntryTypeSkill, BodyOptional: "Content 1", Status: domain.StatusActive}, []string{"tag1"})
	estore.Save(ctx, domain.Entry{ID: "e2", Title: "Entry 2", Slug: "entry-2", Type: domain.EntryTypePrompt, BodyOptional: "Content 2", Status: domain.StatusArchived}, []string{"tag2"})

	astore.Save(ctx, domain.Artifact{ID: "art-1", Title: "Artifact 1", Slug: "artifact-1", Type: domain.ArtifactTypeMarkdown, FilePath: "/tmp/test.md", MimeType: "text/markdown", Summary: "Test artifact", ContentHash: "abc123", SizeBytes: 100})

	wfstore.Save(ctx, domain.Workflow{ID: "wf-1", Name: "Test WF", Slug: "test-wf", Description: "A workflow", Status: domain.StatusActive}, []domain.WorkflowStep{
		{Title: "Step 1", Instruction: "Do step 1", OrderIndex: 1, Required: true},
		{Title: "Step 2", Instruction: "Do step 2", OrderIndex: 2, Required: false},
	})

	rstore.CreateRun(ctx, domain.WorkflowRun{
		ID: "run-1", WorkflowID: "wf-1", Input: "input.md", Output: "result",
		Status: domain.RunStatusCompleted,
	}, []domain.WorkflowRunStep{
		{ID: "rs-1", RunID: "run-1", StepID: 1, EntryID: "e1", Input: "input.md", Output: "step1 done", Status: domain.RunStatusCompleted},
		{ID: "rs-2", RunID: "run-1", StepID: 2, EntryID: "e2", Input: "", Output: "step2 done", Status: domain.RunStatusCompleted},
	})

	sstore.Save(ctx, domain.Series{ID: "ser-1", Name: "Test Series", Slug: "test-series", Status: domain.StatusActive})

	tagStore.Save(ctx, domain.Tag{ID: "tag1", Name: "tag1", Slug: "tag1"})
	tagStore.Save(ctx, domain.Tag{ID: "tag2", Name: "tag2", Slug: "tag2"})

	linkStore.Save(ctx, domain.EntryLink{FromEntryID: "e1", ToEntryID: "e2", RelationType: domain.RelationReferences})

	exported, err := iestore.ExportAll(ctx)
	if err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}

	if exported.SchemaVersion != 3 {
		t.Errorf("SchemaVersion = %d, want 3", exported.SchemaVersion)
	}
	if exported.AppVersion != version.Display() {
		t.Errorf("AppVersion = %q, want %q", exported.AppVersion, version.Display())
	}
	if exported.ExportedAt == "" {
		t.Error("ExportedAt should not be empty")
	}
	if exported.Source != "skillvault" {
		t.Errorf("Source = %q, want 'skillvault'", exported.Source)
	}

	if len(exported.Data.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(exported.Data.Projects))
	}
	if len(exported.Data.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(exported.Data.Entries))
	}
	if len(exported.Data.EntryTags) != 2 {
		t.Errorf("expected 2 entry_tags, got %d", len(exported.Data.EntryTags))
	}
	if len(exported.Data.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(exported.Data.Tags))
	}
	if len(exported.Data.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(exported.Data.Artifacts))
	}
	if len(exported.Data.Workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(exported.Data.Workflows))
	}
	if len(exported.Data.WorkflowSteps) != 2 {
		t.Errorf("expected 2 workflow_steps, got %d", len(exported.Data.WorkflowSteps))
	}
	if len(exported.Data.Series) != 1 {
		t.Errorf("expected 1 series, got %d", len(exported.Data.Series))
	}
	if len(exported.Data.EntryLinks) != 1 {
		t.Errorf("expected 1 entry_link, got %d", len(exported.Data.EntryLinks))
	}
	if len(exported.Data.WorkflowRuns) != 1 {
		t.Errorf("expected 1 workflow_run, got %d", len(exported.Data.WorkflowRuns))
	}
	if len(exported.Data.WorkflowRunSteps) != 2 {
		t.Errorf("expected 2 workflow_run_steps, got %d", len(exported.Data.WorkflowRunSteps))
	}

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

	reexported, err := ieStore2.ExportAll(ctx)
	if err != nil {
		t.Fatalf("Re-ExportAll failed: %v", err)
	}
	if len(reexported.Data.Entries) != 2 {
		t.Errorf("round-trip: expected 2 entries, got %d", len(reexported.Data.Entries))
	}
	if len(reexported.Data.Projects) != 2 {
		t.Errorf("round-trip: expected 2 projects, got %d", len(reexported.Data.Projects))
	}
	if len(reexported.Data.EntryLinks) != 1 {
		t.Errorf("round-trip: expected 1 entry_link, got %d", len(reexported.Data.EntryLinks))
	}
	if len(reexported.Data.WorkflowRuns) != 1 {
		t.Errorf("round-trip: expected 1 workflow_run, got %d", len(reexported.Data.WorkflowRuns))
	}
	if len(reexported.Data.WorkflowRunSteps) != 2 {
		t.Errorf("round-trip: expected 2 workflow_run_steps, got %d", len(reexported.Data.WorkflowRunSteps))
	}
	if reexported.Data.WorkflowRuns[0].ID != "run-1" {
		t.Errorf("round-trip: workflow_run ID = %q, want 'run-1'", reexported.Data.WorkflowRuns[0].ID)
	}
}

func TestImportRejectsMissingVersion(t *testing.T) {
	_, _, _, _, _, _, _, _, iestore, cleanup := setupImportExportStore(t)
	defer cleanup()
	ctx := context.Background()

	err := iestore.ImportAll(ctx, domain.VaultExport{})
	if err == nil {
		t.Fatal("expected error for missing schema_version")
	}
}

func TestImportRejectsHigherVersion(t *testing.T) {
	_, _, _, _, _, _, _, _, iestore, cleanup := setupImportExportStore(t)
	defer cleanup()
	ctx := context.Background()

	err := iestore.ImportAll(ctx, domain.VaultExport{SchemaVersion: 99})
	if err == nil {
		t.Fatal("expected error for unsupported schema_version")
	}
}
