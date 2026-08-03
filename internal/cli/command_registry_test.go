package cli

import (
	"strings"
	"testing"
)

func TestTopLevelCommandMeta_DisplayNamesAndAliases(t *testing.T) {
	tests := []struct {
		id          string
		displayName string
		aliases     []string
		group       string
	}{
		{id: "memory-index", displayName: "memory index", aliases: []string{"memory-index"}, group: "Maintenance"},
		{id: "entry-history", displayName: "entry history", aliases: []string{"entry-history"}, group: "Maintenance"},
		{id: "mcp-config", displayName: "mcp config", aliases: []string{"mcp-config", "setup mcp", "mcp setup"}, group: "Integrations"},
		{id: "search", displayName: "search", aliases: []string{"find", "lookup", "look up", "search memory"}, group: "Find"},
		{id: "doctor", displayName: "doctor", aliases: []string{"check", "setup doctor", "doctor setup", "check setup", "check vault"}, group: "Setup"},
		{id: "pending", displayName: "pending", aliases: []string{"todo"}, group: "Context"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			meta, ok := TopLevelCommandMeta(tt.id)
			if !ok {
				t.Fatalf("missing command metadata for %q", tt.id)
			}
			if meta.DisplayName != tt.displayName {
				t.Fatalf("DisplayName = %q, want %q", meta.DisplayName, tt.displayName)
			}
			if len(meta.Aliases) != len(tt.aliases) {
				t.Fatalf("Aliases len = %d, want %d", len(meta.Aliases), len(tt.aliases))
			}
			if meta.Group != tt.group {
				t.Fatalf("Group = %q, want %q", meta.Group, tt.group)
			}
			for i := range tt.aliases {
				if meta.Aliases[i] != tt.aliases[i] {
					t.Fatalf("Aliases[%d] = %q, want %q", i, meta.Aliases[i], tt.aliases[i])
				}
			}
		})
	}
}

func TestNormalizeArgs_ResolvesIntentAliases(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "find", args: []string{"skillvault", "find", "auth"}, want: []string{"skillvault", "search", "auth"}},
		{name: "read", args: []string{"skillvault", "read", "entry-1"}, want: []string{"skillvault", "get", "entry-1"}},
		{name: "project list", args: []string{"skillvault", "project", "list"}, want: []string{"skillvault", "list-projects"}},
		{name: "project start", args: []string{"skillvault", "project", "start", "--name", "Codex"}, want: []string{"skillvault", "add-project", "--name", "Codex"}},
		{name: "backup all", args: []string{"skillvault", "backup", "all"}, want: []string{"skillvault", "backup"}},
		{name: "setup mcp", args: []string{"skillvault", "setup", "mcp"}, want: []string{"skillvault", "mcp-config"}},
		{name: "workflow import", args: []string{"skillvault", "workflow", "import", "--file", "wf.yaml"}, want: []string{"skillvault", "import-workflow", "--file", "wf.yaml"}},
		{name: "setup doctor", args: []string{"skillvault", "setup", "doctor"}, want: []string{"skillvault", "doctor"}},
		{name: "context project", args: []string{"skillvault", "context", "project", "--project", "codex"}, want: []string{"skillvault", "get-context", "--project", "codex"}},
		{name: "open entry", args: []string{"skillvault", "open", "entry-1"}, want: []string{"skillvault", "get", "entry-1"}},
		{name: "todo add", args: []string{"skillvault", "todo", "add", "--project", "codex", "Update presentation"}, want: []string{"skillvault", "pending", "add", "--project", "codex", "Update presentation"}},
		{name: "fuzzy doctor typo", args: []string{"skillvault", "docter"}, want: []string{"skillvault", "doctor"}},
		{name: "fuzzy projects typo", args: []string{"skillvault", "projcts"}, want: []string{"skillvault", "list-projects"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestUnknownCommandMessageSuggestsIntentFirstAlternatives(t *testing.T) {
	message := UnknownCommandMessage("projct")
	if !strings.Contains(message, "skillvault project start") {
		t.Fatalf("expected project start suggestion, got: %s", message)
	}
	if !strings.Contains(message, "matches: project") {
		t.Fatalf("expected intent match hint, got: %s", message)
	}
	if !strings.Contains(message, "skillvault projects") {
		t.Fatalf("expected projects suggestion, got: %s", message)
	}
	if !strings.Contains(message, "skillvault help <command>") {
		t.Fatalf("expected help hint, got: %s", message)
	}
}

func TestResolveHelpTopic(t *testing.T) {
	commandID, showCommand, ok := ResolveHelpTopic([]string{"docs"})
	if !ok || showCommand || commandID != "" {
		t.Fatalf("expected docs to resolve to top-level help, got ok=%v showCommand=%v commandID=%q", ok, showCommand, commandID)
	}

	commandID, showCommand, ok = ResolveHelpTopic([]string{"mcp", "config", "help"})
	if !ok || !showCommand || commandID != "mcp-config" {
		t.Fatalf("expected mcp config help alias, got ok=%v showCommand=%v commandID=%q", ok, showCommand, commandID)
	}
}
