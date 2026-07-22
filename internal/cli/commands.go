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
	case "init", "version", "mcp", "http", "list-projects", "export", "tui", "stats":
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
	case "run":
		if len(args) < 4 {
			return "", fmt.Errorf("run requires <workflow> <file>")
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
			return "", fmt.Errorf("entry requires a subcommand (ref, history, restore)")
		}
		sub2 := args[2]
		switch sub2 {
		case "ref":
			return "entry-ref", nil
		case "history":
			if len(args) < 4 {
				return "", fmt.Errorf("entry history requires an entry ID")
			}
			return "entry-history", nil
		case "restore":
			if len(args) < 4 {
				return "", fmt.Errorf("entry restore requires an entry ID")
			}
			return "entry-restore", nil
		default:
			return "", fmt.Errorf("unknown entry subcommand: %s", sub2)
		}
	case "ref":
		return "entry-ref", nil
	case "compare-entries":
		if len(args) < 4 {
			return "", fmt.Errorf("compare-entries requires two entry IDs")
		}
		return sub, nil
	case "setup-vectors":
		if len(args) < 3 {
			return "", fmt.Errorf("setup-vectors requires a GloVe file path")
		}
		return sub, nil
	case "reindex-embeddings":
		return sub, nil
	case "import":
		if len(args) < 3 {
			return "", fmt.Errorf("import requires a file path")
		}
		return sub, nil
	case "import-workflow":
		return sub, nil
	case "route":
		return sub, nil
	case "save-result":
		return sub, nil
	case "update":
		return sub, nil
	case "secrets":
		return sub, nil
	case "sync":
		if len(args) < 3 {
			return "", fmt.Errorf("sync requires a subcommand (push, pull)")
		}
		sub2 := args[2]
		switch sub2 {
		case "push":
			return "sync-push", nil
		case "pull":
			return "sync-pull", nil
		default:
			return "", fmt.Errorf("unknown sync subcommand: %s", sub2)
		}
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
	Purpose string
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
	fs.StringVar(&flags.Purpose, "purpose", "", "Entry purpose (WORK, KNOWLEDGE, LEARNING, RELATIONSHIP, STATE)")

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
	Purpose         string
	IncludeArchived bool
	Limit           int
	Vector          bool
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
	fs.BoolVar(&flags.Vector, "vector", false, "Use vector/cosine similarity search")
	fs.StringVar(&flags.Purpose, "purpose", "", "Filter by purpose (WORK, KNOWLEDGE, LEARNING, RELATIONSHIP, STATE)")

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

// EntryHistoryFlags holds the entry ID for entry history command.
type EntryHistoryFlags struct {
	ID string
}

// ParseEntryHistoryFlags parses the entry ID from positional args.
// Usage: skillvault entry history <id>
func ParseEntryHistoryFlags(args []string) (*EntryHistoryFlags, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("entry ID is required")
	}
	return &EntryHistoryFlags{ID: args[3]}, nil
}

// EntryRestoreFlags holds the entry ID and version number for entry restore.
type EntryRestoreFlags struct {
	ID      string
	Version int
}

// ParseEntryRestoreFlags parses entry restore flags from args.
// Usage: skillvault entry restore <id> --version N
func ParseEntryRestoreFlags(args []string) (*EntryRestoreFlags, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("entry ID is required")
	}

	flags := &EntryRestoreFlags{ID: args[3], Version: 0}

	fs := flag.NewFlagSet("entry-restore", flag.ContinueOnError)
	fs.IntVar(&flags.Version, "version", 0, "Version number to restore (required)")
	fs.SetOutput(&nullWriter{})

	if len(args) > 4 {
		if err := fs.Parse(args[4:]); err != nil {
			return nil, fmt.Errorf("parse entry restore flags: %w", err)
		}
	}

	if flags.Version < 1 {
		return nil, fmt.Errorf("--version is required (must be >= 1)")
	}

	return flags, nil
}

// CompareEntriesFlags holds the two entry IDs for compare-entries.
type CompareEntriesFlags struct {
	ID1 string
	ID2 string
}

// ParseCompareEntriesFlags parses two positional entry IDs from args.
func ParseCompareEntriesFlags(args []string) (*CompareEntriesFlags, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("compare-entries requires two entry IDs")
	}
	return &CompareEntriesFlags{ID1: args[2], ID2: args[3]}, nil
}

// SetupVectorsFlags holds the GloVe file path for setup-vectors.
type SetupVectorsFlags struct {
	Path string
}

// ParseSetupVectorsFlags parses the GloVe file path from positional args.
func ParseSetupVectorsFlags(args []string) (*SetupVectorsFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("GloVe file path is required")
	}
	return &SetupVectorsFlags{Path: args[2]}, nil
}

// ReindexEmbeddingsFlags holds optional flags for reindex-embeddings.
type ReindexEmbeddingsFlags struct{}

// ParseReindexEmbeddingsFlags parses reindex-embeddings flags (currently none).
func ParseReindexEmbeddingsFlags(args []string) (*ReindexEmbeddingsFlags, error) {
	return &ReindexEmbeddingsFlags{}, nil
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
	Pack       bool
}

// ParseExportFlags parses export-specific flags from args.
func ParseExportFlags(args []string) (*ExportFlags, error) {
	flags := &ExportFlags{OutputPath: "skillvault-export.json"}

	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.StringVar(&flags.OutputPath, "output", "skillvault-export.json", "Output file path")
	fs.BoolVar(&flags.Pack, "pack", false, "Export as a skill pack (.svpack) with metadata")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse export flags: %w", err)
		}
	}

	return flags, nil
}

// ExportPackFlags holds parsed pack export-specific flags.
type ExportPackFlags struct {
	Pack        bool
	Author      string
	Version     string
	Description string
	OutputPath  string
}

// ParseExportPackFlags parses pack export flags from args.
func ParseExportPackFlags(args []string) (*ExportPackFlags, error) {
	flags := &ExportPackFlags{Pack: true, OutputPath: "skillvault-pack.svpack"}

	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.BoolVar(&flags.Pack, "pack", true, "Export as a skill pack (.svpack) with metadata")
	fs.StringVar(&flags.Author, "author", "", "Pack author (required)")
	fs.StringVar(&flags.Version, "version", "", "Pack version (required)")
	fs.StringVar(&flags.Description, "description", "", "Pack description")
	fs.StringVar(&flags.OutputPath, "output", "skillvault-pack.svpack", "Output file path")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse pack export flags: %w", err)
		}
	}

	if flags.Author == "" {
		return nil, fmt.Errorf("--author is required for pack export")
	}
	if flags.Version == "" {
		return nil, fmt.Errorf("--version is required for pack export")
	}

	return flags, nil
}

// ImportFlags holds parsed import command flags.
type ImportFlags struct {
	FilePath string
	Prefix   string
	Pack     bool
}

// RunFlags holds parsed run command flags.
type RunFlags struct {
	Workflow string
	FilePath string
	SavePath string
}

// ParseRunFlags parses run-specific flags from args.
// Usage: skillvault run <workflow> <file> [--save <path>]
func ParseRunFlags(args []string) (*RunFlags, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("run requires <workflow> <file>")
	}

	flags := &RunFlags{
		Workflow: args[2],
		FilePath: args[3],
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.StringVar(&flags.SavePath, "save", "", "Save output to file path")

	fs.SetOutput(&nullWriter{})

	if len(args) > 4 {
		if err := fs.Parse(args[4:]); err != nil {
			return nil, fmt.Errorf("parse run flags: %w", err)
		}
	}

	return flags, nil
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

	flags := &ImportFlags{FilePath: args[2]}

	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.StringVar(&flags.Prefix, "prefix", "", "Prefix all imported entity IDs (e.g. 'ns/')")
	fs.BoolVar(&flags.Pack, "pack", false, "Import as a skill pack")

	fs.SetOutput(&nullWriter{})

	if len(args) > 3 {
		if err := fs.Parse(args[3:]); err != nil {
			return nil, fmt.Errorf("parse import flags: %w", err)
		}
	}

	return flags, nil
}

// ImportWorkflowFlags holds parsed import-workflow command flags.
type ImportWorkflowFlags struct {
	File    string
	Project string
}

// ParseImportWorkflowFlags parses import-workflow-specific flags from args.
func ParseImportWorkflowFlags(args []string) (*ImportWorkflowFlags, error) {
	flags := &ImportWorkflowFlags{}

	fs := flag.NewFlagSet("import-workflow", flag.ContinueOnError)
	fs.StringVar(&flags.File, "file", "", "Path to workflow-builder YAML file (required)")
	fs.StringVar(&flags.Project, "project", "", "Project slug or ID for scoped import")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse import-workflow flags: %w", err)
		}
	}

	if flags.File == "" {
		return nil, fmt.Errorf("--file is required")
	}

	return flags, nil
}

// RouteFlags holds parsed route command flags.
type RouteFlags struct {
	Scenario string
	JSON     bool
}

// ParseRouteFlags parses route-specific flags from args.
// Usage: skillvault route <scenario> [--json]
func ParseRouteFlags(args []string) (*RouteFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("route requires a scenario string")
	}

	flags := &RouteFlags{Scenario: args[2]}

	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	fs.BoolVar(&flags.JSON, "json", false, "Output as JSON")

	fs.SetOutput(&nullWriter{})

	if len(args) > 3 {
		if err := fs.Parse(args[3:]); err != nil {
			return nil, fmt.Errorf("parse route flags: %w", err)
		}
	}

	return flags, nil
}

// StatsFlags holds parsed stats command flags.
type StatsFlags struct {
	WorkflowRuns bool
	JSON         bool
}

// ParseStatsFlags parses stats-specific flags from args.
func ParseStatsFlags(args []string) (*StatsFlags, error) {
	flags := &StatsFlags{}

	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.BoolVar(&flags.WorkflowRuns, "workflow-runs", false, "Include workflow run analytics")
	fs.BoolVar(&flags.JSON, "json", false, "Output as JSON")

	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse stats flags: %w", err)
		}
	}

	return flags, nil
}

// UpdateFlags holds parsed update command flags.
type UpdateFlags struct {
	Repo        string
	InstallPath string
}

// ParseUpdateFlags parses update-specific flags from args.
func ParseUpdateFlags(args []string) (*UpdateFlags, error) {
	flags := &UpdateFlags{}

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.StringVar(&flags.Repo, "repo", "", "Path to local git repo (overrides SKILLVAULT_REPO)")
	fs.StringVar(&flags.InstallPath, "install-path", "", "Path to install the rebuilt binary (overrides SKILLVAULT_INSTALL_PATH)")
	fs.SetOutput(&nullWriter{})

	if len(args) > 2 {
		if err := fs.Parse(args[2:]); err != nil {
			return nil, fmt.Errorf("parse update flags: %w", err)
		}
	}

	return flags, nil
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

// PrintError prints an error message to stderr with sk-vault prefix.
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
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

// SyncFlags holds parsed sync push/pull command flags.
type SyncFlags struct {
	Transport  string
	RemotePath string
	DryRun     bool
}

// ParseSyncFlags parses sync-specific flags from args.
// Usage: skillvault sync push|pull --transport s3|github --remote-path <path> [--dry-run]
func ParseSyncFlags(args []string) (*SyncFlags, error) {
	flags := &SyncFlags{}

	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.StringVar(&flags.Transport, "transport", "", "Transport backend: s3 or github (required)")
	fs.StringVar(&flags.RemotePath, "remote-path", "", "Remote object key or asset name")
	fs.BoolVar(&flags.DryRun, "dry-run", false, "Show what would be transferred without actually doing it")
	fs.SetOutput(&nullWriter{})

	if len(args) > 3 {
		if err := fs.Parse(args[3:]); err != nil {
			return nil, fmt.Errorf("parse sync flags: %w", err)
		}
	}

	if flags.Transport == "" {
		return nil, fmt.Errorf("--transport is required (s3 or github)")
	}
	switch flags.Transport {
	case "s3", "github":
	default:
		return nil, fmt.Errorf("invalid transport %q: must be s3 or github", flags.Transport)
	}
	// remote-path is required unless it's a dry-run (the user might just want to check)
	if flags.RemotePath == "" && !flags.DryRun {
		return nil, fmt.Errorf("--remote-path is required")
	}

	return flags, nil
}
