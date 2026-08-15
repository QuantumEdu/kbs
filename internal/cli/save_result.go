package cli

import (
	"flag"
	"fmt"
	"strings"
)

// SaveResultFlags holds parsed save-result command flags.
type SaveResultFlags struct {
	Name           string
	Content        string
	Type           string
	Category       string
	Tags           []string
	ProjectID      string
	SourcePromptID string
	Model          string
}

// SaveResultOutput holds the result for output formatting.
type SaveResultOutput struct {
	EntryID   string
	Name      string
	Type      string
	ProjectID string
}

// ParseSaveResultFlags parses save-result-specific flags from args.
func ParseSaveResultFlags(args []string) (*SaveResultFlags, error) {
	flags := &SaveResultFlags{Type: "note"}

	fs := flag.NewFlagSet("save-result", flag.ContinueOnError)
	fs.StringVar(&flags.Name, "name", "", "Result name (required)")
	fs.StringVar(&flags.Content, "content", "", "Result content (required)")
	fs.StringVar(&flags.Content, "body", "", "Alias for --content")
	fs.StringVar(&flags.Type, "type", "note", "Entry type (skill|agent|workflow|prompt|context|note)")
	fs.StringVar(&flags.Category, "category", "", "Classification label")
	var tagsRaw string
	fs.StringVar(&tagsRaw, "tags", "", "Comma-separated tags")
	fs.StringVar(&flags.ProjectID, "project-id", "", "Target project ID")
	fs.StringVar(&flags.SourcePromptID, "source-prompt-id", "", "Source prompt entry ID")
	fs.StringVar(&flags.Model, "model", "", "LLM model identifier")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse save-result flags: %w", err)
		}
	}

	// Validate required
	if flags.Name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if flags.Content == "" {
		return nil, fmt.Errorf("--content or --body is required")
	}

	// Parse comma-separated tags (raw — normalization happens in service layer)
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			flags.Tags = append(flags.Tags, t)
		}
	}

	return flags, nil
}

// FormatSaveResultOutput produces the human-readable confirmation string.
func FormatSaveResultOutput(o SaveResultOutput) string {
	return fmt.Sprintf("Saved: %s\n  Name:    %s\n  Type:    %s\n  Project: %s\n",
		o.EntryID, o.Name, o.Type, o.ProjectID)
}
