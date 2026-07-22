# agent-telemetry-plugins Specification

## Purpose

Agent-specific event emitters that translate agent lifecycle into canonical telemetry events. Two emission strategies: native plugin (OpenCode) and CLI wrapper fallback (stdin/stdout/git-diff inference).

## Requirements

### Requirement: REQ-PLUG-01 — Plugin Interface Contract

The system MUST define a Go interface `EventEmitter` that all plugins implement:

```go
type EventEmitter interface {
    StartRun(ctx context.Context, opts RunOpts) (string, error)           // → run.started
    CompleteRun(ctx context.Context, runID string) error                  // → run.completed
    FailRun(ctx context.Context, runID string, err error) error           // → run.failed
    EmitEvent(ctx context.Context, e Event) error                        // → generic single event
    EmitEvents(ctx context.Context, events []Event) error                // → batch
    Close() error
}
```

`RunOpts` MUST carry: `AgentID`, `AgentVersion`, `Workspace` (pwd). Git context populated automatically.

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Plugin emits run lifecycle** | plugin initialized, daemon available | `StartRun` → do work → `CompleteRun` | `run.started` + `run.completed` stored in DB with matching run_id |
| **Plugin emits run failure** | plugin initialized, daemon available | `StartRun` → work fails → `FailRun` | `run.started` + `run.failed` stored, error_type and error_message captured |
| **Daemon unreachable** | plugin initialized, daemon socket not found | any emit call | returns error, plugin logs warning, caller MAY retry or discard |

### Requirement: REQ-PLUG-02 — OpenCode Plugin

The system MUST provide an OpenCode plugin (~200 lines) that connects to the daemon and emits events from OpenCode hook callbacks. Emitted events: `run.started`, `run.completed`, `run.failed`, `prompt.submitted`, `response.received`, `model.usage`, `step.started`, `step.completed`, `tool.called`, `tool.completed`.

**Assumption**: OpenCode exposes hook callbacks (pre/post-run, pre/post-step, pre/post-tool) similar to Claude Code hooks. If the exact callback surface differs, the adapter is still ~200 lines — adapt to whatever surface exists in OpenCode v0.1.

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Hook-driven lifecycle** | OpenCode agent running with plugin enabled | agent starts → submits prompt → calls tool → receives response | all 9 event types emitted with correct correlation_id chain |

### Requirement: REQ-PLUG-03 — CLI Wrapper Fallback

The system MUST provide a CLI wrapper (`telemetrywrap`) that intercepts stdin/stdout of unhooked agents and infers events:

```
telemetrywrap --agent <id> -- <agent-command>
```

Inference rules (heuristic, `confidence_level: heuristic`):

| Observation | Events Inferred |
|---|---|
| Wrapper starts | `run.started` (git context from PWD) |
| Text sent to stdin | `prompt.submitted` (hash only, `prompt_char_count`) |
| Text received from stdout | `response.received` (hash only, `response_char_count`) |
| stdout contains `usage` JSON | `model.usage` with `estimation_method: char-div-4`; if API `usage` block found, `estimation_method: api-response` |
| `git diff --stat` change detected (periodic, 2s poll) | `file.created` / `file.modified` / `file.deleted` |
| Agent process exits | `run.completed` (or `run.failed` if exit code ≠ 0) |

**Limitations**: Wrapper cannot detect step boundaries or tool calls. Events will have `confidence_level: heuristic` and step_id will be null. Documented in output.

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Wrapper captures agent run** | agent binary with no plugin support | `telemetrywrap --agent claude-code -- claude` | run lifecycle + prompt/response events emitted, git-diff detects file changes |
| **Wrapper detects token usage** | agent stdout includes `"usage":{"input_tokens":500,...}` | wrapper parses stdout line | `model.usage` emitted with `estimation_method: api-response` |
| **Wrapper timeout** | agent process hangs | 30m timeout elapsed | SIGTERM → wait 30s → SIGKILL, `run.failed` emitted with error_type `timeout` |

### Requirement: REQ-PLUG-04 — Plugin Configuration

The system MUST support plugin enablement via environment variables:

| Env Var | Default | Purpose |
|---|---|---|
| `TELEMETRY_ENABLED` | `true` | Master switch |
| `TELEMETRY_SOCKET` | `$XDG_RUNTIME_DIR/telemetryd.sock` | Daemon socket path |
| `TELEMETRY_STORE_PROMPTS` | `false` | Opt-in prompt body storage |
| `TELEMETRY_REDACTION_PATTERNS` | (built-in) | Custom regex redaction patterns |

## Error Handling

| Error | Plugin Behavior |
|---|---|
| Daemon unreachable at init | Log warning, `StartRun` returns error; caller decides to proceed without telemetry |
| Daemon disconnects mid-run | Buffer events (max 1000 in memory), reconnect on next emit; drop oldest if buffer full |
| Invalid event (schema mismatch) | Log error with event type, skip event, continue |

## Non-Functional Requirements

| NFR | Requirement |
|---|---|
| **Overhead** | Plugin MUST NOT add > 5ms latency per event emission |
| **Failure isolation** | Plugin failure MUST NOT crash or block the agent process |
| **Memory** | CLI wrapper MUST use < 50MB RSS when wrapping an agent |
