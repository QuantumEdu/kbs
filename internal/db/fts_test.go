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

func TestSearchByName(t *testing.T) {
	estore, sstore, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.UpsertEntry(ctx, domain.Entry{ID: "e1", Name: "FastAPI Design", Type: domain.EntryTypeSkill, Content: "FastAPI backend", Active: true}, []string{"python"}, nil)
	estore.UpsertEntry(ctx, domain.Entry{ID: "e2", Name: "Go CLI", Type: domain.EntryTypeSkill, Content: "CLI tool", Active: true}, []string{"go"}, nil)

	// Search by name
	results, err := sstore.SearchEntries(ctx, domain.SearchQuery{Query: "FastAPI"})
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
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
	estore, sstore, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.UpsertEntry(ctx, domain.Entry{ID: "e1", Name: "E1", Type: domain.EntryTypeSkill, Content: "Design backend architecture", Active: true}, nil, nil)

	results, err := sstore.SearchEntries(ctx, domain.SearchQuery{Query: "architecture"})
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected result for 'architecture'")
	}
}

func TestSearchByTag(t *testing.T) {
	estore, sstore, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.UpsertEntry(ctx, domain.Entry{ID: "e1", Name: "Go Tool", Type: domain.EntryTypeSkill, Content: "tool", Active: true}, []string{"go", "cli"}, nil)

	results, err := sstore.SearchEntries(ctx, domain.SearchQuery{Query: "cli"})
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected result for tag 'cli'")
	}
}

func TestSearchExcludesArchived(t *testing.T) {
	estore, sstore, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.UpsertEntry(ctx, domain.Entry{ID: "e1", Name: "Active Entry", Type: domain.EntryTypeNote, Content: "visible", Active: true}, nil, nil)
	estore.UpsertEntry(ctx, domain.Entry{ID: "e2", Name: "Archived Entry", Type: domain.EntryTypeNote, Content: "hidden", Active: true}, nil, nil)
	estore.ArchiveEntry(ctx, "e2")

	results, err := sstore.SearchEntries(ctx, domain.SearchQuery{Query: "entry"})
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 active result, got %d", len(results))
	}

	// With include_archived
	results, err = sstore.SearchEntries(ctx, domain.SearchQuery{Query: "entry", IncludeArchived: true})
	if err != nil {
		t.Fatalf("SearchEntries with include_archived failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with include_archived, got %d", len(results))
	}
}

func TestSearchFilterByType(t *testing.T) {
	estore, sstore, cleanup := setupSearchStore(t)
	defer cleanup()
	ctx := context.Background()

	estore.UpsertEntry(ctx, domain.Entry{ID: "e1", Name: "Skill A", Type: domain.EntryTypeSkill, Content: "skill a", Active: true}, nil, nil)
	estore.UpsertEntry(ctx, domain.Entry{ID: "e2", Name: "Prompt A", Type: domain.EntryTypePrompt, Content: "prompt a", Active: true}, nil, nil)

	skillType := "skill"
	results, err := sstore.SearchEntries(ctx, domain.SearchQuery{Query: "a", Type: &skillType})
	if err != nil {
		t.Fatalf("SearchEntries by type failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 skill result, got %d", len(results))
	}
}
