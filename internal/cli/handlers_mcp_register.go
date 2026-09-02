package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantum-6/skillvault/internal/security"
)

// MCPRegistrationTarget defines an MCP client configuration target.
type MCPRegistrationTarget struct {
	Client string
	Path   string
	Format string // "mcpServers", "opencode", "toml"
}

// GetRegistrationTargets returns all candidate client targets for registration.
func GetRegistrationTargets() []MCPRegistrationTarget {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	return []MCPRegistrationTarget{
		{
			Client: "Gemini",
			Path:   filepath.Join(home, ".gemini", "config", "mcp_config.json"),
			Format: "mcpServers",
		},
		{
			Client: "Antigravity",
			Path:   filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json"),
			Format: "mcpServers",
		},
		{
			Client: "OpenCode",
			Path:   filepath.Join(home, ".config", "opencode", "opencode.json"),
			Format: "opencode",
		},
		{
			Client: "Codex",
			Path:   filepath.Join(home, ".codex", "config.toml"),
			Format: "toml",
		},
		{
			Client: "Claude Desktop",
			Path:   filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"),
			Format: "mcpServers",
		},
		{
			Client: "Cursor",
			Path:   filepath.Join(home, ".cursor", "mcp.json"),
			Format: "mcpServers",
		},
	}
}

// RegisterMCP registers the skillvault binary in the specified client's MCP configuration.
func RegisterMCP(w io.Writer, clientFilter string, customPath string) error {
	binPath, err := os.Executable()
	if err != nil {
		binPath = "skillvault"
	}
	// Prefer installed tool location if available
	home, _ := os.UserHomeDir()
	if home != "" {
		stdPath := filepath.Join(home, "tools", "skillvault")
		if fi, err := os.Stat(stdPath); err == nil && !fi.IsDir() {
			binPath = stdPath
		}
	}

	targets := GetRegistrationTargets()
	if customPath != "" {
		format := "mcpServers"
		if strings.HasSuffix(customPath, ".toml") {
			format = "toml"
		} else if strings.Contains(customPath, "opencode") {
			format = "opencode"
		}
		targets = []MCPRegistrationTarget{
			{Client: "Custom", Path: customPath, Format: format},
		}
	}

	clientFilter = strings.ToLower(strings.TrimSpace(clientFilter))
	if clientFilter == "" {
		clientFilter = "all"
	}

	fmt.Fprintf(w, "[sk-vault] Registering SkillVault MCP server (%s)...\n", binPath)

	auditor := security.NewMCPConfigAuditor()
	registeredCount := 0

	for _, t := range targets {
		if clientFilter != "all" && !strings.Contains(strings.ToLower(t.Client), clientFilter) {
			continue
		}

		// Only register if target file or its parent directory exists
		parentDir := filepath.Dir(t.Path)
		if _, err := os.Stat(parentDir); err != nil && os.IsNotExist(err) {
			continue
		}

		updated, err := applyMCPRegistration(t, binPath)
		if err != nil {
			fmt.Fprintf(w, "  %-20s ERROR: %v\n", t.Client+":", err)
			continue
		}

		status := "ALREADY CONFIGURED"
		if updated {
			status = "REGISTERED"
		}
		fmt.Fprintf(w, "  %-20s %-18s %s\n", t.Client+":", status, t.Path)
		registeredCount++

		// Validate with audit if it's a JSON config
		if t.Format != "toml" {
			if rep, err := auditor.AuditConfigFile(t.Path); err == nil {
				if rep.CriticalCount > 0 || rep.HighCount > 0 {
					fmt.Fprintf(w, "    [warning] audit detected %d critical, %d high issues\n", rep.CriticalCount, rep.HighCount)
				}
			}
		}
	}

	if registeredCount == 0 {
		fmt.Fprintln(w, "[sk-vault] No compatible client directories found. Specify --path <config-file>.")
	} else {
		fmt.Fprintf(w, "[sk-vault] Successfully verified/registered %d client configuration(s).\n", registeredCount)
	}

	return nil
}

func applyMCPRegistration(target MCPRegistrationTarget, binPath string) (bool, error) {
	switch target.Format {
	case "toml":
		return applyTomlRegistration(target.Path, binPath)
	case "opencode":
		return applyOpenCodeRegistration(target.Path, binPath)
	default:
		return applyMCPServersRegistration(target.Path, binPath)
	}
}

func applyMCPServersRegistration(path string, binPath string) (bool, error) {
	var root map[string]interface{}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse json: %w", err)
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	servers, ok := root["mcpServers"].(map[string]interface{})
	if !ok {
		servers = make(map[string]interface{})
		root["mcpServers"] = servers
	}

	existing, exists := servers["skillvault"].(map[string]interface{})
	if exists && existing["command"] == binPath {
		return false, nil
	}

	servers["skillvault"] = map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp"},
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("format json: %w", err)
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o644)
}

func applyOpenCodeRegistration(path string, binPath string) (bool, error) {
	var root map[string]interface{}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse json: %w", err)
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	mcpMap, ok := root["mcp"].(map[string]interface{})
	if !ok {
		mcpMap = make(map[string]interface{})
		root["mcp"] = mcpMap
	}

	existing, exists := mcpMap["skillvault"].(map[string]interface{})
	if exists {
		if cmdSlice, ok := existing["command"].([]interface{}); ok && len(cmdSlice) > 0 && cmdSlice[0] == binPath {
			return false, nil
		}
	}

	mcpMap["skillvault"] = map[string]interface{}{
		"command": []string{binPath, "mcp"},
		"enabled": true,
		"type":    "local",
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("format json: %w", err)
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o644)
}

func applyTomlRegistration(path string, binPath string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			content := fmt.Sprintf("\n[mcp_servers.skillvault]\ncommand = %q\nargs = [\"mcp\"]\n", binPath)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return false, err
			}
			return true, os.WriteFile(path, []byte(content), 0o644)
		}
		return false, err
	}

	content := string(data)
	if strings.Contains(content, "[mcp_servers.skillvault]") {
		if strings.Contains(content, binPath) {
			return false, nil
		}
	}

	snippet := fmt.Sprintf("\n[mcp_servers.skillvault]\ncommand = %q\nargs = [\"mcp\"]\n", binPath)
	content += snippet

	return true, os.WriteFile(path, []byte(content), 0o644)
}
