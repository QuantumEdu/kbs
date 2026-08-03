package cli

import (
	"strings"
	"testing"
)

func TestParseSaveResultFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    SaveResultFlags
		wantErr bool
	}{
		{
			name: "required flags only",
			args: []string{"skillvault", "save-result", "--name", "Test", "--content", "body"},
			want: SaveResultFlags{
				Name:    "Test",
				Content: "body",
				Type:    "note",
			},
		},
		{
			name: "body alias",
			args: []string{"skillvault", "save-result", "--name", "Test", "--body", "body"},
			want: SaveResultFlags{
				Name:    "Test",
				Content: "body",
				Type:    "note",
			},
		},
		{
			name: "all flags",
			args: []string{"skillvault", "save-result",
				"--name", "Arch Review",
				"--content", "FastAPI decision",
				"--type", "prompt",
				"--category", "architecture",
				"--tags", " Go ,CLI,go,",
				"--project-id", "kbs",
				"--source-prompt-id", "prd-1",
				"--model", "claude-3",
			},
			want: SaveResultFlags{
				Name:           "Arch Review",
				Content:        "FastAPI decision",
				Type:           "prompt",
				Category:       "architecture",
				Tags:           []string{" Go ", "CLI", "go", ""},
				ProjectID:      "kbs",
				SourcePromptID: "prd-1",
				Model:          "claude-3",
			},
		},
		{
			name:    "missing name",
			args:    []string{"skillvault", "save-result", "--content", "body"},
			wantErr: true,
		},
		{
			name:    "missing content",
			args:    []string{"skillvault", "save-result", "--name", "Test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseSaveResultFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSaveResultFlags failed: %v", err)
			}
			if flags.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", flags.Name, tt.want.Name)
			}
			if flags.Content != tt.want.Content {
				t.Errorf("Content = %q, want %q", flags.Content, tt.want.Content)
			}
			if flags.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", flags.Type, tt.want.Type)
			}
			if flags.Category != tt.want.Category {
				t.Errorf("Category = %q, want %q", flags.Category, tt.want.Category)
			}
			if flags.ProjectID != tt.want.ProjectID {
				t.Errorf("ProjectID = %q, want %q", flags.ProjectID, tt.want.ProjectID)
			}
			if flags.SourcePromptID != tt.want.SourcePromptID {
				t.Errorf("SourcePromptID = %q, want %q", flags.SourcePromptID, tt.want.SourcePromptID)
			}
			if flags.Model != tt.want.Model {
				t.Errorf("Model = %q, want %q", flags.Model, tt.want.Model)
			}
			// Tags are kept raw (normalization happens in the service layer)
			for i, tag := range tt.want.Tags {
				if i >= len(flags.Tags) || flags.Tags[i] != tag {
					t.Errorf("Tags[%d] = %q, want %q", i, flags.Tags[i], tag)
				}
			}
		})
	}
}

func TestParseSaveResultFlags_MissingContentErrorMentionsAlias(t *testing.T) {
	_, err := ParseSaveResultFlags([]string{"skillvault", "save-result", "--name", "Test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--content or --body") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatSaveResultOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  SaveResultOutput
		contain []string
	}{
		{
			name: "project entry",
			output: SaveResultOutput{
				EntryID:   "res-abc123",
				Name:      "Arch Review",
				Type:      "prompt",
				ProjectID: "kbs",
			},
			contain: []string{"res-abc123", "Arch Review", "prompt", "kbs"},
		},
		{
			name: "global entry",
			output: SaveResultOutput{
				EntryID:   "res-def456",
				Name:      "Note",
				Type:      "note",
				ProjectID: "global",
			},
			contain: []string{"res-def456", "Note", "note", "global"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSaveResultOutput(tt.output)
			for _, want := range tt.contain {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q: %s", want, result)
				}
			}
		})
	}
}
