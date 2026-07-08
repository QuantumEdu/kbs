package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"trims whitespace", []string{" Go ", " CLI "}, []string{"go", "cli"}},
		{"lowercases", []string{"Go", "CLI", "Api"}, []string{"go", "cli", "api"}},
		{"spaces to dashes", []string{"cli tool", "clean architecture"}, []string{"cli-tool", "clean-architecture"}},
		{"rejects empty strings", []string{"Go", "", "cli", "  "}, []string{"go", "cli"}},
		{"deduplicates", []string{"go", "Go", "GO", "cli", "CLI"}, []string{"go", "cli"}},
		{"handles nil input", nil, nil},
		{"handles empty input", []string{}, []string{}},
		{"complex mixed case", []string{" Go ", "Go", "", "cli-tool", "CLI TOOL", "  "}, []string{"go", "cli-tool"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTags(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("NormalizeTags(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidateEntryType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid prompt", "prompt", true},
		{"valid skill", "skill", true},
		{"valid workflow_note", "workflow_note", true},
		{"valid reference", "reference", true},
		{"valid user", "user", true},
		{"valid feedback", "feedback", true},
		{"valid project_state", "project_state", true},
		{"valid session", "session", true},
		{"valid decision", "decision", true},
		{"valid artifact_summary", "artifact_summary", true},
		{"valid handoff", "handoff", true},
		{"valid routing", "routing", true},
		{"invalid empty", "", false},
		{"invalid random", "unknown", false},
		{"invalid old type", "agent", false},
		{"invalid case", "Skill", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEntryType(tt.input)
			if tt.wantValid && err != nil {
				t.Errorf("ValidateEntryType(%q) = %v, want nil", tt.input, err)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("ValidateEntryType(%q) = nil, want error", tt.input)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid draft", "draft", true},
		{"valid active", "active", true},
		{"valid archived", "archived", true},
		{"valid deprecated", "deprecated", true},
		{"valid canonical", "canonical", true},
		{"invalid empty", "", false},
		{"invalid random", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatus(tt.input)
			if tt.wantValid && err != nil {
				t.Errorf("ValidateStatus(%q) = %v, want nil", tt.input, err)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("ValidateStatus(%q) = nil, want error", tt.input)
			}
		})
	}
}

func TestValidateArtifactType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid markdown", "markdown", true},
		{"valid json", "json", true},
		{"valid txt", "txt", true},
		{"valid html", "html", true},
		{"valid pdf_reference", "pdf_reference", true},
		{"valid ai_output", "ai_output", true},
		{"valid pdf_analysis", "pdf_analysis", true},
		{"valid spec", "spec", true},
		{"valid report", "report", true},
		{"valid session_output", "session_output", true},
		{"invalid empty", "", false},
		{"invalid random", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArtifactType(tt.input)
			if tt.wantValid && err != nil {
				t.Errorf("ValidateArtifactType(%q) = %v, want nil", tt.input, err)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("ValidateArtifactType(%q) = nil, want error", tt.input)
			}
		})
	}
}

func TestValidateRelationType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid references", "references", true},
		{"valid supersedes", "supersedes", true},
		{"valid related_to", "related_to", true},
		{"valid part_of", "part_of", true},
		{"valid derived_from", "derived_from", true},
		{"valid implements", "implements", true},
		{"invalid empty", "", false},
		{"invalid random", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRelationType(tt.input)
			if tt.wantValid && err != nil {
				t.Errorf("ValidateRelationType(%q) = %v, want nil", tt.input, err)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("ValidateRelationType(%q) = nil, want error", tt.input)
			}
		})
	}
}

func TestValidateEntryTypeErrorMessage(t *testing.T) {
	err := ValidateEntryType("bogus")
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	msg := err.Error()
	if !strings.Contains(msg, "handoff") {
		t.Errorf("error message missing 'handoff': %s", msg)
	}
	if !strings.Contains(msg, "routing") {
		t.Errorf("error message missing 'routing': %s", msg)
	}
}

func TestValidateSearchQuery(t *testing.T) {
	err := ValidateSearchQuery(SearchQuery{Query: "fastapi"})
	if err != nil {
		t.Errorf("ValidateSearchQuery(query:fastapi) = %v, want nil", err)
	}

	err = ValidateSearchQuery(SearchQuery{})
	if err != nil {
		t.Errorf("ValidateSearchQuery(empty) = %v, want nil", err)
	}

	err = ValidateSearchQuery(SearchQuery{Query: "x", Type: strPtr("skill")})
	if err != nil {
		t.Errorf("ValidateSearchQuery(type:skill) = %v, want nil", err)
	}

	err = ValidateSearchQuery(SearchQuery{Query: "x", Type: strPtr("invalid")})
	if err == nil {
		t.Errorf("ValidateSearchQuery(type:invalid) = nil, want error")
	}

	err = ValidateSearchQuery(SearchQuery{Limit: -1})
	if err == nil {
		t.Errorf("ValidateSearchQuery(limit:-1) = nil, want error")
	}
}

func TestValidateSeriesScope(t *testing.T) {
	t.Run("global series with global entry", func(t *testing.T) {
		err := ValidateSeriesScope(nil, nil)
		if err != nil {
			t.Errorf("global series + global entry should be valid: %v", err)
		}
	})

	t.Run("global series with project entry", func(t *testing.T) {
		projID := "vitacare"
		err := ValidateSeriesScope(nil, &projID)
		if err == nil {
			t.Errorf("global series + project entry should be rejected")
		}
	})

	t.Run("project series with global entry", func(t *testing.T) {
		projID := "vitacare"
		err := ValidateSeriesScope(&projID, nil)
		if err != nil {
			t.Errorf("project series + global entry should be valid: %v", err)
		}
	})

	t.Run("project series with same project entry", func(t *testing.T) {
		projID := "vitacare"
		err := ValidateSeriesScope(&projID, &projID)
		if err != nil {
			t.Errorf("same project should be valid: %v", err)
		}
	})

	t.Run("project series with different project entry", func(t *testing.T) {
		seriesProj := "vitacare"
		entryProj := "other-proj"
		err := ValidateSeriesScope(&seriesProj, &entryProj)
		if err == nil {
			t.Errorf("different project should be rejected")
		}
	})
}

func TestPurpose_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		purpose   Purpose
		wantValid bool
	}{
		{"valid WORK", PurposeWork, true},
		{"valid KNOWLEDGE", PurposeKnowledge, true},
		{"valid LEARNING", PurposeLearning, true},
		{"valid RELATIONSHIP", PurposeRelationship, true},
		{"valid STATE", PurposeState, true},
		{"valid empty (unset)", Purpose(""), true},
		{"invalid random", Purpose("INVALID"), false},
		{"invalid lowercase", Purpose("work"), false},
		{"invalid whitespace", Purpose(" "), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.purpose.IsValid()
			if got != tt.wantValid {
				t.Errorf("Purpose(%q).IsValid() = %v, want %v", tt.purpose, got, tt.wantValid)
			}
		})
	}
}

func TestValidatePurpose(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
		errMsg    string
	}{
		{"valid WORK", "WORK", true, ""},
		{"valid KNOWLEDGE", "KNOWLEDGE", true, ""},
		{"valid LEARNING", "LEARNING", true, ""},
		{"valid RELATIONSHIP", "RELATIONSHIP", true, ""},
		{"valid STATE", "STATE", true, ""},
		{"valid empty string", "", true, ""},
		{"invalid random", "INVALID_VALUE", false, "INVALID_VALUE"},
		{"invalid lowercase", "work", false, "work"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePurpose(tt.input)
			if tt.wantValid && err != nil {
				t.Errorf("ValidatePurpose(%q) = %v, want nil", tt.input, err)
			}
			if !tt.wantValid {
				if err == nil {
					t.Errorf("ValidatePurpose(%q) = nil, want error", tt.input)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidatePurpose(%q) error = %q, want it to contain %q", tt.input, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
