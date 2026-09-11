# agent-telemetry-core Specification

## Purpose

Canonical event schema, collector daemon, SQLite storage, and CLI for local-first agent telemetry.

## Canonical Event Schema

All events embed a `headers` envelope:

```json
{
  "event_id": "evt-<uuidv7>",
  "event_type": "run.started",
  "timestamp": "2026-07-22T10:15:00Z",
  "run_id": "run-<uuidv7>",
  "agent_id": "opencode",
  "agent_version": "0.1.0",
  "source": "plugin|wrapper|daemon",
  "correlation_id": "<optional-parent-uuid>",
  "step_id": "<optional, set for step/tool/command events>",
  "redaction_policy": "hash-args|none",
  "confidence_level": "measured|estimated|heuristic",
  "payload": { }
}
```

### Required Events

| Event Type | Required Payload Fields | Emitted By |
|---|---|---|
| `run.started` | `repo_url`, `branch`, `commit_sha`, `workspace`, `agent_config` | Plugin/Wrapper |
| `run.completed` | `duration_ms`, `total_tokens`, `total_cost_usd` | Plugin/Wrapper |
| `run.failed` | `duration_ms`, `error_type`, `error_message` | Plugin/Wrapper |
| `prompt.submitted` | `prompt_hash`, `prompt_char_count`, `model` | Plugin/Wrapper |
| `response.received` | `prompt_hash`, `response_hash`, `response_char_count`, `latency_ms` | Plugin/Wrapper |
| `model.usage` | `model`, `input_tokens`, `output_tokens`, `total_tokens`, `cost_usd`, `estimation_method` | Plugin/Wrapper |
| `step.started` | `step_name`, `step_index` | Plugin/Wrapper |
| `step.completed` | `step_name`, `duration_ms`, `step_index` | Plugin/Wrapper |
| `tool.called` | `tool_name`, `args_hash`, `call_index` | Plugin/Wrapper |
| `tool.completed` | `tool_name`, `call_index`, `duration_ms`, `success`, `error_type` | Plugin/Wrapper |
| `command.started` | `command_hash`, `command_char_count`, `running_from` | Daemon (process hook) |
| `command.completed` | `command_hash`, `duration_ms`, `exit_code`, `stdout_char_count`, `stderr_char_count` | Daemon (process hook) |
| `file.created` | `file_path`, `file_size` | Daemon (git-diff / fs watch) |
| `file.modified` | `file_path`, `file_size`, `additions`, `deletions` | Daemon (git-diff / fs watch) |
| `file.deleted` | `file_path` | Daemon (git-diff / fs watch) |
| `test.started` | `test_command`, `test_framework` | Plugin/Wrapper |
| `test.completed` | `test_command`, `total`, `passed`, `failed`, `skipped`, `exit_code` | Plugin/Wrapper |
| `approval.recorded` | `decision_id`, `approved`, `reason`, `tool_name` | Plugin/Wrapper |
| `loop.detected` | `pattern`, `tool_name`, `repeat_count`, `window_ms` | Daemon (quality signal) |
| `policy.violation` | `policy_name`, `severity`, `detail` | Daemon (quality signal) |

### Event JSON Examples

```json
{"event_id":"evt-01","event_type":"prompt.submitted","timestamp":"2026-07-22T10:15:00Z","run_id":"run-01","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","correlation_id":null,"step_id":"step-01","redaction_policy":"hash-args","confidence_level":"measured","payload":{"prompt_hash":"sha256:abc123","prompt_char_count":2450,"model":"claude-sonnet-4-5"}}
```
```json
{"event_id":"evt-02","event_type":"model.usage","timestamp":"2026-07-22T10:15:30Z","run_id":"run-01","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","correlation_id":"evt-01","step_id":"step-01","redaction_policy":"hash-args","confidence_level":"measured","payload":{"model":"claude-sonnet-4-5","input_tokens":2450,"output_tokens":820,"total_tokens":3270,"cost_usd":0.049,"estimation_method":"api-response"}}
```
```json
{"event_id":"evt-03","event_type":"tool.called","timestamp":"2026-07-22T10:16:00Z","run_id":"run-01","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","correlation_id":"evt-02","step_id":"step-01","redaction_policy":"hash-args","confidence_level":"measured","payload":{"tool_name":"bash","args_hash":"sha256:def456","call_index":1}}
```

## Requirements

### Requirement: REQ-CORE-01 — Collector Daemon

The system MUST provide a daemon (`telemetryd`) that listens on a Unix socket (`$XDG_RUNTIME_DIR/telemetryd.sock` or `/tmp/telemetryd.sock`), accepts line-delimited JSON events, validates them against the canonical schema, and persists to SQLite.

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Ingest valid event** | daemon running, socket open | plugin sends valid `run.started` JSON | event stored in `events` table, 200 OK-like ack returned |
| **Reject invalid event** | daemon running | plugin sends JSON missing required field `run_id` | event rejected, error ack with field name returned, event NOT stored |
| **Startup socket unavailable** | port/address in use | daemon starts | daemon logs warning, retries 3x with 1s backoff, exits if still unavailable |
| **Graceful shutdown** | daemon running with pending writes | SIGTERM received | drains pending events (5s timeout), closes socket, exits 0 |

### Requirement: REQ-CORE-02 — SQLite Storage Schema

The system MUST persist events to a SQLite database at `~/.telemetry/telemetry.db` with immutable append-only event log.

**Core tables:**

| Table | Key Columns | Purpose |
|---|---|---|
| `agent_runs` | `id (TEXT PK)`, `agent_id`, `repo_url`, `branch`, `commit_sha`, `workspace`, `started_at`, `completed_at`, `status (running|completed|failed)`, `total_tokens`, `total_cost_usd` | Run lifecycle |
| `agent_steps` | `id (TEXT PK)`, `run_id (FK)`, `step_name`, `step_index`, `started_at`, `completed_at`, `duration_ms` | Step boundaries |
| `tool_calls` | `id (TEXT PK)`, `run_id (FK)`, `step_id (FK)`, `tool_name`, `args_hash`, `call_index`, `started_at`, `completed_at`, `duration_ms`, `success`, `error_type` | Tool invocations |
| `command_executions` | `id (TEXT PK)`, `run_id (FK)`, `step_id (FK)`, `command_hash`, `exit_code`, `started_at`, `completed_at`, `duration_ms`, `stdout_char_count`, `stderr_char_count` | Shell commands |
| `file_changes` | `id (TEXT PK)`, `run_id (FK)`, `step_id (FK)`, `file_path`, `change_type (created|modified|deleted)`, `file_size`, `additions`, `deletions`, `detected_at` | File system mutations |
| `test_runs` | `id (TEXT PK)`, `run_id (FK)`, `test_command`, `test_framework`, `total`, `passed`, `failed`, `skipped`, `exit_code`, `started_at`, `completed_at` | Test executions |
| `token_usage` | `id (TEXT PK)`, `run_id (FK)`, `step_id (FK)`, `model`, `input_tokens`, `output_tokens`, `total_tokens`, `cost_usd`, `estimation_method (api-response|char-div-4|manual)` | Token + cost tracking |
| `events` | `id (TEXT PK)`, `run_id`, `event_type`, `timestamp`, `source`, `correlation_id`, `step_id`, `payload (JSON TEXT)`, `created_at` | Immutable append-only event log |

### Requirement: REQ-CORE-03 — CLI

The system MUST provide `telemetryctl` with subcommands:

| Command | Args | Output |
|---|---|---|
| `run list` | `--limit N`, `--agent X`, `--since Y` | Table: run_id, agent, status, tokens, cost, duration |
| `run show ID` | run_id | Run detail + step tree + token/cost breakdown |
| `run recent` | (none) | Last 5 runs summary |
| `status` | (none) | Daemon health, uptime, events ingested, DB size |

**Scenario: List filtered runs**
- GIVEN 10 runs in DB, 3 from opencode, 2 from claude-code
- WHEN `telemetryctl run list --agent opencode --limit 2`
- THEN table shows 2 most recent opencode runs with token counts and cost

**Scenario: Show run detail**
- GIVEN run `run-abc` with 3 steps, 8 tool calls
- WHEN `telemetryctl run show run-abc`
- THEN prints header (agent, repo, branch, duration, tokens, cost) + step tree with tool call counts

### Requirement: REQ-CORE-04 — Git Context Capture

The system MUST capture `repo_url` (from `git remote`), `branch` (from `git branch --show-current`), and `commit_sha` (from `git rev-parse HEAD`) at run start. If outside a git repo, fields MUST be `null`.

**Scenario: Git repo detected**
- GIVEN working directory is inside a git repo
- WHEN run starts
- THEN `run.started` payload includes repo_url, branch, and commit_sha

### Requirement: REQ-CORE-05 — File Change Detection

The system SHOULD detect file changes via `git diff --stat` at step boundaries falling back to LCS diff when git unavailable. Changes emitted as `file.created`, `file.modified`, `file.deleted` events.

### Error Handling

| Error | Behavior |
|---|---|
| DB locked | Retry 3x with exponential backoff (10ms, 100ms, 1s); drop event on final failure |
| Malformed JSON | Log warning with raw bytes (truncated to 256 chars), return error ack |
| Socket write failure | Log error, close client connection, daemon continues accepting new clients |
| DB file permissions | Daemon exits with code 2, logs path and permission bits |

### Non-Functional Requirements

| NFR | Requirement |
|---|---|
| **Performance** | `telemetryd` MUST handle 100 events/sec sustained; event validation < 1ms |
| **Storage** | DB MUST support 100K+ events without degradation; VACUUM on daemon start |
| **Reliability** | Event loss on crash: last 1s buffer MAY be lost; daemon writes to WAL mode |
| **Simplicity** | Zero external daemon dependencies; SQLite via modernc.org (pure Go, zero CGo) |
| **Portability** | Daemon and CLI MUST compile and run on linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 |
