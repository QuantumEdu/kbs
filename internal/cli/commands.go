package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// ParseCommand determines the subcommand from command-line arguments.
func ParseCommand(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: skillvault <command> [args...]")
	}

	sub := args[1]
	switch sub {
	case "graph":
		return sub, nil
	case "init", "version", "mcp", "http", "list-projects", "export":
		return sub, nil
	case "add-entry", "search", "save-artifact", "get-context", "add-project", "session-wrap":
		return sub, nil
	case "get", "archive":
		if len(args) < 3 {
			return "", fmt.Errorf("%s requires an entry ID or slug", sub)
		}
		return sub, nil
	case "add-workflow":
		if len(args) < 3 {
			return "", fmt.Errorf("add-workflow requires a JSON file path")
		}
		return sub, nil
	case "render-workflow":
		if len(args) < 3 {
			return "", fmt.Errorf("render-workflow requires a workflow ID")
		}
		return sub, nil
	case "memory":
		if len(args) < 3 {
			return "", fmt.Errorf("memory requires a subcommand (index, reindex, list-external)")
		}
		sub2 := args[2]
		switch sub2 {
		case "index":
			return "memory-index", nil
		case "reindex":
			return "memory-reindex", nil
		case "list-external":
			return "memory-list-external", nil
		default:
			return "", fmt.Errorf("unknown memory subcommand: %s", sub2)
		}
	case "entry":
		if len(args) < 3 {
			return "", fmt.Errorf("entry requires a subcommand (ref)")
		}
		sub2 := args[2]
		switch sub2 {
		case "ref":
			return "entry-ref", nil
		default:
			return "", fmt.Errorf("unknown entry subcommand: %s", sub2)
		}
	case "ref":
		return "entry-ref", nil
	case "import":
		if len(args) < 3 {
			return "", fmt.Errorf("import requires a file path")
		}
		return sub, nil
	case "save-result":
		return sub, nil
	default:
		return "", fmt.Errorf("unknown command: %s", sub)
	}
}

// AddEntryFlags holds parsed add-entry command flags.
type AddEntryFlags struct {
	Title   string
	Type    string
	Summary string
	Body    string
	Project string
	Tags    string
	Status  string
}

// ParseAddEntryFlags parses add-entry-specific flags from args.
func ParseAddEntryFlags(args []string) (*AddEntryFlags, error) {
	flags := &AddEntryFlags{Type: "reference"}

	fs := flag.NewFlagSet("add-entry", flag.ContinueOnError)
	fs.StringVar(&flags.Title, "title", "", "Entry title (required)")
	fs.StringVar(&flags.Type, "type", "reference", "Entry type")
	fs.StringVar(&flags.Summary, "summary", "", "Entry summary (required)")
	fs.StringVar(&flags.Body, "body", "", "Entry body text")
	fs.StringVar(&flags.Project, "project", "", "Project slug or ID")
	fs.StringVar(&flags.Tags, "tags", "", "Comma-separated tags")
	fs.StringVar(&flags.Status, "status", "", "Entry status")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse add-entry flags: %w", err)
		}
	}

	if flags.Title == "" {
		return nil, fmt.Errorf("--title is required")
	}
	if flags.Summary == "" {
		return nil, fmt.Errorf("--summary is required")
	}

	return flags, nil
}

// SearchFlags holds parsed search command flags.
type SearchFlags struct {
	Query           string
	ProjectID       string
	Type            string
	Tag             string
	IncludeArchived bool
	Limit           int
}

// ParseSearchFlags parses search-specific flags from args.
func ParseSearchFlags(args []string) (*SearchFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("search requires a query")
	}

	flags := &SearchFlags{Query: args[2], Limit: 20}

	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.StringVar(&flags.ProjectID, "project", "", "Filter by project slug or ID")
	fs.StringVar(&flags.Type, "type", "", "Filter by entry type")
	fs.StringVar(&flags.Tag, "tag", "", "Filter by tag")
	fs.BoolVar(&flags.IncludeArchived, "include-archived", false, "Include archived entries")
	fs.IntVar(&flags.Limit, "limit", 20, "Max results")

	fs.SetOutput(&nullWriter{})

	if len(args) > 3 {
		if err := fs.Parse(args[3:]); err != nil {
			return nil, fmt.Errorf("parse search flags: %w", err)
		}
	}

	return flags, nil
}

// GetEntryFlags holds the entry identifier for get/archive commands.
type GetEntryFlags struct {
	ID string
}

// ParseGetEntryFlags parses the entry ID from positional args.
func ParseGetEntryFlags(args []string) (*GetEntryFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("entry ID or slug is required")
	}
	return &GetEntryFlags{ID: args[2]}, nil
}

// SaveArtifactFlags holds parsed save-artifact command flags.
type SaveArtifactFlags struct {
	Title   string
	Type    string
	File    string
	Content string
	Project string
	Summary string
	Tags    string
	Source  string
}

// ParseSaveArtifactFlags parses save-artifact-specific flags from args.
func ParseSaveArtifactFlags(args []string) (*SaveArtifactFlags, error) {
	flags := &SaveArtifactFlags{Type: "markdown"}

	fs := flag.NewFlagSet("save-artifact", flag.ContinueOnError)
	fs.StringVar(&flags.Title, "title", "", "Artifact title (required)")
	fs.StringVar(&flags.Type, "type", "markdown", "Artifact type")
	fs.StringVar(&flags.File, "file", "", "Path to file to store")
	fs.StringVar(&flags.Content, "content", "", "Inline content")
	fs.StringVar(&flags.Project, "project", "", "Project slug or ID")
	fs.StringVar(&flags.Summary, "summary", "", "Artifact summary")
	fs.StringVar(&flags.Tags, "tags", "", "Comma-separated tags")
	fs.StringVar(&flags.Source, "source", "", "Source URL or reference")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse save-artifact flags: %w", err)
		}
	}

	if flags.Title == "" {
		return nil, fmt.Errorf("--title is required")
	}
	if flags.File == "" && flags.Content == "" {
		return nil, fmt.Errorf("either --file or --content is required")
	}

	return flags, nil
}

// GetContextFlags holds parsed get-context command flags.
type GetContextFlags struct {
	Mode     string
	Project  string
	Query    string
	Include  string
	MaxChars int
}

// ParseGetContextFlags parses get-context-specific flags from args.
func ParseGetContextFlags(args []string) (*GetContextFlags, error) {
	flags := &GetContextFlags{Mode: "project", MaxChars: 12000}

	fs := flag.NewFlagSet("get-context", flag.ContinueOnError)
	fs.StringVar(&flags.Mode, "mode", "project", "Context mode (profile|planning|project)")
	fs.StringVar(&flags.Project, "project", "", "Project slug or ID")
	fs.StringVar(&flags.Query, "query", "", "Additional search query")
	fs.StringVar(&flags.Include, "include", "", "Comma-separated sections to include")
	fs.IntVar(&flags.MaxChars, "max-chars", 12000, "Max characters in context pack")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse get-context flags: %w", err)
		}
	}

	return flags, nil
}

// AddProjectFlags holds parsed add-project command flags.
type AddProjectFlags struct {
	Name        string
	Description string
}

// ParseAddProjectFlags parses add-project-specific flags from args.
func ParseAddProjectFlags(args []string) (*AddProjectFlags, error) {
	flags := &AddProjectFlags{}

	fs := flag.NewFlagSet("add-project", flag.ContinueOnError)
	fs.StringVar(&flags.Name, "name", "", "Project name (required)")
	fs.StringVar(&flags.Description, "description", "", "Project description")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse add-project flags: %w", err)
		}
	}

	if flags.Name == "" {
		return nil, fmt.Errorf("--name is required")
	}

	return flags, nil
}

// WorkflowFileFlags holds the file path for add-workflow.
type WorkflowFileFlags struct {
	FilePath string
}

// ParseWorkflowFileFlags parses the file path for add-workflow.
func ParseWorkflowFileFlags(args []string) (*WorkflowFileFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("JSON file path is required")
	}
	return &WorkflowFileFlags{FilePath: args[2]}, nil
}

// RenderWorkflowFlags holds the workflow ID for render-workflow.
type RenderWorkflowFlags struct {
	WorkflowID string
}

// ParseRenderWorkflowFlags parses the workflow ID for render-workflow.
func ParseRenderWorkflowFlags(args []string) (*RenderWorkflowFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("workflow ID is required")
	}
	return &RenderWorkflowFlags{WorkflowID: args[2]}, nil
}

// SessionWrapFlags holds parsed session-wrap command flags.
type SessionWrapFlags struct {
	Project   string
	Summary   string
	Decisions string
	Pending   string
	Learnings string
}

// ParseSessionWrapFlags parses session-wrap-specific flags from args.
func ParseSessionWrapFlags(args []string) (*SessionWrapFlags, error) {
	flags := &SessionWrapFlags{}

	fs := flag.NewFlagSet("session-wrap", flag.ContinueOnError)
	fs.StringVar(&flags.Project, "project", "", "Project slug or ID")
	fs.StringVar(&flags.Summary, "summary", "", "Session summary (required)")
	fs.StringVar(&flags.Decisions, "decisions", "", "Comma-separated decisions")
	fs.StringVar(&flags.Pending, "pending", "", "Comma-separated pending items")
	fs.StringVar(&flags.Learnings, "learnings", "", "Comma-separated learnings")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse session-wrap flags: %w", err)
		}
	}

	if flags.Summary == "" {
		return nil, fmt.Errorf("--summary is required")
	}

	return flags, nil
}

// ExportFlags holds parsed export command flags.
type ExportFlags struct {
	OutputPath string
}

// ParseExportFlags parses export-specific flags from args.
func ParseExportFlags(args []string) (*ExportFlags, error) {
	flags := &ExportFlags{OutputPath: "skillvault-export.json"}

	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.StringVar(&flags.OutputPath, "output", "skillvault-export.json", "Output file path")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse export flags: %w", err)
		}
	}

	return flags, nil
}

// ImportFlags holds parsed import command flags.
type ImportFlags struct {
	FilePath string
}

// ParseImportFlags parses import-specific flags from args.
// GraphFlags holds parsed graph command flags.
type GraphFlags struct {
	EntryID   string
	Depth     int
	Format    string
	Direction string
}

// ParseGraphFlags parses graph-specific flags from args.
func ParseGraphFlags(args []string) (*GraphFlags, error) {
	flags := &GraphFlags{Depth: 3, Format: "mermaid", Direction: "both"}

	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.StringVar(&flags.EntryID, "entry", "", "Entry ID to root the graph (required)")
	fs.IntVar(&flags.Depth, "depth", 3, "Max traversal depth (default 3, max 10)")
	fs.StringVar(&flags.Format, "format", "mermaid", "Output format: mermaid, json, dot")
	fs.StringVar(&flags.Direction, "direction", "both", "Traversal direction: outgoing, incoming, both")
	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse graph flags: %w", err)
		}
	}

	if flags.EntryID == "" {
		return nil, fmt.Errorf("--entry is required")
	}
	if flags.Depth < 1 {
		flags.Depth = 1
	}
	if flags.Depth > 10 {
		flags.Depth = 10
	}
	switch flags.Format {
	case "mermaid", "json", "dot":
	default:
		return nil, fmt.Errorf("invalid format %q, expected: mermaid, json, dot", flags.Format)
	}
	switch flags.Direction {
	case "outgoing", "incoming", "both":
	default:
		return nil, fmt.Errorf("invalid direction %q, expected: outgoing, incoming, both", flags.Direction)
	}

	return flags, nil
}

// MemoryIndexFlags holds parsed memory index/reindex/list-external command flags.
type MemoryIndexFlags struct {
	Path           string
	ProjectID      string
	ParseWikilinks bool
}

// ParseMemoryIndexFlags parses memory index/reindex/list-external flags from args.
func ParseMemoryIndexFlags(args []string) (*MemoryIndexFlags, error) {
	flags := &MemoryIndexFlags{}

	fs := flag.NewFlagSet("memory", flag.ContinueOnError)
	fs.StringVar(&flags.Path, "path", "", "Path to pi-memory-md directory (required for index/reindex)")
	fs.StringVar(&flags.ProjectID, "project", "", "Target project ID in SkillVault (required)")
	fs.BoolVar(&flags.ParseWikilinks, "wikilinks", false, "Parse [[wikilinks]] from body to create entry_refs")
	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse memory flags: %w", err)
		}
	}

	// Path is required for index/reindex but not for list-external
	cmd := ""
	if len(args) >= 2 {
		cmd = args[1]
	}
	if cmd != "memory-list-external" && flags.Path == "" {
		return nil, fmt.Errorf("--path is required")
	}
	if flags.ProjectID == "" {
		return nil, fmt.Errorf("--project is required")
	}

	return flags, nil
}

func ParseImportFlags(args []string) (*ImportFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("import requires a file path")
	}
	return &ImportFlags{FilePath: args[2]}, nil
}

// nullWriter discards writes (used to suppress flag package errors).
type nullWriter struct{}

func (n *nullWriter) Write(p []byte) (int, error) { return len(p), nil }

// FormatTable formats data as a human-readable table.
func FormatTable[T any](data []T, headers []string, rowFn func(T) []string) string {
	if len(data) == 0 {
		return "(empty)\n"
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, item := range data {
		row := rowFn(item)
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder
	for i, h := range headers {
		sb.WriteString(fmt.Sprintf("%-*s", widths[i]+2, h))
	}
	sb.WriteString("\n")
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w+2))
	}
	sb.WriteString("\n")
	for _, item := range data {
		row := rowFn(item)
		for i, cell := range row {
			if i < len(widths) {
				sb.WriteString(fmt.Sprintf("%-*s", widths[i]+2, cell))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// PrintJSON prints data as indented JSON to stdout.
func PrintJSON(v interface{}) error {
	return fmt.Errorf("not implemented")
}

// PrintError prints an error message to stderr.
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
}

// TagItems splits a comma-separated tag string and trims each item.
func TagItems(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

// SplitLines splits a comma-separated string into trimmed items.
func SplitLines(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}
