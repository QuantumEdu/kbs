package agenttelemetry

import "os"

// EnvEnabled returns true if TELEMETRY_ENABLED is "true" or "1".
func EnvEnabled() bool {
	v := os.Getenv("TELEMETRY_ENABLED")
	return v == "true" || v == "1"
}

// EnvSocketPath returns the daemon Unix socket path from TELEMETRY_SOCKET
// env var, or the default path matching config.go socketPath() logic.
func EnvSocketPath() string {
	return socketPath()
}

// EnvStorePrompts returns true if TELEMETRY_STORE_PROMPTS is "true" or "1".
func EnvStorePrompts() bool {
	return storePrompts()
}
