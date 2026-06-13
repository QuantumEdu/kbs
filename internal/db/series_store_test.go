package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupSeriesStore(t *testing.T) (EntryStore, SeriesStore, func()) {
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
	seriesStore := &sqliteSeriesStore{db: db}
	cleanup := func() { db.Close() }
	return entryStore, seriesStore, cleanup
}

func TestSaveSeriesCreate(t *testing.T) {
	_, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	series := domain.Series{
		ID:          "sdd-cycle",
		Name:        "SDD Cycle",
		Slug:        "sdd-cycle",
		Description: "Full SDD process",
		Status:      domain.StatusActive,
	}
	err := sstore.Save(ctx, series)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	result, err := sstore.Get(ctx, "sdd-cycle", false)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result.Series.ID != "sdd-cycle" {
		t.Errorf("ID = %q, want 'sdd-cycle'", result.Series.ID)
	}
	if result.Series.Name != "SDD Cycle" {
		t.Errorf("Name = %q, want 'SDD Cycle'", result.Series.Name)
	}
	if result.TotalSteps != 0 {
		t.Errorf("TotalSteps = %d, want 0", result.TotalSteps)
	}
}

func TestReplaceSeriesEntriesRenumber(t *testing.T) {
	estore, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, e := range []domain.Entry{
		{ID: "e1", Title: "E1", Slug: "e1", Type: domain.EntryTypeSkill, BodyOptional: "C1", Status: domain.StatusActive},
		{ID: "e2", Title: "E2", Slug: "e2", Type: domain.EntryTypePrompt, BodyOptional: "C2", Status: domain.StatusActive},
		{ID: "e3", Title: "E3", Slug: "e3", Type: domain.EntryTypeReference, BodyOptional: "C3", Status: domain.StatusActive},
	} {
		if err := estore.Save(ctx, e, nil); err != nil {
			t.Fatalf("Save %s failed: %v", e.ID, err)
		}
	}

	series := domain.Series{ID: "s1", Name: "S1", Slug: "s1", Status: domain.StatusActive}
	if err := sstore.Save(ctx, series); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	entries := []domain.SeriesEntry{
		{EntryID: "e1"},
		{EntryID: "e2"},
		{EntryID: "e3"},
	}
	if err := sstore.ReplaceSeriesEntries(ctx, "s1", entries); err != nil {
		t.Fatalf("ReplaceSeriesEntries failed: %v", err)
	}

	result, err := sstore.Get(ctx, "s1", false)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
	if result.TotalSteps != 3 {
		t.Errorf("TotalSteps = %d, want 3", result.TotalSteps)
	}

	for i, entry := range result.Entries {
		expectedOrder := i + 1
		if entry.OrderIndex != expectedOrder {
			t.Errorf("Entry %d OrderIndex = %d, want %d", i, entry.OrderIndex, expectedOrder)
		}
	}

	newEntries := []domain.SeriesEntry{
		{EntryID: "e3"},
		{EntryID: "e1"},
	}
	if err := sstore.ReplaceSeriesEntries(ctx, "s1", newEntries); err != nil {
		t.Fatalf("second ReplaceSeriesEntries failed: %v", err)
	}

	result, err = sstore.Get(ctx, "s1", false)
	if err != nil {
		t.Fatalf("Get after replace failed: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries after replace, got %d", len(result.Entries))
	}
	if result.TotalSteps != 2 {
		t.Errorf("TotalSteps = %d, want 2", result.TotalSteps)
	}
	for i, entry := range result.Entries {
		if entry.OrderIndex != i+1 {
			t.Errorf("Entry %d OrderIndex = %d, want %d", i, entry.OrderIndex, i+1)
		}
	}
}

func TestListSeries(t *testing.T) {
	_, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, s := range []domain.Series{
		{ID: "s1", Name: "S1", Slug: "s1", Status: domain.StatusActive},
		{ID: "s2", Name: "S2", Slug: "s2", Status: domain.StatusActive},
		{ID: "s3", Name: "S3", Slug: "s3", Status: domain.StatusArchived},
	} {
		if err := sstore.Save(ctx, s); err != nil {
			t.Fatalf("Save %s failed: %v", s.ID, err)
		}
	}

	results, err := sstore.List(ctx, domain.SeriesFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 active series, got %d", len(results))
	}

	results, err = sstore.List(ctx, domain.SeriesFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("List with include_archived failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 series with include_archived, got %d", len(results))
	}
}

func TestComposeSeries(t *testing.T) {
	estore, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, e := range []domain.Entry{
		{ID: "e1", Title: "First", Slug: "first", Type: domain.EntryTypeSkill, BodyOptional: "C1", Status: domain.StatusActive},
		{ID: "e2", Title: "Second", Slug: "second", Type: domain.EntryTypePrompt, BodyOptional: "C2", Status: domain.StatusActive},
		{ID: "e3", Title: "Third", Slug: "third", Type: domain.EntryTypeReference, BodyOptional: "C3", Status: domain.StatusActive},
	} {
		if err := estore.Save(ctx, e, nil); err != nil {
			t.Fatalf("Save %s failed: %v", e.ID, err)
		}
	}

	series := domain.Series{ID: "s1", Name: "S1", Slug: "s1", Status: domain.StatusActive}
	if err := sstore.Save(ctx, series); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := sstore.ReplaceSeriesEntries(ctx, "s1", []domain.SeriesEntry{
		{EntryID: "e1"}, {EntryID: "e2"}, {EntryID: "e3"},
	}); err != nil {
		t.Fatalf("ReplaceSeriesEntries failed: %v", err)
	}

	entries, err := sstore.Compose(ctx, "s1")
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].ID != "e1" {
		t.Errorf("entries[0].ID = %q, want 'e1'", entries[0].ID)
	}
	if entries[1].ID != "e2" {
		t.Errorf("entries[1].ID = %q, want 'e2'", entries[1].ID)
	}
	if entries[2].ID != "e3" {
		t.Errorf("entries[2].ID = %q, want 'e3'", entries[2].ID)
	}
}
