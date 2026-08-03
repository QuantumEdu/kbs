package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/api"
	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/cli"
	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/files"
	"github.com/quantum-6/skillvault/internal/mcp"
	"github.com/quantum-6/skillvault/internal/security"
	"github.com/quantum-6/skillvault/internal/sync"
	"github.com/quantum-6/skillvault/internal/vector"
)

const version = "v3"

type vaultServices struct {
	store           *db.Store
	entrySvc        *app.EntryService
	entryVersionSvc *app.EntryVersionService
	packExportSvc   *app.VaultPackExportService
	entryRefSvc     *app.EntryRefService
	memoryIndexSvc  *app.MemoryIndexService
	artifactSvc     *app.ArtifactService
	workflowSvc     *app.WorkflowService
	workflowRunSvc  *app.WorkflowRunService
	seriesSvc       *app.SeriesService
	projectSvc      *app.ProjectService
	contextSvc      *app.ContextService
	sessionSvc      *app.SessionService
	exportSvc       *app.VaultExportService
	importSvc       *app.VaultImportService
	saveResultSvc   *app.SavePromptResultService
	compareSvc      *app.VectorService
	statsSvc        *app.StatsService
	fileSvc         *files.ArtifactFileService
	scanner         *security.SecretScanner
	syncSvc         *app.SyncService
}

func main() {
	if filepath.Base(os.Args[0]) == "mcp" {
		runMCP()
		return
	}

	if handled, exitCode := handleHelp(os.Args); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}
	os.Args = cli.NormalizeArgs(os.Args)

	if len(os.Args) < 2 {
		printTopLevelUsage(os.Stderr)
		os.Exit(1)
	}

	cmd, err := cli.ParseCommand(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "version":
		traceCmd("version")
		fmt.Println("SkillVault", version)
	case "init":
		traceCmd("init")
		runInit()
	case "doctor":
		traceCmd("doctor")
		if !runDoctor(os.Stdout) {
			os.Exit(1)
		}
	case "mcp":
		traceCmd("mcp")
		runMCP()
	case "http":
		traceCmd("http")
		svc := openVault()
		apiKey := ""
		for i, a := range os.Args[2:] {
			if a == "--api-key" && i+1 < len(os.Args[2:]) {
				apiKey = os.Args[2+i+1]
			}
		}
		srv := api.NewServer("127.0.0.1", 7438,
			svc.entrySvc, svc.artifactSvc, svc.contextSvc,
			svc.projectSvc, svc.sessionSvc, svc.workflowSvc,
			svc.exportSvc, svc.importSvc,
		).WithAPIKey(apiKey)
		fmt.Fprintf(os.Stderr, "[sk-vault] HTTP API server starting on 127.0.0.1:7438\n")
		if err := srv.Start(); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintln(os.Stderr, "[sk-vault] HTTP API server shut down.")
	case "update":
		traceCmd("update")
		runUpdate()
	case "install-telemetry":
		traceCmd("install-telemetry")
		installDir := defaultInstallDir()
		installTelemetry(installDir)
	case "mcp-config":
		traceCmd("mcp-config")
		if err := printMCPConfigSnippet(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "[sk-vault] error: print MCP config snippet: %v\n", err)
			os.Exit(1)
		}
	case "backup":
		traceCmd("backup")
		runCLI("backup")
	case "secrets":
		traceCmd("secrets")
		runSecrets()
	default:
		traceCmd(cmd)
		runCLI(cmd)
	}
}

func runDoctor(w io.Writer) bool {
	vd := vaultDir()
	db := dbPath()
	checks := []struct {
		name string
		ok   bool
		info string
	}{
		{name: "Vault home", ok: pathExists(vd), info: vd},
		{name: "Database", ok: fileExists(db), info: db},
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

func pingVaultDB(path string) error {
	dbConn, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer dbConn.Close()
	return dbConn.Ping()
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func vaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".skillvault")
	}
	return filepath.Join(home, ".skillvault")
}

func dbPath() string {
	return filepath.Join(vaultDir(), "vault.db")
}

func runInit() {
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
		installTelemetry(installDir)
	}
}

func runUpdate() {
	flags, err := cli.ParseUpdateFlags(os.Args)
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
		installTelemetry(installDir)
	}
}

func runSecrets() {
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

// installTelemetry builds telemetryd, telemetryctl, and telemetrywrap from the repo
// and installs them to installDir.
func installTelemetry(installDir string) {
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

func openVault() *vaultServices {
	sqlDB, err := db.OpenDB(dbPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}

	store := db.NewStore(sqlDB)
	fileSvc, err := files.NewArtifactFileService(vaultDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating file service: %v\n", err)
		os.Exit(1)
	}
	scanner := security.New()

	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	artifactSvc := app.NewArtifactService(store.Artifacts, store.Entries, store.Projects)
	workflowSvc := app.NewWorkflowService(store.Workflows)
	seriesSvc := app.NewSeriesService(store.Series, store.Entries)
	projectSvc := app.NewProjectService(store.Projects)
	entryRefSvc := app.NewEntryRefService(store.EntryLinks, store.Entries)
	memoryIndexSvc := app.NewMemoryIndexService(store.Entries, store.Projects, entryRefSvc)
	contextSvc := app.NewContextService(store.Entries, store.Projects, store.Series, store.Workflows, store.Artifacts, entrySvc)
	sessionSvc := app.NewSessionService(entrySvc, artifactSvc, projectSvc, store.Entries, store.Artifacts, store.Projects)
	exportSvc := app.NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	importSvc := app.NewVaultImportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts)
	entryVersionSvc := app.NewEntryVersionService(store.EntryVersions, store.Entries)
	packExportSvc := app.NewVaultPackExportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts, store.Workflows)
	statsSvc := app.NewStatsService(store.Entries, store.Artifacts, store.Projects).WithWorkflowRunStore(store.WorkflowRuns)
	saveResultSvc := app.NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)
	workflowRunSvc := app.NewWorkflowRunService(store.Workflows, store.WorkflowRuns, store.Entries)
	compareSvc := app.NewVectorService(store.Entries, store.Embeddings)

	// Load GloVe vectors from environment if configured.
	if glovePath := os.Getenv("SKILLVAULT_GLOVE_PATH"); glovePath != "" {
		gv, err := vector.LoadGlove(glovePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load GloVe vectors from %s: %v\n", glovePath, err)
		} else {
			compareSvc.SetGlove(gv)
		}
	}

	// Wire VectorService into EntryService for auto-embed on save.
	entrySvc.SetVectorService(compareSvc)

	// Wire WorkflowStore into EntryService for route scenario resolution.
	entrySvc.SetWorkflowStore(store.Workflows)

	// Sync service: load config, build gzip transport from config defaults.
	// The transport can be overridden at runtime via CLI flags.
	var gzipTransport sync.Transport
	if cfg, err := sync.LoadConfig(sync.DefaultConfigPath()); err == nil {
		gzipTransport = buildTransport(cfg)
	}
	syncSvc := app.NewSyncService(exportSvc, importSvc, gzipTransport)

	return &vaultServices{
		store:           store,
		entrySvc:        entrySvc,
		entryVersionSvc: entryVersionSvc,
		packExportSvc:   packExportSvc,
		entryRefSvc:     entryRefSvc,
		memoryIndexSvc:  memoryIndexSvc,
		artifactSvc:     artifactSvc,
		workflowSvc:     workflowSvc,
		workflowRunSvc:  workflowRunSvc,
		seriesSvc:       seriesSvc,
		projectSvc:      projectSvc,
		contextSvc:      contextSvc,
		sessionSvc:      sessionSvc,
		exportSvc:       exportSvc,
		importSvc:       importSvc,
		saveResultSvc:   saveResultSvc,
		compareSvc:      compareSvc,
		statsSvc:        statsSvc,
		fileSvc:         fileSvc,
		scanner:         scanner,
		syncSvc:         syncSvc,
	}
}

func runCLI(cmd string) {
	svc := openVault()
	ctx := context.Background()

	switch cmd {
	case "add-entry":
		flags, err := cli.ParseAddEntryFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		result, err := svc.entrySvc.SaveEntry(ctx, app.SaveEntryInput{
			Title:   flags.Title,
			Type:    flags.Type,
			Summary: flags.Summary,
			Body:    flags.Body,
			Project: flags.Project,
			Tags:    cli.TagItems(flags.Tags),
			Status:  flags.Status,
			Purpose: flags.Purpose,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		proj := "global"
		if result.Entry.Entry.ProjectID != nil {
			proj = *result.Entry.Entry.ProjectID
		}
		fmt.Printf("Saved: %s\n", result.Entry.Entry.ID)
		fmt.Printf("  Title:   %s\n", result.Entry.Entry.Title)
		fmt.Printf("  Type:    %s\n", result.Entry.Entry.Type)
		fmt.Printf("  Project: %s\n", proj)
		fmt.Printf("  Status:  %s\n", result.Entry.Entry.Status)

	case "search":
		flags, err := cli.ParseSearchFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		// Vector search path.
		if flags.Vector {
			results, err := svc.compareSvc.SearchVectors(ctx, flags.Query, flags.Limit)
			if err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			if len(results) == 0 {
				fmt.Println("No results found.")
				return
			}
			fmt.Printf("Found %d result(s) (vector):\n", len(results))
			for _, r := range results {
				proj := "global"
				if r.Entry.ProjectID != nil {
					proj = *r.Entry.ProjectID
				}
				fmt.Printf("\n  [%s] %s\n", r.Entry.ID, r.Entry.Title)
				fmt.Printf("    Type:    %s\n", r.Entry.Type)
				fmt.Printf("    Summary: %s\n", r.Entry.Summary)
				fmt.Printf("    Project: %s\n", proj)
				fmt.Printf("    Status:  %s\n", r.Entry.Status)
			}
			return
		}

		// FTS5 search path (existing behavior).
		var projectID *string
		if flags.ProjectID != "" {
			projectID = &flags.ProjectID
		}
		var typePtr *string
		if flags.Type != "" {
			typePtr = &flags.Type
		}

		var purposePtr *string
		if flags.Purpose != "" {
			purposePtr = &flags.Purpose
		}

		results, err := svc.entrySvc.SearchEntries(ctx, flags.Query, domain.SearchQuery{
			ProjectID:       projectID,
			Type:            typePtr,
			Purpose:         purposePtr,
			IncludeArchived: flags.IncludeArchived,
			Limit:           flags.Limit,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return
		}

		fmt.Printf("Found %d result(s):\n", len(results))
		for _, r := range results {
			proj := "global"
			if r.Entry.ProjectID != nil {
				proj = *r.Entry.ProjectID
			}
			fmt.Printf("\n  [%s] %s\n", r.Entry.ID, r.Entry.Title)
			fmt.Printf("    Type:    %s\n", r.Entry.Type)
			fmt.Printf("    Summary: %s\n", r.Entry.Summary)
			fmt.Printf("    Project: %s\n", proj)
			fmt.Printf("    Status:  %s\n", r.Entry.Status)
		}

	case "get":
		flags, err := cli.ParseGetEntryFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		result, err := svc.entrySvc.GetEntry(ctx, flags.ID)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		proj := "global"
		if result.Entry.Entry.ProjectID != nil {
			proj = *result.Entry.Entry.ProjectID
		}
		fmt.Printf("ID:      %s\n", result.Entry.Entry.ID)
		fmt.Printf("Title:   %s\n", result.Entry.Entry.Title)
		fmt.Printf("Type:    %s\n", result.Entry.Entry.Type)
		fmt.Printf("Summary: %s\n", result.Entry.Entry.Summary)
		fmt.Printf("Project: %s\n", proj)
		fmt.Printf("Status:  %s\n", result.Entry.Entry.Status)
		if result.Artifact != nil {
			fmt.Printf("\nArtifact:\n  ID:   %s\n  Title: %s\n  Type:  %s\n  File:  %s\n",
				result.Artifact.ID, result.Artifact.Title, result.Artifact.Type, result.Artifact.FilePath)
		}

	case "save-artifact":
		flags, err := cli.ParseSaveArtifactFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		artifact, err := svc.artifactSvc.SaveArtifact(ctx, app.SaveArtifactInput{
			Title:    flags.Title,
			Type:     flags.Type,
			Content:  flags.Content,
			FilePath: flags.File,
			Summary:  flags.Summary,
			Project:  flags.Project,
			Tags:     cli.TagItems(flags.Tags),
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Saved artifact: %s\n", artifact.ID)
		fmt.Printf("  Title:   %s\n", artifact.Title)
		fmt.Printf("  Type:    %s\n", artifact.Type)
		fmt.Printf("  File:    %s\n", artifact.FilePath)

	case "get-context":
		flags, err := cli.ParseGetContextFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		pack, err := svc.contextSvc.GetContext(ctx, app.ContextInput{
			Mode:     flags.Mode,
			Project:  flags.Project,
			Query:    flags.Query,
			Include:  cli.SplitLines(flags.Include),
			MaxChars: flags.MaxChars,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Print(pack.Raw)

	case "add-project":
		flags, err := cli.ParseAddProjectFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		proj, err := svc.projectSvc.SaveProject(ctx, app.SaveProjectInput{
			Name:        flags.Name,
			Description: flags.Description,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Project saved: %s\n", proj.ID)
		fmt.Printf("  Name:        %s\n", proj.Name)
		fmt.Printf("  Description: %s\n", proj.Description)

	case "pending-add":
		flags, err := cli.ParsePendingAddFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		result, err := svc.entrySvc.SavePending(ctx, app.SavePendingInput{
			Project: flags.Project,
			Title:   flags.Title,
			Note:    flags.Note,
			Tags:    cli.TagItems(flags.Tags),
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Pending saved: %s\n", result.Entry.Entry.ID)
		fmt.Printf("  Project: %s\n", flags.Project)
		fmt.Printf("  Title:   %s\n", result.Entry.Entry.Title)
		if result.Entry.Entry.BodyOptional != "" {
			fmt.Printf("  Note:    %s\n", result.Entry.Entry.BodyOptional)
		}

	case "pending-list":
		flags, err := cli.ParsePendingListFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		items, err := svc.entrySvc.ListPendingWithOptions(ctx, app.ListPendingInput{
			Project:         flags.Project,
			IncludeArchived: flags.IncludeArchived,
			Query:           flags.Query,
			Tag:             flags.Tag,
			Limit:           flags.Limit,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		if len(items) == 0 && flags.Query == "" && flags.Tag == "" && !flags.IncludeArchived {
			fmt.Printf("No pending items for project %s.\n", flags.Project)
			return
		}
		printPendingList(os.Stdout, "Pending items", flags.Project, items, flags.Query, flags.Tag, flags.IncludeArchived)

	case "pending-review":
		flags, err := cli.ParsePendingListFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		items, err := svc.entrySvc.ListPendingWithOptions(ctx, app.ListPendingInput{
			Project:         flags.Project,
			IncludeArchived: flags.IncludeArchived,
			Query:           flags.Query,
			Tag:             flags.Tag,
			Limit:           flags.Limit,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		printPendingList(os.Stdout, "Pending review", flags.Project, items, flags.Query, flags.Tag, flags.IncludeArchived)
		if len(items) == 0 {
			fmt.Println("Next: add one with `skillvault pending add --project <project> \"Title\"`.")
			return
		}
		fmt.Println("Next:")
		fmt.Println("  skillvault pending show <id>")
		fmt.Println("  skillvault pending done <id>")

	case "pending-show":
		flags, err := cli.ParsePendingShowFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		item, err := svc.entrySvc.GetPending(ctx, flags.ID, true)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		printPendingDetails(os.Stdout, item)

	case "pending-done":
		flags, err := cli.ParsePendingDoneFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if err := svc.entrySvc.ResolvePending(ctx, flags.ID); err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Resolved pending item: %s\n", flags.ID)

	case "list-projects":
		projects, err := svc.projectSvc.ListProjects(ctx)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return
		}

		fmt.Printf("Projects (%d):\n", len(projects))
		for _, p := range projects {
			fmt.Printf("\n  [%s] %s\n", p.ID, p.Name)
			fmt.Printf("    Status:      %s\n", p.Status)
			if p.Description != "" {
				fmt.Printf("    Description: %s\n", p.Description)
			}
		}

	case "archive":
		flags, err := cli.ParseGetEntryFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if err := svc.entrySvc.ArchiveEntry(ctx, flags.ID); err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Archived entry: %s\n", flags.ID)

	case "add-workflow":
		flags, err := cli.ParseWorkflowFileFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		data, err := os.ReadFile(flags.FilePath)
		if err != nil {
			cli.PrintError(fmt.Errorf("read workflow file: %w", err))
			os.Exit(1)
		}

		var input app.SaveWorkflowInput
		if err := json.Unmarshal(data, &input); err != nil {
			cli.PrintError(fmt.Errorf("parse workflow JSON: %w", err))
			os.Exit(1)
		}

		wf, err := svc.workflowSvc.SaveWorkflow(ctx, input)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Workflow saved: %s\n", wf.ID)
		fmt.Printf("  Name:        %s\n", wf.Name)
		fmt.Printf("  Description: %s\n", wf.Description)

	case "render-workflow":
		flags, err := cli.ParseRenderWorkflowFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		steps, err := svc.workflowSvc.RenderWorkflow(ctx, flags.WorkflowID)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Workflow: %s\n", flags.WorkflowID)
		fmt.Println("Steps:")
		for _, s := range steps {
			req := ""
			if s.Required {
				req = " [REQUIRED]"
			}
			fmt.Printf("  %d. %s%s\n", s.OrderIndex, s.Title, req)
			if s.Instruction != "" {
				fmt.Printf("     %s\n", s.Instruction)
			}
			if s.ExpectedOutput != "" {
				fmt.Printf("     Expected: %s\n", s.ExpectedOutput)
			}
		}

	case "run":
		flags, err := cli.ParseRunFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		// Read input from file or stdin
		var input string
		if flags.FilePath == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				cli.PrintError(fmt.Errorf("read stdin: %w", err))
				os.Exit(1)
			}
			input = string(data)
		} else {
			data, err := os.ReadFile(flags.FilePath)
			if err != nil {
				cli.PrintError(fmt.Errorf("read input file %q: %w", flags.FilePath, err))
				os.Exit(1)
			}
			input = string(data)
		}

		// Run the pipeline
		run, output, err := svc.workflowRunSvc.RunPipeline(ctx, flags.Workflow, input, os.Stdin, os.Stdout)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		// Write output to --save path if specified
		if flags.SavePath != "" {
			if err := os.WriteFile(flags.SavePath, []byte(output), 0644); err != nil {
				cli.PrintError(fmt.Errorf("write output to %q: %w", flags.SavePath, err))
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Run %s completed. Output saved to %s\n", run.ID, flags.SavePath)
		} else {
			fmt.Fprintf(os.Stderr, "Run %s completed.\n", run.ID)
		}

	case "session-wrap":
		flags, err := cli.ParseSessionWrapFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		input := app.SessionWrapInput{
			Project:   flags.Project,
			Summary:   flags.Summary,
			Decisions: cli.SplitLines(flags.Decisions),
			Pending:   cli.SplitLines(flags.Pending),
			Learnings: cli.SplitLines(flags.Learnings),
		}

		output, err := svc.sessionSvc.SessionWrap(ctx, input)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Session saved: %s\n", output.Entry.Entry.Entry.ID)
		fmt.Printf("  Summary:   %s\n", output.Entry.Entry.Entry.Summary)
		fmt.Printf("  Decisions: %d\n", len(input.Decisions))
		fmt.Printf("  Pending:   %d\n", len(input.Pending))
		fmt.Printf("  Learnings: %d\n", len(input.Learnings))

	case "export":
		packMode := false
		for _, a := range os.Args {
			if a == "--pack" {
				packMode = true
				break
			}
		}

		if packMode {
			packFlags, err := cli.ParseExportPackFlags(os.Args)
			if err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			if err := svc.packExportSvc.ExportPack(ctx, app.ExportPackInput{
				Author:      packFlags.Author,
				Version:     packFlags.Version,
				Description: packFlags.Description,
				OutputPath:  packFlags.OutputPath,
			}); err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			fmt.Printf("Pack exported to %s\n", packFlags.OutputPath)
			return
		}

		flags, err := cli.ParseExportFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if err := svc.exportSvc.Export(ctx, flags.OutputPath); err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Exported to %s\n", flags.OutputPath)

	case "backup":
		outputPath := filepath.Join(vaultDir(), "exports", fmt.Sprintf("skillvault-backup-%s.json", time.Now().Format("20060102-150405")))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			cli.PrintError(fmt.Errorf("prepare backup directory: %w", err))
			os.Exit(1)
		}
		if err := svc.exportSvc.Export(ctx, outputPath); err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Backup written to %s\n", outputPath)

	case "import":
		flags, err := cli.ParseImportFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if err := svc.importSvc.ImportWithPrefix(ctx, flags.FilePath, flags.Prefix); err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		fmt.Println("Import completed.")

	case "import-workflow":
		flags, err := cli.ParseImportWorkflowFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		filePath, err := filepath.Abs(flags.File)
		if err != nil {
			cli.PrintError(fmt.Errorf("resolve workflow file path: %w", err))
			os.Exit(1)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			cli.PrintError(fmt.Errorf("read workflow file: %w", err))
			os.Exit(1)
		}

		var projectID *string
		if flags.Project != "" {
			proj, err := svc.projectSvc.GetProject(ctx, flags.Project)
			if err != nil {
				cli.PrintError(fmt.Errorf("project %q not found: %w", flags.Project, err))
				os.Exit(1)
			}
			projectID = &proj.ID
		}

		wf, slugs, err := svc.store.ImportWorkflowWithEntries(ctx, data, projectID)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Workflow imported: %s\n", wf.ID)
		fmt.Printf("  Name:     %s\n", wf.Name)
		if wf.Description != "" {
			fmt.Printf("  Description: %s\n", wf.Description)
		}
		fmt.Printf("  Phases:   %d\n", len(slugs))
		if flags.Project != "" {
			fmt.Printf("  Project:  %s\n", flags.Project)
		}

	case "route":
		flags, err := cli.ParseRouteFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		result, err := svc.entrySvc.RouteScenario(ctx, flags.Scenario)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if flags.JSON {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				cli.PrintError(fmt.Errorf("marshal route result: %w", err))
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		fmt.Printf("Route: %s → %s (%s)\n", result.Scenario, result.Target, result.Type)
		if result.Description != "" {
			fmt.Printf("  Description: %s\n", result.Description)
		}
		if result.Workflow != nil {
			fmt.Printf("  Workflow: %s (%s)\n", result.Workflow.Name, result.Workflow.ID)
			steps, err := svc.workflowSvc.RenderWorkflow(ctx, result.Workflow.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[sk-vault] warning: could not render workflow steps: %v\n", err)
			} else if len(steps) > 0 {
				fmt.Println("  Steps:")
				for _, s := range steps {
					req := ""
					if s.Required {
						req = " [REQUIRED]"
					}
					fmt.Printf("    %d. %s%s\n", s.OrderIndex, s.Title, req)
					if s.Instruction != "" {
						fmt.Printf("       %s\n", s.Instruction)
					}
				}
			}
		}

	case "save-result":
		flags, err := cli.ParseSaveResultFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		output, err := svc.saveResultSvc.Save(ctx, app.SavePromptResultInput{
			Name:           flags.Name,
			Content:        flags.Content,
			Type:           flags.Type,
			Category:       flags.Category,
			Tags:           flags.Tags,
			ProjectID:      flags.ProjectID,
			SourcePromptID: flags.SourcePromptID,
			Model:          flags.Model,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Print(cli.FormatSaveResultOutput(cli.SaveResultOutput{
			EntryID:   output.EntryID,
			Name:      output.Name,
			Type:      output.Type,
			ProjectID: output.ProjectID,
		}))

	case "stats":
		flags, err := cli.ParseStatsFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		stats, err := svc.statsSvc.GetStats(ctx)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if flags.JSON {
			data, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				cli.PrintError(fmt.Errorf("marshal stats: %w", err))
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			fmt.Println(app.FormatStats(stats))
			if flags.WorkflowRuns && stats.WorkflowRuns != nil {
				fmt.Print(app.FormatWorkflowRunStats(stats.WorkflowRuns))
			}
		}

	case "memory-index", "memory-reindex", "memory-list-external":
		flags, err := cli.ParseMemoryIndexFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		listMode := cmd == "memory-list-external"

		if listMode {
			results, err := svc.entrySvc.Search(ctx, domain.SearchQuery{
				Query:           "pimem",
				IncludeArchived: true,
				Limit:           9999,
			})
			if err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			if len(results) == 0 {
				fmt.Println("No shadow entries found.")
				return
			}
			fmt.Printf("Shadow entries (%d):\n", len(results))
			for _, r := range results {
				if r.Entry.ExternalRef == "" {
					continue
				}
				fmt.Printf("  [%s] %s\n", r.Entry.ID, r.Entry.Title)
				fmt.Printf("    Path:  %s\n", r.Entry.ExternalRef)
				fmt.Printf("    Type:  %s | Status: %s\n", r.Entry.Type, r.Entry.Status)
			}
			return
		}

		result, err := svc.memoryIndexSvc.Index(ctx, flags.Path, flags.ProjectID, flags.ParseWikilinks)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Memory index complete for %s -> project %s\n", flags.Path, flags.ProjectID)
		fmt.Printf("  Indexed:   %d\n", result.Indexed)
		fmt.Printf("  Orphaned:  %d\n", result.Orphaned)
		if result.Skipped > 0 {
			fmt.Printf("  Skipped:   %d\n", result.Skipped)
		}
		if result.Failed > 0 {
			fmt.Printf("  Failed:    %d\n", result.Failed)
			for _, f := range result.FailedFiles {
				fmt.Printf("    - %s\n", f)
			}
		}
		if len(result.MissingTargets) > 0 {
			fmt.Printf("  Missing wikilink targets: %d\n", len(result.MissingTargets))
		}

	case "compare-entries":
		flags, err := cli.ParseCompareEntriesFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		diff, err := svc.compareSvc.CompareEntries(ctx, flags.ID1, flags.ID2)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Print(diff)

	case "setup-vectors":
		flags, err := cli.ParseSetupVectorsFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		gv, err := vector.LoadGlove(flags.Path)
		if err != nil {
			cli.PrintError(fmt.Errorf("load GloVe vectors: %w", err))
			os.Exit(1)
		}

		fmt.Printf("Loaded %d word vectors (%d dimensions) from %s\n", gv.Len(), gv.Dims(), flags.Path)

		// Validate with a sample embedding.
		emb, err := vector.Embed("test query", gv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: test embedding failed: %v\n", err)
		} else if emb == nil {
			fmt.Fprintf(os.Stderr, "warning: test embedding produced no tokens\n")
		} else {
			fmt.Println("Test embedding OK.")
		}

		fmt.Println("\nTo enable vector search for future commands, set the environment variable:")
		fmt.Printf("  export SKILLVAULT_GLOVE_PATH=%s\n", flags.Path)
		fmt.Println("Then run: skillvault reindex-embeddings")

	case "reindex-embeddings":
		flags, err := cli.ParseReindexEmbeddingsFlags(os.Args)
		_ = flags
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		count, err := svc.compareSvc.ReindexAll(ctx)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Reindex complete: %d entries embedded.\n", count)

	case "graph":
		flags, err := cli.ParseGraphFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		result, err := svc.entryRefSvc.GetEntryGraph(ctx, app.GetGraphInput{
			EntryID:   flags.EntryID,
			Direction: flags.Direction,
			MaxDepth:  flags.Depth,
		})
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		switch flags.Format {
		case "json":
			out := map[string]interface{}{
				"root_entry": flags.EntryID,
				"direction":  flags.Direction,
				"depth":      flags.Depth,
				"node_count": len(result.Nodes),
				"edge_count": len(result.Edges),
				"nodes":      result.Nodes,
				"edges":      result.Edges,
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(data))

		case "dot":
			fmt.Println("digraph G {")
			fmt.Printf("  // Root: %s, %d nodes, %d edges\n", flags.EntryID, len(result.Nodes), len(result.Edges))
			for _, n := range result.Nodes {
				fmt.Printf("  %q;\n", n.EntryID)
			}
			for _, e := range result.Edges {
				label := e.Label
				if label == "" {
					label = string(e.RelationType)
				}
				fmt.Printf("  %q -> %q [label=%q];\n", e.FromEntryID, e.ToEntryID, label)
			}
			fmt.Println("}")

		default: // mermaid
			fmt.Println("graph TD")
			fmt.Printf("  %% Root: %s, depth %d, direction: %s\n", flags.EntryID, flags.Depth, flags.Direction)
			for _, e := range result.Edges {
				label := e.Label
				if label == "" {
					label = string(e.RelationType)
				}
				fmt.Printf("  %s -->|%s| %s\n", e.FromEntryID, label, e.ToEntryID)
			}
			if len(result.Edges) == 0 {
				fmt.Printf("  %s[\"No connections\"]\n", flags.EntryID)
			}
		}

	case "sync-push", "sync-pull":
		flags, err := cli.ParseSyncFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		// Build transport from flag (overrides config defaults).
		var transport sync.Transport
		switch flags.Transport {
		case "s3":
			t, err := sync.NewS3Transport(&sync.S3Config{
				Bucket:          os.Getenv("S3_BUCKET"),
				Region:          os.Getenv("AWS_REGION"),
				Endpoint:        os.Getenv("AWS_ENDPOINT"),
				AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
				SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			})
			if err != nil {
				cli.PrintError(fmt.Errorf("s3 transport: %w", err))
				os.Exit(1)
			}
			transport = sync.NewGzipTransport(t)
		case "github":
			t, err := sync.NewGitHubTransport(&sync.GitHubConfig{
				Token: os.Getenv("GITHUB_TOKEN"),
				Owner: os.Getenv("GITHUB_OWNER"),
				Repo:  os.Getenv("GITHUB_REPO"),
			})
			if err != nil {
				cli.PrintError(fmt.Errorf("github transport: %w", err))
				os.Exit(1)
			}
			transport = sync.NewGzipTransport(t)
		}

		svc.syncSvc.SetTransport(transport)

		if cmd == "sync-push" {
			if err := svc.syncSvc.Push(ctx, flags.RemotePath, flags.DryRun); err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			if !flags.DryRun {
				fmt.Printf("Pushed vault snapshot to %s\n", flags.RemotePath)
			}
		} else {
			if err := svc.syncSvc.Pull(ctx, flags.RemotePath, flags.DryRun); err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			if !flags.DryRun {
				fmt.Printf("Pulled vault snapshot from %s\n", flags.RemotePath)
			}
		}

	case "entry-history":
		flags, err := cli.ParseEntryHistoryFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		versions, err := svc.entryVersionSvc.ListVersions(ctx, flags.ID)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if len(versions) == 0 {
			fmt.Printf("No version history for entry %s.\n", flags.ID)
			return
		}

		fmt.Printf("Version history for entry %s:\n", flags.ID)
		fmt.Print(cli.FormatTable(versions,
			[]string{"#", "Title", "Saved At"},
			func(v domain.EntryVersion) []string {
				return []string{
					fmt.Sprintf("%d", v.VersionNumber),
					v.Title,
					v.SavedAt,
				}
			}))

	case "entry-restore":
		flags, err := cli.ParseEntryRestoreFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if err := svc.entryVersionSvc.RestoreVersion(ctx, flags.ID, flags.Version); err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		fmt.Printf("Restored entry %s to version %d.\n", flags.ID, flags.Version)

	case "entry-ref":
		if len(os.Args) < 4 {
			cli.PrintError(fmt.Errorf("usage: skillvault entry ref <subcommand> [args...]"))
			os.Exit(1)
		}
		sub := os.Args[3]
		switch sub {
		case "add":
			if len(os.Args) < 7 {
				cli.PrintError(fmt.Errorf("usage: skillvault entry ref add <source_id> <target_id> <ref_type> [--label <txt>]"))
				os.Exit(1)
			}
			sourceID := os.Args[4]
			targetID := os.Args[5]
			refType := os.Args[6]
			label := ""
			for i, a := range os.Args[7:] {
				if a == "--label" && i+1 < len(os.Args[7:]) {
					label = os.Args[7+i+1]
				}
			}
			link, err := svc.entryRefSvc.SaveRef(ctx, app.AddRefInput{
				SourceID: sourceID,
				TargetID: targetID,
				RefType:  refType,
				Label:    label,
			})
			if err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			fmt.Printf("Saved ref: %s --[%s]--> %s\n", link.FromEntryID, link.RelationType, link.ToEntryID)
			if link.Label != "" {
				fmt.Printf("  Label: %s\n", link.Label)
			}

		case "list":
			var srcPtr, tgtPtr, typePtr *string
			includeArchived := false
			for i, a := range os.Args[4:] {
				v := ""
				if i+1 < len(os.Args[4:]) {
					v = os.Args[4+i+1]
				}
				switch a {
				case "--source":
					srcPtr = &v
				case "--target":
					tgtPtr = &v
				case "--type":
					typePtr = &v
				case "--include-archived":
					includeArchived = true
				}
			}
			links, err := svc.entryRefSvc.ListRefs(ctx, app.ListRefsInput{
				SourceID:        srcPtr,
				TargetID:        tgtPtr,
				RefType:         typePtr,
				IncludeArchived: includeArchived,
			})
			if err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			if len(links) == 0 {
				fmt.Println("No refs found.")
				return
			}
			fmt.Printf("Found %d ref(s):\n", len(links))
			for _, l := range links {
				la := ""
				if l.Label != "" {
					la = fmt.Sprintf(" (%s)", l.Label)
				}
				fmt.Printf("  %s --[%s%s]--> %s\n", l.FromEntryID, l.RelationType, la, l.ToEntryID)
			}

		case "remove":
			if len(os.Args) < 7 {
				cli.PrintError(fmt.Errorf("usage: skillvault entry ref remove <source_id> <target_id> <ref_type>"))
				os.Exit(1)
			}
			sourceID := os.Args[4]
			targetID := os.Args[5]
			refType := os.Args[6]
			if err := svc.entryRefSvc.RemoveRef(ctx, sourceID, targetID, refType); err != nil {
				cli.PrintError(err)
				os.Exit(1)
			}
			fmt.Printf("Removed ref: %s --[%s]--> %s\n", sourceID, refType, targetID)

		default:
			cli.PrintError(fmt.Errorf("unknown entry ref subcommand: %s", sub))
			os.Exit(1)
		}

	case "tui":
		runTUI(svc)
	}
}

func buildTransport(cfg *sync.Config) sync.Transport {
	if cfg == nil {
		return nil
	}
	switch cfg.Transport {
	case "s3":
		t, err := sync.NewS3Transport(cfg.S3)
		if err != nil {
			fmt.Fprintf(os.Stderr, "s3 transport: %v\n", err)
			return nil
		}
		return sync.NewGzipTransport(t)
	case "github":
		t, err := sync.NewGitHubTransport(cfg.GitHub)
		if err != nil {
			fmt.Fprintf(os.Stderr, "github transport: %v\n", err)
			return nil
		}
		return sync.NewGzipTransport(t)
	}
	return nil
}

func runMCP() {
	svc := openVault()

	reg := mcp.NewServiceToolRegistry(
		svc.entrySvc,
		svc.artifactSvc,
		svc.contextSvc,
		svc.seriesSvc,
		svc.workflowSvc,
		svc.sessionSvc,
		svc.projectSvc,
	).WithEntryRefService(svc.entryRefSvc).WithCompareService(svc.compareSvc).WithSaveResultService(svc.saveResultSvc).WithWorkflowRunService(svc.workflowRunSvc).WithStatsService(svc.statsSvc).WithEntryVersionService(svc.entryVersionSvc)
	server := mcp.NewServer(reg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintln(os.Stderr, "[sk-vault] MCP server starting...")
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "[sk-vault] MCP server error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "[sk-vault] MCP server shut down.")
}

func printPendingList(w io.Writer, heading, project string, items []domain.EntryListResult, query, tag string, includeArchived bool) {
	activeCount := 0
	archivedCount := 0
	for _, item := range items {
		if item.Entry.Status == domain.StatusArchived {
			archivedCount++
		} else {
			activeCount++
		}
	}

	state := "active"
	if includeArchived {
		state = "active + resolved"
	}
	filterSummary := []string{state}
	if strings.TrimSpace(query) != "" {
		filterSummary = append(filterSummary, fmt.Sprintf("query=%q", strings.TrimSpace(query)))
	}
	if strings.TrimSpace(tag) != "" {
		filterSummary = append(filterSummary, fmt.Sprintf("tag=%q", strings.TrimSpace(tag)))
	}

	fmt.Fprintf(w, "%s for project %s\n", heading, project)
	fmt.Fprintf(w, "Showing %d item(s) [%s]\n", len(items), strings.Join(filterSummary, "; "))
	fmt.Fprintf(w, "Counts: active=%d resolved=%d\n", activeCount, archivedCount)
	if len(items) == 0 {
		return
	}

	for _, item := range items {
		fmt.Fprintf(w, "\n[%s] %s\n", item.Entry.ID, item.Entry.Title)
		fmt.Fprintf(w, "  Status: %s\n", item.Entry.Status)
		if item.Entry.BodyOptional != "" {
			fmt.Fprintf(w, "  Note:   %s\n", item.Entry.BodyOptional)
		}
		if tags := pendingTagNames(item.Tags); len(tags) > 0 {
			fmt.Fprintf(w, "  Tags:   %s\n", strings.Join(tags, ", "))
		}
	}
}

func printPendingDetails(w io.Writer, item domain.EntryResult) {
	project := "global"
	if item.Entry.ProjectID != nil {
		project = *item.Entry.ProjectID
	}

	fmt.Fprintf(w, "Pending item: %s\n", item.Entry.ID)
	fmt.Fprintf(w, "  Title:   %s\n", item.Entry.Title)
	fmt.Fprintf(w, "  Project: %s\n", project)
	fmt.Fprintf(w, "  Status:  %s\n", item.Entry.Status)
	if item.Entry.BodyOptional != "" {
		fmt.Fprintf(w, "  Note:    %s\n", item.Entry.BodyOptional)
	}
	if tags := pendingTagNames(item.Tags); len(tags) > 0 {
		fmt.Fprintf(w, "  Tags:    %s\n", strings.Join(tags, ", "))
	}
	fmt.Fprintln(w, "  Next:    skillvault pending done "+item.Entry.ID)
}

func pendingTagNames(tags []domain.Tag) []string {
	if len(tags) == 0 {
		return nil
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}
