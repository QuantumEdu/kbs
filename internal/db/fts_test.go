package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupSearchStore(t *testing.T) (EntryStore, SearchStore, func()) {
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
	searchStore := &sqliteSearchStore{db: db}
	cleanup := func() { db.Close() }
	return entryStore, searchStore, cleanup
}

func TestSearchByTitle(t *testing.T) {
	estore, _, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.Save(ctx, domain.Entry{ID: "e1", Title: "FastAPI Design", Slug: "fastapi-design", Type: domain.EntryTypeSkill, BodyOptional: "FastAPI backend", Status: domain.StatusActive}, []string{"python"})
	estore.Save(ctx, domain.Entry{ID: "e2", Title: "Go CLI", Slug: "go-cli", Type: domain.EntryTypeSkill, BodyOptional: "CLI tool", Status: domain.StatusActive}, []string{"go"})

	results, err := estore.Search(ctx, domain.SearchQuery{Query: "FastAPI"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'FastAPI'")
	}
	found := false
	for _, r := range results {
		if r.Entry.ID == "e1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected e1 in search results")
	}
}

func TestSearchByContent(t *testing.T) {
	estore, _, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.Save(ctx, domain.Entry{ID: "e1", Title: "E1", Slug: "e1", Type: domain.EntryTypeSkill, BodyOptional: "Design backend architecture", Status: domain.StatusActive}, nil)

	results, err := estore.Search(ctx, domain.SearchQuery{Query: "architecture"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected result for 'architecture'")
	}
}

func TestSearchByTag(t *testing.T) {
	estore, _, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.Save(ctx, domain.Entry{ID: "e1", Title: "Go Tool", Slug: "go-tool", Type: domain.EntryTypeSkill, BodyOptional: "tool", Status: domain.StatusActive}, []string{"go", "cli"})

	results, err := estore.Search(ctx, domain.SearchQuery{Query: "cli"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected result for tag 'cli'")
	}
}

func TestSearchExcludesArchived(t *testing.T) {
	estore, _, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.Save(ctx, domain.Entry{ID: "e1", Title: "Active Entry", Slug: "active-entry", Type: domain.EntryTypeSession, BodyOptional: "visible", Status: domain.StatusActive}, nil)
	estore.Save(ctx, domain.Entry{ID: "e2", Title: "Archived Entry", Slug: "archived-entry", Type: domain.EntryTypeSession, BodyOptional: "hidden", Status: domain.StatusActive}, nil)
	estore.Archive(ctx, "e2")

	results, err := estore.Search(ctx, domain.SearchQuery{Query: "entry"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 active result, got %d", len(results))
	}

	results, err = estore.Search(ctx, domain.SearchQuery{Query: "entry", IncludeArchived: true})
	if err != nil {
		t.Fatalf("Search with include_archived failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with include_archived, got %d", len(results))
	}
}

func TestSearchFilterByType(t *testing.T) {
	estore, _, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.Save(ctx, domain.Entry{ID: "e1", Title: "Skill A", Slug: "skill-a", Type: domain.EntryTypeSkill, BodyOptional: "skill a", Status: domain.StatusActive}, nil)
	estore.Save(ctx, domain.Entry{ID: "e2", Title: "Prompt A", Slug: "prompt-a", Type: domain.EntryTypePrompt, BodyOptional: "prompt a", Status: domain.StatusActive}, nil)

	skillType := "skill"
	results, err := estore.Search(ctx, domain.SearchQuery{Query: "a", Type: &skillType})
	if err != nil {
		t.Fatalf("Search by type failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 skill result, got %d", len(results))
	}
}
