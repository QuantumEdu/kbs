package domain

// SearchQuery is used for FTS5 search operations.
type SearchQuery struct {
	Query           string
	ProjectID       *string
	SeriesID        *string
	Type            *string
	Tags            []string
	IncludeArchived bool
	Limit           int
}

// VaultExport is the top-level export structure.
type VaultExport struct {
	SchemaVersion int       `json:"schema_version"`
	AppVersion    string    `json:"app_version"`
	ExportedAt    string    `json:"exported_at"`
	Source        string    `json:"source"`
	Data          VaultData `json:"data"`
}

// VaultData contains all vault records for import/export.
type VaultData struct {
	Projects      []Project      `json:"projects"`
	Entries       []Entry        `json:"entries"`
	EntryTags     []EntryTag     `json:"entry_tags"`
	Series        []Series       `json:"series"`
	SeriesEntries []SeriesEntry  `json:"series_entries"`
	WorkflowSteps []WorkflowStep `json:"workflow_steps"`
}
