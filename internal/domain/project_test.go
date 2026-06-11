package domain

import "testing"

func TestProjectStruct(t *testing.T) {
	p := Project{
		ID:          "vitacare",
		Name:        "VitaCare",
		Description: "Healthcare platform",
		Active:      true,
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
	if !p.Active {
		t.Errorf("Active should default to true")
	}
}

func TestProjectInactive(t *testing.T) {
	p := Project{
		ID:     "archived-project",
		Name:   "Old Project",
		Active: false,
	}
	if p.Active {
		t.Errorf("Active should be false for archived project")
	}
}
