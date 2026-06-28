package domain

type SearchQuery struct {
	Query           string
	ProjectID       *string
	SeriesID        *string
	Type            *string
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

// VaultPackExport wraps a VaultExport with pack-level metadata for portable sharing.
type VaultPackExport struct {
	Pack PackMetadata `json:"pack"`
	Data VaultExport  `json:"data"`
}

// PackMetadata carries authorship and versioning information for a skill pack.
type PackMetadata struct {
	PackID      string `json:"pack_id"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Description string `json:"description"`
	ExportedAt  string `json:"exported_at"`
	Source      string `json:"source"`
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
