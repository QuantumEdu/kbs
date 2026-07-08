package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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

var commandDescs = map[string]string{
	"version":              "Print version information",
	"init":                 "Initialize vault directory, database, and subdirectories",
	"mcp":                  "Start MCP JSON-RPC 2.0 server over stdio",
	"http":                 "Start HTTP REST API server on 127.0.0.1:7438",
	"add-entry":            "Save a new entry to the vault",
	"search":               "Full-text or vector search across vault entries",
	"get":                  "Retrieve an entry by ID or slug",
	"save-artifact":        "Save a file-backed artifact to the vault",
	"get-context":          "Compile a context pack for AI agent consumption",
	"add-project":          "Create a new project in the vault",
	"list-projects":        "List all projects in the vault",
	"archive":              "Soft-delete an entry (status → archived)",
	"add-workflow":         "Create a workflow from a JSON definition file",
	"render-workflow":      "Render a workflow as a human-readable checklist",
	"run":                  "Execute a workflow pipeline with input",
	"session-wrap":         "Create a session entry with decisions, pending, learnings",
	"export":               "Export vault contents to a JSON file",
	"import":               "Import vault contents from a JSON file",
	"import-workflow":      "Import a workflow-builder YAML file as entries + workflow",
	"route":                "Resolve a scenario to its matching workflow or skill",
	"save-result":          "Save an AI prompt result to the vault",
	"stats":                "Show vault statistics and entry counts",
	"memory-index":         "Index pi-memory markdown files into the vault",
	"memory-reindex":       "Reindex all memory entries from external sources",
	"memory-list-external": "List shadow entries linked to external memory files",
	"compare-entries":      "Show unified diff between two entries",
	"setup-vectors":        "Load GloVe word vectors for semantic search",
	"reindex-embeddings":   "Recompute vector embeddings for all vault entries",
	"graph":                "Traverse and render the entry reference graph",
	"sync-push":            "Push vault snapshot to remote storage",
	"sync-pull":            "Pull vault snapshot from remote storage",
	"entry-ref":            "Manage entry reference links (add/list/remove)",
	"tui":                  "Start the interactive Bubble Tea terminal UI",
}

func traceCmd(cmd string) {
	desc, ok := commandDescs[cmd]
	if !ok {
		desc = "Execute " + cmd + " command"
	}
	fmt.Fprintf(os.Stderr, "[sk-vault] %s — %s\n", cmd, desc)
}

type vaultServices struct {
	store          *db.Store
	entrySvc       *app.EntryService
	entryRefSvc    *app.EntryRefService
	memoryIndexSvc *app.MemoryIndexService
	artifactSvc    *app.ArtifactService
	workflowSvc    *app.WorkflowService
	workflowRunSvc *app.WorkflowRunService
	seriesSvc      *app.SeriesService
	projectSvc     *app.ProjectService
	contextSvc     *app.ContextService
	sessionSvc     *app.SessionService
	exportSvc      *app.VaultExportService
	importSvc      *app.VaultImportService
	saveResultSvc  *app.SavePromptResultService
	compareSvc     *app.VectorService
	statsSvc       *app.StatsService
	fileSvc        *files.ArtifactFileService
	scanner        *security.SecretScanner
	syncSvc        *app.SyncService
}

func main() {
	if filepath.Base(os.Args[0]) == "mcp" {
		runMCP()
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "SkillVault %s — [sk-vault] prefix on all stderr traces\nUsage: skillvault <command> [args...]\n", version)
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
	default:
		traceCmd(cmd)
		runCLI(cmd)
	}
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
	statsSvc := app.NewStatsService(store.Entries, store.Artifacts, store.Projects)
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
		store:          store,
		entrySvc:       entrySvc,
		entryRefSvc:    entryRefSvc,
		memoryIndexSvc: memoryIndexSvc,
		artifactSvc:    artifactSvc,
		workflowSvc:    workflowSvc,
		workflowRunSvc: workflowRunSvc,
		seriesSvc:      seriesSvc,
		projectSvc:     projectSvc,
		contextSvc:     contextSvc,
		sessionSvc:     sessionSvc,
		exportSvc:      exportSvc,
		importSvc:      importSvc,
		saveResultSvc:  saveResultSvc,
		compareSvc:     compareSvc,
		statsSvc:       statsSvc,
		fileSvc:        fileSvc,
		scanner:        scanner,
		syncSvc:        syncSvc,
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

		results, err := svc.entrySvc.SearchEntries(ctx, flags.Query, domain.SearchQuery{
			ProjectID:       projectID,
			Type:            typePtr,
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

	case "import":
		flags, err := cli.ParseImportFlags(os.Args)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}

		if err := svc.importSvc.Import(ctx, flags.FilePath); err != nil {
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
		stats, err := svc.statsSvc.GetStats(ctx)
		if err != nil {
			cli.PrintError(err)
			os.Exit(1)
		}
		fmt.Println(app.FormatStats(stats))

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
	).WithEntryRefService(svc.entryRefSvc).WithCompareService(svc.compareSvc).WithSaveResultService(svc.saveResultSvc)
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
