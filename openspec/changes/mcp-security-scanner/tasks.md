# Tasks: Security Audit Engine & Import Gate (Phases 1 & 2)

## Phase 1: Security Audit Engine
- [x] 1.1 Create `internal/security/audit.go` with `Finding`, `AuditReport`, rule definitions (injection, secrets, commands), and core scanning methods.
- [x] 1.2 Create `internal/security/audit_test.go` verifying prompt injection detection, secret detection, dangerous commands, and false-positive resilience.
- [x] 1.3 Add pack and file auditing methods to `Auditor`.

## Phase 2: CLI Integration & Strict Import Gate
- [x] 2.1 Create `internal/app/audit.go` with `AuditService` scanning vault entries.
- [x] 2.2 Update `internal/cli/flags.go` to support `ParseAuditFlags` and `--strict-audit` in `ParseImportFlags`.
- [x] 2.3 Wire `audit` subcommand into `cmd/skillvault/main.go` supporting table output and JSON formatting.
- [x] 2.4 Integrate `--strict-audit` into `import` subcommand in `cmd/skillvault/main.go` and `internal/app/import_export.go`.
- [x] 2.5 Run `go test ./...` and verify end-to-end audit and import gate functionality.

## Phase 3: Telemetry Injection Detector
- [x] 3.1 Create `internal/agenttelemetry/injectiondetector.go` to intercept prompt injection and dangerous command patterns in telemetry streams.
- [x] 3.2 Create `internal/agenttelemetry/injectiondetector_test.go` with unit tests for real-time injection and hazard detection.
- [x] 3.3 Wire `InjectionDetector` into `Collector.ingest` and `telemetryd` entry point.

## Phase 4: MCP Config Audit & SARIF Format
- [x] 4.1 Create `internal/security/sarif.go` for SARIF v2.1.0 report generation and export.
- [x] 4.2 Create `internal/security/mcp_config.go` for scanning client MCP configuration files (`~/.cursor/mcp.json`, `claude_desktop_config.json`, `opencode.json`, `windsurf`) for plaintext secrets and unsafe settings.
- [x] 4.3 Update `internal/cli/commands.go` and `cmd/skillvault/main.go` to support `skillvault mcp audit` and `skillvault audit --format sarif`.
- [x] 4.4 Run full test suite and verify end-to-end functionality.
