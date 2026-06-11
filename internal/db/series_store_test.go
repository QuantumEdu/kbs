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

func TestUpsertSeriesCreate(t *testing.T) {
	_, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	series := domain.Series{
		ID:          "sdd-cycle",
		Name:        "SDD Cycle",
		Description: "Full SDD process",
		Active:      true,
	}
	err := sstore.UpsertSeries(ctx, series)
	if err != nil {
		t.Fatalf("UpsertSeries failed: %v", err)
	}

	result, err := sstore.GetSeries(ctx, "sdd-cycle", false)
	if err != nil {
		t.Fatalf("GetSeries failed: %v", err)
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

func TestSeriesWithProjectScope(t *testing.T) {
	_, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	projID := "vitacare"
	// Create project first for FK constraint
	pstore := &sqliteProjectStore{db: sstore.(*sqliteSeriesStore).db}
	if err := pstore.UpsertProject(ctx, domain.Project{ID: projID, Name: "VitaCare", Active: true}); err != nil {
		t.Fatalf("UpsertProject failed: %v", err)
	}

	series := domain.Series{
		ID:        "project-series",
		Name:      "Project Series",
		ProjectID: &projID,
		Active:    true,
	}
	if err := sstore.UpsertSeries(ctx, series); err != nil {
		t.Fatalf("UpsertSeries failed: %v", err)
	}

	result, err := sstore.GetSeries(ctx, "project-series", false)
	if err != nil {
		t.Fatalf("GetSeries failed: %v", err)
	}
	if result.Series.ProjectID == nil || *result.Series.ProjectID != "vitacare" {
		t.Errorf("ProjectID = %v, want 'vitacare'", result.Series.ProjectID)
	}
}

func TestReplaceSeriesEntriesRenumber(t *testing.T) {
	estore, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create entries first
	for _, e := range []domain.Entry{
		{ID: "e1", Name: "E1", Type: domain.EntryTypeSkill, Content: "C1", Active: true},
		{ID: "e2", Name: "E2", Type: domain.EntryTypePrompt, Content: "C2", Active: true},
		{ID: "e3", Name: "E3", Type: domain.EntryTypeContext, Content: "C3", Active: true},
	} {
		if err := estore.UpsertEntry(ctx, e, nil, nil); err != nil {
			t.Fatalf("UpsertEntry %s failed: %v", e.ID, err)
		}
	}

	// Create series
	series := domain.Series{ID: "s1", Name: "S1", Active: true}
	if err := sstore.UpsertSeries(ctx, series); err != nil {
		t.Fatalf("UpsertSeries failed: %v", err)
	}

	// Replace entries
	entries := []domain.SeriesEntryInput{
		{EntryID: "e1", Label: "Step A"},
		{EntryID: "e2", Label: "Step B"},
		{EntryID: "e3", Label: "Step C"},
	}
	if err := sstore.ReplaceSeriesEntries(ctx, "s1", entries); err != nil {
		t.Fatalf("ReplaceSeriesEntries failed: %v", err)
	}

	// Verify via GetSeries
	result, err := sstore.GetSeries(ctx, "s1", false)
	if err != nil {
		t.Fatalf("GetSeries failed: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
	if result.TotalSteps != 3 {
		t.Errorf("TotalSteps = %d, want 3", result.TotalSteps)
	}

	// Steps should be numbered 1,2,3
	for i, entry := range result.Entries {
		expectedStep := i + 1
		if entry.StepNum != expectedStep {
			t.Errorf("Entry %d StepNum = %d, want %d", i, entry.StepNum, expectedStep)
		}
	}

	// Replace with different count
	newEntries := []domain.SeriesEntryInput{
		{EntryID: "e3", Label: "Step Alpha"},
		{EntryID: "e1", Label: "Step Beta"},
	}
	if err := sstore.ReplaceSeriesEntries(ctx, "s1", newEntries); err != nil {
		t.Fatalf("second ReplaceSeriesEntries failed: %v", err)
	}

	result, err = sstore.GetSeries(ctx, "s1", false)
	if err != nil {
		t.Fatalf("GetSeries after replace failed: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries after replace, got %d", len(result.Entries))
	}
	if result.TotalSteps != 2 {
		t.Errorf("TotalSteps = %d, want 2", result.TotalSteps)
	}
	for i, entry := range result.Entries {
		if entry.StepNum != i+1 {
			t.Errorf("Entry %d StepNum = %d, want %d", i, entry.StepNum, i+1)
		}
	}
}

func TestListSeries(t *testing.T) {
	_, sstore, cleanup := setupSeriesStore(t)
	defer cleanup()
	ctx := context.Background()

	projID := "p1"
	// Create project first
	pstore := &sqliteProjectStore{db: sstore.(*sqliteSeriesStore).db}
	if err := pstore.UpsertProject(ctx, domain.Project{ID: projID, Name: "P1", Active: true}); err != nil {
		t.Fatalf("UpsertProject failed: %v", err)
	}
	for _, s := range []domain.Series{
		{ID: "s1", Name: "S1", Active: true},
		{ID: "s2", Name: "S2", ProjectID: &projID, Active: true},
		{ID: "s3", Name: "S3", Active: false},
	} {
		if err := sstore.UpsertSeries(ctx, s); err != nil {
			t.Fatalf("UpsertSeries %s failed: %v", s.ID, err)
		}
	}

	// List all active
	results, err := sstore.ListSeries(ctx, domain.SeriesFilter{})
	if err != nil {
		t.Fatalf("ListSeries failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 active series, got %d", len(results))
	}

	// List with include_archived
	results, err = sstore.ListSeries(ctx, domain.SeriesFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListSeries with include_archived failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 series with include_archived, got %d", len(results))
	}

	// List by project
	results, err = sstore.ListSeries(ctx, domain.SeriesFilter{ProjectID: &projID})
	if err != nil {
		t.Fatalf("ListSeries by project failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 series for project p1, got %d", len(results))
	}
}
