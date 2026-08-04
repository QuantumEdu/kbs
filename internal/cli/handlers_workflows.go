package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/quantum-6/skillvault/internal/app"
)

func runAddWorkflow(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseWorkflowFileFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	data, err := os.ReadFile(flags.FilePath)
	if err != nil {
		PrintError(fmt.Errorf("read workflow file: %w", err))
		os.Exit(1)
	}

	var input app.SaveWorkflowInput
	if err := json.Unmarshal(data, &input); err != nil {
		PrintError(fmt.Errorf("parse workflow JSON: %w", err))
		os.Exit(1)
	}

	wf, err := svc.workflowSvc.SaveWorkflow(ctx, input)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}
	fmt.Printf("Workflow saved: %s\n", wf.ID)
	fmt.Printf("  Name:        %s\n", wf.Name)
	fmt.Printf("  Description: %s\n", wf.Description)
}

func runRenderWorkflow(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseRenderWorkflowFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	steps, err := svc.workflowSvc.RenderWorkflow(ctx, flags.WorkflowID)
	if err != nil {
		PrintError(err)
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
}

func runRun(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseRunFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	// Read input from file or stdin
	var input string
	if flags.FilePath == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			PrintError(fmt.Errorf("read stdin: %w", err))
			os.Exit(1)
		}
		input = string(data)
	} else {
		data, err := os.ReadFile(flags.FilePath)
		if err != nil {
			PrintError(fmt.Errorf("read input file %q: %w", flags.FilePath, err))
			os.Exit(1)
		}
		input = string(data)
	}

	// Run the pipeline
	run, output, err := svc.workflowRunSvc.RunPipeline(ctx, flags.Workflow, input, os.Stdin, os.Stdout)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	// Write output to --save path if specified
	if flags.SavePath != "" {
		if err := os.WriteFile(flags.SavePath, []byte(output), 0644); err != nil {
			PrintError(fmt.Errorf("write output to %q: %w", flags.SavePath, err))
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Run %s completed. Output saved to %s\n", run.ID, flags.SavePath)
	} else {
		fmt.Fprintf(os.Stderr, "Run %s completed.\n", run.ID)
	}
}

func runImportWorkflow(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseImportWorkflowFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	filePath, err := filepath.Abs(flags.File)
	if err != nil {
		PrintError(fmt.Errorf("resolve workflow file path: %w", err))
		os.Exit(1)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		PrintError(fmt.Errorf("read workflow file: %w", err))
		os.Exit(1)
	}

	var projectID *string
	if flags.Project != "" {
		proj, err := svc.projectSvc.GetProject(ctx, flags.Project)
		if err != nil {
			PrintError(fmt.Errorf("project %q not found: %w", flags.Project, err))
			os.Exit(1)
		}
		projectID = &proj.ID
	}

	wf, slugs, err := svc.store.ImportWorkflowWithEntries(ctx, data, projectID)
	if err != nil {
		PrintError(err)
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
}

func runRoute(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseRouteFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	result, err := svc.entrySvc.RouteScenario(ctx, flags.Scenario)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if flags.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			PrintError(fmt.Errorf("marshal route result: %w", err))
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
}
