package domain

// EntryType represents the type of a vault entry.
type EntryType string

const (
	EntryTypeSkill    EntryType = "skill"
	EntryTypeAgent    EntryType = "agent"
	EntryTypeWorkflow EntryType = "workflow"
	EntryTypePrompt   EntryType = "prompt"
	EntryTypeContext  EntryType = "context"
	EntryTypeNote     EntryType = "note"
)

// IsValid checks if the EntryType is one of the defined types.
func (et EntryType) IsValid() bool {
	switch et {
	case EntryTypeSkill, EntryTypeAgent, EntryTypeWorkflow,
		EntryTypePrompt, EntryTypeContext, EntryTypeNote:
		return true
	}
	return false
}

// Entry is the core unit of knowledge in the vault.
type Entry struct {
	ID          string
	Name        string
	Type        EntryType
	ProjectID   *string // nil means global
	Description string
	Content     string
	Vars        string // JSON array of variable names
	Active      bool
}

// EntryTag links an entry to a normalized tag.
type EntryTag struct {
	EntryID string
	Tag     string
}

// EntryFilter is used to filter list operations on entries.
type EntryFilter struct {
	ProjectID       *string
	Type            *string
	Tags            []string
	IncludeArchived bool
}

// EntryResult is the full result of a GetEntry operation.
type EntryResult struct {
	Entry Entry
	Tags  []string
	Steps []WorkflowStep
}

// EntryListResult is a lightweight result for listing entries.
type EntryListResult struct {
	Entry Entry
	Tags  []string
}

// EntrySearchResult is the result of a search operation.
type EntrySearchResult struct {
	Entry      Entry
	Tags       []string
	SeriesRefs []SeriesRef
}

// SeriesRef is a lightweight reference to a series that contains an entry.
type SeriesRef struct {
	SeriesID   string
	SeriesName string
	StepNum    int
	TotalSteps int
	Label      string
}
