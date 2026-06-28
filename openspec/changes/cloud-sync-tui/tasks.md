# Tasks: Cloud Sync (Snapshot) + Optional TUI

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1400 (4 PRs) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 |
| Delivery strategy | force-chained |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Base | Notes |
|------|------|-----------|------|-------|
| 1 | Transport interface + config + sanitizer | PR 1 | feature/tracker | ~360 lines; zero network deps |
| 2 | S3 + GitHub transport backends | PR 2 | PR 1 branch | ~280 lines; adds minio-go, go-github, oauth2 |
| 3 | SyncService + CLI push/pull wiring | PR 3 | PR 2 branch | ~350 lines; wires app layer and CLI |
| 4 | Bubble Tea TUI (build-tag gated) | PR 4 | PR 3 branch | ~390 lines; adds bubbletea; zero cost without tag |

## Phase 1: Sync Foundation — PR 1 (Transport, Config, Sanitizer)

- [x] 1.1 Create `internal/sync/transport.go` — `Transport` interface (`Push`/`Pull` with `ctx`, `io.Reader`/`io.Writer`, `key`), `GzipTransport` decorator
- [x] 1.2 Create `internal/sync/config.go` — `Config`, `S3Config`, `GitHubConfig` structs; `LoadConfig()` reads `~/.skillvault/config.yaml` with env override
- [x] 1.3 Create `internal/sync/sanitizer.go` — `SanitizingWriter` wrapping `io.Writer`, masks `key`/`secret`/`token` patterns via regex
- [x] 1.4 Write `internal/sync/transport_test.go` — table-driven gzip round-trip tests with `bytes.Buffer`
- [x] 1.5 Write `internal/sync/config_test.go` — temp YAML files; verify env > file precedence
- [x] 1.6 Write `internal/sync/sanitizer_test.go` — verify secret patterns masked, safe content unchanged
- [x] 1.7 Update `go.mod` — add `gopkg.in/yaml.v3` (transitively available; verify no new dep needed)

## Phase 2: Cloud Backends — PR 2 (S3 + GitHub Transports)

- [x] 2.1 Create `internal/sync/s3.go` — `S3Transport`: bucket config, minio client init from env/config; `Push` uploads object, `Pull` downloads object
- [x] 2.2 Create `internal/sync/github.go` — `GitHubTransport`: owner/repo, release asset upload/download via go-github; `Push` creates/updates release, `Pull` downloads asset
- [x] 2.3 Write `internal/sync/s3_test.go` — mock minio client (interface-based) or skip if SDK requires network
- [x] 2.4 Write `internal/sync/github_test.go` — mock GitHub client (interface-based); verify upload/download calls
- [x] 2.5 Update `go.mod` — add `minio/minio-go/v7`, `google/go-github/v69`, `golang.org/x/oauth2`

## Phase 3: SyncService + CLI — PR 3 (App Service + Wiring)

- [ ] 3.1 Create `internal/app/sync.go` — `SyncService{Push, Pull}`: wires `ExportJSON()` → gzip → transport push; transport pull → decompress → `ImportVault()`. Add `--dry-run` (export+compress, print size+timestamp diff, skip transfer). Add timestamp compare (last-write-wins).
- [ ] 3.2 Modify `internal/cli/commands.go` — add `"sync"`→subcommand dispatch (return `"sync-push"`/`"sync-pull"`, same pattern as `"memory"`→`"memory-index"`); add `ParseSyncFlags()` with `--transport`, `--remote-path`, `--dry-run`
- [ ] 3.3 Modify `cmd/skillvault/main.go` — add `syncSvc` to `vaultServices`; wire `NewSyncService(exportSvc, importSvc, transport)` in `openVault()`; add `case "sync-push"`, `case "sync-pull"` in `runCLI`
- [ ] 3.4 Write `internal/app/sync_test.go` — in-memory SQLite + mock Transport; round-trip (push→pull preserves all data), dry-run (no transfer, correct size), timestamp comparison
- [ ] 3.5 Write `internal/cli/commands_test.go` — table-driven `TestParseSyncFlags` matching existing pattern

## Phase 4: TUI — PR 4 (Bubble Tea Build-Tag Gated)

- [ ] 4.1 Create `internal/tui/model.go` — `//go:build tui`; Bubble Tea model: entry list state, search input, selected entry, detail pane
- [ ] 4.2 Create `internal/tui/views.go` — `//go:build tui`; View functions: browse (entry list), search bar (FTS5), entry detail (metadata/tags/body, read-only)
- [ ] 4.3 Create `internal/tui/run.go` — `//go:build tui`; `Run(svc)` entry point, update loop dispatching to model updates
- [ ] 4.4 Create `cmd/skillvault/main_tui.go` — `//go:build tui`; `runTUI(svc)` launches `tui.Run(svc)`
- [ ] 4.5 Create `cmd/skillvault/main_notui.go` — `//go:build !tui`; prints rebuild message to stderr, exits 1
- [ ] 4.6 Modify `cmd/skillvault/main.go` — add `case "tui"` dispatch in `main()` switch; call `runTUI(svc)` for tui builds or stub for non-tui
- [ ] 4.7 Modify `Makefile` — add `build-tui: go build -tags tui -o $(BINARY) ./cmd/skillvault`
- [ ] 4.8 Update `go.mod` — add `charmbracelet/bubbletea`, `charmbracelet/bubbles`
- [ ] 4.9 Write `internal/tui/model_test.go` — Bubble Tea `teatest` for state transitions: search input, entry selection, detail view display
