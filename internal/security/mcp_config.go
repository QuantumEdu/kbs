package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MCPConfigFinding details a security issue in an MCP client configuration.
type MCPConfigFinding struct {
	ServerName   string `json:"server_name"`
	RuleID       string `json:"rule_id"`
	Severity     string `json:"severity"` // "critical", "high", "medium", "low"
	Description  string `json:"description"`
	Location     string `json:"location"` // e.g. "env.OPENAI_API_KEY", "args[2]", "url"
	MatchSnippet string `json:"match_snippet"`
	Suggestion   string `json:"suggestion"`
}

// MCPConfigAuditReport summarizes the security analysis of one or more MCP configs.
type MCPConfigAuditReport struct {
	ConfigPath    string             `json:"config_path"`
	ClientType    string             `json:"client_type"`
	ServersFound  int                `json:"servers_found"`
	Findings      []MCPConfigFinding `json:"findings"`
	Passed        bool               `json:"passed"`
	CriticalCount int                `json:"critical_count"`
	HighCount     int                `json:"high_count"`
	MediumCount   int                `json:"medium_count"`
	LowCount      int                `json:"low_count"`
}

// MCPConfigAuditor scans MCP client configuration files for security vulnerabilities.
type MCPConfigAuditor struct {
	auditor *Auditor
}

// NewMCPConfigAuditor creates a new MCPConfigAuditor.
func NewMCPConfigAuditor() *MCPConfigAuditor {
	return &MCPConfigAuditor{
		auditor: NewAuditor(),
	}
}

// KnownConfigPath represents a default configuration location for a specific client.
type KnownConfigPath struct {
	Client string
	Path   string
}

// GetKnownConfigPaths returns standard MCP config paths for Cursor, Claude, Windsurf, OpenCode.
func GetKnownConfigPaths() []KnownConfigPath {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	return []KnownConfigPath{
		{
			Client: "Cursor",
			Path:   filepath.Join(home, ".cursor", "mcp.json"),
		},
		{
			Client: "Claude Desktop",
			Path:   filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"),
		},
		{
			Client: "Claude Desktop (macOS)",
			Path:   filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		},
		{
			Client: "Windsurf",
			Path:   filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"),
		},
		{
			Client: "OpenCode",
			Path:   filepath.Join(home, ".config", "opencode", "opencode.json"),
		},
	}
}

// AuditConfigFile scans an MCP config JSON file.
func (a *MCPConfigAuditor) AuditConfigFile(path string) (MCPConfigAuditReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MCPConfigAuditReport{}, fmt.Errorf("read mcp config %q: %w", path, err)
	}

	report := MCPConfigAuditReport{
		ConfigPath: path,
		ClientType: detectClientType(path),
		Findings:   []MCPConfigFinding{},
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return report, fmt.Errorf("parse json in %q: %w", path, err)
	}

	// Look for servers under "mcpServers" (Claude/Cursor) or "mcp" (OpenCode)
	var servers map[string]interface{}
	if s, ok := root["mcpServers"].(map[string]interface{}); ok {
		servers = s
	} else if s, ok := root["mcp"].(map[string]interface{}); ok {
		servers = s
	} else {
		// Try treating root as servers map
		servers = root
	}

	for serverName, serverDefRaw := range servers {
		serverDef, ok := serverDefRaw.(map[string]interface{})
		if !ok {
			continue
		}
		report.ServersFound++
		a.auditServerDef(serverName, serverDef, &report)
	}

	for _, f := range report.Findings {
		switch f.Severity {
		case "critical":
			report.CriticalCount++
		case "high":
			report.HighCount++
		case "medium":
			report.MediumCount++
		case "low":
			report.LowCount++
		}
	}

	report.Passed = report.CriticalCount == 0 && report.HighCount == 0
	return report, nil
}

func (a *MCPConfigAuditor) auditServerDef(serverName string, def map[string]interface{}, report *MCPConfigAuditReport) {
	// 1. Audit Environment variables
	if env, ok := def["env"].(map[string]interface{}); ok {
		for envKey, envVal := range env {
			valStr := fmt.Sprintf("%v", envVal)
			sub := a.auditor.AuditContent(envKey, valStr)
			for _, f := range sub.Findings {
				report.Findings = append(report.Findings, MCPConfigFinding{
					ServerName:   serverName,
					RuleID:       f.RuleID,
					Severity:     f.Severity,
					Description:  f.Description,
					Location:     fmt.Sprintf("env.%s", envKey),
					MatchSnippet: f.MatchSnippet,
					Suggestion:   "Move plaintext secret to q-secrets and pass dynamically",
				})
			}
		}
	}

	// 2. Audit Command and Args
	if cmd, ok := def["command"].(string); ok {
		sub := a.auditor.AuditContent("command", cmd)
		for _, f := range sub.Findings {
			report.Findings = append(report.Findings, MCPConfigFinding{
				ServerName:   serverName,
				RuleID:       f.RuleID,
				Severity:     f.Severity,
				Description:  f.Description,
				Location:     "command",
				MatchSnippet: f.MatchSnippet,
				Suggestion:   f.Suggestion,
			})
		}
	}

	if args, ok := def["args"].([]interface{}); ok {
		for idx, arg := range args {
			argStr := fmt.Sprintf("%v", arg)
			sub := a.auditor.AuditContent(fmt.Sprintf("args[%d]", idx), argStr)
			for _, f := range sub.Findings {
				report.Findings = append(report.Findings, MCPConfigFinding{
					ServerName:   serverName,
					RuleID:       f.RuleID,
					Severity:     f.Severity,
					Description:  f.Description,
					Location:     fmt.Sprintf("args[%d]", idx),
					MatchSnippet: f.MatchSnippet,
					Suggestion:   f.Suggestion,
				})
			}
		}
	}

	// 3. Audit URL for insecure HTTP
	if urlVal, ok := def["url"].(string); ok {
		if strings.HasPrefix(urlVal, "http://") && !strings.Contains(urlVal, "localhost") && !strings.Contains(urlVal, "127.0.0.1") {
			report.Findings = append(report.Findings, MCPConfigFinding{
				ServerName:   serverName,
				RuleID:       "NET-001",
				Severity:     "high",
				Description:  "Insecure unencrypted HTTP endpoint for remote MCP server",
				Location:     "url",
				MatchSnippet: urlVal,
				Suggestion:   "Use HTTPS with valid TLS certificates for remote MCP connections",
			})
		}
	}
}

func detectClientType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "cursor"):
		return "Cursor"
	case strings.Contains(lower, "claude"):
		return "Claude Desktop"
	case strings.Contains(lower, "windsurf"):
		return "Windsurf"
	case strings.Contains(lower, "opencode"):
		return "OpenCode"
	case strings.Contains(lower, "antigravity") || strings.Contains(lower, "gemini"):
		return "Antigravity"
	default:
		return "Custom MCP Config"
	}
}
