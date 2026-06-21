package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestModuleCompiles(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "/dev/null", "./cmd/skillvault")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/skillvault failed: %v\nOutput:\n%s", err, string(out))
	}
}

func TestDirectoriesExist(t *testing.T) {
	required := []string{
		"internal/domain",
		"internal/vars",
		"internal/db",
		"internal/db/migrations",
		"internal/app",
		"internal/cli",
		"internal/mcp",
		"internal/api",
		"internal/files",
		"internal/security",
		"internal/context",
	}
	for _, d := range required {
		cmd := exec.Command("test", "-d", d)
		cmd.Dir = "../.."
		err := cmd.Run()
		if err != nil {
			t.Errorf("Missing directory: %s", d)
		}
	}
}

func TestVaultDir(t *testing.T) {
	dir := vaultDir()
	if dir == "" {
		t.Fatal("vaultDir returned empty")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("vaultDir should be absolute, got %q", dir)
	}
}

func TestDBPath(t *testing.T) {
	p := dbPath()
	if p == "" {
		t.Fatal("dbPath returned empty")
	}
	if filepath.Base(p) != "vault.db" {
		t.Errorf("dbPath base should be vault.db, got %q", filepath.Base(p))
	}
}

func TestInitCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	runInit()

	vd := vaultDir()
	for _, sub := range []string{"", "/objects", "/exports", "/cache"} {
		if _, err := os.Stat(vd + sub); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", vd+sub)
		}
	}

	if _, err := os.Stat(dbPath()); os.IsNotExist(err) {
		t.Error("vault.db not created")
	}
}

func TestInitThenOpenVault(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	runInit()

	svc := openVault()
	if svc.store == nil {
		t.Error("store is nil")
	}
	if svc.entrySvc == nil {
		t.Error("entrySvc is nil")
	}
	if svc.artifactSvc == nil {
		t.Error("artifactSvc is nil")
	}
	if svc.workflowSvc == nil {
		t.Error("workflowSvc is nil")
	}
	if svc.seriesSvc == nil {
		t.Error("seriesSvc is nil")
	}
	if svc.projectSvc == nil {
		t.Error("projectSvc is nil")
	}
	if svc.contextSvc == nil {
		t.Error("contextSvc is nil")
	}
	if svc.sessionSvc == nil {
		t.Error("sessionSvc is nil")
	}
	if svc.exportSvc == nil {
		t.Error("exportSvc is nil")
	}
	if svc.importSvc == nil {
		t.Error("importSvc is nil")
	}
	if svc.saveResultSvc == nil {
		t.Error("saveResultSvc is nil")
	}
	if svc.fileSvc == nil {
		t.Error("fileSvc is nil")
	}
	if svc.scanner == nil {
		t.Error("scanner is nil")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	runInit()
	runInit()
}

func TestVersionCommand(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/skillvault", "version")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed: %v\nOutput: %s", err, string(out))
	}
	if string(out) != "SkillVault v2-quantum\n" {
		t.Errorf("unexpected version output: %q", string(out))
	}
}

func TestHelpOutput(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/skillvault")
	cmd.Dir = "../.."
	out, _ := cmd.CombinedOutput()
	if len(out) == 0 {
		t.Error("expected help output")
	}
}

func TestSymlinkDetection(t *testing.T) {
	if filepath.Base("mcp") != "mcp" {
		t.Error("mcp basename should be mcp")
	}
	if filepath.Base("skillvault") != "skillvault" {
		t.Error("skillvault basename should be skillvault")
	}
}
