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

func TestUpsertProjectCreate(t *testing.T) {
	store, cleanup := setupProjectStore(t)
	defer cleanup()
	ctx := context.Background()

	p := domain.Project{
		ID:          "vitacare",
		Name:        "VitaCare",
		Description: "Healthcare platform",
		Active:      true,
	}
	err := store.UpsertProject(ctx, p)
	if err != nil {
		t.Fatalf("UpsertProject failed: %v", err)
	}

	projects, err := store.ListProjects(ctx, false)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
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

func TestUpsertProjectUpdate(t *testing.T) {
	store, cleanup := setupProjectStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create
	p := domain.Project{ID: "vitacare", Name: "VitaCare", Active: true}
	if err := store.UpsertProject(ctx, p); err != nil {
		t.Fatalf("initial UpsertProject failed: %v", err)
	}

	// Update
	p.Name = "VitaCare v2"
	p.Description = "Updated platform"
	if err := store.UpsertProject(ctx, p); err != nil {
		t.Fatalf("update UpsertProject failed: %v", err)
	}

	projects, err := store.ListProjects(ctx, false)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
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

	if err := store.UpsertProject(ctx, domain.Project{ID: "p1", Name: "P1", Active: true}); err != nil {
		t.Fatalf("UpsertProject p1 failed: %v", err)
	}
	if err := store.UpsertProject(ctx, domain.Project{ID: "p2", Name: "P2", Active: false}); err != nil {
		t.Fatalf("UpsertProject p2 failed: %v", err)
	}

	// Without include_archived — only active
	active, err := store.ListProjects(ctx, false)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active project, got %d", len(active))
	}

	// With include_archived — both
	all, err := store.ListProjects(ctx, true)
	if err != nil {
		t.Fatalf("ListProjects with include_archived failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 projects with include_archived, got %d", len(all))
	}
}
