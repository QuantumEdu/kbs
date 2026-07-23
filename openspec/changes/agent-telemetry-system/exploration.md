# sdd/agent-telemetry-system/explore

## Exploration: Agent Telemetry/Observability System

### 1. Current State

Six CLI coding agents evaluated: OpenCode (plugin system, JSONL logs), Claude Code (hook system, JSONL logs), Codex CLI (MCP tools, no hooks), Kiro CLI (unknown), Gentle-AI (OpenCode-compatible), Aider (analytics only). kbs itself has no telemetry infrastructure — sessions are manually logged, no structured metrics, no token counting.

Existing open-source landscape: OpenTelemetry (standard), Langfuse/Phoenix (LLM observability platforms), AgentOps (agent-specific), LangSmith (tracing), MLflow (tracking). Key gap: tiktoken (Python-only tokenizer) has no mature Go equivalent.

### 2. Architecture Evaluation

Proposed architecture evaluated (Agent CLI → plugin/wrapper → Go collector → SQLite → OTel → backend → dashboard). Pros: fast Go binary, zero-config SQLite, vendor-neutral OTel export. Cons: plugin maintenance burden, wrapper misses internal state, sparse Go tokenizer ecosystem.

Five alternatives analyzed: Pure CLI wrapper (simplest, blind to internals), Agent-native plugins (deepest, high adoption burden), Sidecar proxy (MITM — anti-pattern, fragile), FS watcher + log parser (zero mod, fragile), Shell wrapper + ptrace (extreme complexity).

**Recommended**: Hybrid — Agent-specific plugins (thin adapters) → Go collector daemon (events via Unix socket) → Local SQLite → OpenTelemetry export (off by default) → Backend (optional).

### 3. Key Design Decisions

- Product form: CLI (`telemetryctl`) + daemon (`telemetryd`) + agent plugins. No proxy. No pure SDK.
- Multi-agent: Canonical event schema (JSON-L) that all plugins translate to.
- CLI wrapper: Fallback for agents without hooks (stdin/stdout/stderr capture, inotify file watch, git diff). ~30% observability vs 90%+ with hooks.
- Child processes: Agent plugin wraps tool calls and reports internally. OS process tree tracking deferred to Phase 3 (research spike).
- File changes: `git diff --stat` primary; fallback to FS watcher + LCS diff (kbs's `internal/diff/diff.go` reusable).
- Git context: `git rev-parse` for repo/branch/commit at run_start.
- Secret protection: Hash args by default, regex redaction, opt-in prompt storage, local-first (no export without opt-in).
- Tokenization: Primary = parse API response usage. Fallback = char÷4 heuristic (marked as estimated). Phase 2 = tiktoken-go for OpenAI/Claude. Phase 3 = gRPC sidecar for arbitrary models. Tokenization is a library, NOT a microservice.
- Normalization: Canonical `Usage` struct with input/output/cache_read/cache_write/reasoning/total fields. Per-model price table.
- Quality signals: Identical tool repeat (3+), unproductive streak (10+ calls, 0 changes), error loop (3+ same error), reasoning overrun (10× input), stalled thinking (60s no call), runaway (50+ calls), unproductive churn (200+ lines, 0 tests).
- Quality metrics: Task success rate, tool success rate, first-tool latency, edit precision, rework ratio, test pass rate delta, context efficiency, session duration efficiency.
- Agent comparison: Identical git branches, same task, isolated runs. Directional patterns only (LLMs are non-deterministic).
- OpenTelemetry: Trace = run (root) → prompt_assembly → llm_call (per turn, with token_usage + thinking_block) → tool_execution (per tool, with command_execution for bash). Post-run analysis span: git_diff + test_results + quality_signals. Metrics: counters + histograms for runs, tokens, cost, duration, tool calls, loops.
- Integrate: Git, SQLite (modernc.org), OpenTelemetry Go SDK (1 import). Avoid in MVP: Langfuse/Phoenix/Tempo (OTLP covers them), HF tokenizers (too heavy), ptrace (research), Presidio (regex suffices), web dashboard (TUI first).
- MVP (Phase 1, 2 weeks): 10 capabilities — canonical schema, collector daemon, CLI, OpenCode plugin, CLI wrapper fallback, git context, git diff analysis, API-response token/cost, heuristic fallback, basic signal detection.
- Postponed: tiktoken-go (Phase 2), sidecar tokenizer (Phase 3), compare command (Phase 2), OTLP export (Phase 2), Claude Code plugin (Phase 2), TUI dashboard (Phase 3), process tracking (Phase 3), statistical comparison (Phase 3).

### 4. Major Risks

- Go tokenizer ecosystem sparse (Medium severity) — mitigated by heuristic fallback + tiktoken-go for 60% coverage
- Agent plugin ecosystem fragmented (High severity) — mitigated by canonical schema + thin adapters (~200 lines each)
- Secrets/PII in stored events (High severity) — mitigated by hash-by-default, regex redaction, opt-in prompt storage
- Agent comparison non-reproducible (Low severity) — documented limitation, statistical approach for Phase 3
- Token costs uncertain without exact tokenization (Medium severity) — flagged as estimated, ±30% with heuristic

### 5. Ready for Proposal

Yes. Recommended monorepo placement: `cmd/telemetryctl/`, `cmd/telemetryd/`, `internal/agent-telemetry/` within kbs — shared toolchain, shared `internal/diff`, simpler CI.
