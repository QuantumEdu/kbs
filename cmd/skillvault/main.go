package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/quantum-6/skillvault/internal/cli"
	appversion "github.com/quantum-6/skillvault/internal/version"
)

var version = appversion.Display()

func main() {
	if filepath.Base(os.Args[0]) == "mcp" {
		cli.RunMCP()
		return
	}

	if handled, exitCode := handleHelp(os.Args); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}
	os.Args = cli.NormalizeArgs(os.Args)

	if len(os.Args) < 2 {
		printTopLevelUsage(os.Stderr)
		os.Exit(1)
	}

	cmd, err := cli.ParseCommand(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "version":
		traceCmd("version")
		fmt.Println("SkillVault", version)
	case "init":
		traceCmd("init")
		cli.RunInit()
	case "doctor":
		traceCmd("doctor")
		if !cli.RunDoctor(os.Stdout) {
			os.Exit(1)
		}
	case "mcp":
		traceCmd("mcp")
		cli.RunMCP()
	case "update":
		traceCmd("update")
		cli.RunUpdate()
	case "install-telemetry":
		traceCmd("install-telemetry")
		cli.InstallTelemetry(cli.DefaultInstallDir())
	case "mcp-config":
		traceCmd("mcp-config")
		if err := printMCPConfigSnippet(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "[sk-vault] error: print MCP config snippet: %v\n", err)
			os.Exit(1)
		}
	case "secrets":
		traceCmd("secrets")
		cli.RunSecrets()
	case "tui":
		traceCmd("tui")
		runTUI(cli.OpenVault())
	default:
		traceCmd(cmd)
		cli.Run(cmd, os.Args)
	}
}
