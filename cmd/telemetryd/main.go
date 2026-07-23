package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/quantum-6/skillvault/internal/agenttelemetry"
)

func main() {
	log.SetFlags(0)

	cfg := agenttelemetry.DefaultConfig()

	// Ensure data directory exists.
	dataDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("telemetryd: create data dir %s: %v", dataDir, err)
	}

	// Open store.
	store, err := agenttelemetry.OpenStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("telemetryd: open store: %v", err)
	}
	defer store.Close()

	log.Printf("telemetryd: store opened at %s", cfg.DBPath)

	// Initialize security pipeline.
	pipeline, err := agenttelemetry.NewSecurityPipeline(cfg.SaltPath, cfg.RedactionPatterns)
	if err != nil {
		log.Fatalf("telemetryd: security pipeline: %v", err)
	}
	log.Printf("telemetryd: security pipeline initialized (salt: %s)", cfg.SaltPath)

	// Create collector.
	collector := agenttelemetry.NewCollector(store, cfg.SocketPath)
	collector.SetSecurityPipeline(pipeline)
	collector.SetDaemonStartTime(time.Now())
	collector.SetDBPath(cfg.DBPath)
	collector.SetPromptStorage(cfg.StorePrompts)

	// Wire quality signal detectors.
	collector.SetLoopDetector(agenttelemetry.NewLoopDetector())
	collector.SetStallDetector(agenttelemetry.NewStallDetector())
	collector.SetStreakDetector(agenttelemetry.NewStreakDetector())
	collector.SetTokenCounter(agenttelemetry.NewTokenCounter())

	// Start with retry.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		var listenErr error
		for attempt := 1; attempt <= 3; attempt++ {
			listenErr = collector.Listen(ctx)
			if listenErr == nil {
				return
			}
			if attempt < 3 {
				backoff := time.Duration(attempt) * time.Second
				log.Printf("telemetryd: listen attempt %d failed: %v (retrying in %v)", attempt, listenErr, backoff)
				time.Sleep(backoff)
			}
		}
		log.Fatalf("telemetryd: listen failed after 3 attempts: %v", listenErr)
	}()

	log.Printf("telemetryd: listening on %s", cfg.SocketPath)

	// Handle signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh

	log.Printf("telemetryd: received %v, shutting down", sig)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := collector.Shutdown(shutdownCtx); err != nil {
		log.Printf("telemetryd: shutdown error: %v", err)
		os.Exit(1)
	}

	log.Println("telemetryd: shutdown complete")
}
