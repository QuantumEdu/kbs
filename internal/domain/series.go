package domain

import "time"

type Series struct {
	ID          string
	Name        string
	Slug        string
	Description string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SeriesEntry struct {
	SeriesID   string
	EntryID    string
	OrderIndex int
}

type SeriesFilter struct {
	ProjectID       *string
	IncludeArchived bool
}

type SeriesResult struct {
	Series     Series
	Entries    []SeriesEntry
	TotalSteps int
}

type SeriesListResult struct {
	Series     Series
	TotalSteps int
}
