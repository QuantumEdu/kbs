package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/quantum-6/skillvault/internal/agenttelemetry"
)

// RunTelemetryService executes lifecycle management commands for the telemetryd daemon.
func RunTelemetryService(w io.Writer, action string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "status"
	}

	cfg := agenttelemetry.DefaultConfig()

	switch action {
	case "status":
		return reportTelemetryStatus(w, cfg)
	case "start":
		return startTelemetryDaemon(w, cfg)
	case "stop":
		return stopTelemetryDaemon(w, cfg)
	case "restart":
		_ = stopTelemetryDaemon(w, cfg)
		time.Sleep(500 * time.Millisecond)
		return startTelemetryDaemon(w, cfg)
	case "install-service":
		return installTelemetrySystemdService(w)
	default:
		return fmt.Errorf("unknown telemetry service action: %s (expected: status, start, stop, restart, install-service)", action)
	}
}

func reportTelemetryStatus(w io.Writer, cfg agenttelemetry.Config) error {
	fmt.Fprintln(w, "SkillVault Telemetry Daemon Service")
	fmt.Fprintf(w, "  Socket:       %s\n", cfg.SocketPath)
	fmt.Fprintf(w, "  Database:     %s\n", cfg.DBPath)

	pids, _ := getTelemetryPIDs()
	sockOK := isSocketReachable(cfg.SocketPath)

	if sockOK {
		fmt.Fprintf(w, "  State:        RUNNING (socket responsive)\n")
		if len(pids) > 0 {
			fmt.Fprintf(w, "  PID(s):       %s\n", strings.Join(pids, ", "))
		}
	} else if len(pids) > 0 {
		fmt.Fprintf(w, "  State:        UNRESPONSIVE (process running with PID %s, but socket offline)\n", strings.Join(pids, ", "))
	} else {
		fmt.Fprintf(w, "  State:        STOPPED\n")
	}

	return nil
}

func startTelemetryDaemon(w io.Writer, cfg agenttelemetry.Config) error {
	if isSocketReachable(cfg.SocketPath) {
		fmt.Fprintln(w, "[sk-vault] telemetryd is already running and reachable.")
		return nil
	}

	binPath := resolveTelemetrydBin()
	if binPath == "" {
		fmt.Fprintln(w, "[sk-vault] telemetryd binary not found, building and installing...")
		InstallTelemetry(defaultInstallDir())
		binPath = resolveTelemetrydBin()
		if binPath == "" {
			return fmt.Errorf("telemetryd binary could not be built or found")
		}
	}

	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".telemetry")
	_ = os.MkdirAll(logDir, 0o755)
	logFile := filepath.Join(logDir, "telemetryd.log")
	outFile, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		outFile = os.Stdout
	}

	cmd := exec.Command(binPath)
	cmd.Stdout = outFile
	cmd.Stderr = outFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start telemetryd: %w", err)
	}

	pid := cmd.Process.Pid
	fmt.Fprintf(w, "[sk-vault] Launched telemetryd (PID %d). Waiting for socket initialization...\n", pid)

	// Wait up to 3 seconds for socket to respond
	ready := false
	for i := 0; i < 15; i++ {
		time.Sleep(200 * time.Millisecond)
		if isSocketReachable(cfg.SocketPath) {
			ready = true
			break
		}
	}

	if ready {
		fmt.Fprintf(w, "[sk-vault] telemetryd is active and ready on %s\n", cfg.SocketPath)
	} else {
		fmt.Fprintf(w, "[sk-vault] Warning: daemon started (PID %d) but socket did not respond within 3s. Check %s\n", pid, logFile)
	}
	return nil
}

func stopTelemetryDaemon(w io.Writer, cfg agenttelemetry.Config) error {
	pids, err := getTelemetryPIDs()
	if err != nil || len(pids) == 0 {
		fmt.Fprintln(w, "[sk-vault] No telemetryd processes currently running.")
		_ = os.Remove(cfg.SocketPath)
		return nil
	}

	fmt.Fprintf(w, "[sk-vault] Stopping telemetryd PID(s): %s...\n", strings.Join(pids, ", "))
	_ = exec.Command("pkill", "-SIGTERM", "-x", "telemetryd").Run()

	time.Sleep(500 * time.Millisecond)
	_ = os.Remove(cfg.SocketPath)
	fmt.Fprintln(w, "[sk-vault] telemetryd stopped.")
	return nil
}

func installTelemetrySystemdService(w io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("create systemd user unit dir: %w", err)
	}

	binPath := resolveTelemetrydBin()
	if binPath == "" {
		binPath = filepath.Join(home, "tools", "telemetryd")
	}

	content := fmt.Sprintf(`[Unit]
Description=SkillVault Agent Telemetry Daemon
After=network.target

[Service]
ExecStart=%s
Restart=always
RestartSec=5
Environment=TELEMETRY_STORE_PROMPTS=0

[Install]
WantedBy=default.target
`, binPath)

	unitPath := filepath.Join(unitDir, "telemetryd.service")
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	fmt.Fprintf(w, "[sk-vault] Created user systemd unit: %s\n", unitPath)

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err == nil {
		_ = exec.Command("systemctl", "--user", "enable", "telemetryd").Run()
		fmt.Fprintln(w, "[sk-vault] Executed systemctl --user daemon-reload and enabled telemetryd.")
	} else {
		fmt.Fprintln(w, "[sk-vault] Note: systemctl --user not active in this environment. Daemon can be managed directly with `skillvault telemetry service start`.")
	}

	return nil
}

// RunTelemetryInstallHooks writes convenient telemetry wrapper scripts for client agents.
func RunTelemetryInstallHooks(w io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	binDir := filepath.Join(home, ".local", "bin")
	_ = os.MkdirAll(binDir, 0o755)

	wrapBin := filepath.Join(home, "tools", "telemetrywrap")
	if _, err := os.Stat(wrapBin); err != nil {
		wrapBin = "telemetrywrap"
	}

	codexHook := fmt.Sprintf(`#!/usr/bin/env bash
# SkillVault Telemetry Wrapper for Codex CLI
exec %s --agent codex -- codex "$@"
`, wrapBin)

	opencodeHook := fmt.Sprintf(`#!/usr/bin/env bash
# SkillVault Telemetry Wrapper for OpenCode CLI
exec %s --agent opencode -- opencode "$@"
`, wrapBin)

	codexPath := filepath.Join(binDir, "codex-telemetry")
	opencodePath := filepath.Join(binDir, "opencode-telemetry")

	if err := os.WriteFile(codexPath, []byte(codexHook), 0o755); err != nil {
		return fmt.Errorf("write codex hook: %w", err)
	}
	if err := os.WriteFile(opencodePath, []byte(opencodeHook), 0o755); err != nil {
		return fmt.Errorf("write opencode hook: %w", err)
	}

	fmt.Fprintln(w, "[sk-vault] Installed agent telemetry hooks:")
	fmt.Fprintf(w, "  Codex:    %s\n", codexPath)
	fmt.Fprintf(w, "  OpenCode: %s\n", opencodePath)
	fmt.Fprintln(w, "Tip: To record telemetry for your agent sessions, invoke via these wrappers or alias them in your shell profile:")
	fmt.Fprintln(w, "  alias codex=\"codex-telemetry\"")
	fmt.Fprintln(w, "  alias opencode=\"opencode-telemetry\"")

	return nil
}

func isSocketReachable(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func getTelemetryPIDs() ([]string, error) {
	out, err := exec.Command("pgrep", "-x", "telemetryd").Output()
	if err != nil {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var pids []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if _, err := strconv.Atoi(l); err == nil {
			pids = append(pids, l)
		}
	}
	return pids, nil
}

func resolveTelemetrydBin() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "tools", "telemetryd"),
		filepath.Join(home, ".local", "bin", "telemetryd"),
		"/usr/local/bin/telemetryd",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	if path, err := exec.LookPath("telemetryd"); err == nil {
		return path
	}
	return ""
}
