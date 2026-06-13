package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupArtifactStore(t *testing.T) (ArtifactStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := &sqliteArtifactStore{db: db}
	cleanup := func() { db.Close() }
	return store, cleanup
}

func TestSaveAndGetArtifact(t *testing.T) {
	store, cleanup := setupArtifactStore(t)
	defer cleanup()
	ctx := context.Background()

	a := domain.Artifact{
		ID:          "art-1",
		Title:       "PDF Analysis Report",
		Slug:        "pdf-analysis-report",
		Type:        domain.ArtifactTypePDFAnalysis,
		FilePath:    "objects/2026/06/report.md",
		MimeType:    "text/markdown",
		Summary:     "Analysis of the forensic PDF",
		ContentHash: "abc123def456",
		SizeBytes:   1024,
	}

	if err := store.Save(ctx, a); err != nil {
		t.Fatalf("Save artifact failed: %v", err)
	}

	got, err := store.Get(ctx, "art-1")
	if err != nil {
		t.Fatalf("Get artifact failed: %v", err)
	}
	if got.ID != "art-1" {
		t.Errorf("ID = %q, want 'art-1'", got.ID)
	}
	if got.Title != "PDF Analysis Report" {
		t.Errorf("Title = %q, want 'PDF Analysis Report'", got.Title)
	}
	if got.Type != domain.ArtifactTypePDFAnalysis {
		t.Errorf("Type = %q, want 'pdf_analysis'", got.Type)
	}
	if got.FilePath != "objects/2026/06/report.md" {
		t.Errorf("FilePath = %q, want 'objects/2026/06/report.md'", got.FilePath)
	}
	if got.ContentHash != "abc123def456" {
		t.Errorf("ContentHash = %q, want 'abc123def456'", got.ContentHash)
	}
}

func TestSaveAndGetArtifactBySlug(t *testing.T) {
	store, cleanup := setupArtifactStore(t)
	defer cleanup()
	ctx := context.Background()

	a := domain.Artifact{
		ID:          "art-2",
		Title:       "Spec v2",
		Slug:        "spec-v2",
		Type:        domain.ArtifactTypeSpec,
		FilePath:    "specs/v2.md",
		MimeType:    "text/markdown",
		ContentHash: "hash",
		SizeBytes:   500,
	}
	if err := store.Save(ctx, a); err != nil {
		t.Fatalf("Save artifact failed: %v", err)
	}

	got, err := store.Get(ctx, "spec-v2")
	if err != nil {
		t.Fatalf("Get artifact by slug failed: %v", err)
	}
	if got.ID != "art-2" {
		t.Errorf("ID = %q, want 'art-2'", got.ID)
	}
}

func TestListArtifacts(t *testing.T) {
	store, cleanup := setupArtifactStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, a := range []domain.Artifact{
		{ID: "a1", Title: "A1", Slug: "a1", Type: domain.ArtifactTypeMarkdown, FilePath: "a1.md", MimeType: "text/markdown", ContentHash: "h1", SizeBytes: 100},
		{ID: "a2", Title: "A2", Slug: "a2", Type: domain.ArtifactTypeJSON, FilePath: "a2.json", MimeType: "application/json", ContentHash: "h2", SizeBytes: 200},
	} {
		if err := store.Save(ctx, a); err != nil {
			t.Fatalf("Save artifact %s failed: %v", a.ID, err)
		}
	}

	all, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("List all failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(all))
	}
}

func TestListArtifactsByProject(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := &sqliteArtifactStore{db: db}
	ctx := context.Background()

	projStore := &sqliteProjectStore{db: db}
	projID := "proj-1"
	projStore.Save(ctx, domain.Project{ID: projID, Name: "Proj 1", Slug: "proj-1", Status: domain.StatusActive})
	for _, a := range []domain.Artifact{
		{ID: "a1", Title: "A1", Slug: "a1", Type: domain.ArtifactTypeMarkdown, FilePath: "a1.md", MimeType: "text/markdown", ContentHash: "h1", SizeBytes: 100, ProjectID: &projID},
		{ID: "a2", Title: "A2", Slug: "a2", Type: domain.ArtifactTypeJSON, FilePath: "a2.json", MimeType: "application/json", ContentHash: "h2", SizeBytes: 200},
	} {
		if err := store.Save(ctx, a); err != nil {
			t.Fatalf("Save artifact %s failed: %v", a.ID, err)
		}
	}

	filtered, err := store.List(ctx, &projID)
	if err != nil {
		t.Fatalf("List by project failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 artifact for project, got %d", len(filtered))
	}
}
