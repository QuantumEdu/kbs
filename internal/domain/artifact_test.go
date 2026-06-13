package domain

import "testing"

func TestArtifactTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant ArtifactType
		expected string
	}{
		{"markdown", ArtifactTypeMarkdown, "markdown"},
		{"json", ArtifactTypeJSON, "json"},
		{"txt", ArtifactTypeTXT, "txt"},
		{"html", ArtifactTypeHTML, "html"},
		{"pdf_reference", ArtifactTypePDFReference, "pdf_reference"},
		{"ai_output", ArtifactTypeAIOutput, "ai_output"},
		{"pdf_analysis", ArtifactTypePDFAnalysis, "pdf_analysis"},
		{"spec", ArtifactTypeSpec, "spec"},
		{"report", ArtifactTypeReport, "report"},
		{"session_output", ArtifactTypeSessionOutput, "session_output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("ArtifactType %s = %q, want %q", tt.name, string(tt.constant), tt.expected)
			}
		})
	}
}

func TestArtifactTypeValidation(t *testing.T) {
	all := []ArtifactType{
		ArtifactTypeMarkdown, ArtifactTypeJSON, ArtifactTypeTXT,
		ArtifactTypeHTML, ArtifactTypePDFReference, ArtifactTypeAIOutput,
		ArtifactTypePDFAnalysis, ArtifactTypeSpec, ArtifactTypeReport,
		ArtifactTypeSessionOutput,
	}
	for _, at := range all {
		if !at.IsValid() {
			t.Errorf("ArtifactType %q should be valid", at)
		}
	}
	if ArtifactType("invalid").IsValid() {
		t.Error("ArtifactType 'invalid' should not be valid")
	}
}

func TestArtifactStruct(t *testing.T) {
	a := Artifact{
		ID:          "art-1",
		Title:       "PDF Analysis Report",
		Slug:        "pdf-analysis-report",
		Type:        ArtifactTypePDFAnalysis,
		FilePath:    "objects/2026/06/analysis.md",
		MimeType:    "text/markdown",
		Summary:     "Analysis of forensic document",
		ContentHash: "abc123def456",
		SizeBytes:   1024,
	}

	if a.ID != "art-1" {
		t.Errorf("ID = %q, want %q", a.ID, "art-1")
	}
	if a.Type != ArtifactTypePDFAnalysis {
		t.Errorf("Type = %q, want %q", a.Type, ArtifactTypePDFAnalysis)
	}
	if a.ContentHash != "abc123def456" {
		t.Errorf("ContentHash = %q, want %q", a.ContentHash, "abc123def456")
	}
	if a.ProjectID != nil {
		t.Errorf("ProjectID should be nil")
	}
}
