package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTelemetryService_Status(t *testing.T) {
	var buf bytes.Buffer
	if err := RunTelemetryService(&buf, "status"); err != nil {
		t.Fatalf("RunTelemetryService(status) failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "SkillVault Telemetry Daemon Service") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "State:") {
		t.Errorf("expected State field, got: %s", out)
	}
}

func TestRunTelemetryInstallHooks(t *testing.T) {
	var buf bytes.Buffer
	if err := RunTelemetryInstallHooks(&buf); err != nil {
		t.Fatalf("RunTelemetryInstallHooks failed: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	codexHook := filepath.Join(home, ".local", "bin", "codex-telemetry")
	if fi, err := os.Stat(codexHook); err != nil || fi.Mode()&0111 == 0 {
		t.Errorf("expected executable hook at %s: %v", codexHook, err)
	}

	opencodeHook := filepath.Join(home, ".local", "bin", "opencode-telemetry")
	if fi, err := os.Stat(opencodeHook); err != nil || fi.Mode()&0111 == 0 {
		t.Errorf("expected executable hook at %s: %v", opencodeHook, err)
	}
}
