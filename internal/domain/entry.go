package domain

import "time"

type Purpose string

const (
	PurposeWork         Purpose = "WORK"
	PurposeKnowledge    Purpose = "KNOWLEDGE"
	PurposeLearning     Purpose = "LEARNING"
	PurposeRelationship Purpose = "RELATIONSHIP"
	PurposeState        Purpose = "STATE"
	PurposeObservability Purpose = "OBSERVABILITY"
)

func (p Purpose) IsValid() bool {
	switch p {
	case PurposeWork, PurposeKnowledge, PurposeLearning,
		PurposeRelationship, PurposeState, PurposeObservability, "":
		return true
	}
	return false
}

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
	EntryTypeRouting         EntryType = "routing"
)

func (et EntryType) IsValid() bool {
	switch et {
	case EntryTypePrompt, EntryTypeSkill, EntryTypeWorkflowNote,
		EntryTypeReference, EntryTypeUser, EntryTypeFeedback,
		EntryTypeProjectState, EntryTypeSession, EntryTypeDecision,
		EntryTypeArtifactSummary, EntryTypeHandoff, EntryTypeRouting:
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
	Purpose      Purpose
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
