package domain

import "time"

type ArtifactType string

const (
	ArtifactTypeMarkdown     ArtifactType = "markdown"
	ArtifactTypeJSON          ArtifactType = "json"
	ArtifactTypeTXT           ArtifactType = "txt"
	ArtifactTypeHTML          ArtifactType = "html"
	ArtifactTypePDFReference  ArtifactType = "pdf_reference"
	ArtifactTypeAIOutput      ArtifactType = "ai_output"
	ArtifactTypePDFAnalysis   ArtifactType = "pdf_analysis"
	ArtifactTypeSpec          ArtifactType = "spec"
	ArtifactTypeReport        ArtifactType = "report"
	ArtifactTypeSessionOutput ArtifactType = "session_output"
)

func (at ArtifactType) IsValid() bool {
	switch at {
	case ArtifactTypeMarkdown, ArtifactTypeJSON, ArtifactTypeTXT,
		ArtifactTypeHTML, ArtifactTypePDFReference, ArtifactTypeAIOutput,
		ArtifactTypePDFAnalysis, ArtifactTypeSpec, ArtifactTypeReport,
		ArtifactTypeSessionOutput:
		return true
	}
	return false
}

type Artifact struct {
	ID            string
	Title         string
	Slug          string
	Type          ArtifactType
	FilePath      string
	MimeType      string
	Summary       string
	ContentHash   string
	SizeBytes     int64
	ProjectID     *string
	SourceEntryID *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
