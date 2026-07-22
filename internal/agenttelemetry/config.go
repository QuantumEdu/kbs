package agenttelemetry

import (
	"os"
	"path/filepath"
)

// Config holds daemon configuration from environment variables.
type Config struct {
	SocketPath string // Unix socket path
	DBPath     string // SQLite database path
}

// DefaultConfig returns a Config populated from environment variables
// with sensible defaults.
func DefaultConfig() Config {
	return Config{
		SocketPath: socketPath(),
		DBPath:     dbPath(),
	}
}

func socketPath() string {
	if v := os.Getenv("TELEMETRY_SOCKET"); v != "" {
		return v
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "telemetryd.sock")
	}
	return "/tmp/telemetryd.sock"
}

func dbPath() string {
	if v := os.Getenv("TELEMETRY_DB_PATH"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", ".telemetry", "telemetry.db")
	}
	return filepath.Join(home, ".telemetry", "telemetry.db")
}
