package domain

type RelationType string

const (
	RelationReferences  RelationType = "references"
	RelationSupersedes  RelationType = "supersedes"
	RelationRelatedTo   RelationType = "related_to"
	RelationPartOf      RelationType = "part_of"
	RelationDerivedFrom RelationType = "derived_from"
	RelationImplements  RelationType = "implements"
)

func (rt RelationType) IsValid() bool {
	switch rt {
	case RelationReferences, RelationSupersedes, RelationRelatedTo,
		RelationPartOf, RelationDerivedFrom, RelationImplements:
		return true
	}
	return false
}

type EntryLink struct {
	FromEntryID  string
	ToEntryID    string
	RelationType RelationType
}
