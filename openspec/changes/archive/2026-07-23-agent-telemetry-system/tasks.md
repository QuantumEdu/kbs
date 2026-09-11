# Tasks: Agent Telemetry System

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2000-2500 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | 7 PRs (feature-branch-chain) |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |
| Slices exceeding 400 lines | PR #2 (~550, Store), PR #5 (~460, Plugins), PR #6 (~420, Quality) |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Event schema + types | PR 1 | `go test ./internal/agenttelemetry/ -run TestEvent` | N/A (unit only) | Delete `event.go`, `git.go`, `otel.go` |
| 2 | Daemon + SQLite store | PR 2 | `go test ./internal/agenttelemetry/ -run 'TestCollector\|TestStore'` | Start daemon on temp socket, send event | Delete `cmd/telemetryd/`, `collector.go`, `store.go` |
| 3 | Security pipeline | PR 3 | `go test ./internal/agenttelemetry/ -run 'TestHasher\|TestRedactor'` | N/A (unit only) | Delete `hasher.go`, `redactor.go`, `entropyscanner.go` |
| 4 | telemetryctl CLI | PR 4 | `go build ./cmd/telemetryctl && telemetryctl status` | `telemetryd &` → `telemetryctl run list` → verify output | Delete `cmd/telemetryctl/` |
| 5 | Plugins + wrapper | PR 5 | `go test ./internal/agenttelemetry/plugin/ -run TestOpenCode` | N/A (no OpenCode runtime) | Delete `plugin/`, `telemetrywrap/` |
| 6 | Quality signal detectors | PR 6 | `go test ./internal/agenttelemetry/ -run 'TestLoop\|TestStall\|TestStreak'` | N/A (unit only) | Delete `loopdetector.go`, `stalldetector.go`, `streakdetector.go`, `tokencounter.go` |
| 7 | Integration + E2E | PR 7 | `go test -tags integration ./internal/agenttelemetry/` | Start daemon → mock emits 20 types → verify DB | Delete `_test.go` files, keep production code |

### PR Chain (feature-branch-chain)

- PR #1 base: `feature/agent-telemetry-system`
- PR #2 base: PR #1 branch
- PRs #3–#7 each base on previous PR branch

## Phase 1: Foundation (PR #1 — ~300 lines)
> REQ-CORE-01, REQ-CORE-04, threat matrix: git

- [x] 1.1 [RED] `internal/agenttelemetry/git_test.go`: non-git dir, missing git, `--` injection (threat matrix)
- [x] 1.2 `internal/agenttelemetry/event.go`: `Event`, `EventEnvelope`, `RunOpts`, `AgentRun`, `ToolCallRecord`, `TokenUsage`, `DaemonStatus` types with JSON-L tags
- [x] 1.3 `internal/agenttelemetry/git.go`: `CaptureGitContext()` — `repo_url`, `branch`, `commit_sha` from git; null for non-git
- [x] 1.4 `internal/agenttelemetry/event_test.go`: JSON round-trip golden tests, required-field validation
- [x] 1.5 `internal/agenttelemetry/otel.go`: OTel stub, `//go:build otel` gate (Phase 2 plumbing)

## Phase 2: Collector + Store (PR #2 — ~550 lines) ⚠️ exceeds 400
> REQ-CORE-01, REQ-CORE-02

- [x] 2.1 `internal/agenttelemetry/validator.go`: required-field check, enum validation, malformed JSON rejection
- [x] 2.2 `internal/agenttelemetry/store.go`: SQLite DDL (5 tables + indexes per spec), `SaveRun`, `SaveEvent`, `GetRun`, `ListRuns`, `Status`, `Close`, WAL mode, retry-on-busy (3x exponential)
- [x] 2.3 `internal/agenttelemetry/store_test.go`: table creation, CRUD, concurrent read/write
- [x] 2.4 `internal/agenttelemetry/collector.go`: Unix socket bind/listen/accept, `bufio.ScanLines`, per-client goroutine, ack `{"status":"ok"}`, graceful shutdown drain 5s
- [x] 2.5 `cmd/telemetryd/main.go`: wire Validator → Collector → Store, socket retry 3x, signal handling, salt check

## Phase 3: Security (PR #3 — ~380 lines)
> REQ-SEC-01, REQ-SEC-02, REQ-SEC-03, REQ-SEC-04

- [x] 3.1 `internal/agenttelemetry/hasher.go`: SHA-256(salt+join(args,"\x00")), `Verify`, `SaltFingerprint`, salt gen 32 bytes 0600
- [x] 3.2 `internal/agenttelemetry/hasher_test.go`: determinism, salt isolation, verification, missing/wrong-perm salt → exit code 3
- [x] 3.3 `internal/agenttelemetry/redactor.go`: 4 built-in regexes, custom via `TELEMETRY_REDACTION_PATTERNS`, compile-error fallback
- [x] 3.4 `internal/agenttelemetry/redactor_test.go`: API key, bearer token, auth header, `--api-key` flag redaction
- [x] 3.5 `internal/agenttelemetry/entropyscanner.go`: base64 ratio > 0.75 + length > 20 → `scanned-warning` flag
- [x] 3.6 Wire security pipeline into Collector.Ingest: hash → redact → entropy scan before Store

## Phase 4: CLI (PR #4 — ~220 lines)
> REQ-CORE-03, REQ-QUAL-05

- [x] 4.1 `cmd/telemetryctl/main.go`: subcommands with `flag` package
- [x] 4.2 `run list`: `--limit`, `--agent`, `--since` filters, table output (run_id, agent, status, tokens, cost, duration)
- [x] 4.3 `run show ID`: header + step tree + token/cost breakdown + quality signal summary
- [x] 4.4 `run recent`: last 5 runs summary
- [x] 4.5 `status`: daemon uptime, events ingested, DB size, salt fingerprint, redaction patterns, prompt storage enabled?

## Phase 5: Plugins (PR #5 — ~460 lines) ⚠️ exceeds 400
> REQ-PLUG-01, REQ-PLUG-02, REQ-PLUG-03, REQ-PLUG-04

- [ ] 5.1 `internal/agenttelemetry/plugin/opencode.go`: `OpenCodeEmitter` implementing `EventEmitter`, 9 event types via hook callbacks, correlation_id chain
- [ ] 5.2 `internal/agenttelemetry/plugin/opencode_test.go`: mock hook callbacks, verify all 9 event types, daemon-unreachable error path
- [ ] 5.3 `internal/agenttelemetry/telemetrywrap/main.go`: `os/exec` with pipes, stdout API-usage parsing, 2s git-diff poll, `confidence_level: heuristic`, timeout (30m → SIGTERM → 30s → SIGKILL)
- [ ] 5.4 `internal/agenttelemetry/env.go`: env vars (`TELEMETRY_ENABLED`, `TELEMETRY_SOCKET`, `TELEMETRY_STORE_PROMPTS`), daemon-unreachable: buffer 1000 events, reconnect on emit

## Phase 6: Quality Signals (PR #6 — ~420 lines) ⚠️ exceeds 400
> REQ-QUAL-01, REQ-QUAL-02, REQ-QUAL-03, REQ-QUAL-04, REQ-QUAL-05

- [x] 6.1 `internal/agenttelemetry/loopdetector.go`: rolling 60s window, 3 identical `args_hash` → `loop.detected`, LRU eviction max 1000
- [x] 6.2 `internal/agenttelemetry/loopdetector_test.go`: 3 identical → detect, different args → no, window expiry → no
- [x] 6.3 `internal/agenttelemetry/stalldetector.go`: wall-clock inactivity check, 60s no events on active run → `policy.violation`
- [x] 6.4 `internal/agenttelemetry/stalldetector_test.go`: 61s inactivity → violation, completed run → no
- [x] 6.5 `internal/agenttelemetry/streakdetector.go`: ≥5 fail streak or ≥3 no-change streak → `policy.violation`
- [x] 6.6 `internal/agenttelemetry/tokencounter.go`: `char-div-4` estimation, `efficiency_ratio < 0.05` → `policy.violation`
- [x] 6.7 Wire detectors into Collector.Ingest: call after Store, emit signals as events

## Phase 7: Integration + E2E (PR #7 — ~360 lines)
> REQ-CORE-01, REQ-CORE-03, REQ-QUAL-05

- [ ] 7.1 `internal/agenttelemetry/collector_test.go`: temp Unix socket, valid/invalid events, verify DB rows (integration)
- [ ] 7.2 `internal/agenttelemetry/mock_plugin_test.go`: mock `EventEmitter` emits all 20 event types, verify DB counts (integration)
- [ ] 7.3 `internal/agenttelemetry/telemetrywrap/telemetrywrap_test.go`: wrap `echo hello`, verify DB run + events, skip in `-short` (E2E)
- [ ] 7.4 Update `Makefile`: add `test-integration` target with `-tags integration -count=1`
