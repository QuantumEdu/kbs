package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupProjectStore(t *testing.T) (ProjectStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := &sqliteProjectStore{db: db}
	cleanup := func() { db.Close() }
	return store, cleanup
}

func TestSaveProjectCreate(t *testing.T) {
	store, cleanup := setupProjectStore(t)
	defer cleanup()
	ctx := context.Background()

	p := domain.Project{
		ID:          "vitacare",
		Name:        "VitaCare",
		Slug:        "vitacare",
		Description: "Healthcare platform",
		Status:      domain.StatusActive,
	}
	err := store.Save(ctx, p)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	projects, err := store.List(ctx, false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].ID != "vitacare" {
		t.Errorf("ID = %q, want 'vitacare'", projects[0].ID)
	}
	if projects[0].Name != "VitaCare" {
		t.Errorf("Name = %q, want 'VitaCare'", projects[0].Name)
	}
}

func TestSaveProjectUpdate(t *testing.T) {
	store, cleanup := setupProjectStore(t)
	defer cleanup()
	ctx := context.Background()

	p := domain.Project{ID: "vitacare", Name: "VitaCare", Slug: "vitacare", Status: domain.StatusActive}
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("initial Save failed: %v", err)
	}

	p.Name = "VitaCare v2"
	p.Description = "Updated platform"
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("update Save failed: %v", err)
	}

	projects, err := store.List(ctx, false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "VitaCare v2" {
		t.Errorf("Name = %q, want 'VitaCare v2'", projects[0].Name)
	}
}

func TestListProjectsIncludeArchived(t *testing.T) {
	store, cleanup := setupProjectStore(t)
	defer cleanup()
	ctx := context.Background()

	if err := store.Save(ctx, domain.Project{ID: "p1", Name: "P1", Slug: "p1", Status: domain.StatusActive}); err != nil {
		t.Fatalf("Save p1 failed: %v", err)
	}
	if err := store.Save(ctx, domain.Project{ID: "p2", Name: "P2", Slug: "p2", Status: domain.StatusArchived}); err != nil {
		t.Fatalf("Save p2 failed: %v", err)
	}

	active, err := store.List(ctx, false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active project, got %d", len(active))
	}

	all, err := store.List(ctx, true)
	if err != nil {
		t.Fatalf("List with include_archived failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 projects with include_archived, got %d", len(all))
	}
}

func TestGetProject(t *testing.T) {
	store, cleanup := setupProjectStore(t)
	defer cleanup()
	ctx := context.Background()

	if err := store.Save(ctx, domain.Project{ID: "p1", Name: "P1", Slug: "p1", Status: domain.StatusActive}); err != nil {
		t.Fatalf("Save p1 failed: %v", err)
	}

	got, err := store.Get(ctx, "p1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "p1" {
		t.Errorf("ID = %q, want 'p1'", got.ID)
	}

	_, err = store.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestArchiveProject(t *testing.T) {
	store, cleanup := setupProjectStore(t)
	defer cleanup()
	ctx := context.Background()

	if err := store.Save(ctx, domain.Project{ID: "p1", Name: "P1", Slug: "p1", Status: domain.StatusActive}); err != nil {
		t.Fatalf("Save p1 failed: %v", err)
	}

	if err := store.Archive(ctx, "p1"); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	got, err := store.Get(ctx, "p1")
	if err != nil {
		t.Fatalf("Get after archive failed: %v", err)
	}
	if got.Status != domain.StatusArchived {
		t.Errorf("Status = %q, want 'archived'", got.Status)
	}

	projects, err := store.List(ctx, false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 active projects after archive, got %d", len(projects))
	}
}
