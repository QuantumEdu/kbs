package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/quantum-6/skillvault/internal/cli"
)

type topLevelUsageRow struct {
	Command string
	Details string
}

func traceCmd(cmd string) {
	fmt.Fprintf(os.Stderr, "[sk-vault] %s — %s\n", cmd, cli.TopLevelCommandDescription(cmd))
}

func handleHelp(args []string) (bool, int) {
	if len(args) < 2 {
		return false, 0
	}

	if commandID, showCommand, ok := cli.ResolveHelpTopic(args[1:]); ok {
		if showCommand {
			printCommandUsage(os.Stdout, commandID)
		} else {
			printTopLevelUsage(os.Stdout)
		}
		return true, 0
	}

	switch args[1] {
	case "help", "--help", "-h":
		if len(args) == 2 {
			printTopLevelUsage(os.Stdout)
			return true, 0
		}
		id, _, ok := cli.ResolveTopLevelCommand(args[2:])
		if !ok {
			fmt.Fprintf(os.Stderr, "[sk-vault] error: %s\n", cli.UnknownCommandMessage(strings.Join(args[2:], " ")))
			return true, 1
		}
		printCommandUsage(os.Stdout, id)
		return true, 0
	}

	last := args[len(args)-1]
	if last != "--help" && last != "-h" {
		return false, 0
	}

	id, _, ok := cli.ResolveTopLevelCommand(args[1:])
	if !ok {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: %s\n", cli.UnknownCommandMessage(strings.Join(args[1:len(args)-1], " ")))
		return true, 1
	}
	printCommandUsage(os.Stdout, id)
	return true, 0
}

func printTopLevelUsage(w io.Writer) {
	fmt.Fprintf(w, "SkillVault %s\n", version)
	fmt.Fprintln(w, "Local-first knowledge and workflow CLI in the kbs tool suite.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: skillvault <command> [args...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Common paths:")
	fmt.Fprintln(w, "  skillvault setup               Create the local vault")
	fmt.Fprintln(w, "  skillvault setup doctor        Check the vault without changing it")
	fmt.Fprintln(w, "  skillvault doctor              Check whether the vault is ready")
	fmt.Fprintln(w, "  skillvault find \"auth\"         Search for something you saved")
	fmt.Fprintln(w, "  skillvault read <entry-id>     Open one saved entry")
	fmt.Fprintln(w, "  skillvault pending review      Review project deferred work")
	fmt.Fprintln(w, "  skillvault context project     Start from the project context command")
	fmt.Fprintln(w, "  skillvault context --project x Build a compact agent context pack")
	fmt.Fprintln(w, "  skillvault tui                 Open the dashboard UI for projects and pending work")
	fmt.Fprintln(w, "  skillvault backup              Write a dated vault backup")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Discovery shortcuts: `skillvault docs`, `skillvault readme`, `skillvault help doctor`")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands by task:")

	for _, group := range cli.CommandGroups() {
		fmt.Fprintf(w, "\n%s:\n", group.Name)
		for _, row := range topLevelUsageRows(group.Commands) {
			fmt.Fprintf(w, "  %-21s %s\n", row.Command, row.Details)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `skillvault mcp config` to print a ready-to-paste MCP client snippet.")
	fmt.Fprintln(w, "Run `make build-tui` before `skillvault tui` if you want the optional Bubble Tea dashboard.")
	fmt.Fprintln(w, "Run `skillvault help <command>` for focused help with examples.")
}

func printCommandUsage(w io.Writer, id string) {
	command, ok := cli.TopLevelCommandMeta(id)
	if !ok {
		fmt.Fprintf(w, "Unknown command: %s\n", id)
		return
	}

	preferred := cli.PreferredInvocation(command)
	usage := command.Usage
	if usage == "" {
		usage = fmt.Sprintf("skillvault %s", preferred)
	}

	fmt.Fprintf(w, "SkillVault %s\n", version)
	fmt.Fprintf(w, "Command: %s\n", usage)
	fmt.Fprintf(w, "%s\n", command.Description)

	if len(command.Aliases) > 0 {
		fmt.Fprintf(w, "\nAliases: %s\n", strings.Join(command.Aliases, ", "))
	}

	if len(command.Intent) > 0 {
		fmt.Fprintf(w, "\nTry saying: %s\n", strings.Join(command.Intent, "; "))
	}

	if len(command.Examples) > 0 {
		fmt.Fprintln(w, "\nExamples:")
		for _, example := range command.Examples {
			fmt.Fprintf(w, "  %s\n", example)
		}
	}

	if len(command.Notes) > 0 {
		fmt.Fprintln(w, "\nNotes:")
		for _, note := range command.Notes {
			fmt.Fprintf(w, "  - %s\n", note)
		}
	}

	if len(command.Related) > 0 {
		fmt.Fprintln(w, "\nRelated:")
		for _, relatedID := range command.Related {
			if related, ok := cli.TopLevelCommandMeta(relatedID); ok {
				fmt.Fprintf(w, "  skillvault %s  %s\n", cli.PreferredInvocation(related), related.Description)
			}
		}
	}
}

func topLevelUsageRows(commands []cli.CommandMeta) []topLevelUsageRow {
	rows := make([]topLevelUsageRow, 0, len(commands))
	for _, command := range commands {
		rows = append(rows, topLevelUsageRow{
			Command: command.DisplayName,
			Details: formatCommandDetails(command),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Command < rows[j].Command
	})

	return rows
}

func formatCommandDetails(command cli.CommandMeta) string {
	if len(command.Aliases) == 0 {
		return command.Description
	}

	return fmt.Sprintf("%s (aliases: %s)", command.Description, strings.Join(command.Aliases, ", "))
}

func printMCPConfigSnippet(w io.Writer) error {
	type serverConfig struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}

	type configSnippet struct {
		MCPServers map[string]serverConfig `json:"mcpServers"`
	}

	commandPath := "skillvault"
	if execPath, err := os.Executable(); err == nil {
		commandPath = execPath
	}
	if absPath, err := filepath.Abs(commandPath); err == nil {
		commandPath = absPath
	}

	snippet := configSnippet{
		MCPServers: map[string]serverConfig{
			"skillvault": {
				Command: commandPath,
				Args:    []string{"mcp"},
			},
		},
	}

	fmt.Fprintln(w, "Use this under `mcpServers` in opencode.json or claude_desktop_config.json:")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(snippet)
}
