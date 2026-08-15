package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/domain"
)

func runAddEntry(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseAddEntryFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	result, err := svc.entrySvc.SaveEntry(ctx, app.SaveEntryInput{
		Title:   flags.Title,
		Type:    flags.Type,
		Summary: flags.Summary,
		Body:    flags.Body,
		Project: flags.Project,
		Tags:    TagItems(flags.Tags),
		Status:  flags.Status,
		Purpose: flags.Purpose,
	})
	if err != nil {
		PrintError(err)
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
}

func runSearch(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseSearchFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	// Vector search path.
	if flags.Vector {
		results, err := svc.compareSvc.SearchVectors(ctx, flags.Query, flags.Limit)
		if err != nil {
			PrintError(err)
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
		PrintError(err)
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
}

func runGet(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseGetEntryFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	result, err := svc.entrySvc.GetEntry(ctx, flags.ID)
	if err != nil {
		PrintError(err)
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
}

func runSaveArtifact(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseSaveArtifactFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	artifact, err := svc.artifactSvc.SaveArtifact(ctx, app.SaveArtifactInput{
		Title:    flags.Title,
		Type:     flags.Type,
		Content:  flags.Content,
		FilePath: flags.File,
		Summary:  flags.Summary,
		Project:  flags.Project,
		Tags:     TagItems(flags.Tags),
	})
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Printf("Saved artifact: %s\n", artifact.ID)
	fmt.Printf("  Title:   %s\n", artifact.Title)
	fmt.Printf("  Type:    %s\n", artifact.Type)
	fmt.Printf("  File:    %s\n", artifact.FilePath)
}

func runArchive(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseGetEntryFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if err := svc.entrySvc.ArchiveEntry(ctx, flags.ID); err != nil {
		PrintError(err)
		os.Exit(1)
	}
	fmt.Printf("Archived entry: %s\n", flags.ID)
}
