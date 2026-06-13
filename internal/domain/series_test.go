package domain

import "testing"

func TestSeriesStruct(t *testing.T) {
	s := Series{
		ID:          "sdd-cycle",
		Name:        "SDD Cycle",
		Slug:        "sdd-cycle",
		Description: "Full SDD workflow",
		Status:      StatusActive,
	}

	if s.ID != "sdd-cycle" {
		t.Errorf("ID = %q, want %q", s.ID, "sdd-cycle")
	}
	if s.Name != "SDD Cycle" {
		t.Errorf("Name = %q, want %q", s.Name, "SDD Cycle")
	}
	if s.Description != "Full SDD workflow" {
		t.Errorf("Description = %q, want %q", s.Description, "Full SDD workflow")
	}
	if s.Status != StatusActive {
		t.Errorf("Status = %q, want %q", s.Status, StatusActive)
	}
}

func TestSeriesEntryStruct(t *testing.T) {
	se := SeriesEntry{
		SeriesID:   "sdd-cycle",
		EntryID:    "prd-fastapi",
		OrderIndex: 3,
	}

	if se.SeriesID != "sdd-cycle" {
		t.Errorf("SeriesID = %q, want %q", se.SeriesID, "sdd-cycle")
	}
	if se.EntryID != "prd-fastapi" {
		t.Errorf("EntryID = %q, want %q", se.EntryID, "prd-fastapi")
	}
	if se.OrderIndex != 3 {
		t.Errorf("OrderIndex = %d, want 3", se.OrderIndex)
	}
}
