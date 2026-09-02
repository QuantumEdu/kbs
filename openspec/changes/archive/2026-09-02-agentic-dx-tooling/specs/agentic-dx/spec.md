# Capability Spec: Agentic DX & Tooling Suite

## Purpose
Define requirements and interfaces for the 6 Agentic DX & Tooling improvements in SkillVault (`kbs`), covering robust builds, multi-client MCP registration, topology discovery, agent handoffs, telemetry integration, and ecosystem interoperability.

## Requirements

### Requirement: Robust Build & Daemon Service (REQ-DX-01)
* The root `Makefile` MUST use atomic installation (`install -m 755` or `cp --remove-destination`) for all target binaries (`skillvault`, `q-secrets`, `telemetryd`, `telemetryctl`, `telemetrywrap`), preventing `ETXTBSY` when daemons are running.
* `skillvault telemetry service` MUST support:
  * `status`: inspect running `telemetryd` process or user service.
  * `start`: launch daemon if not running.
  * `stop`: send SIGTERM and await clean exit.
  * `restart`: stop and relaunch.
  * `install-service`: generate and enable `~/.config/systemd/user/telemetryd.service`.

### Requirement: Multi-Client MCP Registration (REQ-DX-02)
* `skillvault mcp register` MUST support the `--client` flag with values:
  * `all`: inspects and updates all detected client configs.
  * `gemini`: updates `~/.gemini/config/mcp_config.json` and `~/.gemini/antigravity-cli/mcp_config.json`.
  * `opencode`: updates `~/.config/opencode/opencode.json`.
  * `codex`: updates `~/.codex/config.toml`.
  * `claude`: updates `claude_desktop_config.json`.
* Registration MUST be idempotent (updating existing `skillvault` entry if path/args differ without duplicating).
* Registration MUST automatically invoke security validation equivalent to `skillvault mcp audit`.

### Requirement: Topology Discovery & Canonical Symlink (REQ-DX-03)
* `skillvault env` MUST output the canonical paths of the system:
  * In human text mode: formatted key-value list.
  * In `--json` mode: valid JSON with keys `version`, `vault_home`, `database`, `socket`, `exports_dir`, `telemetry_db`, `binary_path`.
* Execution time of `skillvault env` MUST be under 15ms.
* `skillvault init` and `skillvault doctor` MUST verify and establish a symlink `~/.skillvault/skillvault.db -> vault.db`.

### Requirement: Agentic Handoffs & Context Compression (REQ-DX-04)
* The MCP server MUST expose:
  * `save_handoff`: persists task checkpoint with `purpose: STATE`, `tags: ["handoff"]`, and metadata (`agent_id`, `task_id`, `status`).
  * `get_handoff`: retrieves the latest or specified handoff entry for a project.
* `skillvault context` / `get-context` MUST support `--format compact` or `--density high`, generating terse Markdown/YAML without verbose JSON wrapper keys.
* `skillvault search` and the `search_entries` MCP tool MUST automatically fall back to FTS5 with BM25 ranking when `vector: true` is supplied but embeddings are unconfigured, returning a non-fatal warning in metadata.

### Requirement: Telemetry Hooks & Live Activity (REQ-DX-05)
* `skillvault telemetry install-hooks` MUST generate hook files for:
  * OpenCode (`~/.config/opencode/hooks/` or hook integration).
  * Codex (`~/.codex/hooks.json`).
* `telemetryctl live` MUST provide a non-blocking or streaming summary of recent events, active agent threads, and detected loop/stall signals.

### Requirement: Ecosystem Interoperability: Engram & OpenSpec (REQ-DX-06)
* `skillvault sync-engram` MUST read Engram SQLite memories (`~/.codex/memories_1.sqlite` or configured engram path) and create or link indexed entries in SkillVault with `source: engram`.
* `skillvault get-context --mode planning` MUST detect `openspec/changes/` in the active repository root and include active change proposals and tasks in the planning brief.
