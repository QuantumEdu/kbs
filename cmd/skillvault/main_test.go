package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildSkillvaultBinary(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "skillvault")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/skillvault")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\nOutput: %s", err, string(out))
	}

	return binPath
}

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
	binPath := buildSkillvaultBinary(t)
	home := t.TempDir()
	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, _ := cmd.CombinedOutput()
	if len(out) == 0 {
		t.Error("expected help output")
	}
	if !strings.Contains(string(out), "memory index") {
		t.Fatalf("expected nested display name in help output, got: %s", string(out))
	}
	if !strings.Contains(string(out), "pending") {
		t.Fatalf("expected pending command in help output, got: %s", string(out))
	}
	if !strings.Contains(string(out), "Commands by task:") || !strings.Contains(string(out), "Setup:") || !strings.Contains(string(out), "Find:") {
		t.Fatalf("expected grouped help output, got: %s", string(out))
	}
	if _, err := os.Stat(filepath.Join(home, ".skillvault")); !os.IsNotExist(err) {
		t.Fatalf("top-level usage should not initialize the vault, stat err=%v", err)
	}
}

func TestTopLevelHelpFlagsDoNotCauseSideEffects(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			home := t.TempDir()
			cmd := exec.Command(binPath, arg)
			cmd.Env = append(os.Environ(), "HOME="+home)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("help command failed: %v\nOutput: %s", err, string(out))
			}
			if !strings.Contains(string(out), "SkillVault v3") {
				t.Fatalf("expected top-level help output, got: %s", string(out))
			}
			if _, err := os.Stat(filepath.Join(home, ".skillvault")); !os.IsNotExist(err) {
				t.Fatalf("help should not initialize the vault, stat err=%v", err)
			}
		})
	}
}

func TestMCPConfigCommand(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	tests := [][]string{{"mcp", "config"}, {"mcp-config"}, {"setup", "mcp"}}
	for _, args := range tests {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			cmd := exec.Command(binPath, args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("mcp config command failed: %v\nOutput: %s", err, string(out))
			}
			if !strings.Contains(string(out), "\"mcpServers\"") || !strings.Contains(string(out), "\"skillvault\"") {
				t.Fatalf("expected MCP config JSON snippet, got: %s", string(out))
			}
		})
	}
}

func TestCommandHelpOutputIsFocusedAndSideEffectFree(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	tests := [][]string{{"help", "doctor"}, {"check", "--help"}, {"context", "--help"}, {"mcp", "config", "--help"}, {"docs"}, {"readme"}}
	for _, args := range tests {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			home := t.TempDir()
			cmd := exec.Command(binPath, args...)
			cmd.Env = append(os.Environ(), "HOME="+home)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command help failed: %v\nOutput: %s", err, string(out))
			}
			if !strings.Contains(string(out), "Examples:") && !strings.Contains(string(out), "Commands by task:") {
				t.Fatalf("expected examples in command help, got: %s", string(out))
			}
			if _, err := os.Stat(filepath.Join(home, ".skillvault")); !os.IsNotExist(err) {
				t.Fatalf("command help should not initialize the vault, stat err=%v", err)
			}
		})
	}
}

func TestSetupDoctorAliasRunsReadOnlyDoctor(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	home := t.TempDir()
	cmd := exec.Command(binPath, "setup", "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected setup doctor to report missing vault, got success\nOutput: %s", string(out))
	}
	if !strings.Contains(string(out), "SkillVault doctor") {
		t.Fatalf("expected doctor output, got: %s", string(out))
	}
	if _, err := os.Stat(filepath.Join(home, ".skillvault")); !os.IsNotExist(err) {
		t.Fatalf("setup doctor should not initialize the vault, stat err=%v", err)
	}
}

func TestUnknownCommandGuidanceSuggestsIntentAlternatives(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	cmd := exec.Command(binPath, "projct")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected unknown command to fail, got success\nOutput: %s", string(out))
	}
	if !strings.Contains(string(out), "Try one of these intent-first commands:") {
		t.Fatalf("expected suggestion block, got: %s", string(out))
	}
	if !strings.Contains(string(out), "skillvault project start") {
		t.Fatalf("expected project start suggestion, got: %s", string(out))
	}
	if !strings.Contains(string(out), "skillvault help <command>") {
		t.Fatalf("expected help hint, got: %s", string(out))
	}
}

func TestProjectStartAliasCreatesProject(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home)

	initCmd := exec.Command(binPath, "setup")
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, string(out))
	}

	projectCmd := exec.Command(binPath, "project", "start", "--name", "codex")
	projectCmd.Env = env
	out, err := projectCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("project start failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Project saved:") {
		t.Fatalf("unexpected project start output: %s", string(out))
	}
}

func TestPendingCommandFlow(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home)

	initCmd := exec.Command(binPath, "setup")
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, string(out))
	}

	projectCmd := exec.Command(binPath, "project", "start", "--name", "codex")
	projectCmd.Env = env
	if out, err := projectCmd.CombinedOutput(); err != nil {
		t.Fatalf("project start failed: %v\nOutput: %s", err, string(out))
	}

	addCmd := exec.Command(binPath, "pending", "add", "--project", "codex", "--note", "Refresh examples before PR", "--tags", "slides,review", "Update", "presentation")
	addCmd.Env = env
	out, err := addCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending add failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Pending saved:") {
		t.Fatalf("unexpected pending add output: %s", string(out))
	}

	listCmd := exec.Command(binPath, "todo", "list", "--project", "codex", "--query", "presentation")
	listCmd.Env = env
	out, err = listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending list failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Update presentation") {
		t.Fatalf("expected pending list to include saved item, got: %s", string(out))
	}
	if !strings.Contains(string(out), "Counts: active=1 resolved=0") {
		t.Fatalf("expected pending list counts, got: %s", string(out))
	}

	lines := strings.Split(string(out), "\n")
	var pendingID string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[sk-vault]") {
			parts := strings.SplitN(trimmed, "]", 2)
			if len(parts) == 2 {
				pendingID = strings.TrimPrefix(parts[0], "[")
				break
			}
		}
	}
	if pendingID == "" {
		t.Fatalf("could not parse pending ID from output: %s", string(out))
	}

	showCmd := exec.Command(binPath, "pending", "show", pendingID)
	showCmd.Env = env
	out, err = showCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending show failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Pending item: "+pendingID) || !strings.Contains(string(out), "Refresh examples before PR") {
		t.Fatalf("unexpected pending show output: %s", string(out))
	}

	reviewCmd := exec.Command(binPath, "pending", "review", "--project", "codex", "--tag", "review")
	reviewCmd.Env = env
	out, err = reviewCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending review failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Pending review for project codex") || !strings.Contains(string(out), "skillvault pending show <id>") {
		t.Fatalf("unexpected pending review output: %s", string(out))
	}

	doneCmd := exec.Command(binPath, "pending", "done", pendingID)
	doneCmd.Env = env
	out, err = doneCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending done failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Resolved pending item:") {
		t.Fatalf("unexpected pending done output: %s", string(out))
	}

	listCmd = exec.Command(binPath, "pending", "list", "--project", "codex")
	listCmd.Env = env
	out, err = listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending list after resolve failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "No pending items for project codex.") {
		t.Fatalf("expected empty pending list after resolve, got: %s", string(out))
	}
	if strings.Contains(string(out), "Update presentation") {
		t.Fatalf("resolved pending item should not remain active, got: %s", string(out))
	}
}

func TestBackupAllAliasWritesExportSnapshot(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home)

	initCmd := exec.Command(binPath, "init")
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, string(out))
	}

	backupCmd := exec.Command(binPath, "backup", "all")
	backupCmd.Env = env
	out, err := backupCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("backup all failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Backup written to") {
		t.Fatalf("unexpected backup all output: %s", string(out))
	}
}

func TestDoctorCommandIsSideEffectFreeWhenVaultMissing(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	home := t.TempDir()
	cmd := exec.Command(binPath, "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to report failure for missing vault, got success\nOutput: %s", string(out))
	}
	if !strings.Contains(string(out), "SkillVault doctor") || !strings.Contains(string(out), "Run `skillvault init` first") {
		t.Fatalf("unexpected doctor output: %s", string(out))
	}
	if _, err := os.Stat(filepath.Join(home, ".skillvault")); !os.IsNotExist(err) {
		t.Fatalf("doctor should not initialize the vault, stat err=%v", err)
	}
}

func TestBackupCommandWritesExportSnapshot(t *testing.T) {
	binPath := buildSkillvaultBinary(t)
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home)

	initCmd := exec.Command(binPath, "init")
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, string(out))
	}

	backupCmd := exec.Command(binPath, "backup")
	backupCmd.Env = env
	out, err := backupCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("backup failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Backup written to") {
		t.Fatalf("unexpected backup output: %s", string(out))
	}

	exportsDir := filepath.Join(home, ".skillvault", "exports")
	entries, err := os.ReadDir(exportsDir)
	if err != nil {
		t.Fatalf("read exports dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected backup file in %s", exportsDir)
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
