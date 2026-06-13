package domain

import "testing"

func TestRelationTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant RelationType
		expected string
	}{
		{"references", RelationReferences, "references"},
		{"supersedes", RelationSupersedes, "supersedes"},
		{"related_to", RelationRelatedTo, "related_to"},
		{"part_of", RelationPartOf, "part_of"},
		{"derived_from", RelationDerivedFrom, "derived_from"},
		{"implements", RelationImplements, "implements"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("RelationType %s = %q, want %q", tt.name, string(tt.constant), tt.expected)
			}
		})
	}
}

func TestRelationTypeValidation(t *testing.T) {
	all := []RelationType{
		RelationReferences, RelationSupersedes, RelationRelatedTo,
		RelationPartOf, RelationDerivedFrom, RelationImplements,
	}
	for _, rt := range all {
		if !rt.IsValid() {
			t.Errorf("RelationType %q should be valid", rt)
		}
	}
	if RelationType("invalid").IsValid() {
		t.Error("RelationType 'invalid' should not be valid")
	}
}

func TestEntryLinkStruct(t *testing.T) {
	el := EntryLink{
		FromEntryID:  "entry-1",
		ToEntryID:    "entry-2",
		RelationType: RelationReferences,
	}

	if el.FromEntryID != "entry-1" {
		t.Errorf("FromEntryID = %q, want %q", el.FromEntryID, "entry-1")
	}
	if el.ToEntryID != "entry-2" {
		t.Errorf("ToEntryID = %q, want %q", el.ToEntryID, "entry-2")
	}
	if el.RelationType != RelationReferences {
		t.Errorf("RelationType = %q, want %q", el.RelationType, RelationReferences)
	}
}
