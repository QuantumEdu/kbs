package app

import (
	"context"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/vector"
)

// writeGloveFixture creates a temporary GloVe-format file with 5 words in 4 dimensions
// and returns the file path. The caller is responsible for cleaning up the file.
func writeGloveFixture(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "glove-test-*.txt")
	if err != nil {
		t.Fatalf("create temp glove file: %v", err)
	}
	content := "machine 0.1 0.2 0.3 0.4\n" +
		"learning 0.5 0.6 0.7 0.8\n" +
		"language 0.9 1.0 1.1 1.2\n" +
		"processing 1.3 1.4 1.5 1.6\n" +
		"test 0.0 0.0 0.0 0.1\n"
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatalf("write glove fixture: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// newTestVectorService creates a VectorService backed by an in-memory SQLite
// database with all migrations applied.
func newTestVectorService(t *testing.T) (*VectorService, *db.Store, func()) {
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

	// Create a project for entries.
	projectSvc := NewProjectService(store.Projects)
	ctx := context.Background()
	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj", Description: "Test"})

	svc := NewVectorService(store.Entries, store.Embeddings)

	cleanup := func() { sqlDB.Close() }
	return svc, store, cleanup
}

func TestVectorService_IsLoaded(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()

	if svc.IsLoaded() {
		t.Error("IsLoaded should be false before SetGlove")
	}
}

func TestVectorService_SetGlove_FromFixture(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()

	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}

	svc.SetGlove(gv)
	if !svc.IsLoaded() {
		t.Error("IsLoaded should be true after loading fixture")
	}
}

func TestVectorService_SearchVectors_NotLoaded(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.SearchVectors(ctx, "test query", 10)
	if err == nil {
		t.Fatal("expected error for unloaded glove")
	}
	if !strings.Contains(err.Error(), "vector model not loaded") {
		t.Errorf("error should mention glove not loaded, got: %v", err)
	}
}

func TestVectorService_SearchVectors_EmptyQuery(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}
	svc.SetGlove(gv)

	// A query with no known words should return empty results, not error.
	results, err := svc.SearchVectors(ctx, "zzzzzzzz unknown", 10)
	if err != nil {
		t.Fatalf("SearchVectors should not error for OOV query, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for OOV query, got %d", len(results))
	}
}

func TestVectorService_EnsureEmbedded_NotLoaded(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	// Should silently succeed when GloVe is not loaded.
	err := svc.EnsureEmbedded(ctx, "nonexistent")
	if err != nil {
		t.Errorf("EnsureEmbedded should silently skip when glove not loaded, got: %v", err)
	}
}

func TestVectorService_ReindexAll_NotLoaded(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.ReindexAll(ctx)
	if err == nil {
		t.Fatal("expected error for unloaded glove")
	}
}

func TestVectorService_EnsureEmbedded_WithRealGlove(t *testing.T) {
	svc, store, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	// Load glove fixture.
	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}
	svc.SetGlove(gv)

	// Save an entry with content containing known words.
	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Machine Learning",
		Type:    "reference",
		Summary: "Machine learning concepts",
		Body:    "machine learning and language processing",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}
	entryID := result.Entry.Entry.ID

	// Manually ensure embedding.
	err = svc.EnsureEmbedded(ctx, entryID)
	if err != nil {
		t.Fatalf("EnsureEmbedded failed: %v", err)
	}

	// Verify embedding was stored.
	data, err := store.Embeddings.GetEmbedding(ctx, entryID)
	if err != nil {
		t.Fatalf("GetEmbedding failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("embedding data should not be empty")
	}

	// EnsureEmbedded again should be idempotent (no error).
	err = svc.EnsureEmbedded(ctx, entryID)
	if err != nil {
		t.Errorf("second EnsureEmbedded should succeed, got: %v", err)
	}
}

func TestVectorService_ReindexAll_WithGlove(t *testing.T) {
	svc, store, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	// Load glove fixture.
	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}
	svc.SetGlove(gv)

	// Save two entries.
	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Entry A",
		Type:    "reference",
		Summary: "First entry",
		Body:    "machine learning test",
		Project: "testproj",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Entry B",
		Type:    "reference",
		Summary: "Second entry",
		Body:    "language processing",
		Project: "testproj",
	})

	// Reindex all.
	count, err := svc.ReindexAll(ctx)
	if err != nil {
		t.Fatalf("ReindexAll failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 embeddings created, got %d", count)
	}

	// Running again should produce 0 new embeddings.
	count, err = svc.ReindexAll(ctx)
	if err != nil {
		t.Fatalf("second ReindexAll failed: %v", err)
	}
	if count != 0 {
		t.Errorf("second ReindexAll should produce 0 new embeddings, got %d", count)
	}
}

func TestVectorService_SearchVectors_WithResults(t *testing.T) {
	svc, store, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	// Load glove fixture.
	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}
	svc.SetGlove(gv)

	// Save two entries with different content.
	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "ML Guide",
		Type:    "reference",
		Summary: "Machine learning guide",
		Body:    "machine learning machine learning machine learning",
		Project: "testproj",
	})
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "NLP Guide",
		Type:    "reference",
		Summary: "Natural language processing",
		Body:    "language processing natural language understanding",
		Project: "testproj",
	})

	// Reindex to generate embeddings.
	_, err = svc.ReindexAll(ctx)
	if err != nil {
		t.Fatalf("ReindexAll failed: %v", err)
	}

	// Search for "machine learning" — should rank ML Guide higher.
	results, err := svc.SearchVectors(ctx, "machine learning", 10)
	if err != nil {
		t.Fatalf("SearchVectors failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	// ML Guide should appear first (higher cosine similarity to "machine learning").
	if !strings.Contains(results[0].Entry.Title, "ML Guide") {
		t.Errorf("expected 'ML Guide' first, got: %s", results[0].Entry.Title)
	}
}

func TestCompareEntries_Integration(t *testing.T) {
	svc, store, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	// Seed two entries.
	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)

	e1, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Entry One",
		Type:    "reference",
		Summary: "First entry",
		Body:    "This is the first entry body.",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry e1 failed: %v", err)
	}

	e2, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Entry Two",
		Type:    "reference",
		Summary: "Second entry",
		Body:    "This is the second entry body with changes.",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry e2 failed: %v", err)
	}

	diff, err := svc.CompareEntries(ctx, e1.Entry.Entry.ID, e2.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("CompareEntries failed: %v", err)
	}
	if diff == "" {
		t.Error("expected non-empty diff")
	}
	if !strings.Contains(diff, "---") || !strings.Contains(diff, "+++") {
		t.Errorf("diff should contain unified diff markers, got: %s", diff)
	}
}

func TestEntryService_AutoEmbed_SkipsWhenNoVectorService(t *testing.T) {
	_, store, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	// No SetVectorService — auto-embed should silently skip.

	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Auto Embed Test",
		Type:    "reference",
		Summary: "Testing auto-embed",
		Body:    "Body content for embedding test.",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	if result.Entry.Entry.ID == "" {
		t.Error("entry ID should not be empty")
	}
}

func TestEntryService_AutoEmbed_WithGlove(t *testing.T) {
	svc, store, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	// Load glove fixture.
	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}
	svc.SetGlove(gv)

	// Wire VectorService into EntryService.
	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	entrySvc.SetVectorService(svc)

	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Auto Embed Entry",
		Type:    "reference",
		Summary: "Auto embeddings",
		Body:    "machine learning test processing",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}
	entryID := result.Entry.Entry.ID

	// Verify embedding was auto-generated.
	data, err := store.Embeddings.GetEmbedding(ctx, entryID)
	if err != nil {
		t.Fatalf("GetEmbedding after auto-embed failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("auto-embed should produce non-empty embedding data")
	}
}

func TestVectorService_SearchVectors_EmptyStore(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	// Load glove fixture.
	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}
	svc.SetGlove(gv)

	// Search with no embeddings in store — should return empty, no error.
	results, err := svc.SearchVectors(ctx, "machine learning", 10)
	if err != nil {
		t.Fatalf("SearchVectors on empty store should not error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty store, got %d", len(results))
	}
}

func TestVectorService_EnsureEmbedded_NonexistentEntry(t *testing.T) {
	svc, _, cleanup := newTestVectorService(t)
	defer cleanup()
	ctx := context.Background()

	path := writeGloveFixture(t)
	gv, err := vector.LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove fixture failed: %v", err)
	}
	svc.SetGlove(gv)

	// EnsuresEmbedded on nonexistent entry should silently skip.
	err = svc.EnsureEmbedded(ctx, "nonexistent-entry")
	if err != nil {
		t.Errorf("EnsureEmbedded on nonexistent entry should silently skip, got: %v", err)
	}
}

// Test that domain.EntrySearchResult is usable across the codebase.
func TestEntrySearchResult_Fields(t *testing.T) {
	r := domain.EntrySearchResult{
		Entry: domain.Entry{
			ID:    "test-id",
			Title: "Test Entry",
			Type:  domain.EntryTypeReference,
		},
		Tags: []domain.Tag{{ID: "t1", Name: "go"}},
	}
	if r.Entry.ID != "test-id" {
		t.Error("entry ID mismatch")
	}
	if len(r.Tags) != 1 || r.Tags[0].Name != "go" {
		t.Error("tags mismatch")
	}
}
