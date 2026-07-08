package domain

type SearchQuery struct {
	Query           string
	ProjectID       *string
	SeriesID        *string
	Type            *string
	Purpose         *string
	Tags            []string
	IncludeArchived bool
	Limit           int
	Vector          bool
}

type VaultExport struct {
	SchemaVersion int       `json:"schema_version"`
	AppVersion    string    `json:"app_version"`
	ExportedAt    string    `json:"exported_at"`
	Source        string    `json:"source"`
	Data          VaultData `json:"data"`
}

type VaultData struct {
	Projects         []Project         `json:"projects"`
	Entries          []Entry           `json:"entries"`
	EntryTags        []EntryTag        `json:"entry_tags"`
	Tags             []Tag             `json:"tags"`
	Series           []Series          `json:"series"`
	SeriesEntries    []SeriesEntry     `json:"series_entries"`
	Workflows        []Workflow        `json:"workflows"`
	WorkflowSteps    []WorkflowStep    `json:"workflow_steps"`
	WorkflowRuns     []WorkflowRun     `json:"workflow_runs"`
	WorkflowRunSteps []WorkflowRunStep `json:"workflow_run_steps"`
	Artifacts        []Artifact        `json:"artifacts"`
	EntryLinks       []EntryLink       `json:"entry_links"`
}
