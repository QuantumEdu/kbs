# Proposal: Cloud Sync (Snapshot) + Optional TUI

## Intent

SkillVault v3 lacks multi-machine sharing beyond manual export/import. Add automated snapshot sync via pluggable transports (S3-compatible, GitHub Releases) and an optional Bubble Tea TUI gated by build tag.

## Scope

### In Scope
- Pluggable `Transport` interface in `internal/sync/` (push/pull byte streams)
- S3 transport (minio-go v7), GitHub Releases transport, gzip compression
- CLI: `skillvault sync push` / `sync pull`; snapshot = last-write-wins
- Credentials via env vars + `~/.skillvault/config.yaml`
- `internal/tui/` with Bubble Tea browse/search views, `//go:build tui` gated
- Runtime message for non-TUI builds; `make build-tui` Makefile target

### Out of Scope
- Daemon, background sync, CRDT merging, encryption-at-rest
- TUI entry creation/editing (read-only browse/search)
- Multi-vault merge, Web UI, sync MCP tools

## Capabilities

### New Capabilities
- `cloud-sync`: Transport interface, S3/GitHub implementations, sync service, gzip, credentials
- `tui`: Build-tag-gated Bubble Tea TUI (search, browse, entry detail)

### Modified Capabilities
- `skillvault`: REQ-HYB-07 (no cloud sync → optional), REQ-SEC-05 (no network → sync network allowed), REQ-CLI-02 (add `sync`, `tui` to commands)

## Approach

Reuse `ExportJSON`/`ImportVault` unchanged. `internal/sync/` defines `Transport`; `SyncService` does export→compress→push and pull→decompress→import. S3 first, GitHub second. CLI: `"sync"` dispatches to `"sync-push"`/`"sync-pull"`.

TUI: `internal/tui/` with `//go:build tui` on every file. `main_tui.go` wires `case "tui"`; `main_notui.go` prints rebuild message. Zero binary cost without tag.

## Affected Areas

| Area | Impact |
|------|--------|
| `internal/sync/` | New: Transport, S3, GitHub, SyncService |
| `internal/tui/` | New: Bubble Tea TUI (build-tag gated) |
| `cmd/skillvault/main.go` | Modified: sync/tui dispatch, service wiring |
| `cmd/skillvault/main_tui.go` | New: TUI launch (`//go:build tui`) |
| `cmd/skillvault/main_notui.go` | New: Stub message for non-TUI builds |
| `internal/cli/commands.go` | Modified: ParseCommand, ParseSyncFlags |
| `go.mod`, `Makefile` | Modified: deps, `build-tui` target |
| `internal/app/import_export.go` | None: reused as-is |
| `internal/db/import_export_store.go` | None: reused as-is |

## Risks

| Risk | Mitigation |
|------|------------|
| Credential leak via logs | Sanitize transport logs |
| Large vault > GitHub release limit | gzip + size warning; S3 unlimited |
| Snapshot overwrite data loss | Timestamp compare; `--dry-run` flag |
| Bubble Tea +3MB binary | Tag excludes from default build |

## Rollback Plan

Remove `internal/sync/` and `internal/tui/`, drop cases from main.go/commands.go, remove deps from go.mod. Vault data never mutated by sync.

## Dependencies

- `minio/minio-go/v7`, `google/go-github/v62`, `charmbracelet/bubbletea`
- All pure Go, zero CGO; bubbletea only linked with `-tags tui`

## Success Criteria

- [ ] `skillvault sync push/pull` round-trips via S3 and GitHub
- [ ] `go build -tags tui && ./skillvault tui` launches TUI; without tag prints rebuild message
- [ ] Default binary grows <500KB; all existing tests pass
- [ ] New packages >80% test coverage
