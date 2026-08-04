package cli

import "context"

// Run executes the handler for a top-level command. args must be the full
// command-line argument slice (including the program name and command).
func Run(cmd string, args []string) {
	svc := OpenVault()
	ctx := context.Background()

	switch cmd {
	case "add-entry":
		runAddEntry(ctx, svc, args)
	case "search":
		runSearch(ctx, svc, args)
	case "get":
		runGet(ctx, svc, args)
	case "save-artifact":
		runSaveArtifact(ctx, svc, args)
	case "archive":
		runArchive(ctx, svc, args)
	case "pending-add":
		runPendingAdd(ctx, svc, args)
	case "pending-list":
		runPendingList(ctx, svc, args)
	case "pending-review":
		runPendingReview(ctx, svc, args)
	case "pending-show":
		runPendingShow(ctx, svc, args)
	case "pending-done":
		runPendingDone(ctx, svc, args)
	case "add-workflow":
		runAddWorkflow(ctx, svc, args)
	case "render-workflow":
		runRenderWorkflow(ctx, svc, args)
	case "run":
		runRun(ctx, svc, args)
	case "import-workflow":
		runImportWorkflow(ctx, svc, args)
	case "route":
		runRoute(ctx, svc, args)
	case "get-context":
		runGetContext(ctx, svc, args)
	case "session-wrap":
		runSessionWrap(ctx, svc, args)
	case "export":
		runExport(ctx, svc, args)
	case "import":
		runImport(ctx, svc, args)
	case "backup":
		runBackup(ctx, svc, args)
	case "list-projects":
		runListProjects(ctx, svc, args)
	case "add-project":
		runAddProject(ctx, svc, args)
	case "save-result":
		runSaveResult(ctx, svc, args)
	case "stats":
		runStats(ctx, svc, args)
	case "memory-index", "memory-reindex", "memory-list-external":
		runMemory(ctx, cmd, svc, args)
	case "compare-entries":
		runCompareEntries(ctx, svc, args)
	case "setup-vectors":
		runSetupVectors(ctx, svc, args)
	case "reindex-embeddings":
		runReindexEmbeddings(ctx, svc, args)
	case "graph":
		runGraph(ctx, svc, args)
	case "sync-push", "sync-pull":
		runSync(ctx, cmd, svc, args)
	case "entry-history":
		runEntryHistory(ctx, svc, args)
	case "entry-restore":
		runEntryRestore(ctx, svc, args)
	case "entry-ref":
		runEntryRef(ctx, svc, args)
	case "http":
		runHTTPServer(ctx, svc, args)
	}
}
