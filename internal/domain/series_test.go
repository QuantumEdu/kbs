package domain

import "testing"

func TestSeriesStruct(t *testing.T) {
	projectID := "vitacare"
	s := Series{
		ID:          "sdd-cycle",
		Name:        "SDD Cycle",
		ProjectID:   &projectID,
		Description: "Full SDD workflow",
		Active:      true,
	}

	if s.ID != "sdd-cycle" {
		t.Errorf("ID = %q, want %q", s.ID, "sdd-cycle")
	}
	if s.Name != "SDD Cycle" {
		t.Errorf("Name = %q, want %q", s.Name, "SDD Cycle")
	}
	if s.ProjectID == nil || *s.ProjectID != "vitacare" {
		t.Errorf("ProjectID should be 'vitacare', got %v", s.ProjectID)
	}
	if !s.Active {
		t.Errorf("Active should default to true")
	}
}

func TestGlobalSeries(t *testing.T) {
	s := Series{
		ID:     "global-series",
		Name:   "Global Series",
		Active: true,
	}
	if s.ProjectID != nil {
		t.Errorf("ProjectID should be nil for global series")
	}
}

func TestSeriesEntryStruct(t *testing.T) {
	se := SeriesEntry{
		SeriesID: "sdd-cycle",
		EntryID:  "prd-fastapi",
		StepNum:  3,
		Label:    "Generate PRD",
		Required: true,
		Active:   true,
	}

	if se.SeriesID != "sdd-cycle" {
		t.Errorf("SeriesID = %q, want %q", se.SeriesID, "sdd-cycle")
	}
	if se.EntryID != "prd-fastapi" {
		t.Errorf("EntryID = %q, want %q", se.EntryID, "prd-fastapi")
	}
	if se.StepNum != 3 {
		t.Errorf("StepNum = %d, want 3", se.StepNum)
	}
	if se.Label != "Generate PRD" {
		t.Errorf("Label = %q, want %q", se.Label, "Generate PRD")
	}
	if !se.Required {
		t.Errorf("Required should default to true")
	}
}
