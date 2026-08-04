package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/quantum-6/skillvault/internal/app"
)

func runGetContext(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseGetContextFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	pack, err := svc.contextSvc.GetContext(ctx, app.ContextInput{
		Mode:     flags.Mode,
		Project:  flags.Project,
		Query:    flags.Query,
		Include:  SplitLines(flags.Include),
		MaxChars: flags.MaxChars,
	})
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Print(pack.Raw)
}

func runAddProject(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseAddProjectFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	proj, err := svc.projectSvc.SaveProject(ctx, app.SaveProjectInput{
		Name:        flags.Name,
		Description: flags.Description,
	})
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Printf("Project saved: %s\n", proj.ID)
	fmt.Printf("  Name:        %s\n", proj.Name)
	fmt.Printf("  Description: %s\n", proj.Description)
}

func runListProjects(ctx context.Context, svc *Services, args []string) {
	projects, err := svc.projectSvc.ListProjects(ctx)
	if err != nil {
		PrintError(err)
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
}

func runSessionWrap(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseSessionWrapFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	input := app.SessionWrapInput{
		Project:   flags.Project,
		Summary:   flags.Summary,
		Decisions: SplitLines(flags.Decisions),
		Pending:   SplitLines(flags.Pending),
		Learnings: SplitLines(flags.Learnings),
	}

	output, err := svc.sessionSvc.SessionWrap(ctx, input)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Printf("Session saved: %s\n", output.Entry.Entry.Entry.ID)
	fmt.Printf("  Summary:   %s\n", output.Entry.Entry.Entry.Summary)
	fmt.Printf("  Decisions: %d\n", len(input.Decisions))
	fmt.Printf("  Pending:   %d\n", len(input.Pending))
	fmt.Printf("  Learnings: %d\n", len(input.Learnings))
}

func runExport(ctx context.Context, svc *Services, args []string) {
	packMode := false
	for _, a := range args {
		if a == "--pack" {
			packMode = true
			break
		}
	}

	if packMode {
		packFlags, err := ParseExportPackFlags(args)
		if err != nil {
			PrintError(err)
			os.Exit(1)
		}
		if err := svc.packExportSvc.ExportPack(ctx, app.ExportPackInput{
			Author:      packFlags.Author,
			Version:     packFlags.Version,
			Description: packFlags.Description,
			OutputPath:  packFlags.OutputPath,
		}); err != nil {
			PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Pack exported to %s\n", packFlags.OutputPath)
		return
	}

	flags, err := ParseExportFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if err := svc.exportSvc.Export(ctx, flags.OutputPath); err != nil {
		PrintError(err)
		os.Exit(1)
	}
	fmt.Printf("Exported to %s\n", flags.OutputPath)
}

func runBackup(ctx context.Context, svc *Services, args []string) {
	outputPath := filepath.Join(vaultDir(), "exports", fmt.Sprintf("skillvault-backup-%s.json", time.Now().Format("20060102-150405")))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		PrintError(fmt.Errorf("prepare backup directory: %w", err))
		os.Exit(1)
	}
	if err := svc.exportSvc.Export(ctx, outputPath); err != nil {
		PrintError(err)
		os.Exit(1)
	}
	fmt.Printf("Backup written to %s\n", outputPath)
}

func runImport(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseImportFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if err := svc.importSvc.ImportWithPrefix(ctx, flags.FilePath, flags.Prefix); err != nil {
		PrintError(err)
		os.Exit(1)
	}
	fmt.Println("Import completed.")
}
