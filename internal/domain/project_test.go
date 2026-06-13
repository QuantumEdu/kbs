package domain

import "testing"

func TestProjectStruct(t *testing.T) {
	p := Project{
		ID:          "vitacare",
		Name:        "VitaCare",
		Slug:        "vitacare",
		Description: "Healthcare platform",
		Status:      StatusActive,
	}

	if p.ID != "vitacare" {
		t.Errorf("ID = %q, want %q", p.ID, "vitacare")
	}
	if p.Name != "VitaCare" {
		t.Errorf("Name = %q, want %q", p.Name, "VitaCare")
	}
	if p.Description != "Healthcare platform" {
		t.Errorf("Description = %q, want %q", p.Description, "Healthcare platform")
	}
	if p.Status != StatusActive {
		t.Errorf("Status = %q, want %q", p.Status, StatusActive)
	}
}

func TestProjectArchived(t *testing.T) {
	p := Project{
		ID:     "archived-project",
		Name:   "Old Project",
		Status: StatusArchived,
	}
	if p.Status != StatusArchived {
		t.Errorf("Status = %q, want %q", p.Status, StatusArchived)
	}
}
