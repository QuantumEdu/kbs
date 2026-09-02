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
	case "env":
		traceCmd("env")
		jsonOutput := false
		for _, arg := range os.Args[2:] {
			if arg == "--json" || arg == "-json" {
				jsonOutput = true
			}
		}
		if err := cli.RunEnv(os.Stdout, jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
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
	case "mcp-register":
		traceCmd("mcp-register")
		clientFilter := "all"
		customPath := ""
		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "--client" && i+1 < len(os.Args) {
				clientFilter = os.Args[i+1]
				i++
			} else if os.Args[i] == "--path" && i+1 < len(os.Args) {
				customPath = os.Args[i+1]
				i++
			}
		}
		if err := cli.RegisterMCP(os.Stdout, clientFilter, customPath); err != nil {
			fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
			os.Exit(1)
		}
	case "telemetry":
		traceCmd("telemetry")
		if len(os.Args) < 3 {
			if err := cli.RunTelemetryService(os.Stdout, "status"); err != nil {
				fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		subCmd := os.Args[2]
		switch subCmd {
		case "service":
			action := "status"
			if len(os.Args) > 3 {
				action = os.Args[3]
			}
			if err := cli.RunTelemetryService(os.Stdout, action); err != nil {
				fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
				os.Exit(1)
			}
		case "status", "start", "stop", "restart", "install-service":
			if err := cli.RunTelemetryService(os.Stdout, subCmd); err != nil {
				fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
				os.Exit(1)
			}
		case "install-hooks", "hooks":
			if err := cli.RunTelemetryInstallHooks(os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "[sk-vault] unknown telemetry command: %s (expected 'service' or 'install-hooks')\n", subCmd)
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
