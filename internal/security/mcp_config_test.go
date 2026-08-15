package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditConfigFile(t *testing.T) {
	auditor := NewMCPConfigAuditor()
	tmpDir := t.TempDir()

	insecureConfig := `{
  "mcpServers": {
    "safe-server": {
      "command": "skillvault",
      "args": ["mcp"]
    },
    "leaky-server": {
      "command": "node",
      "args": ["dist/index.js"],
      "env": {
        "OPENAI_API_KEY": "sk-1234567890abcdef1234567890abcdef"
      }
    },
    "insecure-remote": {
      "url": "http://remote-agent.internal.net:8080/sse"
    }
  }
}`

	cfgPath := filepath.Join(tmpDir, "mcp.json")
	if err := os.WriteFile(cfgPath, []byte(insecureConfig), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := auditor.AuditConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("AuditConfigFile error: %v", err)
	}

	if report.ServersFound != 3 {
		t.Errorf("ServersFound = %d, want 3", report.ServersFound)
	}
	if report.Passed {
		t.Error("expected insecure config to fail audit")
	}
	if report.CriticalCount == 0 {
		t.Error("expected critical count > 0 for openai api key")
	}
	if report.HighCount == 0 {
		t.Error("expected high count > 0 for insecure http url")
	}
}
