package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/api"
	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/cli"
	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/files"
	"github.com/quantum-6/skillvault/internal/mcp"
	"github.com/quantum-6/skillvault/internal/security"
)

const version = "v2-hermes"

type vaultServices struct {
	store         *db.Store
	entrySvc      *app.EntryService
	artifactSvc   *app.ArtifactService
	workflowSvc   *app.WorkflowService
	seriesSvc     *app.SeriesService
	projectSvc    *app.ProjectService
	contextSvc    *app.ContextService
	sessionSvc    *app.SessionService
	exportSvc     *app.VaultExportService
	importSvc     *app.VaultImportService
	saveResultSvc *app.SavePromptResultService
	fileSvc       *files.ArtifactFileService
	scanner       *security.SecretScanner
}

func main() {
	if filepath.Base(os.Args[0]) == "mcp" {
		runMCP()
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "SkillVault %s\nUsage: skillvault <command> [args...]\n", version)
		os.Exit(1)
	}

	cmd, err := cli.ParseCommand(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "version":
		fmt.Println("SkillVault", version)
	case "init":
		runInit()
	case "mcp":
		runMCP()
	case "http":
		svc := openVault()
		srv := api.NewServer("127.0.0.1", 7438,
			svc.entrySvc, svc.artifactSvc, svc.contextSvc,
			svc.projectSvc, svc.sessionSvc, svc.workflowSvc,
			svc.exportSvc, svc.importSvc,
		)
		fmt.Fprintf(os.Stderr, "HTTP API server starting on 127.0.0.1:7438\n")
		log.Fatal(srv.Start())
	default:
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
	contextSvc := app.NewContextService(store.Entries, store.Projects, store.Series, store.Workflows, store.Artifacts, entrySvc)
	sessionSvc := app.NewSessionService(entrySvc, artifactSvc, projectSvc, store.Entries, store.Artifacts, store.Projects)
	exportSvc := app.NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	importSvc := app.NewVaultImportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts)
	saveResultSvc := app.NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)

	return &vaultServices{
		store:         store,
		entrySvc:      entrySvc,
		artifactSvc:   artifactSvc,
		workflowSvc:   workflowSvc,
		seriesSvc:     seriesSvc,
		projectSvc:    projectSvc,
		contextSvc:    contextSvc,
		sessionSvc:    sessionSvc,
		exportSvc:     exportSvc,
		importSvc:     importSvc,
		saveResultSvc: saveResultSvc,
		fileSvc:       fileSvc,
		scanner:       scanner,
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
	}
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
	)
	server := mcp.NewServer(reg)

	fmt.Fprintln(os.Stderr, "SkillVault MCP server starting...")
	if err := server.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
