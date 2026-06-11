package domain

// Series is an ordered sequence of entries.
type Series struct {
	ID          string
	Name        string
	ProjectID   *string // nil means global
	Description string
	Vars        string // JSON array of expected variable names
	Active      bool
}

// SeriesEntry links an entry to a series with ordering.
type SeriesEntry struct {
	SeriesID string
	EntryID  string
	StepNum  int
	Label    string
	Required bool
	Notes    string
	Active   bool
}

// SeriesEntryInput is used when replacing series entries.
type SeriesEntryInput struct {
	EntryID  string
	StepNum  int
	Label    string
	Required bool
	Notes    string
}

// SeriesFilter is used to filter list operations on series.
type SeriesFilter struct {
	ProjectID       *string
	IncludeArchived bool
}

// SeriesResult is the full result of a GetSeries operation.
type SeriesResult struct {
	Series     Series
	Entries    []SeriesEntry
	TotalSteps int
}

// SeriesListResult is a lightweight result for listing series.
type SeriesListResult struct {
	Series     Series
	TotalSteps int
}
