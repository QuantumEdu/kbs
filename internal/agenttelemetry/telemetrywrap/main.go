// Command telemetrywrap wraps an arbitrary CLI command and emits heuristic
// telemetry events to the agent telemetry daemon. It is a fallback for agents
// that do not have a native plugin integration.
//
// Usage:
//
//	telemetrywrap --agent <id> -- <cmd> [args...]
//
// Events are flagged with confidence_level: heuristic and source: wrapper.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	agentFlag := flag.String("agent", "", "Agent ID (required)")
	flag.Parse()

	if *agentFlag == "" || flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: telemetrywrap --agent <id> -- <cmd> [args...]\n")
		os.Exit(1)
	}

	cmdArgs := flag.Args()
	code := run(*agentFlag, cmdArgs)
	os.Exit(code)
}

func run(agentID string, cmdArgs []string) int {
	socketPath := resolveSocket()
	workspace, _ := os.Getwd()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Emit run.started.
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	startPayload, _ := json.Marshal(map[string]string{
		"command": strings.Join(cmdArgs, " "),
		"cwd":     workspace,
	})
	emitEvent(socketPath, agentEvent(agentID, runID, "run.started", "heuristic", startPayload))

	// Emit command.started.
	cmdPayload, _ := json.Marshal(map[string]string{
		"command": strings.Join(cmdArgs, " "),
	})
	emitEvent(socketPath, agentEvent(agentID, runID, "command.started", "heuristic", cmdPayload))

	// Start the command. stdout is tee'd: the user sees the child's output
	// live while the parser scans the same stream for model/token usage.
	// stderr passes through untouched.
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		emitEvent(socketPath, agentEvent(agentID, runID, "run.failed", "heuristic",
			json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error()))))
		return 1
	}
	cmd.Stdout = io.MultiWriter(os.Stdout, stdoutW)
	cmd.Stderr = os.Stderr

	// Start git polling in background.
	gitPollDone := make(chan struct{})
	go pollGitChanges(socketPath, agentID, runID, workspace, gitPollDone)

	// Start stdout parsing in background.
	modelDone := make(chan struct{})
	go parseStdoutForModels(socketPath, agentID, runID, stdoutR, modelDone)

	// Run the command.
	err = cmd.Start()
	if err != nil {
		emitEvent(socketPath, agentEvent(agentID, runID, "run.failed", "heuristic",
			json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error()))))
		stdoutW.Close() // parser scanner hits EOF and its goroutine exits
		close(gitPollDone)
		close(modelDone)
		return 1
	}

	// Wait for completion or timeout.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var runErr error
	select {
	case runErr = <-done:
		// Normal completion.
	case <-ctx.Done():
		// Timeout: SIGTERM, then grace period, then SIGKILL.
		cmd.Process.Signal(os.Interrupt)
		select {
		case runErr = <-done:
		case <-time.After(30 * time.Second):
			cmd.Process.Kill()
			runErr = <-done
		}
	}

	// cmd.Wait has reaped the child and its dup of the pipe write end on
	// every path (normal exit, timeout SIGTERM, timeout SIGKILL), so closing
	// our write end delivers EOF to the parser scanner and its goroutine
	// exits. Single close per path: the Start-error branch above returns
	// before reaching this point.
	stdoutW.Close()
	close(gitPollDone)
	close(modelDone)

	if runErr != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": runErr.Error()})
		emitEvent(socketPath, agentEvent(agentID, runID, "run.failed", "heuristic", errPayload))
		// Transparency: propagate the child's exit code. ExitCode returns -1
		// when the process was killed by a signal; fall back to 1 there and
		// for non-exit errors.
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			if code := exitErr.ExitCode(); code > 0 {
				return code
			}
		}
		return 1
	}

	// Emit run.completed.
	compPayload, _ := json.Marshal(map[string]string{"status": "completed"})
	emitEvent(socketPath, agentEvent(agentID, runID, "run.completed", "heuristic", compPayload))

	return 0
}

// agentEvent builds a basic event with common fields filled in.
func agentEvent(agentID, runID, eventType, confidence string, payload json.RawMessage) map[string]interface{} {
	return map[string]interface{}{
		"event_id":         fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		"event_type":       eventType,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"run_id":           runID,
		"agent_id":         agentID,
		"agent_version":    "",
		"source":           "wrapper",
		"redaction_policy": "hash-args",
		"confidence_level": confidence,
		"payload":          payload,
	}
}

// emitEvent sends a line-delimited JSON event to the daemon socket.
// Errors are silently ignored — the wrapper should not fail if telemetry is down.
func emitEvent(socketPath string, evt map[string]interface{}) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	data = append(data, '\n')

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	conn.Write(data)
}

// pollGitChanges polls git diff --stat every 2 seconds and emits
// file.created, file.modified, and file.deleted events when changes are detected.
func pollGitChanges(socketPath, agentID, runID, workspace string, done <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	prevSnapshot := getGitStat(workspace)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			current := getGitStat(workspace)
			if current == prevSnapshot {
				continue
			}
			prevSnapshot = current

			// Emit generic file.change event for the diff stat.
			payload, _ := json.Marshal(map[string]string{
				"diff_stat": current,
			})
			emitEvent(socketPath, agentEvent(agentID, runID, "file.modified", "heuristic", payload))
		}
	}
}

// getGitStat returns the output of git diff --stat or empty string if not in a repo.
func getGitStat(workspace string) string {
	cmd := exec.Command("git", "diff", "--stat")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// parseStdoutForModels scans the command's stdout for model/token patterns
// and emits model.usage events when detected.
func parseStdoutForModels(socketPath, agentID, runID string, r io.Reader, done <-chan struct{}) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-done:
			return
		default:
		}
		info, ok := parseUsageLine(scanner.Text())
		if info == nil {
			continue
		}
		payloadInfo := make(map[string]interface{}, len(info))
		for key, value := range info {
			payloadInfo[key] = value
		}
		delete(payloadInfo, "provider")
		delete(payloadInfo, "model")
		delete(payloadInfo, "effort")
		payload, _ := json.Marshal(payloadInfo)
		evt := agentEvent(agentID, runID, "model.usage", "heuristic", payload)
		evt["coverage"] = "unknown"
		if ok {
			evt["provider"], evt["model"], evt["effort"] = info["provider"], info["model"], info["effort"]
			evt["interaction_id"], evt["coverage"] = info["interaction_id"], "bounded"
		}
		emitEvent(socketPath, evt)
	}
}

// parseUsageLine accepts only a bounded provider JSON contract. Ambiguous
// human output becomes explicit unknown coverage and its raw line is discarded.
func parseUsageLine(line string) (map[string]interface{}, bool) {
	var input struct {
		Provider      string `json:"provider"`
		Model         string `json:"model"`
		Effort        string `json:"effort"`
		SampleID      string `json:"sample_id"`
		InteractionID string `json:"interaction_id"`
		Usage         struct {
			Input      *int64 `json:"input"`
			Output     *int64 `json:"output"`
			CacheRead  *int64 `json:"cache_read"`
			CacheWrite *int64 `json:"cache_write"`
			Reasoning  *int64 `json:"reasoning"`
		} `json:"usage"`
	}
	err := json.Unmarshal([]byte(line), &input)
	known := input.Usage.Input != nil || input.Usage.Output != nil || input.Usage.CacheRead != nil || input.Usage.CacheWrite != nil || input.Usage.Reasoning != nil
	if err != nil || input.Provider == "" || input.Model == "" || input.SampleID == "" || input.InteractionID == "" || !known {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "token") || strings.Contains(lower, "usage") {
			return map[string]interface{}{"coverage": "unknown", "reason": "unsupported_wrapper"}, false
		}
		return nil, false
	}
	return map[string]interface{}{"schema_version": 1, "sample_id": input.SampleID, "interaction_id": input.InteractionID, "mode": "delta", "segment_id": "", "reset": false, "method": "estimated", "estimated_method": "wrapper_bounded_json", "tokens": map[string]*int64{"input": input.Usage.Input, "output": input.Usage.Output, "cache_read": input.Usage.CacheRead, "cache_write": input.Usage.CacheWrite, "reasoning": input.Usage.Reasoning}, "provider": input.Provider, "model": input.Model, "effort": input.Effort}, true
}

// resolveSocket returns the telemetry daemon socket path.
func resolveSocket() string {
	if v := os.Getenv("TELEMETRY_SOCKET"); v != "" {
		return v
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "telemetryd.sock")
	}
	return "/tmp/telemetryd.sock"
}
