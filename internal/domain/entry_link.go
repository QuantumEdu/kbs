package domain

import "time"

type RelationType string

const (
	RelationReferences    RelationType = "references"
	RelationSupersedes    RelationType = "supersedes"
	RelationRelatedTo     RelationType = "related_to"
	RelationPartOf        RelationType = "part_of"
	RelationDerivedFrom   RelationType = "derived_from"
	RelationImplements    RelationType = "implements"
	RelationUses          RelationType = "uses"
	RelationExtends       RelationType = "extends"
	RelationHandoffOf     RelationType = "handoff_of"
	RelationGeneratedFrom RelationType = "generated_from"
	RelationDependsOn     RelationType = "depends_on"
)

// CycleProne returns true if this relation type can create cycles and
// requires cycle detection before insertion.
func (rt RelationType) CycleProne() bool {
	switch rt {
	case RelationDependsOn, RelationPartOf, RelationSupersedes:
		return true
	}
	return false
}

func (rt RelationType) IsValid() bool {
	switch rt {
	case RelationReferences, RelationSupersedes, RelationRelatedTo,
		RelationPartOf, RelationDerivedFrom, RelationImplements,
		RelationUses, RelationExtends, RelationHandoffOf,
		RelationGeneratedFrom, RelationDependsOn:
		return true
	}
	return false
}

type EntryLink struct {
	FromEntryID  string
	ToEntryID    string
	RelationType RelationType
	Label        string
	Active       bool
	CreatedAt    time.Time
}
