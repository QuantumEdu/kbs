package agenttelemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SocketPath == "" {
		t.Error("SocketPath should not be empty")
	}
	if cfg.DBPath == "" {
		t.Error("DBPath should not be empty")
	}
}

func TestConfigEnvOverride(t *testing.T) {
	t.Setenv("TELEMETRY_SOCKET", "/custom/socket.sock")
	t.Setenv("TELEMETRY_DB_PATH", "/custom/db.sqlite")

	cfg := DefaultConfig()
	if cfg.SocketPath != "/custom/socket.sock" {
		t.Errorf("SocketPath = %q, want /custom/socket.sock", cfg.SocketPath)
	}
	if cfg.DBPath != "/custom/db.sqlite" {
		t.Errorf("DBPath = %q, want /custom/db.sqlite", cfg.DBPath)
	}
}

func TestConfigSocketPathXDG(t *testing.T) {
	// Clear custom env.
	t.Setenv("TELEMETRY_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	path := socketPath()
	expected := filepath.Join("/run/user/1000", "telemetryd.sock")
	if path != expected {
		t.Errorf("socketPath() = %q, want %q", path, expected)
	}
}

func TestConfigDBPathHome(t *testing.T) {
	t.Setenv("TELEMETRY_DB_PATH", "")

	path := dbPath()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".telemetry", "telemetry.db")
	if path != expected {
		t.Errorf("dbPath() = %q, want %q", path, expected)
	}
}
