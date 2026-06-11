package domain

import (
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "trims whitespace",
			input:    []string{" Go ", " CLI "},
			expected: []string{"go", "cli"},
		},
		{
			name:     "lowercases",
			input:    []string{"Go", "CLI", "Api"},
			expected: []string{"go", "cli", "api"},
		},
		{
			name:     "spaces to dashes",
			input:    []string{"cli tool", "clean architecture"},
			expected: []string{"cli-tool", "clean-architecture"},
		},
		{
			name:     "rejects empty strings",
			input:    []string{"Go", "", "cli", "  "},
			expected: []string{"go", "cli"},
		},
		{
			name:     "deduplicates",
			input:    []string{"go", "Go", "GO", "cli", "CLI"},
			expected: []string{"go", "cli"},
		},
		{
			name:     "handles nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "handles empty input",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "complex mixed case",
			input:    []string{" Go ", "Go", "", "cli-tool", "CLI TOOL", "  "},
			expected: []string{"go", "cli-tool"},
		},
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
		{"valid skill", "skill", true},
		{"valid agent", "agent", true},
		{"valid workflow", "workflow", true},
		{"valid prompt", "prompt", true},
		{"valid context", "context", true},
		{"valid note", "note", true},
		{"invalid empty", "", false},
		{"invalid random", "unknown", false},
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

func TestValidateSearchQuery(t *testing.T) {
	// Valid queries
	err := ValidateSearchQuery(SearchQuery{Query: "fastapi"})
	if err != nil {
		t.Errorf("ValidateSearchQuery(query:fastapi) = %v, want nil", err)
	}

	// Empty query is valid (lists all)
	err = ValidateSearchQuery(SearchQuery{})
	if err != nil {
		t.Errorf("ValidateSearchQuery(empty) = %v, want nil", err)
	}

	// Valid type filter
	err = ValidateSearchQuery(SearchQuery{Query: "x", Type: strPtr("skill")})
	if err != nil {
		t.Errorf("ValidateSearchQuery(type:skill) = %v, want nil", err)
	}

	// Invalid type
	err = ValidateSearchQuery(SearchQuery{Query: "x", Type: strPtr("invalid")})
	if err == nil {
		t.Errorf("ValidateSearchQuery(type:invalid) = nil, want error")
	}

	// Negative limit is invalid
	err = ValidateSearchQuery(SearchQuery{Limit: -1})
	if err == nil {
		t.Errorf("ValidateSearchQuery(limit:-1) = nil, want error")
	}
}

func TestValidateSeriesScope(t *testing.T) {
	// Global series can only have global entries
	t.Run("global series with global entry", func(t *testing.T) {
		err := ValidateSeriesScope(nil, nil)
		if err != nil {
			t.Errorf("global series + global entry should be valid: %v", err)
		}
	})

	// Global series with project entry → rejected
	t.Run("global series with project entry", func(t *testing.T) {
		projID := "vitacare"
		err := ValidateSeriesScope(nil, &projID)
		if err == nil {
			t.Errorf("global series + project entry should be rejected")
		}
	})

	// Project series with global entry → accepted
	t.Run("project series with global entry", func(t *testing.T) {
		projID := "vitacare"
		err := ValidateSeriesScope(&projID, nil)
		if err != nil {
			t.Errorf("project series + global entry should be valid: %v", err)
		}
	})

	// Project series with same-project entry → accepted
	t.Run("project series with same project entry", func(t *testing.T) {
		projID := "vitacare"
		err := ValidateSeriesScope(&projID, &projID)
		if err != nil {
			t.Errorf("same project should be valid: %v", err)
		}
	})

	// Project series with different-project entry → rejected
	t.Run("project series with different project entry", func(t *testing.T) {
		seriesProj := "vitacare"
		entryProj := "other-proj"
		err := ValidateSeriesScope(&seriesProj, &entryProj)
		if err == nil {
			t.Errorf("different project should be rejected")
		}
	})
}

func TestValidateStepNumbers(t *testing.T) {
	// Valid: sequential from 1, no gaps
	t.Run("valid sequential 1-3", func(t *testing.T) {
		steps := []int{1, 2, 3}
		err := ValidateStepNumbers(steps)
		if err != nil {
			t.Errorf("1,2,3 should be valid: %v", err)
		}
	})

	// Valid: single step
	t.Run("valid single step", func(t *testing.T) {
		steps := []int{1}
		err := ValidateStepNumbers(steps)
		if err != nil {
			t.Errorf("single step 1 should be valid: %v", err)
		}
	})

	// Invalid: doesn't start at 1
	t.Run("invalid start at 2", func(t *testing.T) {
		steps := []int{2, 3, 4}
		err := ValidateStepNumbers(steps)
		if err == nil {
			t.Errorf("2,3,4 should be invalid (doesn't start at 1)")
		}
	})

	// Invalid: has gap
	t.Run("invalid gap", func(t *testing.T) {
		steps := []int{1, 3}
		err := ValidateStepNumbers(steps)
		if err == nil {
			t.Errorf("1,3 should be invalid (gap)")
		}
	})

	// Invalid: not sequential
	t.Run("invalid not sequential", func(t *testing.T) {
		steps := []int{1, 2, 4}
		err := ValidateStepNumbers(steps)
		if err == nil {
			t.Errorf("1,2,4 should be invalid (gap)")
		}
	})

	// Valid: empty
	t.Run("valid empty", func(t *testing.T) {
		steps := []int{}
		err := ValidateStepNumbers(steps)
		if err != nil {
			t.Errorf("empty should be valid: %v", err)
		}
	})
}

func strPtr(s string) *string {
	return &s
}
