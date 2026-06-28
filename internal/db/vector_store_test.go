package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/quantum-6/skillvault/internal/vector"
)

func setupVectorStore(t *testing.T) (*sql.DB, VectorStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := &sqliteVectorStore{db: db}
	cleanup := func() { db.Close() }
	return db, store, cleanup
}

// ensureEntry inserts a minimal entry so the foreign key on
// entry_embeddings(entry_id) is satisfied.
func ensureEntry(t *testing.T, db *sql.DB, entryID string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO entries (id, title, slug, type, status) VALUES (?, ?, ?, 'skill', 'active')`,
		entryID, entryID, entryID,
	)
	if err != nil {
		t.Fatalf("ensure entry %q: %v", entryID, err)
	}
}

func TestVectorStore_SaveAndGet(t *testing.T) {
	rawDB, store, cleanup := setupVectorStore(t)
	defer cleanup()
	ctx := context.Background()

	ensureEntry(t, rawDB, "entry1")

	vec := []float32{0.1, 0.2, 0.3}
	data := vector.Serialize(vec)

	err := store.SaveEmbedding(ctx, "entry1", data, 3, "glove")
	if err != nil {
		t.Fatalf("SaveEmbedding failed: %v", err)
	}

	got, err := store.GetEmbedding(ctx, "entry1")
	if err != nil {
		t.Fatalf("GetEmbedding failed: %v", err)
	}

	restored, err := vector.Deserialize(got)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}
	if len(restored) != 3 {
		t.Fatalf("expected 3 dims, got %d", len(restored))
	}
	for i, v := range restored {
		if v != vec[i] {
			t.Errorf("dim %d: got %f, want %f", i, v, vec[i])
		}
	}
}

func TestVectorStore_SaveReplace(t *testing.T) {
	rawDB, store, cleanup := setupVectorStore(t)
	defer cleanup()
	ctx := context.Background()

	ensureEntry(t, rawDB, "e1")

	v1 := vector.Serialize([]float32{1.0, 2.0})
	v2 := vector.Serialize([]float32{3.0, 4.0})

	if err := store.SaveEmbedding(ctx, "e1", v1, 2, "glove"); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.SaveEmbedding(ctx, "e1", v2, 2, "glove"); err != nil {
		t.Fatalf("second save: %v", err)
	}

	got, err := store.GetEmbedding(ctx, "e1")
	if err != nil {
		t.Fatalf("GetEmbedding: %v", err)
	}
	restored, err := vector.Deserialize(got)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if restored[0] != 3.0 || restored[1] != 4.0 {
		t.Errorf("got %v, want [3.0, 4.0]", restored)
	}
}

func TestVectorStore_GetMissing(t *testing.T) {
	_, store, cleanup := setupVectorStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.GetEmbedding(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing embedding")
	}
}

func TestVectorStore_Delete(t *testing.T) {
	rawDB, store, cleanup := setupVectorStore(t)
	defer cleanup()
	ctx := context.Background()

	ensureEntry(t, rawDB, "e1")

	data := vector.Serialize([]float32{1.0})
	if err := store.SaveEmbedding(ctx, "e1", data, 1, "glove"); err != nil {
		t.Fatalf("SaveEmbedding: %v", err)
	}

	if err := store.DeleteEmbedding(ctx, "e1"); err != nil {
		t.Fatalf("DeleteEmbedding: %v", err)
	}

	_, err := store.GetEmbedding(ctx, "e1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestVectorStore_SearchSimilar(t *testing.T) {
	rawDB, store, cleanup := setupVectorStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c"} {
		ensureEntry(t, rawDB, id)
	}

	store.SaveEmbedding(ctx, "a", vector.Serialize([]float32{1.0, 0.0, 0.0}), 3, "glove")
	store.SaveEmbedding(ctx, "b", vector.Serialize([]float32{0.0, 1.0, 0.0}), 3, "glove")
	store.SaveEmbedding(ctx, "c", vector.Serialize([]float32{-1.0, 0.0, 0.0}), 3, "glove")

	query := []float32{1.0, 0.0, 0.0}
	results, err := store.SearchSimilar(ctx, query, 0)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].EntryID != "a" {
		t.Errorf("expected 'a' first, got %s", results[0].EntryID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("expected score near 1.0, got %f", results[0].Score)
	}
}

func TestVectorStore_SearchSimilar_Empty(t *testing.T) {
	_, store, cleanup := setupVectorStore(t)
	defer cleanup()
	ctx := context.Background()

	results, err := store.SearchSimilar(ctx, []float32{1.0}, 10)
	if err != nil {
		t.Fatalf("SearchSimilar on empty: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestVectorStore_DeleteAndSearch(t *testing.T) {
	rawDB, store, cleanup := setupVectorStore(t)
	defer cleanup()
	ctx := context.Background()

	ensureEntry(t, rawDB, "keep")
	ensureEntry(t, rawDB, "drop")

	store.SaveEmbedding(ctx, "keep", vector.Serialize([]float32{1.0, 0.0}), 2, "glove")
	store.SaveEmbedding(ctx, "drop", vector.Serialize([]float32{0.0, 1.0}), 2, "glove")

	if err := store.DeleteEmbedding(ctx, "drop"); err != nil {
		t.Fatalf("DeleteEmbedding: %v", err)
	}

	results, err := store.SearchSimilar(ctx, []float32{1.0, 0.0}, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after delete, got %d", len(results))
	}
	if results[0].EntryID != "keep" {
		t.Errorf("expected 'keep', got %s", results[0].EntryID)
	}
}
