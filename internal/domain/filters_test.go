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

func TestEntryTagStruct(t *testing.T) {
	et := EntryTag{
		EntryID: "prd-fastapi",
		TagID:   "go",
	}

	if et.EntryID != "prd-fastapi" {
		t.Errorf("EntryID = %q, want 'prd-fastapi'", et.EntryID)
	}
	if et.TagID != "go" {
		t.Errorf("TagID = %q, want 'go'", et.TagID)
	}
}

func TestEntryResultStruct(t *testing.T) {
	r := EntryResult{
		Entry: Entry{
			ID:           "prd-fastapi",
			Title:        "FastAPI PRD",
			Type:         EntryTypeSkill,
			BodyOptional: "Design FastAPI backend",
			Status:       StatusActive,
		},
		Tags: []Tag{},
	}

	if r.Entry.ID != "prd-fastapi" {
		t.Errorf("Entry.ID = %q, want 'prd-fastapi'", r.Entry.ID)
	}
}

func TestSeriesResultStruct(t *testing.T) {
	r := SeriesResult{
		Series: Series{
			ID:     "sdd-cycle",
			Name:   "SDD Cycle",
			Status: StatusActive,
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
			ID:           "prd-fastapi",
			Title:        "FastAPI PRD",
			Type:         EntryTypeSkill,
			BodyOptional: "Design FastAPI backend",
			Status:       StatusActive,
		},
		Tags:       []Tag{},
		SeriesRefs: []SeriesRef{},
	}

	if r.Entry.ID != "prd-fastapi" {
		t.Errorf("Entry.ID = %q, want 'prd-fastapi'", r.Entry.ID)
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
}

func TestVaultExportStruct(t *testing.T) {
	e := VaultExport{
		SchemaVersion: 2,
		AppVersion:    "v3",
		ExportedAt:    "2026-06-10T18:30:00Z",
		Source:        "skillvault",
		Data:          VaultData{},
	}

	if e.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2", e.SchemaVersion)
	}
	if e.AppVersion != "v3" {
		t.Errorf("AppVersion = %q, want 'v3'", e.AppVersion)
	}
}

func TestVaultDataStruct(t *testing.T) {
	d := VaultData{
		Projects:       []Project{},
		Entries:        []Entry{},
		EntryTags:      []EntryTag{},
		Tags:           []Tag{},
		Series:         []Series{},
		SeriesEntries:  []SeriesEntry{},
		Workflows:      []Workflow{},
		WorkflowSteps:  []WorkflowStep{},
		WorkflowRuns:   []WorkflowRun{},
		WorkflowRunSteps: []WorkflowRunStep{},
		Artifacts:      []Artifact{},
		EntryLinks:     []EntryLink{},
	}

	if d.Projects == nil {
		t.Errorf("Projects should not be nil")
	}
	if d.Entries == nil {
		t.Errorf("Entries should not be nil")
	}
}

func TestVaultDataPipelineFields(t *testing.T) {
	d := VaultData{
		WorkflowRuns: []WorkflowRun{
			{ID: "run-001", WorkflowID: "wf-abc", Status: RunStatusCompleted, Input: "in", Output: "out"},
		},
		WorkflowRunSteps: []WorkflowRunStep{
			{ID: "rst-001", RunID: "run-001", StepID: 1, EntryID: "entry-abc", Status: RunStatusCompleted, Input: "in", Output: "out"},
		},
	}

	if len(d.WorkflowRuns) != 1 {
		t.Errorf("WorkflowRuns length = %d, want 1", len(d.WorkflowRuns))
	}
	if d.WorkflowRuns[0].ID != "run-001" {
		t.Errorf("WorkflowRuns[0].ID = %q, want 'run-001'", d.WorkflowRuns[0].ID)
	}

	if len(d.WorkflowRunSteps) != 1 {
		t.Errorf("WorkflowRunSteps length = %d, want 1", len(d.WorkflowRunSteps))
	}
	if d.WorkflowRunSteps[0].ID != "rst-001" {
		t.Errorf("WorkflowRunSteps[0].ID = %q, want 'rst-001'", d.WorkflowRunSteps[0].ID)
	}
}

func stringPtr(s string) *string {
	return &s
}
