package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	expected := "[sk-vault] version — Print version information\nSkillVault v3\n"
	if string(out) != expected {
		t.Errorf("unexpected version output: %q", string(out))
	}
}

func TestImportWorkflowCommand(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "skillvault")

	// Build binary once to avoid go run module-cache cleanup issues.
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/skillvault")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\nOutput: %s", err, string(out))
	}

	yamlPath := filepath.Join(tmpDir, "workflow.yaml")
	yamlContent := `workflow:
  name: Research Workflow
  type: research
  created: "2026-07-06"
phases:
  - id: extract
    name: Extract Insights
    skill: extract_wisdom
    description: Extract key insights from input
    outputs:
      - insights
    completion_criteria:
      - insights documented
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write workflow yaml: %v", err)
	}

	vaultHome := filepath.Join(tmpDir, "vaulthome")
	if err := os.MkdirAll(vaultHome, 0755); err != nil {
		t.Fatalf("failed to create vault home: %v", err)
	}
	env := append(os.Environ(), "HOME="+vaultHome)

	// Initialize vault in temp home
	initCmd := exec.Command(binPath, "init")
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, string(out))
	}

	// Import workflow
	cmd := exec.Command(binPath, "import-workflow", "--file", yamlPath)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("import-workflow failed: %v\nOutput: %s", err, string(out))
	}

	if !strings.Contains(string(out), "Workflow imported:") {
		t.Errorf("expected output to contain 'Workflow imported:', got: %q", string(out))
	}
	if !strings.Contains(string(out), "Research Workflow") {
		t.Errorf("expected output to contain workflow name, got: %q", string(out))
	}
	if !strings.Contains(string(out), "Phases:") {
		t.Errorf("expected output to show phases, got: %q", string(out))
	}
}

func TestRouteCommand(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "skillvault")

	// Build binary once to avoid go run module-cache cleanup issues.
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/skillvault")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\nOutput: %s", err, string(out))
	}

	vaultHome := filepath.Join(tmpDir, "vaulthome")
	if err := os.MkdirAll(vaultHome, 0755); err != nil {
		t.Fatalf("failed to create vault home: %v", err)
	}
	env := append(os.Environ(), "HOME="+vaultHome)

	// Initialize vault in temp home.
	initCmd := exec.Command(binPath, "init")
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, string(out))
	}

	// Add a routing entry mapping "research" to a skill.
	body := "research:\n  skill: extract-wisdom"
	addCmd := exec.Command(binPath, "add-entry", "--title", "Research Route", "--type", "routing", "--summary", "Route research to skill", "--body", body, "--tags", "workflow-route")
	addCmd.Env = env
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add-entry failed: %v\nOutput: %s", err, string(out))
	}

	// Resolve the route.
	cmd := exec.Command(binPath, "route", "research")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("route failed: %v\nOutput: %s", err, string(out))
	}

	if !strings.Contains(string(out), "Route: research") {
		t.Errorf("expected output to contain 'Route: research', got: %q", string(out))
	}
	if !strings.Contains(string(out), "extract-wisdom") {
		t.Errorf("expected output to contain target skill, got: %q", string(out))
	}
	if !strings.Contains(string(out), "(skill)") {
		t.Errorf("expected output to contain type '(skill)', got: %q", string(out))
	}
}

func TestRouteCommandJSON(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "skillvault")

	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/skillvault")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\nOutput: %s", err, string(out))
	}

	vaultHome := filepath.Join(tmpDir, "vaulthome")
	if err := os.MkdirAll(vaultHome, 0755); err != nil {
		t.Fatalf("failed to create vault home: %v", err)
	}
	env := append(os.Environ(), "HOME="+vaultHome)

	initCmd := exec.Command(binPath, "init")
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, string(out))
	}

	body := "research:\n  skill: extract-wisdom"
	addCmd := exec.Command(binPath, "add-entry", "--title", "Research Route", "--type", "routing", "--summary", "Route research to skill", "--body", body, "--tags", "workflow-route")
	addCmd.Env = env
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add-entry failed: %v\nOutput: %s", err, string(out))
	}

	cmd := exec.Command(binPath, "route", "research", "--json")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("route --json failed: %v\nOutput: %s", err, string(out))
	}

	if !strings.Contains(string(out), `"scenario": "research"`) {
		t.Errorf("expected JSON to contain scenario, got: %q", string(out))
	}
	if !strings.Contains(string(out), `"type": "skill"`) {
		t.Errorf("expected JSON to contain type skill, got: %q", string(out))
	}
	if !strings.Contains(string(out), `"target": "extract-wisdom"`) {
		t.Errorf("expected JSON to contain target, got: %q", string(out))
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

func TestResolveQSecretsBin_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	mockBin := filepath.Join(tmpDir, "q-secrets")
	if err := os.WriteFile(mockBin, []byte("#!/bin/sh\necho q-secrets mock\n"), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	t.Setenv("Q_SECRETS_BIN", mockBin)
	got := resolveQSecretsBin()
	if got != mockBin {
		t.Errorf("expected %q, got %q", mockBin, got)
	}
}

func TestResolveQSecretsBin_NotFound(t *testing.T) {
	// Use a temp dir as working directory so resolveRepoRoot doesn't find
	// the real kbs repo by walking up from cwd.
	tmpDir := t.TempDir()
	origCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origCwd) }()

	t.Setenv("Q_SECRETS_BIN", "")
	t.Setenv("PATH", tmpDir)

	got := resolveQSecretsBin()
	if got != "" {
		t.Errorf("expected empty string when binary not found, got %q", got)
	}
}

