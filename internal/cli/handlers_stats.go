package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/app"
)

func runSaveResult(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseSaveResultFlags(args)
	if err != nil {
		PrintError(err)
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
		PrintError(err)
		os.Exit(1)
	}

	fmt.Print(FormatSaveResultOutput(SaveResultOutput{
		EntryID:   output.EntryID,
		Name:      output.Name,
		Type:      output.Type,
		ProjectID: output.ProjectID,
	}))
}

func runStats(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseStatsFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	stats, err := svc.statsSvc.GetStats(ctx)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if flags.JSON {
		data, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			PrintError(fmt.Errorf("marshal stats: %w", err))
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		fmt.Println(app.FormatStats(stats))
		if flags.WorkflowRuns && stats.WorkflowRuns != nil {
			fmt.Print(app.FormatWorkflowRunStats(stats.WorkflowRuns))
		}
	}
}
