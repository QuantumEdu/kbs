package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/app"
)

func runSyncEngram(ctx context.Context, svc *Services, args []string) {
	fs := flag.NewFlagSet("sync-engram", flag.ExitOnError)
	dbPath := fs.String("db", "", "Path to engram.db database")
	project := fs.String("project", "", "Target project name or filter")
	dryRun := fs.Bool("dry-run", false, "Preview import count without modifying vault")
	if len(args) > 2 {
		_ = fs.Parse(args[2:])
	}

	fmt.Println("[sk-vault] Syncing observations from Engram...")
	res, err := svc.entrySvc.SyncEngram(ctx, app.EngramSyncOptions{
		DBPath:  *dbPath,
		Project: *project,
		DryRun:  *dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sk-vault] error: %v\n", err)
		os.Exit(1)
	}

	mode := "Imported"
	if *dryRun {
		mode = "Would import"
	}
	fmt.Printf("[sk-vault] Sync complete: %d observations found. %s: %d, Skipped: %d, Errors: %d\n",
		res.Total, mode, res.Imported, res.Skipped, res.Errors)
}
