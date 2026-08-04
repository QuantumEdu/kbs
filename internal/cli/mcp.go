package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/quantum-6/skillvault/internal/mcp"
)

// RunMCP starts the stdio MCP JSON-RPC server backed by the vault services.
func RunMCP() {
	svc := OpenVault()

	reg := mcp.NewServiceToolRegistry(
		svc.entrySvc,
		svc.artifactSvc,
		svc.contextSvc,
		svc.seriesSvc,
		svc.workflowSvc,
		svc.sessionSvc,
		svc.projectSvc,
	).WithEntryRefService(svc.entryRefSvc).WithCompareService(svc.compareSvc).WithSaveResultService(svc.saveResultSvc).WithWorkflowRunService(svc.workflowRunSvc).WithStatsService(svc.statsSvc).WithEntryVersionService(svc.entryVersionSvc)
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
