package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// ParseCommand determines the subcommand from command-line arguments.
// Returns the canonical command name and an error if unknown.
func ParseCommand(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: skillvault <command> [args...]")
	}

	sub := args[1]
	switch sub {
	case "init", "list", "mcp", "version", "save-result":
		return sub, nil
	case "get", "search":
		if len(args) < 3 {
			return "", fmt.Errorf("%s requires an argument", sub)
		}
		return sub, nil
	case "export", "import":
		if len(args) < 3 {
			return "", fmt.Errorf("%s requires a file path", sub)
		}
		return sub, nil
	case "entry":
		return parseEntryCommand(args)
	case "project":
		return parseProjectCommand(args)
	case "series":
		return parseSeriesCommand(args)
	case "workflow":
		return parseWorkflowCommand(args)
	default:
		return "", fmt.Errorf("unknown command: %s", sub)
	}
}

func parseEntryCommand(args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("entry requires subcommand: upsert|archive")
	}
	switch args[2] {
	case "upsert":
		if len(args) < 4 {
			return "", fmt.Errorf("entry upsert requires a file path")
		}
		return "entry-upsert", nil
	case "archive":
		if len(args) < 4 {
			return "", fmt.Errorf("entry archive requires an entry ID")
		}
		return "entry-archive", nil
	default:
		return "", fmt.Errorf("unknown entry subcommand: %s", args[2])
	}
}

func parseProjectCommand(args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("project requires subcommand: upsert|list")
	}
	switch args[2] {
	case "upsert":
		if len(args) < 4 {
			return "", fmt.Errorf("project upsert requires a file path")
		}
		return "project-upsert", nil
	case "list":
		return "project-list", nil
	default:
		return "", fmt.Errorf("unknown project subcommand: %s", args[2])
	}
}

func parseSeriesCommand(args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("series requires subcommand: get|list|upsert|replace")
	}
	switch args[2] {
	case "get":
		if len(args) < 4 {
			return "", fmt.Errorf("series get requires a series ID")
		}
		return "series-get", nil
	case "list":
		return "series-list", nil
	case "upsert":
		if len(args) < 4 {
			return "", fmt.Errorf("series upsert requires a file path")
		}
		return "series-upsert", nil
	case "replace":
		if len(args) < 5 {
			return "", fmt.Errorf("series replace requires series ID and file path")
		}
		return "series-replace", nil
	default:
		return "", fmt.Errorf("unknown series subcommand: %s", args[2])
	}
}

func parseWorkflowCommand(args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("workflow requires subcommand: run")
	}
	switch args[2] {
	case "run":
		if len(args) < 4 {
			return "", fmt.Errorf("workflow run requires a workflow ID")
		}
		return "workflow-run", nil
	default:
		return "", fmt.Errorf("unknown workflow subcommand: %s", args[2])
	}
}

// SearchFlags holds parsed search command flags.
type SearchFlags struct {
	Query           string
	ProjectID       string
	Type            string
	Tag             string
	IncludeArchived bool
}

// ParseSearchFlags parses search-specific flags from args.
func ParseSearchFlags(args []string) (*SearchFlags, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("search requires a query")
	}

	flags := &SearchFlags{Query: args[2]}

	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.StringVar(&flags.ProjectID, "project", "", "Filter by project ID")
	fs.StringVar(&flags.Type, "type", "", "Filter by entry type")
	fs.StringVar(&flags.Tag, "tag", "", "Filter by tag")
	fs.BoolVar(&flags.IncludeArchived, "include-archived", false, "Include archived entries")

	// Don't print to stderr on error
	fs.SetOutput(&nullWriter{})

	if len(args) > 3 {
		if err := fs.Parse(args[3:]); err != nil {
			return nil, fmt.Errorf("parse search flags: %w", err)
		}
	}

	return flags, nil
}

type nullWriter struct{}

func (n *nullWriter) Write(p []byte) (int, error) { return len(p), nil }

// FormatTable formats data as a human-readable table.
func FormatTable[T any](data []T, headers []string, rowFn func(T) []string) string {
	if len(data) == 0 {
		return "(empty)\n"
	}

	// Calculate column widths
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

	// Header
	for i, h := range headers {
		sb.WriteString(fmt.Sprintf("%-*s", widths[i]+2, h))
	}
	sb.WriteString("\n")

	// Separator
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w+2))
	}
	sb.WriteString("\n")

	// Rows
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
