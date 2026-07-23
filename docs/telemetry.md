# Agent Telemetry System

Local-first observability for CLI LLM coding agents. Collects, redacts, and persists
tool calls, token usage, quality signals, and agent lifecycle events to SQLite.

## Components

- **`telemetryd`** — Unix socket daemon that validates, redacts, hashes, and persists events
- **`telemetryctl`** — CLI to query runs, events, and daemon status
- **`telemetrywrap`** — CLI wrapper that infers telemetry events from any command
- **OpenCode plugin** — Native `EventEmitter` emitting 9 canonical event types
- **`q-secrets`** (optional) — Local encrypted secret manager, installable via `skillvault secrets install` or `skillvault init --with-secrets`

## Installation

### From the kbs repo (any machine)

```bash
# Clone the repo (first time only)
git clone https://github.com/QuantumEdu/kbs.git
cd kbs
git submodule update --init

# One-shot install: all binaries to ~/tools/
make install-all

# Or use skillvault (if already installed)
skillvault init --all         # init vault + install q-secrets + install telemetry
skillvault install-telemetry  # install telemetry only
skillvault update             # pull latest + rebuild everything
```

### Via skillvault (once installed)

```bash
skillvault install-telemetry        # build + install telemetryd, telemetryctl, telemetrywrap
skillvault init --with-telemetry    # init vault + install telemetry
skillvault init --all               # init vault + q-secrets + telemetry
skillvault update                   # pull latest, rebuild skillvault + q-secrets + telemetry
```

Environment variables:

| Env var | Effect |
|---------|--------|
| `SKILLVAULT_REPO` | Point to the kbs repo (auto-detected if skillvault inside the repo) |
| `SKIP_Q_SECRETS=1` | Skip q-secrets rebuild during `update` |
| `SKIP_TELEMETRY=1` | Skip telemetry rebuild during `update` |

### Via Makefile

```bash
make install-telemetry   # build + install to ~/tools/
make install-all          # skillvault + q-secrets + telemetry
```

## Quick Start

```bash
# Start the daemon
telemetryd &

# Check daemon status
telemetryctl status

# List recent runs
telemetryctl run recent

# List all runs with filters
telemetryctl run list --limit 20 --agent opencode

# Show run details
telemetryctl run show <run-id>
```

## Daemon (telemetryd)

The daemon listens on a Unix socket for line-delimited JSON events. Each event is
validated, run through the security pipeline (hash → redact → entropy scan), and
persisted to SQLite. Quality signals (loop, stall, streak, token efficiency) are
computed synchronously on each event write.

### Starting the daemon

```bash
telemetryd &
```

The daemon retries socket bind up to 3 times, handles SIGTERM/SIGINT with a 5s
drain period, and exits code 3 if the salt file has wrong permissions.

### Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `TELEMETRY_SOCKET` | `$XDG_RUNTIME_DIR/telemetryd.sock` or `/tmp/telemetryd.sock` | Unix socket path |
| `TELEMETRY_DB_PATH` | `~/.telemetry/telemetry.db` | SQLite database path |
| `TELEMETRY_SALT_PATH` | `~/.telemetry/salt` | Salt file for arg hashing |
| `TELEMETRY_REDACTION_PATTERNS` | (built-in) | Comma-separated custom regex patterns |
| `TELEMETRY_STORE_PROMPTS` | `false` | Opt-in prompt storage |

## CLI (telemetryctl)

Query runs, events, and daemon status from the terminal.

### Run List

List agent runs with optional filters:

```bash
# Default (last 20 runs)
telemetryctl run list

# Custom limit and agent filter
telemetryctl run list --limit 10 --agent opencode

# Filter by date
telemetryctl run list --since 2026-07-01T00:00:00Z
```

Output: tabular view with run ID, agent, status, tokens, cost, and duration.

### Run Show

Detailed view of a single run:

```bash
telemetryctl run show <run-id>
```

Shows:
- Agent info, version, workspace, repo
- Step tree with timings
- Token and cost breakdown by model
- Quality signal summary (loops, stalls, policy violations)

### Run Recent

Last 5 runs summary:

```bash
telemetryctl run recent
```

Same table format as `run list`, ordered by most recent first.

### Status

Daemon operational status:

```bash
telemetryctl status
```

Shows: uptime, events ingested, DB size, salt fingerprint, redaction patterns,
and prompt storage setting.

## CLI Wrapper (telemetrywrap)

Wraps any command and infers telemetry events:

```bash
telemetrywrap --agent my-agent -- echo hello
```

- Captures stdout/stderr and parses for model/token usage patterns
- Polls `git diff --stat` every 2 seconds for file changes
- All events flagged `confidence_level: heuristic`
- 30-minute timeout → SIGTERM → 30s grace → SIGKILL
- Emits: `run.started`, `run.completed`, `run.failed`, `command.started`,
  `command.completed`, `file.created`, `file.modified`, `file.deleted`

## OpenCode Plugin

The `OpenCodeEmitter` implements the `EventEmitter` interface and connects to the
daemon via Unix socket. It emits 9 canonical event types from hook callbacks with
a `correlation_id` chain.

On daemon unreachable, it buffers up to 1000 events and attempts reconnection on
the next emit (no data loss for transient daemon restarts).

## Security Pipeline

### Arg Hashing

Tool and command arguments are hashed with SHA-256(salt + NUL-joined args). The salt
is 32 random bytes stored at `~/.telemetry/salt` with `0600` permissions. Plaintext
args are never stored — only the hash is persisted.

### Regex Redaction

Four built-in patterns are applied before storage:

| Pattern | Example |
|---------|---------|
| OpenAI keys | `sk-abc...` (32+ chars) |
| Bearer tokens | `Bearer eyJ...` |
| Auth headers | `Authorization: Basic ...` |
| API key flags | `--api-key secret` |

Custom patterns can be added via `TELEMETRY_REDACTION_PATTERNS` (comma-separated).

### Entropy Scanning

Best-effort detection of base64-encoded secrets: tokens with an alphanumeric ratio
> 0.75 and length > 20 are flagged with `redaction_policy: scanned-warning`.

## Quality Signals

Four detectors run synchronously on each event write:

| Detector | Trigger | Emitted Event |
|----------|---------|---------------|
| LoopDetector | 3+ identical `args_hash` within 60s | `loop.detected` |
| StallDetector | No events for 60s on active run | `policy.violation` |
| StreakDetector | 5+ consecutive failures or 3+ no-change | `policy.violation` |
| TokenCounter | Efficiency ratio < 0.05 | `policy.violation` |

## Event Types (20 canonical)

| Event Type | Description |
|------------|-------------|
| `run.started` | Agent run started |
| `run.completed` | Agent run completed successfully |
| `run.failed` | Agent run failed with error |
| `prompt.submitted` | User prompt submitted |
| `response.received` | Model response received |
| `model.usage` | Token usage and cost |
| `step.started` | Reasoning/planning step started |
| `step.completed` | Step completed |
| `tool.called` | Tool invocation |
| `tool.completed` | Tool completed |
| `command.started` | Shell command started |
| `command.completed` | Shell command completed |
| `file.created` | File created |
| `file.modified` | File modified |
| `file.deleted` | File deleted |
| `test.started` | Test run started |
| `test.completed` | Test run completed |
| `approval.recorded` | User approval/rejection |
| `loop.detected` | Loop detector signal |
| `policy.violation` | Quality policy violation |

## Architecture

```
Plugin/Wrapper ──Unix socket──▶ telemetryd ──▶ Validator ──▶ Security Pipeline
                                (line-delimited JSON)        ├── ArgHasher
                                                            ├── Redactor
                                                            └── EntropyScanner
                                  ──▶ Store ──▶ SQLite (WAL mode)
                                  ──▶ Quality Signals
                                       ├── LoopDetector
                                       ├── StallDetector
                                       ├── StreakDetector
                                       └── TokenCounter

telemetryctl ──▶ SQLite (read-only) for run queries
              ──▶ Daemon socket for status
```

### Storage

- SQLite at `~/.telemetry/telemetry.db` with WAL mode
- Tables: `agent_runs`, `agent_steps`, `tool_calls`, `token_usage`, `events`
- Zero CGo (`modernc.org/sqlite`) — portable across linux/darwin amd64+arm64

### Wire Protocol

Line-delimited JSON (`\n` framing) over Unix `SOCK_STREAM`. Events are JSON-L
envelopes with metadata fields (`event_id`, `event_type`, `timestamp`, `run_id`,
`agent_id`, `redaction_policy`, `confidence_level`) and a raw `payload`.

## Building

All components are part of the kbs monorepo:

```bash
# Via Makefile
make build-telemetry

# Or direct go build
go build -ldflags="-s -w" -o telemetryd ./cmd/telemetryd/
go build -ldflags="-s -w" -o telemetryctl ./cmd/telemetryctl/
go build -ldflags="-s -w" -o telemetrywrap ./internal/agenttelemetry/telemetrywrap/
```

## Testing

```bash
# Unit tests
go test ./internal/agenttelemetry/...

# Integration tests (requires build tag)
go test -tags integration -count=1 ./internal/agenttelemetry/...

# Via Makefile
make test-integration
```
