package domain

import "testing"

func TestTagStruct(t *testing.T) {
	tag := Tag{
		ID:   "go",
		Name: "Go",
		Slug: "go",
	}

	if tag.ID != "go" {
		t.Errorf("ID = %q, want %q", tag.ID, "go")
	}
	if tag.Name != "Go" {
		t.Errorf("Name = %q, want %q", tag.Name, "Go")
	}
	if tag.Slug != "go" {
		t.Errorf("Slug = %q, want %q", tag.Slug, "go")
	}
}
