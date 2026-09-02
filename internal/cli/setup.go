package cli

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/quantum-6/skillvault/internal/db"
)

// RunDoctor checks the vault health without changing anything and reports to w.
func RunDoctor(w io.Writer) bool {
	vd := vaultDir()
	db := dbPath()
	symlink := filepath.Join(vd, "skillvault.db")
	if fileExists(db) && !fileExists(symlink) {
		_ = EnsureDBSymlink(vd)
	}
	checks := []struct {
		name string
		ok   bool
		info string
	}{
		{name: "Vault home", ok: pathExists(vd), info: vd},
		{name: "Database", ok: fileExists(db), info: db},
		{name: "DB Symlink", ok: fileExists(filepath.Join(vd, "skillvault.db")), info: filepath.Join(vd, "skillvault.db")},
		{name: "Objects dir", ok: dirExists(filepath.Join(vd, "objects")), info: filepath.Join(vd, "objects")},
		{name: "Exports dir", ok: dirExists(filepath.Join(vd, "exports")), info: filepath.Join(vd, "exports")},
		{name: "Cache dir", ok: dirExists(filepath.Join(vd, "cache")), info: filepath.Join(vd, "cache")},
	}

	healthy := true
	fmt.Fprintln(w, "SkillVault doctor")
	fmt.Fprintf(w, "  Vault home: %s\n", vd)
	for _, check := range checks {
		status := "OK"
		if !check.ok {
			status = "MISSING"
			healthy = false
		}
		fmt.Fprintf(w, "  %-12s %-7s %s\n", check.name+":", status, check.info)
	}

	if fileExists(db) {
		status := "OK"
		info := "SQLite opened successfully"
		if err := pingVaultDB(db); err != nil {
			status = "ERROR"
			info = err.Error()
			healthy = false
		}
		fmt.Fprintf(w, "  %-12s %-7s %s\n", "DB open:", status, info)
	}

	secretPath := resolveQSecretsBin()
	if secretPath == "" {
		fmt.Fprintf(w, "  %-12s %-7s %s\n", "q-secrets:", "INFO", "not installed")
	} else {
		fmt.Fprintf(w, "  %-12s %-7s %s\n", "q-secrets:", "OK", secretPath)
	}

	if healthy {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "SkillVault is ready.")
		return true
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "SkillVault needs setup or repair. Run `skillvault init` first if this vault is new.")
	return false
}

// EnsureDBSymlink creates or updates ~/.skillvault/skillvault.db -> vault.db
func EnsureDBSymlink(vd string) error {
	link := filepath.Join(vd, "skillvault.db")
	target := "vault.db"
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			dest, err := os.Readlink(link)
			if err == nil && (dest == target || dest == filepath.Join(vd, target)) {
				return nil
			}
		}
		_ = os.Remove(link)
	}
	return os.Symlink(target, link)
}

func pingVaultDB(path string) error {
	dbConn, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer dbConn.Close()
	return dbConn.Ping()
}

// RunInit creates the vault directories and runs database migrations.
func RunInit() {
	vd := vaultDir()
	dirs := []string{
		vd,
		filepath.Join(vd, "objects"),
		filepath.Join(vd, "exports"),
		filepath.Join(vd, "cache"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	sqlDB, err := db.OpenDB(dbPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := db.RunMigrations(sqlDB); err != nil {
		fmt.Fprintf(os.Stderr, "error running migrations: %v\n", err)
		os.Exit(1)
	}

	if err := EnsureDBSymlink(vd); err != nil {
		fmt.Fprintf(os.Stderr, "warning: creating db symlink: %v\n", err)
	}

	fmt.Println("SkillVault initialized at", vd)

	installDir := defaultInstallDir()
	hasSecrets := false
	hasTelemetry := false
	for _, a := range os.Args[2:] {
		switch a {
		case "--with-secrets":
			hasSecrets = true
		case "--with-telemetry":
			hasTelemetry = true
		case "--all":
			hasSecrets = true
			hasTelemetry = true
		}
	}
	if hasSecrets {
		fmt.Fprintf(os.Stderr, "[sk-vault] init: --with-secrets flag set, installing q-secrets to %s\n", installDir)
		installQSecrets(installDir)
	}
	if hasTelemetry {
		fmt.Fprintf(os.Stderr, "[sk-vault] init: --with-telemetry flag set, installing telemetry binaries to %s\n", installDir)
		InstallTelemetry(installDir)
	}
}

// RunUpdate rebuilds and reinstalls the skillvault binary from the repo.
func RunUpdate() {
	flags, err := ParseUpdateFlags(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
		os.Exit(1)
	}

	// Resolve repo path in order of priority:
	//   1. --repo flag  2. SKILLVAULT_REPO env  3. Parent dir of executable if it's inside a kbs git repo  4. Default
	repoPath := flags.Repo
	if repoPath == "" {
		repoPath = os.Getenv("SKILLVAULT_REPO")
	}
	if repoPath == "" {
		if execPath, err := os.Executable(); err == nil {
			absExec, _ := filepath.Abs(execPath)
			parent := filepath.Dir(absExec)
			if info, err := os.Stat(filepath.Join(parent, ".git")); err == nil && info.IsDir() {
				repoPath = parent
			}
		}
	}
	if repoPath == "" {
		repoPath = "/home/ubuntu/dev/kbs"
	}

	// Resolve install path in order of priority:
	//   1. --install-path flag  2. SKILLVAULT_INSTALL_PATH env  3. Current executable path  4. Default
	installPath := flags.InstallPath
	if installPath == "" {
		installPath = os.Getenv("SKILLVAULT_INSTALL_PATH")
	}
	if installPath == "" {
		if execPath, err := os.Executable(); err == nil {
			installPath = execPath
		}
	}
	if installPath == "" {
		installPath = "/home/ubuntu/tools/skillvault"
	}

	// Step 1: Validate repo exists and is a git repo.
	if err := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: %s is not a git repository\n", repoPath)
		os.Exit(1)
	}

	// Step 2: Pull latest changes.
	fmt.Fprintf(os.Stderr, "[sk-vault] pulling latest from %s ...\n", repoPath)
	pullCmd := exec.Command("git", "-C", repoPath, "pull")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: git pull failed in %s: %v\n", repoPath, err)
		os.Exit(1)
	}

	// Step 3: Build to a temporary path (same directory as install target for atomic rename).
	installDir := filepath.Dir(installPath)
	tmpFile, err := os.CreateTemp(installDir, ".skillvault-build-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: create temp file in %s: %v\n", installDir, err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	fmt.Fprintf(os.Stderr, "[sk-vault] building from %s ...\n", repoPath)
	buildCmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", tmpPath, "./cmd/skillvault")
	buildCmd.Dir = repoPath
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "[sk-vault] error: go build failed: %v\n", err)
		os.Exit(1)
	}

	// Step 4: Make temp binary executable.
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "[sk-vault] error: chmod temp binary: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Atomically replace install path with temp binary.
	if err := os.Rename(tmpPath, installPath); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "[sk-vault] error: install to %s: %v\n", installPath, err)
		os.Exit(1)
	}

	// Step 6: Print success message with new version.
	verCmd := exec.Command(installPath, "version")
	verCmd.Stdout = os.Stdout
	verCmd.Stderr = os.Stderr
	_ = verCmd.Run()
	fmt.Printf("[sk-vault] update: rebuilt and installed to %s\n", installPath)

	// Step 7: Optionally rebuild q-secrets if present in repo.
	if os.Getenv("SKIP_Q_SECRETS") == "1" {
		fmt.Fprintf(os.Stderr, "[sk-vault] SKIP_Q_SECRETS=1; skipping q-secrets update\n")
	} else {
		installQSecrets(installDir)
	}

	// Step 8: Rebuild telemetry binaries (unless skipped).
	if os.Getenv("SKIP_TELEMETRY") == "1" {
		fmt.Fprintf(os.Stderr, "[sk-vault] SKIP_TELEMETRY=1; skipping telemetry build\n")
	} else {
		InstallTelemetry(installDir)
	}
}

// RunSecrets proxies to q-secrets or installs it for the "secrets install" subcommand.
func RunSecrets() {
	// Handle "secrets install" subcommand: build and install q-secrets.
	if len(os.Args) > 2 && os.Args[2] == "install" {
		installDir := defaultInstallDir()
		fmt.Fprintf(os.Stderr, "[sk-vault] installing q-secrets to %s\n", installDir)
		installQSecrets(installDir)
		return
	}

	secretPath := resolveQSecretsBin()
	if secretPath == "" {
		fmt.Fprintf(os.Stderr, "[sk-vault] q-secrets not found. Install it with:\n")
		fmt.Fprintf(os.Stderr, "  skillvault secrets install\n")
		fmt.Fprintf(os.Stderr, "or pass --with-secrets to 'skillvault init' to install during initialization.\n")
		fmt.Fprintf(os.Stderr, "Also set Q_SECRETS_BIN to its path if already installed elsewhere.\n")
		os.Exit(1)
	}

	// Build the command: q-secrets <args...>
	cmd := exec.Command(secretPath, os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

// defaultInstallDir returns the directory where q-secrets should be installed.
// Priority: SKILLVAULT_INSTALL_PATH parent dir → current executable dir → ~/tools.
func defaultInstallDir() string {
	if p := os.Getenv("SKILLVAULT_INSTALL_PATH"); p != "" {
		return filepath.Dir(p)
	}
	if execPath, err := os.Executable(); err == nil {
		return filepath.Dir(execPath)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/usr/local/bin"
	}
	return filepath.Join(home, "tools")
}

// DefaultInstallDir returns the directory where q-secrets should be installed.
func DefaultInstallDir() string {
	return defaultInstallDir()
}

// installQSecrets builds q-secrets from the submodule and installs it to installDir.
func installQSecrets(installDir string) {
	// Resolve repo root to find q-secrets/ submodule.
	repoPath := os.Getenv("SKILLVAULT_REPO")
	if repoPath == "" {
		repoPath = resolveRepoRoot()
	}

	qModPath := filepath.Join(repoPath, "q-secrets", "go.mod")
	if _, err := os.Stat(qModPath); err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] q-secrets/ not found in repo at %s; skipping install\n", qModPath)
		fmt.Fprintf(os.Stderr, "[sk-vault]   Ensure the submodule is initialized: git submodule update --init\n")
		return
	}

	qTmp, err := os.CreateTemp(installDir, ".q-secrets-build-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] warning: could not create temp file: %v\n", err)
		return
	}
	qTmpPath := qTmp.Name()
	qTmp.Close()

	qBuildCmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", qTmpPath, ".")
	qBuildCmd.Dir = filepath.Join(repoPath, "q-secrets")
	qBuildCmd.Stdout = os.Stdout
	qBuildCmd.Stderr = os.Stderr
	qBuildCmd.Env = append(os.Environ(), "GOFLAGS=")
	if err := qBuildCmd.Run(); err != nil {
		os.Remove(qTmpPath)
		fmt.Fprintf(os.Stderr, "[sk-vault] warning: q-secrets build failed: %v\n", err)
		return
	}

	if err := os.Chmod(qTmpPath, 0755); err != nil {
		os.Remove(qTmpPath)
		fmt.Fprintf(os.Stderr, "[sk-vault] warning: chmod q-secrets binary: %v\n", err)
		return
	}

	qInstallPath := filepath.Join(installDir, "q-secrets")
	if err := os.Rename(qTmpPath, qInstallPath); err != nil {
		os.Remove(qTmpPath)
		fmt.Fprintf(os.Stderr, "[sk-vault] warning: install q-secrets to %s: %v\n", qInstallPath, err)
		return
	}

	fmt.Printf("[sk-vault] q-secrets installed to %s\n", qInstallPath)
}

// InstallTelemetry builds telemetryd, telemetryctl, and telemetrywrap from the repo
// and installs them to installDir.
func InstallTelemetry(installDir string) {
	repoPath := os.Getenv("SKILLVAULT_REPO")
	if repoPath == "" {
		repoPath = resolveRepoRoot()
	}
	if repoPath == "" {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: cannot find kbs repo root\n")
		return
	}

	if err := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: %s is not a valid git repository\n", repoPath)
		return
	}

	type telemetryBin struct {
		Name  string
		GoPkg string
	}
	bins := []telemetryBin{
		{"telemetryd", "./cmd/telemetryd"},
		{"telemetryctl", "./cmd/telemetryctl"},
		{"telemetrywrap", "./internal/agenttelemetry/telemetrywrap"},
	}

	for _, bin := range bins {
		tmpFile, err := os.CreateTemp(installDir, ".telemetry-build-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sk-vault] warning: create temp file in %s: %v\n", installDir, err)
			continue
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()

		buildCmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", tmpPath, bin.GoPkg)
		buildCmd.Dir = repoPath
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			os.Remove(tmpPath)
			fmt.Fprintf(os.Stderr, "[sk-vault] warning: build %s failed: %v\n", bin.Name, err)
			continue
		}

		if err := os.Chmod(tmpPath, 0755); err != nil {
			os.Remove(tmpPath)
			fmt.Fprintf(os.Stderr, "[sk-vault] warning: chmod %s: %v\n", tmpPath, err)
			continue
		}

		installPath := filepath.Join(installDir, bin.Name)
		if err := os.Rename(tmpPath, installPath); err != nil {
			os.Remove(tmpPath)
			fmt.Fprintf(os.Stderr, "[sk-vault] warning: install %s to %s: %v\n", bin.Name, installPath, err)
			continue
		}

		fmt.Printf("[sk-vault] %s installed to %s\n", bin.Name, installPath)
	}
}

// resolveQSecretsBin returns the path to the q-secrets binary, or empty if not found.
func resolveQSecretsBin() string {
	// 1. Q_SECRETS_BIN env var (full path override)
	if p := os.Getenv("Q_SECRETS_BIN"); p != "" {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	// 2. Same directory as the current skillvault executable
	if execPath, err := os.Executable(); err == nil {
		dir := filepath.Dir(execPath)
		candidate := filepath.Join(dir, "q-secrets")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
	}

	// 3. q-secrets/q-secrets relative to the kbs repo root (useful for development)
	repoRoot := resolveRepoRoot()
	if repoRoot != "" {
		candidate := filepath.Join(repoRoot, "q-secrets", "q-secrets")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
	}

	// 4. q-secrets in PATH
	if p, err := exec.LookPath("q-secrets"); err == nil {
		return p
	}

	return ""
}

// resolveRepoRoot returns the path to the kbs repository root, or empty if not found.
func resolveRepoRoot() string {
	if execPath, err := os.Executable(); err == nil {
		absExec, _ := filepath.Abs(execPath)
		parent := filepath.Dir(absExec)
		if info, err := os.Stat(filepath.Join(parent, ".git")); err == nil && info.IsDir() {
			return parent
		}
	}
	// Fallback: walk up from cwd looking for .git
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for {
			if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}
