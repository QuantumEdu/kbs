package cli

import (
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/files"
	"github.com/quantum-6/skillvault/internal/security"
	"github.com/quantum-6/skillvault/internal/sync"
	"github.com/quantum-6/skillvault/internal/vector"
)

// Services bundles the application services used by the CLI commands.
type Services struct {
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

// EntryService exposes the entry service to the TUI entry point.
func (s *Services) EntryService() *app.EntryService { return s.entrySvc }

// ProjectService exposes the project service to the TUI entry point.
func (s *Services) ProjectService() *app.ProjectService { return s.projectSvc }

// ContextService exposes the context service to the TUI entry point.
func (s *Services) ContextService() *app.ContextService { return s.contextSvc }

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

// OpenVault opens the vault database and wires all application services.
func OpenVault() *Services {
	return openVault()
}

func openVault() *Services {
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

	return &Services{
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
