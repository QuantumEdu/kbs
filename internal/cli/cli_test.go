package cli

import (
	"strings"
	"testing"
)

func TestParseSubcommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd string
		wantErr bool
	}{
		// v2 commands
		{"init", []string{"skillvault", "init"}, "init", false},
		{"add-entry", []string{"skillvault", "add-entry", "--title", "T", "--summary", "S"}, "add-entry", false},
		{"search", []string{"skillvault", "search", "query"}, "search", false},
		{"get", []string{"skillvault", "get", "entry-id"}, "get", false},
		{"save-artifact", []string{"skillvault", "save-artifact", "--title", "A", "--file", "f.md"}, "save-artifact", false},
		{"get-context", []string{"skillvault", "get-context", "--project", "p"}, "get-context", false},
		{"add-project", []string{"skillvault", "add-project", "--name", "P"}, "add-project", false},
		{"list-projects", []string{"skillvault", "list-projects"}, "list-projects", false},
		{"archive", []string{"skillvault", "archive", "entry-id"}, "archive", false},
		{"add-workflow", []string{"skillvault", "add-workflow", "wf.json"}, "add-workflow", false},
		{"render-workflow", []string{"skillvault", "render-workflow", "wf-id"}, "render-workflow", false},
		{"session-wrap", []string{"skillvault", "session-wrap", "--summary", "S"}, "session-wrap", false},
		{"export", []string{"skillvault", "export"}, "export", false},
		{"import", []string{"skillvault", "import", "file.json"}, "import", false},
		{"import-workflow", []string{"skillvault", "import-workflow", "--file", "wf.yaml"}, "import-workflow", false},

		// Legacy commands
		{"version", []string{"skillvault", "version"}, "version", false},
		{"mcp", []string{"skillvault", "mcp"}, "mcp", false},
		{"save-result", []string{"skillvault", "save-result", "--name", "X", "--content", "Y"}, "save-result", false},

		// Required args
		{"get no arg", []string{"skillvault", "get"}, "", true},
		{"archive no arg", []string{"skillvault", "archive"}, "", true},
		{"add-workflow no arg", []string{"skillvault", "add-workflow"}, "", true},
		{"render-workflow no arg", []string{"skillvault", "render-workflow"}, "", true},
		{"import no arg", []string{"skillvault", "import"}, "", true},
		{"graph no entry", []string{"skillvault", "graph", "--format", "json"}, "graph", false},
		{"memory index no project", []string{"skillvault", "memory", "index", "--path", "/tmp"}, "memory-index", false},
		{"run no args", []string{"skillvault", "run"}, "", true},
		{"run missing file", []string{"skillvault", "run", "wf"}, "", true},

		// New v1-final commands
		{"graph", []string{"skillvault", "graph", "--entry", "e1", "--format", "json"}, "graph", false},
		{"memory index", []string{"skillvault", "memory", "index", "--path", "/tmp/mem", "--project", "p"}, "memory-index", false},
		{"memory reindex", []string{"skillvault", "memory", "reindex", "--path", "/tmp/mem", "--project", "p"}, "memory-reindex", false},
		{"memory list-external", []string{"skillvault", "memory", "list-external", "--project", "p"}, "memory-list-external", false},
		{"entry ref add", []string{"skillvault", "entry", "ref", "add", "s", "t", "depends_on"}, "entry-ref", false},
		{"run", []string{"skillvault", "run", "wf", "input.md"}, "run", false},
		{"run with save", []string{"skillvault", "run", "wf", "input.md", "--save", "out.md"}, "run", false},
		{"run with stdin", []string{"skillvault", "run", "wf", "-"}, "run", false},

		// Sync commands (push/pull subcommands)
		{"sync push", []string{"skillvault", "sync", "push", "--transport", "s3", "--remote-path", "vault.gz"}, "sync-push", false},
		{"sync pull", []string{"skillvault", "sync", "pull", "--transport", "github", "--remote-path", "vault.gz"}, "sync-pull", false},
		{"sync push dry-run", []string{"skillvault", "sync", "push", "--transport", "s3", "--remote-path", "vault.gz", "--dry-run"}, "sync-push", false},
		{"sync no subcommand", []string{"skillvault", "sync"}, "", true},
		{"sync invalid subcommand", []string{"skillvault", "sync", "unknown"}, "", true},

		// TUI command
		{"tui", []string{"skillvault", "tui"}, "tui", false},

		// Errors
		{"no args", []string{"skillvault"}, "", true},
		{"invalid subcommand", []string{"skillvault", "invalid"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseCommand(tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for args %v", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.wantCmd)
			}
		})
	}
}

func TestParseAddEntryFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    AddEntryFlags
		wantErr bool
	}{
		{
			name: "required flags only",
			args: []string{"skillvault", "add-entry", "--title", "My Entry", "--summary", "A summary"},
			want: AddEntryFlags{Title: "My Entry", Summary: "A summary", Type: "reference"},
		},
		{
			name: "all flags",
			args: []string{"skillvault", "add-entry",
				"--title", "Full Entry",
				"--type", "decision",
				"--summary", "Key decision",
				"--body", "Body text here",
				"--project", "myproj",
				"--tags", "go,testing",
				"--status", "draft",
			},
			want: AddEntryFlags{
				Title: "Full Entry", Type: "decision", Summary: "Key decision",
				Body: "Body text here", Project: "myproj", Tags: "go,testing", Status: "draft",
			},
		},
		{
			name:    "missing title",
			args:    []string{"skillvault", "add-entry", "--summary", "S"},
			wantErr: true,
		},
		{
			name:    "missing summary",
			args:    []string{"skillvault", "add-entry", "--title", "T"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseAddEntryFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAddEntryFlags failed: %v", err)
			}
			if flags.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", flags.Title, tt.want.Title)
			}
			if flags.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", flags.Type, tt.want.Type)
			}
			if flags.Summary != tt.want.Summary {
				t.Errorf("Summary = %q, want %q", flags.Summary, tt.want.Summary)
			}
			if flags.Project != tt.want.Project {
				t.Errorf("Project = %q, want %q", flags.Project, tt.want.Project)
			}
		})
	}
}

func TestParseSearchFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantQuery string
		wantLimit int
	}{
		{"basic", []string{"skillvault", "search", "fastapi"}, "fastapi", 20},
		{"with project", []string{"skillvault", "search", "fastapi", "--project", "vitacare"}, "fastapi", 20},
		{"with type", []string{"skillvault", "search", "fastapi", "--type", "skill"}, "fastapi", 20},
		{"with limit", []string{"skillvault", "search", "test", "--limit", "5"}, "test", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseSearchFlags(tt.args)
			if err != nil {
				t.Fatalf("ParseSearchFlags failed: %v", err)
			}
			if flags.Query != tt.wantQuery {
				t.Errorf("Query = %q, want %q", flags.Query, tt.wantQuery)
			}
			if flags.Limit != tt.wantLimit {
				t.Errorf("Limit = %d, want %d", flags.Limit, tt.wantLimit)
			}
		})
	}

	t.Run("missing query", func(t *testing.T) {
		_, err := ParseSearchFlags([]string{"skillvault", "search"})
		if err == nil {
			t.Fatal("expected error for missing query")
		}
	})
}

func TestParseSaveArtifactFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    SaveArtifactFlags
		wantErr bool
	}{
		{
			name: "with file",
			args: []string{"skillvault", "save-artifact", "--title", "Doc", "--file", "doc.md"},
			want: SaveArtifactFlags{Title: "Doc", File: "doc.md", Type: "markdown"},
		},
		{
			name: "with content",
			args: []string{"skillvault", "save-artifact", "--title", "Doc", "--content", "body"},
			want: SaveArtifactFlags{Title: "Doc", Content: "body", Type: "markdown"},
		},
		{
			name: "all flags",
			args: []string{"skillvault", "save-artifact",
				"--title", "Analysis",
				"--type", "pdf_analysis",
				"--file", "analysis.md",
				"--project", "myproj",
				"--summary", "PDF analysis",
				"--tags", "pdf,review",
				"--source", "https://example.com/doc",
			},
			want: SaveArtifactFlags{
				Title: "Analysis", Type: "pdf_analysis", File: "analysis.md",
				Project: "myproj", Summary: "PDF analysis", Tags: "pdf,review",
				Source: "https://example.com/doc",
			},
		},
		{
			name:    "missing title",
			args:    []string{"skillvault", "save-artifact", "--file", "f.md"},
			wantErr: true,
		},
		{
			name:    "missing file and content",
			args:    []string{"skillvault", "save-artifact", "--title", "T"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseSaveArtifactFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSaveArtifactFlags failed: %v", err)
			}
			if flags.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", flags.Title, tt.want.Title)
			}
			if flags.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", flags.Type, tt.want.Type)
			}
			if flags.File != tt.want.File {
				t.Errorf("File = %q, want %q", flags.File, tt.want.File)
			}
		})
	}
}

func TestParseGetContextFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want GetContextFlags
	}{
		{
			name: "default mode",
			args: []string{"skillvault", "get-context"},
			want: GetContextFlags{Mode: "project", MaxChars: 12000},
		},
		{
			name: "with project and mode",
			args: []string{"skillvault", "get-context", "--project", "myapp", "--mode", "planning"},
			want: GetContextFlags{Mode: "planning", Project: "myapp", MaxChars: 12000},
		},
		{
			name: "with include and max-chars",
			args: []string{"skillvault", "get-context", "--include", "decisions,workflows", "--max-chars", "5000"},
			want: GetContextFlags{Mode: "project", Include: "decisions,workflows", MaxChars: 5000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseGetContextFlags(tt.args)
			if err != nil {
				t.Fatalf("ParseGetContextFlags failed: %v", err)
			}
			if flags.Mode != tt.want.Mode {
				t.Errorf("Mode = %q, want %q", flags.Mode, tt.want.Mode)
			}
			if flags.Project != tt.want.Project {
				t.Errorf("Project = %q, want %q", flags.Project, tt.want.Project)
			}
			if flags.MaxChars != tt.want.MaxChars {
				t.Errorf("MaxChars = %d, want %d", flags.MaxChars, tt.want.MaxChars)
			}
			if flags.Include != tt.want.Include {
				t.Errorf("Include = %q, want %q", flags.Include, tt.want.Include)
			}
		})
	}
}

func TestParseAddProjectFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    AddProjectFlags
		wantErr bool
	}{
		{
			name: "required flags",
			args: []string{"skillvault", "add-project", "--name", "My Project"},
			want: AddProjectFlags{Name: "My Project"},
		},
		{
			name: "with description",
			args: []string{"skillvault", "add-project", "--name", "P", "--description", "Desc"},
			want: AddProjectFlags{Name: "P", Description: "Desc"},
		},
		{
			name:    "missing name",
			args:    []string{"skillvault", "add-project"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseAddProjectFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAddProjectFlags failed: %v", err)
			}
			if flags.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", flags.Name, tt.want.Name)
			}
			if flags.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", flags.Description, tt.want.Description)
			}
		})
	}
}

func TestParseSessionWrapFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    SessionWrapFlags
		wantErr bool
	}{
		{
			name: "required flags",
			args: []string{"skillvault", "session-wrap", "--summary", "Completed auth"},
			want: SessionWrapFlags{Summary: "Completed auth"},
		},
		{
			name: "all flags",
			args: []string{"skillvault", "session-wrap",
				"--project", "myapp",
				"--summary", "Added JWT",
				"--decisions", "use JWT,no sessions",
				"--pending", "add refresh token",
				"--learnings", "JWT expiry must be short",
			},
			want: SessionWrapFlags{
				Project: "myapp", Summary: "Added JWT",
				Decisions: "use JWT,no sessions", Pending: "add refresh token",
				Learnings: "JWT expiry must be short",
			},
		},
		{
			name:    "missing summary",
			args:    []string{"skillvault", "session-wrap", "--project", "p"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseSessionWrapFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSessionWrapFlags failed: %v", err)
			}
			if flags.Summary != tt.want.Summary {
				t.Errorf("Summary = %q, want %q", flags.Summary, tt.want.Summary)
			}
			if flags.Project != tt.want.Project {
				t.Errorf("Project = %q, want %q", flags.Project, tt.want.Project)
			}
			if flags.Decisions != tt.want.Decisions {
				t.Errorf("Decisions = %q, want %q", flags.Decisions, tt.want.Decisions)
			}
			if flags.Pending != tt.want.Pending {
				t.Errorf("Pending = %q, want %q", flags.Pending, tt.want.Pending)
			}
		})
	}
}

func TestParseGetEntryFlags(t *testing.T) {
	flags, err := ParseGetEntryFlags([]string{"skillvault", "get", "sv-abc123"})
	if err != nil {
		t.Fatalf("ParseGetEntryFlags failed: %v", err)
	}
	if flags.ID != "sv-abc123" {
		t.Errorf("ID = %q, want %q", flags.ID, "sv-abc123")
	}

	_, err = ParseGetEntryFlags([]string{"skillvault", "get"})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestParseWorkflowFileFlags(t *testing.T) {
	flags, err := ParseWorkflowFileFlags([]string{"skillvault", "add-workflow", "wf.json"})
	if err != nil {
		t.Fatalf("ParseWorkflowFileFlags failed: %v", err)
	}
	if flags.FilePath != "wf.json" {
		t.Errorf("FilePath = %q, want %q", flags.FilePath, "wf.json")
	}

	_, err = ParseWorkflowFileFlags([]string{"skillvault", "add-workflow"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestParseRenderWorkflowFlags(t *testing.T) {
	flags, err := ParseRenderWorkflowFlags([]string{"skillvault", "render-workflow", "wf-abc"})
	if err != nil {
		t.Fatalf("ParseRenderWorkflowFlags failed: %v", err)
	}
	if flags.WorkflowID != "wf-abc" {
		t.Errorf("WorkflowID = %q, want %q", flags.WorkflowID, "wf-abc")
	}

	_, err = ParseRenderWorkflowFlags([]string{"skillvault", "render-workflow"})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestParseExportFlags(t *testing.T) {
	flags, err := ParseExportFlags([]string{"skillvault", "export"})
	if err != nil {
		t.Fatalf("ParseExportFlags failed: %v", err)
	}
	if flags.OutputPath != "skillvault-export.json" {
		t.Errorf("OutputPath = %q, want default", flags.OutputPath)
	}

	flags, err = ParseExportFlags([]string{"skillvault", "export", "--output", "myexport.json"})
	if err != nil {
		t.Fatalf("ParseExportFlags failed: %v", err)
	}
	if flags.OutputPath != "myexport.json" {
		t.Errorf("OutputPath = %q, want %q", flags.OutputPath, "myexport.json")
	}
}

func TestParseImportFlags(t *testing.T) {
	flags, err := ParseImportFlags([]string{"skillvault", "import", "data.json"})
	if err != nil {
		t.Fatalf("ParseImportFlags failed: %v", err)
	}
	if flags.FilePath != "data.json" {
		t.Errorf("FilePath = %q, want %q", flags.FilePath, "data.json")
	}

	_, err = ParseImportFlags([]string{"skillvault", "import"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestParseImportWorkflowFlags(t *testing.T) {
	// Happy path with --file
	flags, err := ParseImportWorkflowFlags([]string{"skillvault", "import-workflow", "--file", "workflow.yaml"})
	if err != nil {
		t.Fatalf("ParseImportWorkflowFlags failed: %v", err)
	}
	if flags.File != "workflow.yaml" {
		t.Errorf("File = %q, want %q", flags.File, "workflow.yaml")
	}
	if flags.Project != "" {
		t.Errorf("Project = %q, want empty", flags.Project)
	}

	// With --project flag
	flags, err = ParseImportWorkflowFlags([]string{"skillvault", "import-workflow", "--file", "wf.yaml", "--project", "my-proj"})
	if err != nil {
		t.Fatalf("ParseImportWorkflowFlags with project failed: %v", err)
	}
	if flags.File != "wf.yaml" {
		t.Errorf("File = %q, want %q", flags.File, "wf.yaml")
	}
	if flags.Project != "my-proj" {
		t.Errorf("Project = %q, want %q", flags.Project, "my-proj")
	}

	// Missing --file
	_, err = ParseImportWorkflowFlags([]string{"skillvault", "import-workflow"})
	if err == nil {
		t.Fatal("expected error for missing --file")
	}
}

func TestTagItems(t *testing.T) {
	tests := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{"go", []string{"go"}},
		{"go,testing", []string{"go", "testing"}},
		{" go , testing ", []string{"go", "testing"}},
		{"a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			result := TagItems(tt.raw)
			if len(result) != len(tt.want) {
				t.Fatalf("got %v, want %v", result, tt.want)
			}
			for i := range result {
				if result[i] != tt.want[i] {
					t.Errorf("result[%d] = %q, want %q", i, result[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseRunFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    RunFlags
		wantErr bool
	}{
		{
			name: "basic workflow and file",
			args: []string{"skillvault", "run", "my_wf", "input.md"},
			want: RunFlags{Workflow: "my_wf", FilePath: "input.md"},
		},
		{
			name: "stdin input with dash",
			args: []string{"skillvault", "run", "my_wf", "-"},
			want: RunFlags{Workflow: "my_wf", FilePath: "-"},
		},
		{
			name: "with save flag",
			args: []string{"skillvault", "run", "my_wf", "input.md", "--save", "output.md"},
			want: RunFlags{Workflow: "my_wf", FilePath: "input.md", SavePath: "output.md"},
		},
		{
			name: "stdin with save",
			args: []string{"skillvault", "run", "research_article", "-", "--save", "out.md"},
			want: RunFlags{Workflow: "research_article", FilePath: "-", SavePath: "out.md"},
		},
		{
			name:    "missing workflow",
			args:    []string{"skillvault", "run"},
			wantErr: true,
		},
		{
			name:    "missing file path",
			args:    []string{"skillvault", "run", "my_wf"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseRunFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRunFlags failed: %v", err)
			}
			if flags.Workflow != tt.want.Workflow {
				t.Errorf("Workflow = %q, want %q", flags.Workflow, tt.want.Workflow)
			}
			if flags.FilePath != tt.want.FilePath {
				t.Errorf("FilePath = %q, want %q", flags.FilePath, tt.want.FilePath)
			}
			if flags.SavePath != tt.want.SavePath {
				t.Errorf("SavePath = %q, want %q", flags.SavePath, tt.want.SavePath)
			}
		})
	}
}

func TestOutputFormat(t *testing.T) {
	t.Run("human readable table", func(t *testing.T) {
		type row struct{ Name, Type string }
		data := []row{{"E1", "skill"}, {"E2", "prompt"}}
		out := FormatTable(data, []string{"Name", "Type"}, func(r row) []string { return []string{r.Name, r.Type} })
		if !strings.Contains(out, "E1") || !strings.Contains(out, "E2") {
			t.Errorf("table missing data: %s", out)
		}
	})

	t.Run("empty table", func(t *testing.T) {
		type row struct{ Name string }
		out := FormatTable([]row{}, []string{"Name"}, func(r row) []string { return []string{r.Name} })
		if out != "(empty)\n" {
			t.Errorf("expected empty, got %q", out)
		}
	})
}

func TestParseSyncFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    SyncFlags
		wantErr bool
	}{
		{
			name: "sync push with s3 transport",
			args: []string{"skillvault", "sync", "push", "--transport", "s3", "--remote-path", "vault.json.gz"},
			want: SyncFlags{Transport: "s3", RemotePath: "vault.json.gz", DryRun: false},
		},
		{
			name: "sync pull with github transport",
			args: []string{"skillvault", "sync", "pull", "--transport", "github", "--remote-path", "snapshot.gz"},
			want: SyncFlags{Transport: "github", RemotePath: "snapshot.gz", DryRun: false},
		},
		{
			name: "sync push with dry-run",
			args: []string{"skillvault", "sync", "push", "--transport", "s3", "--remote-path", "vault.gz", "--dry-run"},
			want: SyncFlags{Transport: "s3", RemotePath: "vault.gz", DryRun: true},
		},
		{
			name: "sync pull with dry-run (github)",
			args: []string{"skillvault", "sync", "pull", "--transport", "github", "--dry-run"},
			want: SyncFlags{Transport: "github", RemotePath: "", DryRun: true},
		},
		{
			name:    "missing transport flag",
			args:    []string{"skillvault", "sync", "push", "--remote-path", "vault.gz"},
			wantErr: true,
		},
		{
			name:    "unknown transport value",
			args:    []string{"skillvault", "sync", "push", "--transport", "ftp", "--remote-path", "vault.gz"},
			wantErr: true,
		},
		{
			name:    "missing remote-path (non-dry-run pull)",
			args:    []string{"skillvault", "sync", "pull", "--transport", "github"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseSyncFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSyncFlags failed: %v", err)
			}
			if flags.Transport != tt.want.Transport {
				t.Errorf("Transport = %q, want %q", flags.Transport, tt.want.Transport)
			}
			if flags.RemotePath != tt.want.RemotePath {
				t.Errorf("RemotePath = %q, want %q", flags.RemotePath, tt.want.RemotePath)
			}
			if flags.DryRun != tt.want.DryRun {
				t.Errorf("DryRun = %v, want %v", flags.DryRun, tt.want.DryRun)
			}
		})
	}
}

func TestParseCommand_SetupVectors(t *testing.T) {
	cmd, err := ParseCommand([]string{"skillvault", "setup-vectors", "/path/to/glove.txt"})
	if err != nil {
		t.Fatalf("ParseCommand failed: %v", err)
	}
	if cmd != "setup-vectors" {
		t.Errorf("cmd = %q, want 'setup-vectors'", cmd)
	}

	_, err = ParseCommand([]string{"skillvault", "setup-vectors"})
	if err == nil {
		t.Fatal("expected error for missing glove path")
	}
}

func TestParseCommand_ReindexEmbeddings(t *testing.T) {
	cmd, err := ParseCommand([]string{"skillvault", "reindex-embeddings"})
	if err != nil {
		t.Fatalf("ParseCommand failed: %v", err)
	}
	if cmd != "reindex-embeddings" {
		t.Errorf("cmd = %q, want 'reindex-embeddings'", cmd)
	}
}

func TestParseSetupVectorsFlags(t *testing.T) {
	flags, err := ParseSetupVectorsFlags([]string{"skillvault", "setup-vectors", "/tmp/glove.6B.300d.txt"})
	if err != nil {
		t.Fatalf("ParseSetupVectorsFlags failed: %v", err)
	}
	if flags.Path != "/tmp/glove.6B.300d.txt" {
		t.Errorf("Path = %q, want '/tmp/glove.6B.300d.txt'", flags.Path)
	}

	_, err = ParseSetupVectorsFlags([]string{"skillvault", "setup-vectors"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestParseReindexEmbeddingsFlags(t *testing.T) {
	flags, err := ParseReindexEmbeddingsFlags([]string{"skillvault", "reindex-embeddings"})
	if err != nil {
		t.Fatalf("ParseReindexEmbeddingsFlags failed: %v", err)
	}
	_ = flags // struct has no fields currently
}

func TestParseSearchFlags_Vector(t *testing.T) {
	// --vector flag should default to false.
	flags, err := ParseSearchFlags([]string{"skillvault", "search", "ml"})
	if err != nil {
		t.Fatalf("ParseSearchFlags failed: %v", err)
	}
	if flags.Vector {
		t.Error("Vector should default to false")
	}

	// --vector flag explicitly set.
	flags, err = ParseSearchFlags([]string{"skillvault", "search", "ml", "--vector"})
	if err != nil {
		t.Fatalf("ParseSearchFlags with --vector failed: %v", err)
	}
	if !flags.Vector {
		t.Error("Vector should be true when --vector flag is present")
	}

	// Combined with other flags.
	flags, err = ParseSearchFlags([]string{"skillvault", "search", "ml", "--vector", "--limit", "5"})
	if err != nil {
		t.Fatalf("ParseSearchFlags with --vector and --limit failed: %v", err)
	}
	if !flags.Vector {
		t.Error("Vector should be true")
	}
	if flags.Limit != 5 {
		t.Errorf("Limit = %d, want 5", flags.Limit)
	}
}
