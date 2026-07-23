package agenttelemetry

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds daemon configuration from environment variables.
type Config struct {
	SocketPath        string   // Unix socket path
	DBPath            string   // SQLite database path
	SaltPath          string   // Path to salt file
	RedactionPatterns []string // Custom redaction regex patterns from env
	StorePrompts      bool     // Opt-in prompt storage
}

// DefaultConfig returns a Config populated from environment variables
// with sensible defaults.
func DefaultConfig() Config {
	return Config{
		SocketPath:        socketPath(),
		DBPath:            dbPath(),
		SaltPath:          saltPath(),
		RedactionPatterns: redactionPatterns(),
		StorePrompts:      storePrompts(),
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

func saltPath() string {
	if v := os.Getenv("TELEMETRY_SALT_PATH"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", ".telemetry", "salt")
	}
	return filepath.Join(home, ".telemetry", "salt")
}

func redactionPatterns() []string {
	raw := os.Getenv("TELEMETRY_REDACTION_PATTERNS")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var patterns []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

func storePrompts() bool {
	v := os.Getenv("TELEMETRY_STORE_PROMPTS")
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}
