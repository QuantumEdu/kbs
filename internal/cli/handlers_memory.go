package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/domain"
)

func runMemory(ctx context.Context, cmd string, svc *Services, args []string) {
	flags, err := ParseMemoryIndexFlags(args)
	if err != nil {
		PrintError(err)
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
			PrintError(err)
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
		PrintError(err)
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
}
