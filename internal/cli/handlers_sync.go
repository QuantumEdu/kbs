package cli

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/quantum-6/skillvault/internal/api"
	"github.com/quantum-6/skillvault/internal/sync"
)

func runSync(ctx context.Context, cmd string, svc *Services, args []string) {
	flags, err := ParseSyncFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	// Build transport from flag (overrides config defaults).
	var transport sync.Transport
	switch flags.Transport {
	case "s3":
		t, err := sync.NewS3Transport(&sync.S3Config{
			Bucket:          os.Getenv("S3_BUCKET"),
			Region:          os.Getenv("AWS_REGION"),
			Endpoint:        os.Getenv("AWS_ENDPOINT"),
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		})
		if err != nil {
			PrintError(fmt.Errorf("s3 transport: %w", err))
			os.Exit(1)
		}
		transport = sync.NewGzipTransport(t)
	case "github":
		t, err := sync.NewGitHubTransport(&sync.GitHubConfig{
			Token: os.Getenv("GITHUB_TOKEN"),
			Owner: os.Getenv("GITHUB_OWNER"),
			Repo:  os.Getenv("GITHUB_REPO"),
		})
		if err != nil {
			PrintError(fmt.Errorf("github transport: %w", err))
			os.Exit(1)
		}
		transport = sync.NewGzipTransport(t)
	}

	svc.syncSvc.SetTransport(transport)

	if cmd == "sync-push" {
		if err := svc.syncSvc.Push(ctx, flags.RemotePath, flags.DryRun); err != nil {
			PrintError(err)
			os.Exit(1)
		}
		if !flags.DryRun {
			fmt.Printf("Pushed vault snapshot to %s\n", flags.RemotePath)
		}
	} else {
		if err := svc.syncSvc.Pull(ctx, flags.RemotePath, flags.DryRun); err != nil {
			PrintError(err)
			os.Exit(1)
		}
		if !flags.DryRun {
			fmt.Printf("Pulled vault snapshot from %s\n", flags.RemotePath)
		}
	}
}

func runHTTPServer(ctx context.Context, svc *Services, args []string) {
	apiKey := ""
	for i, a := range args[2:] {
		if a == "--api-key" && i+1 < len(args[2:]) {
			apiKey = args[2+i+1]
		}
	}
	srv := api.NewServer("127.0.0.1", 7438,
		svc.entrySvc, svc.artifactSvc, svc.contextSvc,
		svc.projectSvc, svc.sessionSvc, svc.workflowSvc,
		svc.exportSvc, svc.importSvc,
	).WithAPIKey(apiKey)
	fmt.Fprintf(os.Stderr, "[sk-vault] HTTP API server starting on 127.0.0.1:7438\n")
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stderr, "[sk-vault] HTTP API server shut down.")
}
