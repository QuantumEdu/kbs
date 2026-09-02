# Tasks: Agentic DX & Tooling Suite

## Phase 1: Build Robustness, Topology Discovery & Canonical Symlinks
- [x] 1.1 Update `Makefile` to use atomic install (`install -m 755` / `cp --remove-destination`) preventing `ETXTBSY` when background daemons are running.
- [x] 1.2 Implement `skillvault env [--json]` in `internal/cli/handlers_env.go` and wire into command registry for sub-10ms topology discovery.
- [x] 1.3 Ensure `skillvault init` and `skillvault doctor` establish and verify canonical symlink `~/.skillvault/skillvault.db -> vault.db`.
- [x] 1.4 Add unit tests for `skillvault env` and symlink creation.

## Phase 2: Multi-Client MCP Auto-Registration
- [x] 2.1 Implement `RegisterMCPConfig` in `internal/security/mcp_config.go` or `internal/cli/handlers_mcp_register.go` supporting OpenCode, Gemini, Codex, and Claude.
- [x] 2.2 Wire `skillvault mcp register [--client all|gemini|opencode|codex|claude]` into CLI command registry and `cmd/skillvault/main.go`.
- [x] 2.3 Integrate post-registration validation using existing `skillvault mcp audit` scanner.
- [x] 2.4 Add unit tests verifying idempotent insertion across JSON and TOML configuration files.

## Phase 3: Agentic Handoffs, Context Density & Search Fallback
- [x] 3.1 Implement `save_handoff` and `get_handoff` MCP tools in `internal/mcp/tools.go`.
- [x] 3.2 Add `--format compact` to `skillvault context` / `get-context` producing high-density agent brief.
- [x] 3.3 Add graceful fallback from vector search to FTS5 BM25 when word vectors are not loaded.
- [x] 3.4 Add tests in `internal/mcp/` and `internal/context/`.

## Phase 4: Telemetry Lifecycle Service & Live Monitor
- [x] 4.1 Implement `skillvault telemetry service [status|start|stop|restart|install-service]` to cleanly manage `telemetryd`.
- [x] 4.2 Implement `telemetryctl live` in `cmd/telemetryctl/` for real-time monitoring of agent token usage and loop/stall signals.
- [x] 4.3 Add `skillvault telemetry install-hooks` generating ready-to-run hooks for OpenCode and Codex.
- [x] 4.4 Add unit tests for service lifecycle and telemetry hooks.

## Phase 5: Ecosystem Interoperability (Engram & OpenSpec)
- [x] 5.1 Implement `skillvault sync-engram` in `internal/app/` to import observations from Engram SQLite databases.
- [x] 5.2 Add OpenSpec active changes discovery to `skillvault get-context --mode planning`.
- [x] 5.3 Verify end-to-end integration across all 6 features with `go test ./...` and `make install-all`.
