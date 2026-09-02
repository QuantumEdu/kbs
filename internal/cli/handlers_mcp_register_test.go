package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterMCP_MCPServersJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "mcp_config.json")
	initial := `{"mcpServers":{"codegraph":{"command":"codegraph"}}}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := RegisterMCP(&buf, "all", cfgPath); err != nil {
		t.Fatalf("RegisterMCP failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "skillvault") {
		t.Errorf("expected skillvault in config: %s", content)
	}
	if !strings.Contains(content, "codegraph") {
		t.Errorf("expected codegraph to be preserved: %s", content)
	}

	// Idempotency check
	buf.Reset()
	if err := RegisterMCP(&buf, "all", cfgPath); err != nil {
		t.Fatalf("RegisterMCP second run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "ALREADY CONFIGURED") {
		t.Errorf("expected ALREADY CONFIGURED on second run, got: %s", buf.String())
	}
}

func TestRegisterMCP_OpenCodeJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "opencode.json")
	initial := `{"mcp":{"codegraph":{"type":"local"}}}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := RegisterMCP(&buf, "all", cfgPath); err != nil {
		t.Fatalf("RegisterMCP OpenCode failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "skillvault") {
		t.Errorf("expected skillvault in config: %s", content)
	}
	if !strings.Contains(content, "codegraph") {
		t.Errorf("expected codegraph preserved: %s", content)
	}
}

func TestRegisterMCP_Toml(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	initial := `model = "gpt-5.6-sol"`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := RegisterMCP(&buf, "all", cfgPath); err != nil {
		t.Fatalf("RegisterMCP TOML failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "[mcp_servers.skillvault]") {
		t.Errorf("expected [mcp_servers.skillvault] in config: %s", content)
	}
	if !strings.Contains(content, "model = \"gpt-5.6-sol\"") {
		t.Errorf("expected initial content preserved: %s", content)
	}
}
