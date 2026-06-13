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
	"github.com/quantum-6/skillvault/internal/files"
	"github.com/quantum-6/skillvault/internal/security"
)

// AC1: Initialize vault
// Given no existing vault, when skillvault init runs,
// then required folders and SQLite database are created.
func TestAC1_InitializeVaultCreatesTablesAndFolders(t *testing.T) {
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer sqlDB.Close()

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	tables := []string{
		"schema_migrations",
		"projects",
		"entries",
		"entry_tags",
		"series",
		"series_entries",
		"workflows",
		"workflow_steps",
		"artifacts",
		"entry_links",
		"tags",
		"entries_fts",
	}
	for _, table := range tables {
		var count int
		err := sqlDB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("check table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("required table %s does not exist", table)
		}
	}

	tmpDir := t.TempDir()
	fileSvc, err := files.NewArtifactFileService(tmpDir)
	if err != nil {
		t.Fatalf("NewArtifactFileService failed: %v", err)
	}

	_, _, _, err = fileSvc.WriteArtifact("init-test", []byte("hello"), "text/plain")
	if err != nil {
		t.Fatalf("WriteArtifact failed: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(tmpDir, "objects"))
	if err != nil {
		t.Fatalf("objects dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected year subdirectories under objects/")
	}
}

// AC2: Save and search entry
// Given an entry is saved, when searching by title/body/tag,
// then it is returned with metadata.
func TestAC2_SaveAndSearchEntryByTitleBodyTag(t *testing.T) {
	_, svc, _, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	svc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Go REST API Design",
		Type:    "skill",
		Summary: "Best practices for REST APIs in Go",
		Body:    "Use net/http or chi router for routing",
		Project: "testproj",
		Tags:    []string{"go", "rest", "api"},
		Status:  "active",
	})

	results, err := svc.SearchEntries(ctx, "REST", domain.SearchQuery{Limit: 10, IncludeArchived: true})
	if err != nil {
		t.Fatalf("SearchEntries by title failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("AC2 FAIL: search by title found no results")
	}
	found := false
	for _, r := range results {
		if r.Entry.Title == "Go REST API Design" {
			found = true
			if r.Entry.Summary != "Best practices for REST APIs in Go" {
				t.Errorf("AC2 FAIL: expected summary metadata, got %q", r.Entry.Summary)
			}
			break
		}
	}
	if !found {
		t.Fatal("AC2 FAIL: search by title did not return the entry")
	}

	results, err = svc.SearchEntries(ctx, "chi router", domain.SearchQuery{Limit: 10, IncludeArchived: true})
	if err != nil {
		t.Fatalf("SearchEntries by body failed: %v", err)
	}
	found = false
	for _, r := range results {
		if r.Entry.Title == "Go REST API Design" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("AC2 FAIL: search by body did not return the entry")
	}

	results, err = svc.SearchEntries(ctx, "go", domain.SearchQuery{Limit: 10, IncludeArchived: true})
	if err != nil {
		t.Fatalf("SearchEntries by tag failed: %v", err)
	}
	found = false
	for _, r := range results {
		if r.Entry.Title == "Go REST API Design" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("AC2 FAIL: search by tag did not return the entry")
	}
}

// AC3: Save long artifact
// Given a long PDF analysis is saved, then content is stored as a file
// and DB stores metadata, summary, hash, and file path.
func TestAC3_SaveLongArtifact(t *testing.T) {
	store, _, artSvc, _, _, projectSvc, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "forense-digital"})

	longAnalysis := strings.Repeat("This is a detailed PDF analysis of the forensic digital report. ", 2000)

	artSvc2 := NewArtifactService(store.Artifacts, store.Entries, store.Projects)
	artifact, err := artSvc2.SaveArtifact(ctx, SaveArtifactInput{
		Title:   "Forense Digital PDF Analysis",
		Type:    "pdf_analysis",
		Content: longAnalysis,
		Summary: "Complete forensic analysis of digital evidence",
		Project: "forense-digital",
	})
	if err != nil {
		t.Fatalf("AC3 FAIL: SaveArtifact failed: %v", err)
	}

	if artifact.Title != "Forense Digital PDF Analysis" {
		t.Errorf("AC3 FAIL: Title = %q, want 'Forense Digital PDF Analysis'", artifact.Title)
	}
	if artifact.Type != domain.ArtifactTypePDFAnalysis {
		t.Errorf("AC3 FAIL: Type = %q, want 'pdf_analysis'", artifact.Type)
	}
	if artifact.Summary != "Complete forensic analysis of digital evidence" {
		t.Errorf("AC3 FAIL: Summary = %q, want 'Complete forensic analysis of digital evidence'", artifact.Summary)
	}
	if artifact.SizeBytes != int64(len(longAnalysis)) {
		t.Errorf("AC3 FAIL: SizeBytes = %d, want %d", artifact.SizeBytes, len(longAnalysis))
	}

	got, err := artSvc.GetArtifact(ctx, artifact.ID)
	if err != nil {
		t.Fatalf("AC3 FAIL: GetArtifact failed: %v", err)
	}
	if got.FilePath == "" {
		t.Error("AC3 FAIL: FilePath should not be empty")
	}
	if got.SizeBytes != int64(len(longAnalysis)) {
		t.Errorf("AC3 FAIL: stored SizeBytes = %d, want %d", got.SizeBytes, len(longAnalysis))
	}
}

// AC4: Context generation
// Given profile, feedback, project decisions, and workflow entries exist,
// when get_context --mode planning is called, then a compact context pack is returned.
func TestAC4_ContextGenerationPlanningMode(t *testing.T) {
	store, entrySvc, _, workflowSvc, _, projectSvc, contextSvc, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, _ := projectSvc.SaveProject(ctx, SaveProjectInput{Name: "skillvault", Description: "SkillVault v2 Hermes"})

	entrySvc.Save(ctx, domain.Entry{
		ID: "fb-prefs", Title: "Prefer Go for backend", Slug: "prefer-go",
		Type: domain.EntryTypeFeedback, Summary: "User prefers Go for backend", Status: domain.StatusActive,
	}, nil)

	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Use SQLite FTS5", Type: "decision", Summary: "FTS5 for full-text search",
		Project: proj.ID, Status: "active",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "No vector DB in v1", Type: "decision", Summary: "Skip vector search in v1",
		Project: proj.ID, Status: "active",
	})

	workflowSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:        "spec-plan-task",
		Description: "Spec first, plan, then tasks",
		Steps: []SaveWorkflowStep{
			{OrderIndex: 1, Title: "Write spec", Instruction: "Draft spec doc", Required: true},
			{OrderIndex: 2, Title: "Plan", Instruction: "Break into tasks", Required: true},
		},
	})

	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Session Apr 10", Type: "session", Summary: "Implemented artifact store",
		Project: proj.ID, Status: "active",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Session Apr 11", Type: "session", Summary: "Added workflow rendering",
		Project: proj.ID, Status: "active",
	})

	_, err := store.Entries.Get(ctx, "fb-prefs", true)
	if err != nil {
		t.Fatalf("AC4 FAIL: feedback entry not saved: %v", err)
	}

	pack, err := contextSvc.GetContext(ctx, ContextInput{
		Mode:            "planning",
		Project:         proj.ID,
		ExcludeArchived: true,
		MaxChars:        10000,
	})
	if err != nil {
		t.Fatalf("AC4 FAIL: GetContext failed: %v", err)
	}

	if pack == nil {
		t.Fatal("AC4 FAIL: context pack is nil")
	}
	if !strings.Contains(pack.Raw, "CONTEXT PACK") {
		t.Fatal("AC4 FAIL: missing CONTEXT PACK header")
	}
	if !strings.Contains(pack.Raw, "skillvault") {
		t.Error("AC4 FAIL: missing project name in context pack")
	}
	if !strings.Contains(pack.Raw, "Use SQLite FTS5") {
		t.Error("AC4 FAIL: missing decision in context pack")
	}
	if !strings.Contains(pack.Raw, "spec-plan-task") {
		t.Error("AC4 FAIL: missing workflow in context pack")
	}
	if !strings.Contains(pack.Raw, "Session Apr 10") {
		t.Error("AC4 FAIL: missing recent session in context pack")
	}
	if !strings.Contains(pack.Raw, "User prefers Go for backend") {
		t.Error("AC4 FAIL: missing user preferences/feedback in context pack")
	}
}

// AC5: Archived content behavior
// Given an entry is archived, normal search/context excludes it
// unless include_archived is true.
func TestAC5_ArchivedContentExcludedByDefault(t *testing.T) {
	_, svc, _, _, _, projectSvc, contextSvc, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, _ := projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	svc.SaveEntry(ctx, SaveEntryInput{
		Title: "Active Decision", Type: "decision", Summary: "Active decision body",
		Project: proj.ID, Status: "active",
	})
	svc.SaveEntry(ctx, SaveEntryInput{
		Title: "Archived Decision", Type: "decision", Summary: "Archived decision body",
		Project: proj.ID, Status: "archived",
	})

	results, err := svc.SearchEntries(ctx, "Decision", domain.SearchQuery{Limit: 10, IncludeArchived: false})
	if err != nil {
		t.Fatalf("AC5 FAIL: SearchEntries failed: %v", err)
	}
	for _, r := range results {
		if r.Entry.Title == "Archived Decision" {
			t.Fatal("AC5 FAIL: archived entry found in search with IncludeArchived=false")
		}
	}

	results, err = svc.SearchEntries(ctx, "Decision", domain.SearchQuery{Limit: 10, IncludeArchived: true})
	if err != nil {
		t.Fatalf("AC5 FAIL: SearchEntries include archived failed: %v", err)
	}
	found := false
	for _, r := range results {
		if r.Entry.Title == "Archived Decision" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("AC5 FAIL: archived entry should appear with IncludeArchived=true")
	}

	pack, err := contextSvc.GetContext(ctx, ContextInput{
		Mode:            "planning",
		Project:         proj.ID,
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("AC5 FAIL: GetContext failed: %v", err)
	}
	if strings.Contains(pack.Raw, "Archived Decision") {
		t.Fatal("AC5 FAIL: archived entry found in context with ExcludeArchived=true")
	}
	if !strings.Contains(pack.Raw, "Active Decision") {
		t.Error("AC5 FAIL: active entry should appear in context")
	}
}

// AC6: Secret protection
// Given content includes a secret-like pattern, saving is rejected or redacted.
func TestAC6_SecretProtectionDetectsAndRedacts(t *testing.T) {
	scanner := security.New()

	secrets := []struct {
		name    string
		content string
		typ     string
	}{
		{"OpenAI API key", "sk-abc" + "123XYZ_-abc123XYZ_-abc123XYZ_-abc12", "openai_api_key"},
		{"RSA private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA", "private_key"},
		{"GitHub token", "ghp_abc" + "123XYZ_abc123XYZ_abc123XYZ_abc1", "github_token"},
		{"Slack bot token", "xoxb" + "-1234567890-1234567890-abcdefghijklmnopqrst", "slack_token"},
	}

	for _, s := range secrets {
		result, err := scanner.Scan(s.content)
		if err != nil {
			t.Errorf("AC6 FAIL: Scan(%s) error: %v", s.name, err)
			continue
		}
		if !result.HasSecret {
			t.Errorf("AC6 FAIL: %s should be detected as secret", s.name)
			continue
		}
		if len(result.Matches) == 0 || result.Matches[0].Type != s.typ {
			got := ""
			if len(result.Matches) > 0 {
				got = result.Matches[0].Type
			}
			t.Errorf("AC6 FAIL: %s type = %q, want %q", s.name, got, s.typ)
		}

		redacted, matches := scanner.Redact(s.content)
		if !strings.Contains(redacted, "[REDACTED]") {
			t.Errorf("AC6 FAIL: %s should be redacted, got %q", s.name, redacted)
		}
		if len(matches) == 0 {
			t.Errorf("AC6 FAIL: %s should have redact matches", s.name)
		}
	}

	clean := "This is a perfectly safe prompt for a coding agent."
	result, err := scanner.Scan(clean)
	if err != nil {
		t.Fatalf("AC6 FAIL: Scan clean error: %v", err)
	}
	if result.HasSecret {
		t.Fatalf("AC6 FAIL: clean content should not have secrets: %+v", result.Matches)
	}

	redacted, matches := scanner.Redact(clean)
	if redacted != clean {
		t.Errorf("AC6 FAIL: clean content changed: %q", redacted)
	}
	if len(matches) != 0 {
		t.Errorf("AC6 FAIL: clean content has unexpected matches: %v", matches)
	}
}

// AC7: Workflow rendering
// Given a workflow has steps, render_workflow returns ordered checklist instructions.
func TestAC7_WorkflowRenderReturnsOrderedChecklist(t *testing.T) {
	_, _, _, svc, _, _, _, _, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	wf, err := svc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name:        "Code Review",
		Description: "Standard code review workflow",
		Steps: []SaveWorkflowStep{
			{OrderIndex: 1, Title: "Check style", Instruction: "Run linter", Required: true},
			{OrderIndex: 2, Title: "Run tests", Instruction: "go test ./...", Required: true},
			{OrderIndex: 3, Title: "Review logic", Instruction: "Check business logic", Required: false},
		},
	})
	if err != nil {
		t.Fatalf("AC7 FAIL: SaveWorkflow failed: %v", err)
	}

	steps, err := svc.RenderWorkflow(ctx, wf.ID)
	if err != nil {
		t.Fatalf("AC7 FAIL: RenderWorkflow failed: %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("AC7 FAIL: expected 3 steps, got %d", len(steps))
	}
	if steps[0].OrderIndex != 1 || steps[0].Title != "Check style" {
		t.Errorf("AC7 FAIL: step 1 = OrderIndex=%d, Title=%q", steps[0].OrderIndex, steps[0].Title)
	}
	if steps[1].OrderIndex != 2 || steps[1].Title != "Run tests" {
		t.Errorf("AC7 FAIL: step 2 = OrderIndex=%d, Title=%q", steps[1].OrderIndex, steps[1].Title)
	}
	if steps[2].OrderIndex != 3 || steps[2].Title != "Review logic" {
		t.Errorf("AC7 FAIL: step 3 = OrderIndex=%d, Title=%q", steps[2].OrderIndex, steps[2].Title)
	}

	if steps[0].Instruction != "Run linter" {
		t.Errorf("AC7 FAIL: step 1 instruction = %q, want 'Run linter'", steps[0].Instruction)
	}
	if !steps[0].Required {
		t.Error("AC7 FAIL: step 1 should be required")
	}
	if steps[2].Required {
		t.Error("AC7 FAIL: step 3 should NOT be required")
	}
}

// AC8: Session wrap
// Given a session summary with decisions and pending items,
// session_wrap creates a session entry linked to the project.
func TestAC8_SessionWrapCreatesSessionEntryLinkedToProject(t *testing.T) {
	_, entrySvc, _, _, _, projectSvc, _, svc, _, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := projectSvc.SaveProject(ctx, SaveProjectInput{Name: "skillvault"})
	if err != nil {
		t.Fatalf("AC8 FAIL: SaveProject failed: %v", err)
	}

	output, err := svc.SessionWrap(ctx, SessionWrapInput{
		Project:   proj.ID,
		Summary:   "Fixed auth middleware bug and added rate limiting",
		Decisions: []string{"Use JWT with refresh tokens"},
		Pending:   []string{"Add rate limiting to prod"},
		Learnings: []string{"JWT expiry must be short"},
	})
	if err != nil {
		t.Fatalf("AC8 FAIL: SessionWrap failed: %v", err)
	}

	if output.Entry == nil {
		t.Fatal("AC8 FAIL: expected session entry in output")
	}
	if output.Entry.Entry.Entry.Type != domain.EntryTypeSession {
		t.Errorf("AC8 FAIL: Type = %q, want 'session'", output.Entry.Entry.Entry.Type)
	}
	if output.Entry.Entry.Entry.ProjectID == nil || *output.Entry.Entry.Entry.ProjectID != proj.ID {
		t.Errorf("AC8 FAIL: ProjectID = %v, want %q", output.Entry.Entry.Entry.ProjectID, proj.ID)
	}
	if !strings.Contains(output.Entry.Entry.Entry.Summary, "auth") {
		t.Errorf("AC8 FAIL: Summary should contain 'auth', got %q", output.Entry.Entry.Entry.Summary)
	}

	result, err := entrySvc.GetEntry(ctx, output.Entry.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("AC8 FAIL: GetEntry after session wrap failed: %v", err)
	}
	if result.Entry.Entry.Type != domain.EntryTypeSession {
		t.Errorf("AC8 FAIL: stored Type = %q, want 'session'", result.Entry.Entry.Type)
	}
	if result.Entry.Entry.ProjectID == nil || *result.Entry.Entry.ProjectID != proj.ID {
		t.Errorf("AC8 FAIL: stored ProjectID = %v, want %q", result.Entry.Entry.ProjectID, proj.ID)
	}
}

// AC9: Import/export
// Given a vault has entries/projects/workflows, export and import preserve them.
func TestAC9_ExportImportPreservesAllEntities(t *testing.T) {
	_, entrySvc, artSvc, workflowSvc, _, projectSvc, _, _, exportSvc, _, cleanup := setupAppServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, _ := projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj", Description: "Test"})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title: "Skill 1", Type: "skill", Summary: "First skill", Project: proj.ID, Status: "active",
	})
	artSvc.SaveArtifact(ctx, SaveArtifactInput{
		Title: "Artifact 1", Type: "markdown", Content: "# Doc", Summary: "A doc", Project: proj.ID,
	})
	workflowSvc.SaveWorkflow(ctx, SaveWorkflowInput{
		Name: "WF 1", Steps: []SaveWorkflowStep{
			{Title: "Step 1", Instruction: "Do it"},
		},
	})

	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "export.json")
	if err := exportSvc.ExportVault(ctx, ExportVaultInput{OutputPath: exportPath, IncludeArtifacts: true}); err != nil {
		t.Fatalf("AC9 FAIL: ExportVault failed: %v", err)
	}
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatal("AC9 FAIL: export file not created")
	}

	sqlDB2, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("AC9 FAIL: OpenDB2 failed: %v", err)
	}
	defer sqlDB2.Close()
	if err := db.RunMigrations(sqlDB2); err != nil {
		t.Fatalf("AC9 FAIL: RunMigrations2 failed: %v", err)
	}
	store2 := db.NewStore(sqlDB2)
	importSvc2 := NewVaultImportService(store2.ImportExport, store2.Entries, store2.Projects, store2.Artifacts)
	if err := importSvc2.Import(ctx, exportPath); err != nil {
		t.Fatalf("AC9 FAIL: Import failed: %v", err)
	}

	projects, err := store2.Projects.List(ctx, true)
	if err != nil {
		t.Fatalf("AC9 FAIL: list projects failed: %v", err)
	}
	foundProj := false
	for _, p := range projects {
		if p.Name == "testproj" {
			foundProj = true
			if p.Description != "Test" {
				t.Errorf("AC9 FAIL: project description = %q, want 'Test'", p.Description)
			}
			break
		}
	}
	if !foundProj {
		t.Fatal("AC9 FAIL: project not preserved after import")
	}

	entries, err := store2.Entries.List(ctx, domain.EntryFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("AC9 FAIL: list entries failed: %v", err)
	}
	foundEntry := false
	for _, e := range entries {
		if e.Entry.Title == "Skill 1" {
			foundEntry = true
			if e.Entry.Summary != "First skill" {
				t.Errorf("AC9 FAIL: entry summary = %q, want 'First skill'", e.Entry.Summary)
			}
			break
		}
	}
	if !foundEntry {
		t.Fatal("AC9 FAIL: entry not preserved after import")
	}

	workflows, err := store2.Workflows.List(ctx, true)
	if err != nil {
		t.Fatalf("AC9 FAIL: list workflows failed: %v", err)
	}
	foundWF := false
	for _, w := range workflows {
		if w.Name == "WF 1" {
			foundWF = true
			break
		}
	}
	if !foundWF {
		t.Fatal("AC9 FAIL: workflow not preserved after import")
	}

	artifacts, err := store2.Artifacts.List(ctx, nil)
	if err != nil {
		t.Fatalf("AC9 FAIL: list artifacts failed: %v", err)
	}
	foundArt := false
	for _, a := range artifacts {
		if a.Title == "Artifact 1" {
			foundArt = true
			if a.Summary != "A doc" {
				t.Errorf("AC9 FAIL: artifact summary = %q, want 'A doc'", a.Summary)
			}
			break
		}
	}
	if !foundArt {
		t.Fatal("AC9 FAIL: artifact not preserved after import")
	}
}
