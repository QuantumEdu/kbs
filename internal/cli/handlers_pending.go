package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/domain"
)

func runPendingAdd(ctx context.Context, svc *Services, args []string) {
	flags, err := ParsePendingAddFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	result, err := svc.entrySvc.SavePending(ctx, app.SavePendingInput{
		Project: flags.Project,
		Title:   flags.Title,
		Note:    flags.Note,
		Tags:    TagItems(flags.Tags),
	})
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Printf("Pending saved: %s\n", result.Entry.Entry.ID)
	fmt.Printf("  Project: %s\n", flags.Project)
	fmt.Printf("  Title:   %s\n", result.Entry.Entry.Title)
	if result.Entry.Entry.BodyOptional != "" {
		fmt.Printf("  Note:    %s\n", result.Entry.Entry.BodyOptional)
	}
}

func runPendingList(ctx context.Context, svc *Services, args []string) {
	flags, err := ParsePendingListFlags(args)
	if err != nil {
		PrintError(err)
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
		PrintError(err)
		os.Exit(1)
	}
	if len(items) == 0 && flags.Query == "" && flags.Tag == "" && !flags.IncludeArchived {
		fmt.Printf("No pending items for project %s.\n", flags.Project)
		return
	}
	printPendingList(os.Stdout, "Pending items", flags.Project, items, flags.Query, flags.Tag, flags.IncludeArchived)
}

func runPendingReview(ctx context.Context, svc *Services, args []string) {
	flags, err := ParsePendingListFlags(args)
	if err != nil {
		PrintError(err)
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
		PrintError(err)
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
}

func runPendingShow(ctx context.Context, svc *Services, args []string) {
	flags, err := ParsePendingShowFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	item, err := svc.entrySvc.GetPending(ctx, flags.ID, true)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}
	printPendingDetails(os.Stdout, item)
}

func runPendingDone(ctx context.Context, svc *Services, args []string) {
	flags, err := ParsePendingDoneFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if err := svc.entrySvc.ResolvePending(ctx, flags.ID); err != nil {
		PrintError(err)
		os.Exit(1)
	}
	fmt.Printf("Resolved pending item: %s\n", flags.ID)
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
