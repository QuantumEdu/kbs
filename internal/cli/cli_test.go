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
		{"init", []string{"skillvault", "init"}, "init", false},
		{"get", []string{"skillvault", "get", "entry-id"}, "get", false},
		{"search", []string{"skillvault", "search", "query"}, "search", false},
		{"list", []string{"skillvault", "list"}, "list", false},
		{"entry upsert", []string{"skillvault", "entry", "upsert", "file.json"}, "entry-upsert", false},
		{"entry archive", []string{"skillvault", "entry", "archive", "entry-id"}, "entry-archive", false},
		{"project upsert", []string{"skillvault", "project", "upsert", "file.json"}, "project-upsert", false},
		{"project list", []string{"skillvault", "project", "list"}, "project-list", false},
		{"series get", []string{"skillvault", "series", "get", "series-id"}, "series-get", false},
		{"series list", []string{"skillvault", "series", "list"}, "series-list", false},
		{"series upsert", []string{"skillvault", "series", "upsert", "file.json"}, "series-upsert", false},
		{"series replace", []string{"skillvault", "series", "replace", "series-id", "file.json"}, "series-replace", false},
		{"workflow run", []string{"skillvault", "workflow", "run", "wf-id"}, "workflow-run", false},
		{"export", []string{"skillvault", "export", "file.json"}, "export", false},
		{"import", []string{"skillvault", "import", "file.json"}, "import", false},
		{"mcp", []string{"skillvault", "mcp"}, "mcp", false},
		{"save-result", []string{"skillvault", "save-result", "--name", "X", "--content", "Y"}, "save-result", false},
		{"version", []string{"skillvault", "version"}, "version", false},
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

func TestSearchFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantQuery string
	}{
		{"basic", []string{"skillvault", "search", "fastapi"}, "fastapi"},
		{"with project", []string{"skillvault", "search", "fastapi", "--project", "vitacare"}, "fastapi"},
		{"with type", []string{"skillvault", "search", "fastapi", "--type", "skill"}, "fastapi"},
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
}
