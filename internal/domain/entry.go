package domain

import "time"

type EntryType string

const (
	EntryTypePrompt          EntryType = "prompt"
	EntryTypeSkill           EntryType = "skill"
	EntryTypeWorkflowNote    EntryType = "workflow_note"
	EntryTypeReference       EntryType = "reference"
	EntryTypeUser            EntryType = "user"
	EntryTypeFeedback        EntryType = "feedback"
	EntryTypeProjectState    EntryType = "project_state"
	EntryTypeSession         EntryType = "session"
	EntryTypeDecision        EntryType = "decision"
	EntryTypeArtifactSummary EntryType = "artifact_summary"
	EntryTypeHandoff         EntryType = "handoff"
)

func (et EntryType) IsValid() bool {
	switch et {
	case EntryTypePrompt, EntryTypeSkill, EntryTypeWorkflowNote,
		EntryTypeReference, EntryTypeUser, EntryTypeFeedback,
		EntryTypeProjectState, EntryTypeSession, EntryTypeDecision,
		EntryTypeArtifactSummary, EntryTypeHandoff:
		return true
	}
	return false
}

type Status string

const (
	StatusDraft      Status = "draft"
	StatusActive     Status = "active"
	StatusArchived   Status = "archived"
	StatusDeprecated Status = "deprecated"
	StatusCanonical  Status = "canonical"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusDraft, StatusActive, StatusArchived, StatusDeprecated, StatusCanonical:
		return true
	}
	return false
}

type Entry struct {
	ID           string
	Title        string
	Slug         string
	Type         EntryType
	Summary      string
	BodyOptional string
	Status       Status
	ProjectID    *string
	ArtifactID   *string
	ExternalRef  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type EntryTag struct {
	EntryID string
	TagID   string
}

type EntryFilter struct {
	ProjectID       *string
	Type            *string
	Tags            []string
	IncludeArchived bool
}

type EntryResult struct {
	Entry Entry
	Tags  []Tag
}

type EntryListResult struct {
	Entry Entry
	Tags  []Tag
}

type EntrySearchResult struct {
	Entry      Entry
	Tags       []Tag
	SeriesRefs []SeriesRef
}

type SeriesRef struct {
	SeriesID   string
	SeriesName string
	StepNum    int
	TotalSteps int
	Label      string
}

// EntryVersion represents a historical snapshot of an entry's content.
type EntryVersion struct {
	VersionID     string
	EntryID       string
	VersionNumber int
	Title         string
	Summary       string
	BodyOptional  string
	SavedAt       time.Time
}
