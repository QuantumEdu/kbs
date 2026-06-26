package db

import (
	"context"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenDBConnection(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB(:memory:) failed: %v", err)
	}
	defer db.Close()

	var v int
	if err := db.QueryRow("SELECT 1").Scan(&v); err != nil {
		t.Fatalf("connection test failed: %v", err)
	}
	if v != 1 {
		t.Errorf("expected 1, got %d", v)
	}
}

func TestNewStoreCreation(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	s := NewStore(db)
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
	if s.Entries == nil {
		t.Error("Entries store is nil")
	}
	if s.Artifacts == nil {
		t.Error("Artifacts store is nil")
	}
	if s.Workflows == nil {
		t.Error("Workflows store is nil")
	}
	if s.WorkflowRuns == nil {
		t.Error("WorkflowRuns store is nil")
	}
	if s.Series == nil {
		t.Error("Series store is nil")
	}
	if s.Tags == nil {
		t.Error("Tags store is nil")
	}
	if s.EntryLinks == nil {
		t.Error("EntryLinks store is nil")
	}
	if s.Projects == nil {
		t.Error("Projects store is nil")
	}
	if s.Search == nil {
		t.Error("Search store is nil")
	}
	if s.ImportExport == nil {
		t.Error("ImportExport store is nil")
	}
	if s.DB() == nil {
		t.Error("DB() returns nil")
	}
}

func TestStoreInterfacesCompile(t *testing.T) {
	var _ EntryStore = (*sqliteEntryStore)(nil)
	var _ ProjectStore = (*sqliteProjectStore)(nil)
	var _ SeriesStore = (*sqliteSeriesStore)(nil)
	var _ WorkflowStore = (*sqliteWorkflowStore)(nil)
	var _ WorkflowRunStore = (*sqliteWorkflowRunStore)(nil)
	var _ ArtifactStore = (*sqliteArtifactStore)(nil)
	var _ TagStore = (*sqliteTagStore)(nil)
	var _ EntryLinkStore = (*sqliteEntryLinkStore)(nil)
	var _ SearchStore = (*sqliteSearchStore)(nil)
	var _ ImportExportStore = (*sqliteImportExportStore)(nil)

	s := &Store{}
	if s == nil {
		t.Fatal("Store should not be nil")
	}
}

type mockSearchStore struct{}

func (m *mockSearchStore) RebuildFTS(ctx context.Context) error { return nil }
