// Command telemetryctl queries the agent telemetry database.
//
// Subcommands:
//
//	run list    table of recent runs (--limit, --agent, --since)
//	run show ID detailed view of a single run
//	run recent  last 5 runs summary
//	status      daemon operational status
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/quantum-6/skillvault/internal/agenttelemetry"
	_ "modernc.org/sqlite"
)

func main() {
	// Global -db flag before subcommand.
	dbFlag := flag.String("db", "", "Path to telemetry database")
	flag.Parse()

	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = resolveDBPath()
	}

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	switch args[0] {
	case "run":
		if len(args) < 2 {
			usage()
			os.Exit(1)
		}
		switch args[1] {
		case "list":
			runList(dbPath, args[2:])
		case "show":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "telemetryctl: run show requires a run ID")
				os.Exit(1)
			}
			runShow(dbPath, args[2])
		case "recent":
			runRecent(dbPath)
		default:
			fmt.Fprintf(os.Stderr, "telemetryctl: unknown run subcommand %q\n", args[1])
			usage()
			os.Exit(1)
		}
	case "status":
		runStatus(dbPath)
	case "report":
		if len(args) == 3 && args[1] == "next-change" && args[2] == "--help" {
			fmt.Fprintln(os.Stdout, "Usage: telemetryctl report next-change\nPrint evidence-cited recommendations and coverage gaps.")
			return
		}
		if len(args) != 2 || args[1] != "next-change" {
			fmt.Fprintln(os.Stderr, "telemetryctl: report supports next-change")
			usage()
			os.Exit(1)
		}
		runNextChange(dbPath)
	default:
		fmt.Fprintf(os.Stderr, "telemetryctl: unknown command %q\n", args[0])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage:
  telemetryctl run list   [--limit N] [--agent ID] [--since RFC3339]
  telemetryctl run show   <run-id>
  telemetryctl run recent
  telemetryctl status
  telemetryctl report next-change

Global flags:
  -db PATH   Override database path (default: ~/.telemetry/telemetry.db or $TELEMETRY_DB_PATH)
`)
}

// resolveDBPath returns the telemetry database path from env var or default.
func resolveDBPath() string {
	if v := os.Getenv("TELEMETRY_DB_PATH"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", ".telemetry", "telemetry.db")
	}
	return filepath.Join(home, ".telemetry", "telemetry.db")
}

// resolveSocketPath returns the daemon Unix socket path from env var or default.
func resolveSocketPath() string {
	if v := os.Getenv("TELEMETRY_SOCKET"); v != "" {
		return v
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "telemetryd.sock")
	}
	return "/tmp/telemetryd.sock"
}

// openDB opens the SQLite database in read-only mode.
func openDB(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite", dbPath+"?mode=ro")
}

// scanRunRow scans a row from agent_runs into a runRow.
type runRow struct {
	ID          string
	AgentID     string
	Status      string
	TotalTokens int64
	CostUSD     float64
	StartedAt   time.Time
	CompletedAt *time.Time
}

func scanRunRow(scanner interface {
	Scan(dest ...interface{}) error
}) (runRow, error) {
	var r runRow
	var startedStr, completedStr sql.NullString
	err := scanner.Scan(
		&r.ID, &r.AgentID, &r.Status,
		&r.TotalTokens, &r.CostUSD,
		&startedStr, &completedStr,
	)
	if err != nil {
		return r, err
	}
	r.StartedAt, _ = time.Parse(time.RFC3339, startedStr.String)
	if completedStr.Valid {
		t, _ := time.Parse(time.RFC3339, completedStr.String)
		r.CompletedAt = &t
	}
	return r, nil
}

// formatDuration returns "Ns", "running", or "—".
func formatDuration(r runRow) string {
	if r.Status == "running" || r.CompletedAt == nil {
		return "running"
	}
	d := r.CompletedAt.Sub(r.StartedAt)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	h := m / 60
	rm := m % 60
	return fmt.Sprintf("%dh%dm", h, rm)
}

// ---------------------------------------------------------------------------
// run list
// ---------------------------------------------------------------------------

func runList(dbPath string, args []string) {
	fs := flag.NewFlagSet("run list", flag.ExitOnError)
	limit := fs.Int("limit", 20, "Maximum runs to show")
	agent := fs.String("agent", "", "Filter by agent ID")
	sinceStr := fs.String("since", "", "Show runs since (RFC3339)")
	_ = fs.Parse(args)

	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var clauses []string
	var params []interface{}
	if *agent != "" {
		clauses = append(clauses, "agent_id = ?")
		params = append(params, *agent)
	}
	if *sinceStr != "" {
		t, err := time.Parse(time.RFC3339, *sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "telemetryctl: invalid --since: %v\n", err)
			os.Exit(1)
		}
		clauses = append(clauses, "started_at >= ?")
		params = append(params, t.UTC().Format(time.RFC3339))
	}

	query := "SELECT id, agent_id, status, total_tokens, total_cost_usd, started_at, completed_at FROM agent_runs"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	params = append(params, *limit)

	rows, err := db.Query(query, params...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: query runs: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tAGENT\tSTATUS\tTOKENS\tCOST\tDURATION")

	count := 0
	for rows.Next() {
		r, err := scanRunRow(rows)
		if err != nil {
			fmt.Fprintf(os.Stderr, "telemetryctl: scan row: %v\n", err)
			os.Exit(1)
		}
		shortID := r.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t$%.4f\t%s\n",
			shortID, r.AgentID, r.Status, r.TotalTokens, r.CostUSD, formatDuration(r))
		count++
	}
	w.Flush()

	if count == 0 {
		fmt.Println("No runs found.")
	}
}

// ---------------------------------------------------------------------------
// run show
// ---------------------------------------------------------------------------

func runShow(dbPath, runID string) {
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()

	// Header: query agent run.
	row := db.QueryRowContext(ctx, `SELECT id, agent_id, COALESCE(agent_version,''), COALESCE(repo_url,''),
		COALESCE(branch,''), COALESCE(commit_sha,''), workspace, started_at, completed_at, status,
		total_tokens, total_cost_usd, COALESCE(error_type,''), COALESCE(error_message,'')
		FROM agent_runs WHERE id = ?`, runID)

	var (
		id, agentID, agentVersion, repoURL, branch, commitSHA, workspace string
		startedStr, completedStr                                         sql.NullString
		status                                                           string
		totalTokens                                                      int64
		totalCost                                                        float64
		errorType, errorMessage                                          string
	)
	if err := row.Scan(&id, &agentID, &agentVersion, &repoURL, &branch, &commitSHA,
		&workspace, &startedStr, &completedStr, &status,
		&totalTokens, &totalCost, &errorType, &errorMessage); err != nil {
		if err == sql.ErrNoRows {
			fmt.Fprintf(os.Stderr, "telemetryctl: run %q not found\n", runID)
		} else {
			fmt.Fprintf(os.Stderr, "telemetryctl: query run: %v\n", err)
		}
		os.Exit(1)
	}

	startedAt, _ := time.Parse(time.RFC3339, startedStr.String)
	var completedAt *time.Time
	if completedStr.Valid {
		t, _ := time.Parse(time.RFC3339, completedStr.String)
		completedAt = &t
	}

	fmt.Println("== Run Header ==")
	fmt.Printf("  Run ID:      %s\n", id)
	fmt.Printf("  Agent:       %s %s\n", agentID, agentVersion)
	fmt.Printf("  Workspace:   %s\n", workspace)
	if repoURL != "" {
		fmt.Printf("  Repo:        %s\n", repoURL)
	}
	if branch != "" {
		fmt.Printf("  Branch:      %s\n", branch)
	}
	if commitSHA != "" {
		fmt.Printf("  Commit:      %s\n", commitSHA)
	}
	fmt.Printf("  Status:      %s\n", status)
	fmt.Printf("  Started:     %s\n", startedAt.Format(time.RFC3339))
	if completedAt != nil {
		fmt.Printf("  Completed:   %s\n", completedAt.Format(time.RFC3339))
	}
	if errorType != "" {
		fmt.Printf("  Error:       %s — %s\n", errorType, errorMessage)
	}
	fmt.Printf("  Total tokens: %d\n", totalTokens)
	fmt.Printf("  Total cost:   $%.4f\n", totalCost)
	fmt.Println()

	// Step tree.
	fmt.Println("== Steps ==")
	stepsQuery := `SELECT id, step_name, step_index, started_at, completed_at, duration_ms
		FROM agent_steps WHERE run_id = ? ORDER BY step_index`
	sRows, err := db.QueryContext(ctx, stepsQuery, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  query steps: %v\n", err)
	} else {
		defer sRows.Close()
		stepCount := 0
		for sRows.Next() {
			var sid, sname string
			var sindex int
			var sStartStr, sEndStr sql.NullString
			var sdur int64
			if err := sRows.Scan(&sid, &sname, &sindex, &sStartStr, &sEndStr, &sdur); err != nil {
				fmt.Fprintf(os.Stderr, "  scan step: %v\n", err)
				continue
			}
			sTime, _ := time.Parse(time.RFC3339, sStartStr.String)
			fmt.Printf("  %2d. %s\n", sindex, sname)
			fmt.Printf("       id: %s\n", sid)
			fmt.Printf("       started: %s\n", sTime.Format(time.RFC3339))
			if sEndStr.Valid {
				eTime, _ := time.Parse(time.RFC3339, sEndStr.String)
				fmt.Printf("       completed: %s\n", eTime.Format(time.RFC3339))
			}
			if sdur > 0 {
				fmt.Printf("       duration: %dms\n", sdur)
			}
			stepCount++
		}
		if stepCount == 0 {
			fmt.Println("  (no steps)")
		}
	}
	fmt.Println()

	// Token/cost breakdown.
	fmt.Println("== Token / Cost Breakdown ==")
	tuQuery := `SELECT id, COALESCE(step_id,''), model, input_tokens, output_tokens,
		total_tokens, cost_usd, estimation_method, efficiency_ratio
		FROM token_usage WHERE run_id = ? ORDER BY recorded_at`
	tuRows, err := db.QueryContext(ctx, tuQuery, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  query token_usage: %v\n", err)
	} else {
		defer tuRows.Close()
		tuCount := 0
		for tuRows.Next() {
			var tid, stepIDRef, model string
			var inTok, outTok, totTok int64
			var cost float64
			var method string
			var effRatio sql.NullFloat64
			if err := tuRows.Scan(&tid, &stepIDRef, &model, &inTok, &outTok, &totTok,
				&cost, &method, &effRatio); err != nil {
				fmt.Fprintf(os.Stderr, "  scan token_usage: %v\n", err)
				continue
			}
			fmt.Printf("  %s: model=%s input=%d output=%d total=%d cost=$%.4f method=%s",
				tid, model, inTok, outTok, totTok, cost, method)
			if effRatio.Valid {
				fmt.Printf(" efficiency=%.4f", effRatio.Float64)
			}
			fmt.Println()
			tuCount++
		}
		if tuCount == 0 {
			fmt.Println("  (no token usage records)")
		}
	}
	fmt.Println()

	// Quality signal summary.
	fmt.Println("== Quality Signals ==")
	evQuery := `SELECT event_type, timestamp, payload FROM events
		WHERE run_id = ? AND event_type IN ('loop.detected','policy.violation')
		ORDER BY timestamp`
	evRows, err := db.QueryContext(ctx, evQuery, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  query events: %v\n", err)
	} else {
		defer evRows.Close()
		qCount := 0
		for evRows.Next() {
			var eType string
			var ts string
			var payload string
			if err := evRows.Scan(&eType, &ts, &payload); err != nil {
				fmt.Fprintf(os.Stderr, "  scan event: %v\n", err)
				continue
			}
			fmt.Printf("  %s at %s\n", eType, ts)
			if payload != "" && payload != "{}" {
				fmt.Printf("    %s\n", payload)
			}
			qCount++
		}
		if qCount == 0 {
			fmt.Println("  (no quality signals)")
		}
	}
}

// ---------------------------------------------------------------------------
// run recent
// ---------------------------------------------------------------------------

func runRecent(dbPath string) {
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, agent_id, status, total_tokens, total_cost_usd,
		started_at, completed_at
		FROM agent_runs ORDER BY started_at DESC LIMIT 5`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: query runs: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tAGENT\tSTATUS\tTOKENS\tCOST\tDURATION")

	count := 0
	for rows.Next() {
		r, err := scanRunRow(rows)
		if err != nil {
			fmt.Fprintf(os.Stderr, "telemetryctl: scan row: %v\n", err)
			os.Exit(1)
		}
		shortID := r.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t$%.4f\t%s\n",
			shortID, r.AgentID, r.Status, r.TotalTokens, r.CostUSD, formatDuration(r))
		count++
	}
	w.Flush()

	if count == 0 {
		fmt.Println("No runs found.")
	}
}

// ---------------------------------------------------------------------------
// report
// ---------------------------------------------------------------------------

func runNextChange(dbPath string) {
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: open db: %v\n", err)
		return
	}
	defer db.Close()
	report, err := agenttelemetry.ReportNextChangesDB(context.Background(), db, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: report next-change: %v\n", err)
		return
	}
	fmt.Println("== Next Change Evidence ==")
	fmt.Println("Time evidence: measured=unknown estimated=unknown inferred=unknown")
	if len(report.Recommendations) == 0 {
		fmt.Println("Recommendations: none (activity alone is not debt evidence)")
	} else {
		for _, item := range report.Recommendations {
			fmt.Printf("- %s %s at %s [evidence:%s confidence:%s coverage:%s]\n", item.Severity, item.Tool, item.Location, item.EvidenceID, item.Confidence, item.Coverage)
		}
	}
	for _, gap := range report.Gaps {
		fmt.Printf("Gap: %s\n", gap)
	}
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func runStatus(dbPath string) {
	// 1. Events ingested (DB query).
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var eventsIngested int64
	if err := db.QueryRow("SELECT COUNT(*) FROM events").Scan(&eventsIngested); err != nil {
		fmt.Fprintf(os.Stderr, "telemetryctl: count events: %v\n", err)
		os.Exit(1)
	}

	// 2. DB size.
	var dbSize int64
	if info, err := os.Stat(dbPath); err == nil {
		dbSize = info.Size()
	}

	fmt.Println("== Daemon Status ==")
	fmt.Printf("  Events ingested:  %d\n", eventsIngested)
	fmt.Printf("  DB size:          %s\n", formatBytes(dbSize))

	// 3. Query daemon via Unix socket for runtime-only fields.
	socketPath := resolveSocketPath()
	conn, dialErr := net.Dial("unix", socketPath)
	if dialErr != nil {
		fmt.Println("  daemon:           not running")
		return
	}
	defer conn.Close()

	// Send status command.
	if _, err := fmt.Fprintf(conn, `{"cmd":"status"}`+"\n"); err != nil {
		fmt.Println("  daemon:           not reachable")
		return
	}

	// Set a read deadline to avoid hanging.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var buf []byte
	b := make([]byte, 4096)
	for {
		n, err := conn.Read(b)
		if n > 0 {
			buf = append(buf, b[:n]...)
			// Check for complete JSON-L line.
			if buf[len(buf)-1] == '\n' {
				break
			}
		}
		if err != nil {
			if err == io.EOF && len(buf) > 0 && buf[len(buf)-1] == '\n' {
				break
			}
			fmt.Println("  daemon:           not reachable")
			return
		}
	}

	// Parse response.
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			UptimeSeconds     int64    `json:"uptime_seconds"`
			SaltFingerprint   string   `json:"salt_fingerprint"`
			RedactionPatterns []string `json:"redaction_patterns"`
			PromptStorage     bool     `json:"prompt_storage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bytesTrimNewline(buf), &resp); err != nil {
		fmt.Fprintf(os.Stderr, "  daemon response parse error: %v\n", err)
		return
	}
	if resp.Status != "ok" {
		fmt.Println("  daemon:           read-only mode")
		return
	}

	fmt.Printf("  Uptime:           %s\n", formatUptime(resp.Data.UptimeSeconds))
	fmt.Printf("  Salt fingerprint: %s\n", resp.Data.SaltFingerprint)
	fmt.Printf("  Redaction patterns: %d\n", len(resp.Data.RedactionPatterns))
	if resp.Data.PromptStorage {
		fmt.Println("  Prompt storage:   enabled")
	} else {
		fmt.Println("  Prompt storage:   disabled")
	}
}

func bytesTrimNewline(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}
	return b
}

func formatBytes(n int64) string {
	if n == 0 {
		return "0 B"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func formatUptime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	m := seconds / 60
	if m < 60 {
		return fmt.Sprintf("%dm%ds", m, seconds%60)
	}
	h := m / 60
	m = m % 60
	if h < 24 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	d := h / 24
	h = h % 24
	return fmt.Sprintf("%dd%dh%dm", d, h, m)
}
