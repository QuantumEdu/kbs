package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetEnvReport(t *testing.T) {
	rep := GetEnvReport()
	if rep.Version == "" {
		t.Error("expected non-empty Version")
	}
	if rep.VaultHome == "" {
		t.Error("expected non-empty VaultHome")
	}
	if !strings.HasSuffix(rep.Database, "vault.db") {
		t.Errorf("expected Database to end with vault.db, got %q", rep.Database)
	}
	if !strings.HasSuffix(rep.DBSymlink, "skillvault.db") {
		t.Errorf("expected DBSymlink to end with skillvault.db, got %q", rep.DBSymlink)
	}
}

func TestRunEnv_Output(t *testing.T) {
	var buf bytes.Buffer
	if err := RunEnv(&buf, false); err != nil {
		t.Fatalf("RunEnv(text) failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "SkillVault Environment Topology") {
		t.Errorf("expected text header, got: %s", out)
	}
	if !strings.Contains(out, "DB Symlink:") {
		t.Errorf("expected DB Symlink entry, got: %s", out)
	}

	buf.Reset()
	if err := RunEnv(&buf, true); err != nil {
		t.Fatalf("RunEnv(json) failed: %v", err)
	}
	var parsed EnvReport
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if parsed.VaultHome == "" || parsed.Database == "" {
		t.Errorf("expected parsed fields to be populated: %+v", parsed)
	}
}

func TestEnsureDBSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "vault.db")
	if err := os.WriteFile(targetPath, []byte("sqlite data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureDBSymlink(tmpDir); err != nil {
		t.Fatalf("EnsureDBSymlink failed: %v", err)
	}

	linkPath := filepath.Join(tmpDir, "skillvault.db")
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("stat on symlink failed: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected file to be a symlink: %v", fi.Mode())
	}

	// Verify idempotency
	if err := EnsureDBSymlink(tmpDir); err != nil {
		t.Fatalf("EnsureDBSymlink idempotency failed: %v", err)
	}
}
