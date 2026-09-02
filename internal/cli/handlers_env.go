package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/quantum-6/skillvault/internal/agenttelemetry"
	"github.com/quantum-6/skillvault/internal/version"
)

// EnvReport represents the instant topology information of the local installation.
type EnvReport struct {
	Version     string `json:"version"`
	VaultHome   string `json:"vault_home"`
	Database    string `json:"database"`
	DBSymlink   string `json:"db_symlink"`
	Socket      string `json:"socket"`
	ExportsDir  string `json:"exports_dir"`
	TelemetryDB string `json:"telemetry_db"`
	BinaryPath  string `json:"binary_path"`
}

// GetEnvReport collects canonical system paths without opening SQLite or heavy resources.
func GetEnvReport() EnvReport {
	vd := vaultDir()
	telemCfg := agenttelemetry.DefaultConfig()
	bin, err := os.Executable()
	if err != nil {
		bin = "skillvault"
	}
	return EnvReport{
		Version:     version.Display(),
		VaultHome:   vd,
		Database:    dbPath(),
		DBSymlink:   filepath.Join(vd, "skillvault.db"),
		Socket:      telemCfg.SocketPath,
		ExportsDir:  filepath.Join(vd, "exports"),
		TelemetryDB: telemCfg.DBPath,
		BinaryPath:  bin,
	}
}

// RunEnv prints the system topology in human-readable or JSON format.
func RunEnv(w io.Writer, jsonOutput bool) error {
	rep := GetEnvReport()
	if jsonOutput {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}

	fmt.Fprintln(w, "SkillVault Environment Topology")
	fmt.Fprintf(w, "  Version:      %s\n", rep.Version)
	fmt.Fprintf(w, "  Vault home:   %s\n", rep.VaultHome)
	fmt.Fprintf(w, "  Database:     %s\n", rep.Database)
	fmt.Fprintf(w, "  DB Symlink:   %s\n", rep.DBSymlink)
	fmt.Fprintf(w, "  Exports:      %s\n", rep.ExportsDir)
	fmt.Fprintf(w, "  Socket:       %s\n", rep.Socket)
	fmt.Fprintf(w, "  Telemetry DB: %s\n", rep.TelemetryDB)
	fmt.Fprintf(w, "  Binary path:  %s\n", rep.BinaryPath)
	return nil
}
