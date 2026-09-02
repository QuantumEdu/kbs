package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func pingDaemon(socketPath string) error {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func runLive(dbPath string, args []string) {
	fs := flag.NewFlagSet("live", flag.ExitOnError)
	interval := fs.Duration("interval", 2*time.Second, "Poll interval")
	once := fs.Bool("once", false, "Render once and exit without streaming")
	limit := fs.Int("limit", 5, "Number of recent runs to show")
	_ = fs.Parse(args)

	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	renderCycle := func() {
		if !*once {
			fmt.Print("\033[H\033[2J")
		}

		now := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("=== SkillVault Agent Telemetry Live Monitor [%s] ===\n", now)

		sock := resolveSocketPath()
		daemonStatus := "OFFLINE"
		if pingDaemon(sock) == nil {
			daemonStatus = "ONLINE (socket active)"
		}
		fmt.Printf("Daemon: %s (%s)\n", daemonStatus, sock)

		rows, err := db.QueryContext(ctx, `
			SELECT id, agent_id, status, total_tokens, total_cost_usd, started_at, completed_at
			FROM agent_runs
			ORDER BY started_at DESC
			LIMIT ?
		`, *limit)
		if err != nil {
			fmt.Printf("Error querying runs: %v\n", err)
			return
		}
		defer rows.Close()

		fmt.Println("\nActive & Recent Agent Runs:")
		fmt.Println("ID                   AGENT       STATUS    TOKENS     COST       DURATION")
		fmt.Println("-------------------------------------------------------------------------")
		count := 0
		for rows.Next() {
			count++
			r, err := scanRunRow(rows)
			if err != nil {
				continue
			}
			runID := r.ID
			if len(runID) > 20 {
				runID = runID[:20]
			}
			agentID := r.AgentID
			if len(agentID) > 10 {
				agentID = agentID[:10]
			}
			fmt.Printf("%-20s %-11s %-9s %-10d $%-9.4f %s\n",
				runID, agentID, r.Status, r.TotalTokens, r.CostUSD, formatDuration(r))
		}
		if count == 0 {
			fmt.Println("  (no agent runs recorded yet)")
		}

		eventRows, err := db.QueryContext(ctx, `
			SELECT event_type, COUNT(*) 
			FROM telemetry_events 
			WHERE timestamp >= datetime('now', '-10 minutes')
			GROUP BY event_type
		`)
		if err == nil {
			fmt.Println("\nRecent Activity (last 10m):")
			hasEvents := false
			for eventRows.Next() {
				var evType string
				var evCount int
				if err := eventRows.Scan(&evType, &evCount); err == nil {
					hasEvents = true
					warning := ""
					if evType == "stall_detected" || evType == "loop_detected" {
						warning = " [WARNING! ATTENTION REQUIRED]"
					}
					fmt.Printf("  %-25s : %d%s\n", evType, evCount, warning)
				}
			}
			eventRows.Close()
			if !hasEvents {
				fmt.Println("  (no events in last 10 minutes)")
			}
		}

		if !*once {
			fmt.Println("\nPress Ctrl+C to exit monitor.")
		}
	}

	renderCycle()
	if *once {
		return
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nExiting live monitor.")
			return
		case <-ticker.C:
			renderCycle()
		}
	}
}
