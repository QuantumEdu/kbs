package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupTagStore(t *testing.T) (TagStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := &sqliteTagStore{db: db}
	cleanup := func() { db.Close() }
	return store, cleanup
}

func TestSaveAndListTags(t *testing.T) {
	store, cleanup := setupTagStore(t)
	defer cleanup()
	ctx := context.Background()

	if err := store.Save(ctx, domain.Tag{ID: "go", Name: "go", Slug: "go"}); err != nil {
		t.Fatalf("Save tag 'go' failed: %v", err)
	}
	if err := store.Save(ctx, domain.Tag{ID: "api", Name: "api", Slug: "api"}); err != nil {
		t.Fatalf("Save tag 'api' failed: %v", err)
	}
	if err := store.Save(ctx, domain.Tag{ID: "backend", Name: "backend", Slug: "backend"}); err != nil {
		t.Fatalf("Save tag 'backend' failed: %v", err)
	}

	tags, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List tags failed: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
}

func TestSearchTags(t *testing.T) {
	store, cleanup := setupTagStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, tag := range []domain.Tag{
		{ID: "golang", Name: "golang", Slug: "golang"},
		{ID: "python", Name: "python", Slug: "python"},
		{ID: "go-micro", Name: "go-micro", Slug: "go-micro"},
	} {
		if err := store.Save(ctx, tag); err != nil {
			t.Fatalf("Save tag %s failed: %v", tag.ID, err)
		}
	}

	results, err := store.Search(ctx, "go")
	if err != nil {
		t.Fatalf("Search tags failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 tags matching 'go', got %d: %v", len(results), results)
	}
}

func TestSaveTagIdempotent(t *testing.T) {
	store, cleanup := setupTagStore(t)
	defer cleanup()
	ctx := context.Background()

	if err := store.Save(ctx, domain.Tag{ID: "go", Name: "go", Slug: "go"}); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	if err := store.Save(ctx, domain.Tag{ID: "go", Name: "golang", Slug: "golang"}); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	tags, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Name != "golang" {
		t.Errorf("Name = %q, want 'golang'", tags[0].Name)
	}
}
