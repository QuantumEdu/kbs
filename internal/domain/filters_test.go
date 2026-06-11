package domain

import "testing"

func TestEntryFilterStruct(t *testing.T) {
	f := EntryFilter{
		ProjectID:       stringPtr("vitacare"),
		Type:            stringPtr("skill"),
		IncludeArchived: false,
	}

	if f.ProjectID == nil || *f.ProjectID != "vitacare" {
		t.Errorf("ProjectID = %v, want 'vitacare'", f.ProjectID)
	}
	if f.Type == nil || *f.Type != "skill" {
		t.Errorf("Type = %v, want 'skill'", f.Type)
	}
	if f.IncludeArchived {
		t.Errorf("IncludeArchived should default to false")
	}
}

func TestSearchQueryStruct(t *testing.T) {
	q := SearchQuery{
		Query:           "fastapi",
		ProjectID:       stringPtr("vitacare"),
		Type:            stringPtr("skill"),
		Tags:            []string{"go", "cli"},
		IncludeArchived: false,
		Limit:           20,
	}

	if q.Query != "fastapi" {
		t.Errorf("Query = %q, want %q", q.Query, "fastapi")
	}
	if q.Limit != 20 {
		t.Errorf("Limit = %d, want 20", q.Limit)
	}
	if len(q.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(q.Tags))
	}
}

func TestSeriesFilterStruct(t *testing.T) {
	f := SeriesFilter{
		ProjectID:       stringPtr("vitacare"),
		IncludeArchived: true,
	}

	if f.ProjectID == nil || *f.ProjectID != "vitacare" {
		t.Errorf("ProjectID = %v, want 'vitacare'", f.ProjectID)
	}
	if !f.IncludeArchived {
		t.Errorf("IncludeArchived should be true")
	}
}

func TestEntryResultStruct(t *testing.T) {
	r := EntryResult{
		Entry: Entry{
			ID:      "prd-fastapi",
			Name:    "FastAPI PRD",
			Type:    EntryTypeSkill,
			Content: "Design FastAPI backend",
			Active:  true,
		},
		Tags:  []string{"go", "api"},
		Steps: []WorkflowStep{},
	}

	if r.Entry.ID != "prd-fastapi" {
		t.Errorf("Entry.ID = %q, want 'prd-fastapi'", r.Entry.ID)
	}
	if len(r.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(r.Tags))
	}
}

func TestSeriesResultStruct(t *testing.T) {
	r := SeriesResult{
		Series: Series{
			ID:     "sdd-cycle",
			Name:   "SDD Cycle",
			Active: true,
		},
		Entries:    []SeriesEntry{},
		TotalSteps: 0,
	}

	if r.Series.ID != "sdd-cycle" {
		t.Errorf("Series.ID = %q, want 'sdd-cycle'", r.Series.ID)
	}
}

func TestEntrySearchResultStruct(t *testing.T) {
	r := EntrySearchResult{
		Entry: Entry{
			ID:      "prd-fastapi",
			Name:    "FastAPI PRD",
			Type:    EntryTypeSkill,
			Content: "Design FastAPI backend",
			Active:  true,
		},
		Tags:        []string{"go", "api"},
		SeriesRefs:  []SeriesRef{},
	}

	if r.Entry.ID != "prd-fastapi" {
		t.Errorf("Entry.ID = %q, want 'prd-fastapi'", r.Entry.ID)
	}
	if len(r.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(r.Tags))
	}
}

func TestSeriesRefStruct(t *testing.T) {
	ref := SeriesRef{
		SeriesID:   "sdd-cycle",
		SeriesName: "SDD Cycle",
		StepNum:    3,
		TotalSteps: 6,
		Label:      "Generate PRD",
	}

	if ref.SeriesID != "sdd-cycle" {
		t.Errorf("SeriesID = %q, want 'sdd-cycle'", ref.SeriesID)
	}
	if ref.StepNum != 3 {
		t.Errorf("StepNum = %d, want 3", ref.StepNum)
	}
	if ref.TotalSteps != 6 {
		t.Errorf("TotalSteps = %d, want 6", ref.TotalSteps)
	}
}

func TestVaultExportStruct(t *testing.T) {
	e := VaultExport{
		SchemaVersion: 1,
		AppVersion:    "v1-alpha",
		ExportedAt:    "2026-06-10T18:30:00Z",
		Source:        "skillvault",
		Data:          VaultData{},
	}

	if e.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", e.SchemaVersion)
	}
	if e.AppVersion != "v1-alpha" {
		t.Errorf("AppVersion = %q, want 'v1-alpha'", e.AppVersion)
	}
}

func TestVaultDataStruct(t *testing.T) {
	d := VaultData{
		Projects:       []Project{},
		Entries:        []Entry{},
		EntryTags:      []EntryTag{},
		Series:         []Series{},
		SeriesEntries:  []SeriesEntry{},
		WorkflowSteps:  []WorkflowStep{},
	}

	if d.Projects == nil {
		t.Errorf("Projects should not be nil")
	}
	if d.Entries == nil {
		t.Errorf("Entries should not be nil")
	}
}

func TestEntryTagStruct(t *testing.T) {
	et := EntryTag{
		EntryID: "prd-fastapi",
		Tag:     "go",
	}

	if et.EntryID != "prd-fastapi" {
		t.Errorf("EntryID = %q, want 'prd-fastapi'", et.EntryID)
	}
	if et.Tag != "go" {
		t.Errorf("Tag = %q, want 'go'", et.Tag)
	}
}

func TestSeriesEntryInputStruct(t *testing.T) {
	sei := SeriesEntryInput{
		EntryID:  "prd-fastapi",
		StepNum:  3,
		Label:    "Generate PRD",
		Required: true,
		Notes:    "Use for new projects",
	}

	if sei.EntryID != "prd-fastapi" {
		t.Errorf("EntryID = %q, want 'prd-fastapi'", sei.EntryID)
	}
	if sei.StepNum != 3 {
		t.Errorf("StepNum = %d, want 3", sei.StepNum)
	}
}

func stringPtr(s string) *string {
	return &s
}
