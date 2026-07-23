# Proposal: Agent Telemetry System

## Intent

CLI LLM coding agents (OpenCode, Claude Code, Codex, Gentle-AI, Kiro) produce no structured observability data — no token counts, cost tracking, tool success rates, or loop detection. Users operate blind. We need a local-first, multi-agent telemetry system that plugs into existing agent hook/plugin surfaces without modifying agent internals.

## Scope

### In Scope
- Canonical JSON-L event schema for agent runs
- Go daemon (`telemetryd`) ingesting events via Unix socket → SQLite
- CLI (`telemetryctl`) for query, status, and comparison
- Agent plugins: OpenCode (v0.1), CLI wrapper fallback (stdin/stdout/git-diff)
- Git context capture (repo, branch, commit at run start)
- File change detection via `git diff --stat` + LCS fallback
- Token/cost from API response (`usage` fields), heuristic fallback (char÷4)
- Quality signal detection: loops, stalls, unproductive streaks
- Secret protection: hash args by default, regex redaction, opt-in prompt storage

### Out of Scope
- OpenTelemetry export (Phase 2)
- tiktoken-go / sidecar tokenizer (Phase 2–3)
- TUI dashboard, web dashboard (Phase 3)
- Claude Code, Codex, Kiro plugins (Phase 2)
- Agent comparison commands (Phase 2)
- OS process tree tracking (Phase 3)
- Statistical comparison (Phase 3)

## Capabilities

### New Capabilities
- `agent-telemetry-core`: Canonical event schema, collector daemon, SQLite storage, CLI
- `agent-telemetry-plugins`: Agent-specific event emitters (OpenCode plugin, CLI wrapper)
- `agent-telemetry-security`: Hash-by-default, regex redaction, opt-in prompt storage
- `agent-telemetry-quality`: Loop/inefficiency detection signals

### Modified Capabilities
None — all capabilities are new.

## Approach

Hybrid architecture (from exploration, option 5): agent-specific plugins (~200-line adapters) emit canonical events to a local Go daemon over Unix socket. Daemon stores in SQLite (modernc.org, zero CGo). CLI wrapper fallback for agents without hooks captures stdin/stdout and infers events via git diff. Tokenization: primary = parse API response `usage`; fallback = char÷4 heuristic (marked `estimated`). Quality signals computed on write, not in separate process.

Monorepo inside kbs: `cmd/telemetryd/`, `cmd/telemetryctl/`, `internal/agent-telemetry/`. Reuses kbs `internal/diff/`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/telemetryd/` | New | Collector daemon entry point |
| `cmd/telemetryctl/` | New | CLI entry point |
| `internal/agent-telemetry/` | New | Core: schema, collector, storage, signals, security |
| `internal/diff/` | Read | Reused for file change detection |
| `go.mod` | Modified | New deps: modernc.org/sqlite, OTel SDK |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Go tokenizer ecosystem sparse | Medium | Heuristic fallback (±30%), tiktoken-go in Phase 2 |
| Agent plugin ecosystem fragmented | High | Canonical schema + thin adapters (~200 lines each) |
| Secrets/PII in stored events | High | Hash-by-default, regex redaction, opt-in prompt storage |
| Agent comparison non-reproducible | Low | Documented limitation, statistical approach in Phase 3 |

## Rollback Plan

`git revert` the merge commit. No DB migrations in MVP — delete `~/.telemetry/` to clean state. No external services to tear down.

## Dependencies

- Go ≥1.22 (toolchain already in kbs)
- modernc.org/sqlite (pure Go SQLite)
- OpenTelemetry Go SDK (opt-in export plumbing, not active in MVP)

## Success Criteria

- [ ] `telemetryd` starts, accepts events over Unix socket, stores in SQLite
- [ ] OpenCode plugin emits run lifecycle, prompt/response, tool calls, tokens
- [ ] CLI wrapper captures commands and file changes for unhooked agents
- [ ] `telemetryctl run list` shows completed runs with token counts and cost estimates
- [ ] Quality signals fire: loop detected after 3+ identical tool calls, stall after 60s inactivity
- [ ] Prompt bodies NOT stored by default; hashed args verified
