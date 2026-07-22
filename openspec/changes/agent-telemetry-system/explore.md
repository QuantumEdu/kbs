# Exploration: Agent Telemetry/Observability System

## 1. Current State — What Exists Today

### Agent Landscape

| Agent | Hooks/Events API | Logs | Wrapper Feasibility |
|-------|-----------------|------|---------------------|
| **OpenCode** | Plugin system (Go plugins, `~/.config/opencode/plugins/`), MCP tools, `opencode.json` events | JSONL session logs in `~/.opencode/` | Medium — plugin can emit structured events |
| **Claude Code** | Hook system (`settings.json` hooks: `PostToolUse`, `Notification`, etc.), MCP tools | JSONL session logs, `~/.claude/debug/` | High — hooks fire on tool use, message completion |
| **Codex CLI** | MCP tools, session exports | JSON session exports, `~/.codex/` | Low — no documented hook system |
| **Kiro CLI** | Unknown — minimal public documentation | Unknown | Low — immature ecosystem |
| **Gentle-AI** | OpenCode-compatible plugin system, MCP tools | JSONL session logs | Medium — inherits OpenCode patterns |
| **Aider** | `--analytics` flag (basic), `--map-tokens`, analytics log | Analytics JSONL, `.aider/.aider.log` | Low — analytics only, no hook system |

### What kbs (this project) provides today

- **Local-first SQLite storage** with FTS5 full-text search
- **MCP tool server** for agent integrations
- **Session tracking** via `session` entry type
- **No telemetry infrastructure** — sessions are manually logged, no structured metrics
- **No token counting**, cost estimation, or agent run tracking

### Existing Open-Source Tools

| Tool | What It Does | Integration Complexity |
|------|-------------|----------------------|
| **OpenTelemetry** | Standard observability framework (traces, spans, metrics, logs) | High — requires SDK instrumentation |
| **Langfuse** | LLM observability platform (traces, cost, evaluations) | Medium — REST API, OpenTelemetry support |
| **Arize Phoenix** | LLM observability + evaluation (traces, spans, embeddings) | Medium — OpenTelemetry, gRPC |
| **AgentOps** | Agent-specific monitoring (runs, tools, costs, sessions) | Low — Python SDK, decorator-based |
| **LangSmith** | LLM application tracing, evaluation, prompt hub | Medium — Python/TS SDK |
| **MLflow** | ML lifecycle tracking (experiments, runs, metrics, artifacts) | Medium — REST API |
| **Grafana Tempo** | Distributed tracing backend | Low — OpenTelemetry OTLP |
| **tiktoken** | OpenAI tokenizer (Python only) | Medium — needs Go port or sidecar |
| **tokenizers (HF)** | Multi-model tokenizer (Rust, Python bindings) | High — FFI to Go, or sidecar service |
| **tokenizer-go** | Go tokenizer libraries (limited model support) | Low — Go native, fewer models |

---

## 2. Architecture Evaluation

### Proposed Architecture (User's Starting Point)

```
Agent CLI → hook/plugin/wrapper → Go collector → local SQLite →
  OpenTelemetry export → optional backend (Langfuse, Phoenix, Tempo, MLflow, AgentOps) → dashboard
```

### Pros

- **Go collector** is fast, self-contained binary — no runtime dependency (unlike Python)
- **Local SQLite** provides zero-config storage, works offline, survives restarts
- **OpenTelemetry export** is the right abstraction — vendor-neutral, widely supported
- **Optional backends** keep MVP lean while enabling enterprise adoption later

### Cons / Risks

- **Plugin dependency**: Each agent needs a custom plugin/hook — maintenance burden scales with agent count
- **Wrapper approach** can miss internal agent state (reasoning chains, internal retries, token counts from provider API)
- **Go tokenizer ecosystem is sparse**: `tiktoken` is Python-only, HuggingFace `tokenizers` is Rust with Python bindings — Go has no mature multi-model tokenizer
- **Daemon/gateway needed** for multi-process observation (shell commands run in child processes)

### Alternative Architectures

| Approach | Pros | Cons | Verdict |
|----------|------|------|---------|
| **A. Pure CLI wrapper** (`telemetryctl run -- agent ...`) | Simplest to implement, captures stdin/stdout/stderr/exit | Blind to agent internals, can't count tokens accurately, no tool-level granularity | **Only for MVP v0** |
| **B. Agent-native plugins + collector** | Deepest observability, token-accurate, tool-level events | N plugins to maintain, adoption depends on agent ecosystem | **Target architecture** |
| **C. Sidecar proxy** (MITM between agent and LLM API) | Token accuracy from API responses, no agent modification | Requires TLS interception or API key proxying, fragile to API changes, breaks streaming | **Anti-pattern — avoid** |
| **D. File-system watcher + log parser** | Zero agent modification, works with any agent | Fragile to log format changes, delayed observation, can't capture in-flight state | **Fallback only** |
| **E. Shell wrapper + ptrace** (strace-style syscall tracing) | Complete process tree visibility, all file I/O, all subprocesses | Extreme complexity, performance overhead, platform-specific (Linux ptrace vs macOS DTrace) | **Research only** |

### Recommended Architecture (Hybrid)

```
Layer 1: Agent-specific plugins (thin, agent-adapted)
  ↓ emit structured JSON events to Unix socket or stdout
Layer 2: Go collector daemon (collector)
  ↓ receives events, enriches with OS/process metadata, stores in SQLite
Layer 3: Local SQLite (embedded via modernc.org/sqlite)
  ↓ queryable via CLI, MCP tools, or SQL
Layer 4: OpenTelemetry SDK (optional, off by default)
  ↓ exports to OTLP-compatible backends (Langfuse, Tempo, Phoenix)
Layer 5: Dashboard (local TUI or web; Phase 2+)
```

**Key design decision**: The collector is a **daemon** (or long-running subprocess), not a one-shot CLI. Why: agents spawn child processes (shell commands, test runners, linters). A one-shot wrapper loses visibility into descendants. A daemon watches the process tree.

---

## 3. Design Questions — Analysis

### Q1: Product form — CLI, daemon, SDK, plugin, proxy, or combination?

**Recommendation**: Combination — **thin CLI + background collector daemon + agent-specific plugins**.

- **CLI**: `telemetryctl start`, `telemetryctl stop`, `telemetryctl query`, `telemetryctl dashboard`
- **Collector daemon**: Receives structured events via Unix domain socket (or named pipe on Windows), enriches, stores, optionally exports
- **Agent plugins**: Emit structured JSON-L events to the collector socket. One plugin per agent, minimal logic
- **No proxy**: Too invasive, breaks streaming, fragile to API changes
- **No pure SDK**: Agents are polyglot (TypeScript, Python, Go) — a library-per-language approach fragments the ecosystem

### Q2: How to support multiple agents without coupling to one?

**Strategy**: Define a **canonical event schema** (vendor-neutral) that all agent plugins translate to.

```json
// Canonical run event
{"type":"run_start","ts":"2026-07-22T10:00:00Z","run_id":"abc123","agent":"opencode","agent_version":"0.45.0","provider":"anthropic","model":"claude-sonnet-4-5","project":"kbs","repo":"github.com/quantum-6/kbs","branch":"feat/telemetry","commit":"a1b2c3d"}
{"type":"tool_call","ts":"...","run_id":"abc123","tool":"bash","args_hash":"sha256:...","duration_ms":1234}
{"type":"tool_result","ts":"...","run_id":"abc123","tool":"bash","success":true,"exit_code":0}
{"type":"file_change","ts":"...","run_id":"abc123","path":"src/main.go","op":"modified","lines_added":12,"lines_deleted":3}
{"type":"run_end","ts":"...","run_id":"abc123","status":"approved","tokens_in":4521,"tokens_out":12340,"tokens_cache":0,"tokens_reasoning":0,"cost_estimated":0.32}
```

Each agent plugin is responsible for mapping its native event model to this canonical schema. The collector is agent-agnostic.

### Q3: Transparent CLI wrapper design

For agents that lack plugins/hooks, a wrapper is the fallback:

```bash
telemetryctl run --agent opencode -- opencode "fix the auth bug"
```

The wrapper:
1. Records `run_start` with git context
2. Spawns the agent process with `os/exec`, capturing stdout, stderr
3. Uses `inotify`/`FSEvents` to watch the workspace for file changes (with debouncing)
4. Uses process tree tracking to watch child processes (shell commands)
5. Records `run_end` on process exit with exit code, duration
6. Estimates tokens from input prompt size (character count ÷ 4 for English text) — **low accuracy**, may be 2-3x off

**Limitations of wrapper-only**: No tool-level events, no reasoning chain visibility, token estimates are rough, can't distinguish provider-reported vs estimated. **This is a fallback tier, not the target.**

### Q4: What can be captured from stdin/stdout/stderr?

| Signal | What Can Be Captured | What Cannot |
|--------|---------------------|-------------|
| **stdin** | Input prompt text (if piped), but not the full context window (system prompt, tools, conversation history) | Full context that the agent assembled internally |
| **stdout** | Agent responses, thought output (if agent prints `Thinking:`), tool call output | Structured distinction between thought, tool call, and final response |
| **stderr** | Errors, warnings, debug logs | Agent-internal state like retry decisions, loop detection |
| **exit code** | 0 (success), non-zero (failure/crash) | Whether the task completed successfully vs was abandoned |
| **duration** | wall-clock time from spawn to exit | Time spent in each phase (thinking, tool calling, editing) |

**Conclusion**: stdin/stdout/stderr capture alone gives ~30% observability. True observability requires agent hooks.

### Q5: How to observe child processes, commands, duration, exit codes?

**Approach**: Process tree tracking via OS-specific mechanisms.

- **Linux**: `/proc/{pid}/task/` and `/proc/{pid}/children` for process tree discovery; `inotify` for file changes
- **macOS**: `libproc` via CGo, `FSEvents` for file changes
- **Windows**: Windows Job Objects or `NtQuerySystemInformation`

Alternatively: instruct the agent plugin to wrap `bash`/`shell` tool calls and measure them internally — far simpler and cross-platform.

**Recommendation for MVP**: The agent plugin reports command-level events (duration, exit code, stdout/stderr size). Do NOT attempt OS-level process tree tracking in MVP — it's a research spike.

### Q6: How to detect file changes and compute added/deleted lines?

**Approaches**:

1. **Pre/post git diff**: `git diff --stat` before and after the run. Simple, accurate. Requires the workspace to be a git repo.
2. **File watcher + LCS diff**: Watch workspace with `inotify`/`FSEvents`, snapshot file contents before/after, compute diff with LCS algorithm. Works without git.
3. **Agent plugin reports edits**: The agent plugin reports each `edit_file`/`write_file` call with path, operation, and lines changed. Most accurate, least overhead.

**Recommendation**: **Hybrid**. Use `git diff --stat` as primary (most agents work in git repos). Fall back to file watcher + LCS diff for non-git workspaces. Agent plugin reports edits as ground truth when available.

kbs already has an LCS diff implementation at `internal/diff/diff.go` — this can be reused or extracted as a library.

### Q7: How to associate a run with repo/branch/commit?

```
git rev-parse --show-toplevel       → repo root
git remote get-url origin            → repo URL
git rev-parse --abbrev-ref HEAD      → branch
git rev-parse HEAD                   → commit SHA
git log -1 --format=%s               → commit message
```

Run at `run_start` time. Cached for the session. Non-git workspaces get `"unknown"`.

### Q8: How to avoid storing secrets, credentials, PII, sensitive code?

**Multi-layer strategy**:

1. **Never store raw stdin/stdout** by default. Store hashes and metrics only.
2. **Redact before storage**: Apply regex patterns for API keys (`sk-...`, `ghp_...`, `Bearer ...`), environment variable values (`=\s*["']?[A-Za-z0-9+/]{20,}`), and file paths containing `.env`, `secrets`, `credentials`.
3. **Hash tool arguments**: Store `sha256(input + salt)` instead of raw args. Allows deduplication without storing secrets.
4. **Configurable PII scanner**: Opt-in Presidio/Spacy integration (Python sidecar) for advanced PII detection (emails, phone numbers, addresses). For MVP, regex-based redaction is sufficient.
5. **Opt-in prompt storage**: Full prompt storage is gated behind explicit user consent with a clear warning.
6. **Local-first by default**: Export to backend is opt-in. Data never leaves the machine unless the user enables OTLP export.

### Q9: Tokenization — exact, approximate, estimated; per-model tokenizers

| Method | Accuracy | Model Coverage | Complexity |
|--------|----------|---------------|------------|
| **Exact (API response)** | 100% | Any provider that reports usage | Low — just parse the API response |
| **tiktoken (OpenAI models)** | ~99.5% | GPT-3.5, GPT-4, GPT-4o, o1, o3, o4-mini | Medium — Python sidecar or Go port of tiktoken-rs |
| **HuggingFace tokenizers** | ~99% | 150+ models (Llama, Mistral, Gemma, Qwen, DeepSeek) | High — FFI to Rust, or gRPC sidecar |
| **Character heuristic** (÷4 for English) | ±50% | All | Trivial — 1 line of code |
| **Word heuristic** (×1.3 for English) | ±30% | All | Trivial |

**Recommendation for MVP**:
1. **Primary source**: Parse `usage` from API response (when agent plugin provides it). This is the only truly accurate source.
2. **Fallback**: Character ÷ 4 heuristic with a clear `estimation_method: "char_div_4"` marker. Never present estimated tokens as exact.
3. **Phase 2**: Integrate `tiktoken-go` (Go port exists: `github.com/pkoukk/tiktoken-go`) for OpenAI models and `github.com/bb7133/tiktoken-go` for Claude models.

### Q10: Should tokenization be a Go microservice or library?

**Library, not microservice**. Reasoning:
- Tokenization is a pure compute function — no state, no I/O, no concurrency issues
- Adding a network hop adds latency (1-5ms) to every token count operation
- Go has viable libraries for OpenAI tokenizers (`tiktoken-go`)
- For models without Go tokenizers (Llama, Mistral), accept that exact tokenization is unavailable in Phase 1 and only use the heuristic
- If exact tokenization for arbitrary models becomes critical, add an **optional gRPC sidecar** (Python + HF tokenizers) in Phase 3 — but don't block MVP on this

### Q11: How to normalize metrics across providers?

Providers report usage differently:

| Provider | `usage` fields | Notes |
|----------|---------------|-------|
| **OpenAI** | `prompt_tokens`, `completion_tokens`, `total_tokens` | Standard. `cached_tokens` on newer models. `reasoning_tokens` for o1/o3. |
| **Anthropic** | `input_tokens`, `output_tokens` | `cache_creation_input_tokens`, `cache_read_input_tokens`. No `reasoning_tokens` — thinking tokens included in output. |
| **Google (Gemini)** | `promptTokenCount`, `candidatesTokenCount`, `totalTokenCount` | `thoughtsTokenCount` for thinking models. |
| **Groq** | OpenAI-compatible format | Standard. |
| **DeepSeek** | OpenAI-compatible | `reasoning_tokens` for R1. |
| **OpenRouter** | OpenAI-compatible + `cache_discount` | Wrapper around all providers — normalization layer already. |

**Canonical normalization**:

```go
type Usage struct {
    InputTokens      int `json:"input_tokens"`       // prompt / input
    OutputTokens     int `json:"output_tokens"`       // completion / output
    CacheReadTokens  int `json:"cache_read_tokens"`   // cache hits (Anthropic), cached_tokens (OpenAI)
    CacheWriteTokens int `json:"cache_write_tokens"`  // cache creation (Anthropic only)
    ReasoningTokens  int `json:"reasoning_tokens"`    // thinking tokens (o1, o3, R1, Gemini thinking)
    TotalTokens      int `json:"total_tokens"`        // sum of all (may differ from provider's total)
}
```

Cost normalization also needs per-model price tables. OpenRouter's API provides current pricing; maintain a local fallback table.

### Q12: Signals for detecting loops, excessive reasoning, unproductive retries

| Signal | Threshold | Action |
|--------|-----------|--------|
| **Identical tool calls repeated** | 3+ consecutive calls with same tool + args_hash | Flag as `suspected_loop` |
| **No file changes after N tool calls** | 10+ tool calls, 0 file changes | Flag as `unproductive_streak` |
| **Tool call → error → retry cycle** | Same tool fails 3+ times with same error pattern | Flag as `error_loop` |
| **Reasoning token explosion** | Reasoning tokens > 10× input tokens | Flag as `reasoning_overrun` |
| **Wall-clock time without tool calls** | 60s+ with no tool call event | Flag as `stalled_thinking` |
| **Total tool calls exceeding threshold** | 50+ tool calls per run | Flag as `runaway_agent` |
| **File churn without progress** | 200+ lines changed, 0 tests passing | Flag as `unproductive_refactor` |

All thresholds are configurable. Detection runs in the collector daemon — no agent modification needed.

### Q13: Quality metrics beyond tokens/latency

| Metric | How to Measure | Requires |
|--------|---------------|----------|
| **Task success rate** | `exit_code == 0` AND `status == approved` | Agent plugin or wrapper |
| **Tool success rate** | `tool_call` with `success: true` / total tool calls | Agent plugin |
| **First-tool latency** | Time from `run_start` to first `tool_call` | Agent plugin |
| **Edit precision** | `lines_accepted` / `total_lines_proposed` | Agent plugin + git diff |
| **Rework ratio** | Lines deleted from agent edits / total lines added by agent | git diff analysis |
| **Test pass rate delta** | `(tests_passing_after - tests_passing_before) / total_tests` | Test runner integration |
| **Prompt iteration count** | Number of user messages in the session (agent asks for clarification) | Agent plugin |
| **Context efficiency** | `input_tokens / output_tokens` (lower = more productive per context) | API response |
| **Session duration efficiency** | `lines_changed / session_duration_minutes` | Git diff + timing |

### Q14: How to compare two agents on the same task?

**Comparison mode**:

```bash
telemetryctl compare --task "refactor auth middleware" \
  --agent opencode --agent claude-code \
  --repo github.com/quantum-6/kbs --branch compare-baseline
```

Process:
1. Create two identical git branches from the same baseline commit
2. Run `telemetryctl run --agent opencode -- "refactor auth middleware"` on branch A
3. Run `telemetryctl run --agent claude-code -- "refactor auth middleware"` on branch B
4. Agent runs on isolated branches — no interference
5. After both complete, compute the following diff:

| Dimension | agent A | agent B | Delta |
|-----------|---------|---------|-------|
| Duration (wall-clock) | 4m32s | 6m11s | -26% |
| Tool calls | 18 | 27 | -33% |
| Tokens in | 4.2k | 6.8k | -38% |
| Tokens out | 11.4k | 18.9k | -40% |
| Cost | $0.28 | $0.51 | -45% |
| Lines added | 142 | 138 | +3% |
| Lines deleted | 23 | 31 | -26% |
| Tests passing (after) | 47/47 | 44/47 | +6% |
| Build success | yes | yes | — |
| Quality (manual review) | TBD | TBD | — |

**Limitation**: Agent comparison is inherently noisy. The same agent on the same task with the same prompt can produce different results (non-deterministic LLM sampling). Comparison is directional, not definitive — it reveals **patterns**, not rankings.

### Q15: What to instrument with OpenTelemetry?

**Trace model**:

```
Trace: agent-run-{run_id}
├── Span: run (root) — attributes: agent, provider, model, project, repo, branch, commit
│   ├── Span: prompt_assembly — attributes: system_prompt_hash, tools_count
│   ├── Span: llm_call (repeat per turn)
│   │   ├── Event: token_usage — attributes: input, output, cache, reasoning
│   │   ├── Event: thinking_block — attributes: length_tokens, duration_ms
│   │   └── Span: tool_execution (repeat per tool call in this turn)
│   │       ├── Event: tool_start — attributes: tool_name, args_hash
│   │       ├── Span: command_execution (for bash/shell tools)
│   │       │   ├── Event: command_start — attributes: command_hash
│   │       │   ├── Event: file_change — attributes: path, op, lines_added, lines_deleted
│   │       │   └── Event: command_end — attributes: exit_code, duration_ms, stdout_len, stderr_len
│   │       └── Event: tool_end — attributes: success, error_type
│   └── Span: post_run_analysis (offline, after run ends)
│       ├── Event: git_diff — attributes: files_changed, insertions, deletions
│       ├── Event: test_results — attributes: passed, failed, skipped
│       └── Event: quality_signals — attributes: loop_detected, unproductive_streak, ...
```

**Metrics** (aggregated, not per-run):
- `agent.runs.total` (counter, labels: agent, provider, model, status)
- `agent.tokens.total` (counter, labels: agent, provider, model, token_type)
- `agent.cost.total` (counter, labels: agent, provider, model)
- `agent.duration.seconds` (histogram, labels: agent, status)
- `agent.tool_calls.total` (counter, labels: agent, tool_name, success)
- `agent.loops.detected` (counter, labels: agent, signal_type)

**Logs**: Raw structured events (canonical JSON schema), stored in SQLite, optionally exported as OTLP logs.

### Q16: Existing tools to integrate vs avoid in MVP

**Integrate in MVP**:
- **Git**: For diff stats, repo context (already available)
- **SQLite** (via `modernc.org/sqlite`): Zero-dependency local storage (kbs already uses this)
- **OpenTelemetry Go SDK** (`go.opentelemetry.io/otel`): Clean, standard, optional export — 1 import, not a heavy dependency

**Avoid in MVP** (defer to Phase 2+):
- **Langfuse/Phoenix/Tempo backends**: OTLP export handles these generically — no need for direct integrations
- **HuggingFace tokenizers**: Too heavy, FFI complexity, not worth it for MVP
- **Process tree tracking (ptrace/strace)**: Research spike only
- **PII detection (Presidio)**: Regex redaction is sufficient for MVP
- **Web dashboard**: Local TUI (`bubbletea`) is faster and more portable

**Keep as optional plugins (not in core)**:
- **AgentOps SDK integration**: Wrapper, not dependency
- **MLflow export**: OTLP-compatible, no direct integration needed
- **Grafana Tempo**: Just an OTLP endpoint — zero integration code

### Q17: MVP scope vs postponed features

**MVP (Phase 1, 2 weeks)**:

| # | Capability | Rationale |
|---|-----------|-----------|
| 1 | **Canonical event schema** (JSON schema + Go types) | Foundation — everything depends on this |
| 2 | **Go collector daemon** — Unix socket listener, SQLite writer, event enrichment | Core runtime |
| 3 | **CLI: `telemetryctl start|stop|query`** | User interface |
| 4 | **OpenCode plugin** — emits canonical events via Unix socket | First agent integration (this project's primary agent) |
| 5 | **CLI wrapper fallback** — for agents without plugins | Covers the rest |
| 6 | **Git context capture** — repo, branch, commit at run_start | Essential metadata |
| 7 | **Git diff analysis** — pre/post run diff stats | Quality metrics foundation |
| 8 | **Token/cost from API response** (when plugin provides it) | Accurate where possible |
| 9 | **Character heuristic fallback** for token estimation | Graceful degradation |
| 10 | **Basic signal detection**: identical tool repeat, N-tool-no-change | First quality signals |

**Postponed (Phase 2, +2 weeks)**:
- `tiktoken-go` integration for exact OpenAI/Claude tokenization
- Tokenizer sidecar (Python + HF tokenizers) for Llama/Mistral/Gemma
- `telemetryctl compare` for agent A/B comparison
- Test result integration (parse test runner output)
- OpenTelemetry OTLP export (traces + metrics)
- Claude Code plugin
- Regex-based PII redaction
- File watcher + LCS diff as git-diff fallback

**Postponed (Phase 3+)**:
- Local TUI dashboard (`bubbletea`)
- Web dashboard
- Process tree tracking
- Presidio PII detection
- Grafana/Phoenix/Langfuse dashboards (via OTLP, zero integration code)
- Codex, Kiro, Aider plugins
- Agent comparison statistical framework (multiple runs, confidence intervals)
- Cost optimization recommendations

### Q18: Technical, legal, privacy, performance risks

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| **Go tokenizer ecosystem is sparse** | Medium | Certain | Accept heuristic fallback in MVP. tiktoken-go covers 60% of use cases. Sidecar for Phase 2 if demanded. |
| **Agent plugin ecosystem is fragmented** | High | High | Design canonical schema first. Each plugin is ~200 lines of adapter code. Start with the agent we control (OpenCode). |
| **Secrets/PII in stored events** | High | Medium | Hash by default, regex redaction, opt-in prompt storage, local-first architecture. Never export raw prompts/tool args without explicit opt-in. |
| **Collector daemon performance overhead** | Medium | Low | Events are small JSON blobs (~200 bytes each). A 100-tool-call run produces ~20KB of event data. Daemon is I/O-bound, not CPU-bound. |
| **SQLite scalability** | Low | Low | A single run produces ~20-100 rows. 1000 runs = ~100K rows. SQLite handles millions of rows comfortably. Vacuum periodically. |
| **Agent-specific API changes break plugins** | Medium | Medium | Plugins are thin adapters. When an agent's hook API changes, only the adapter changes — canonical schema and collector are stable. |
| **Privacy regulation (GDPR)** | Medium | Low (MVP) | Local-first — no data export by default. User controls export. DPO review needed before enterprise deployment. |
| **Agent comparison is not reproducible** | Low | Certain | Document limitation. Comparison is directional (pattern detection), not definitive ranking. Statistical approach (N runs) for Phase 3. |
| **Token costs uncertain without exact tokenization** | Medium | Certain | Flag estimated tokens clearly. Cost estimates will be ±30% with heuristic, ±5% with tiktoken, ±1% with API-reported usage. |

---

## 4. Component Decomposition (Target Architecture)

```
┌─────────────────────────────────────────────────────────────────┐
│  agent-telemetry                                                 │
├─────────────────────────────────────────────────────────────────┤
│  cmd/                                                            │
│  ├── telemetryctl/       CLI entry point (cobra)                 │
│  └── telemetryd/         Collector daemon entry point            │
│                                                                  │
│  internal/                                                       │
│  ├── schema/             Canonical event types (domain)          │
│  ├── collector/          Event ingestion, enrichment, routing    │
│  ├── store/              SQLite repository (FTS5)                │
│  ├── diff/               Git diff + LCS diff (from kbs)         │
│  ├── signals/            Quality signal detection engine        │
│  ├── otel/               OpenTelemetry bridge (optional)        │
│  ├── tokenize/           Token estimation (heuristic + tiktoken)│
│  ├── redact/             PII/secret redaction patterns          │
│  ├── gitutil/            Git context extraction                 │
│  └── proc/               Process tree tracking (Phase 3)        │
│                                                                  │
│  plugins/                                                       │
│  ├── opencode/           OpenCode plugin (Go, Unix socket)      │
│  ├── claude-code/        Claude Code hook (TypeScript)          │
│  └── wrapper/            CLI wrapper fallback                   │
│                                                                  │
│  config/                                                         │
│  └── schema.yaml         Event schema (single source of truth)  │
│                                                                  │
│  docs/                                                           │
│  ├── EVENT_SCHEMA.md     Canonical event specification          │
│  └── PLUGIN_GUIDE.md     How to write an agent plugin           │
└─────────────────────────────────────────────────────────────────┘
```

**Key architectural constraints**:
- Go 1.25+ (matches kbs toolchain)
- SQLite via `modernc.org/sqlite` (same as kbs, zero CGo)
- CLI via `cobra` (or `charmbracelet` if TUI is desired)
- Event schema defined in Go structs with JSON tags — single source of truth
- Plugins communicate via Unix domain socket (`/tmp/telemetryd.sock`) with newline-delimited JSON

---

## 5. Go Package Ecosystem Fit

### Already in kbs (can reuse/extract)
- `modernc.org/sqlite` — embedded SQLite
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/charmbracelet/bubbletea` — TUI framework
- `internal/diff/diff.go` — LCS diff algorithm

### New dependencies (MVP)
- `github.com/spf13/cobra` — CLI framework
- `github.com/pkoukk/tiktoken-go` — OpenAI tokenizer (Phase 2)
- `go.opentelemetry.io/otel` — OpenTelemetry SDK (Phase 2, optional)

### No heavy dependencies
- No gRPC (Unix sockets for local IPC)
- No FFI/CGo (pure Go)
- No Python runtime (sidecar is Phase 3 if needed)

---

## 6. Ready for Proposal

**Yes**. The exploration has:

- Evaluated 5 architecture alternatives with tradeoffs
- Recommended a concrete hybrid architecture (plugins + daemon + CLI)
- Defined a canonical event schema approach
- Scoped MVP to 10 capabilities achievable in 2 weeks
- Identified the biggest technical risks (tokenizer ecosystem, plugin fragmentation)
- Deferred high-complexity features (process tracking, web dashboard, statistical comparison) to later phases
- Established that Go + SQLite is the right stack, reusing kbs patterns and the existing `internal/diff` module

**Next phase**: `sdd-propose` — formalize the proposal with scope, approach, rollback plan, and explicit acceptance criteria.

**Open question for proposal phase**: Should this be a separate Go module (`agent-telemetry`) in a monorepo, or a standalone repository? The proposed architecture is self-contained (no dependency on kbs other than borrowing `diff.go`), which favors a standalone repo. But sharing the kbs SQLite patterns and toolchain suggests a monorepo. **Recommendation**: Monorepo as `cmd/telemetryctl/`, `cmd/telemetryd/`, `internal/agent-telemetry/` within kbs — simpler CI, shared toolchain, shared diffusion patterns.
