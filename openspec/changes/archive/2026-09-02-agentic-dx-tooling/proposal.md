# Proposal: Agentic DX & Tooling Improvements for SkillVault

## Intent

As AI coding agents and multi-agent harnesses (Gemini/Antigravity, OpenCode, Codex, Claude Code) become the primary operators of local development environments, developer tooling must adapt to avoid agent-induced friction:
1. Long, uncontrolled disk scans (`find /home ...`) when looking for vault databases.
2. Binary installation collisions (`Text file busy`) caused by background daemons like `telemetryd`.
3. Multi-client MCP configuration drift and friction requiring manual JSON editing across diverse formats (`mcp_config.json`, `opencode.json`, `config.toml`).
4. High context token overhead in subagent handoffs and uncompressed context packs.
5. Lack of turnkey agent lifecycle hooks for real-time telemetry observation.
6. Knowledge fragmentation between Engram (session memories), OpenSpec (planning artifacts), and SkillVault.

This change delivers a cohesive set of 6 features to make SkillVault the seamless, agent-native knowledge and telemetry backbone across all local agent harnesses.

## Features & Scope

### 1. Robust Binary Installation & Daemon Lifecycle (REQ-DX-01)
- Update `Makefile` to use atomic installation (`install -m 755` / `cp --remove-destination`) preventing `ETXTBSY` ("Text file busy") when `telemetryd` is active.
- Add `skillvault telemetry service [start|stop|restart|status|install-service]` to control `telemetryd` cleanly and generate user systemd units.

### 2. Zero-Friction MCP Auto-Registration (REQ-DX-02)
- Introduce `skillvault mcp register [--client all|gemini|opencode|codex|claude|cursor]`.
- Safely parse, mutate, and validate MCP configurations across all supported harnesses without overwriting existing settings.
- Automatically verify resulting configurations via `skillvault mcp audit`.

### 3. Instant Topology Discovery & Canonical Symlinks (REQ-DX-03)
- Introduce `skillvault env [--json]` to report all system paths (`vault_home`, `database`, `socket`, `exports`, `telemetry_db`, `binary`) in under 10ms.
- Ensure `skillvault init` and `skillvault doctor` establish a canonical `~/.skillvault/skillvault.db -> vault.db` symlink to prevent heuristic search failures.

### 4. Agentic Handoffs & Context Compression (REQ-DX-04)
- Add MCP tools `save_handoff` and `get_handoff` (`purpose: STATE`, `tags: ["handoff"]`) for compact parent/child subagent checkpointing.
- Add `--format compact` in `get_context` to produce high-density YAML/Markdown saving 25-40% context window tokens.
- Implement graceful fallback to FTS5 with BM25 ranking when vector search is requested but embeddings are not loaded.

### 5. Turnkey Agent Telemetry Hooks & Live Activity (REQ-DX-05)
- Provide `skillvault telemetry install-hooks` generating ready-to-run hook configurations for OpenCode, Codex, and Gemini.
- Introduce `telemetryctl live` (or `telemetryctl top`) for real-time monitoring of agent token consumption, active runs, and loop/stall signals.

### 6. Ecosystem Interoperability: Engram & OpenSpec (REQ-DX-06)
- Introduce `skillvault sync-engram` to import and cross-reference Engram SQLite observations into the SkillVault knowledge graph.
- Add OpenSpec awareness to `skillvault get-context --mode planning` to automatically summarize active changes in `openspec/changes/`.
