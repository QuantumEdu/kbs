# Design: Cloud Sync (Snapshot) + Optional TUI

## Technical Approach

Reuse `ExportAll()`/`ImportVault()` from `db.ImportExportStore` unchanged. Wrap them in `internal/sync/Transport` and `app.SyncService`: export→gzip→push, pull→decompress→import. Pluggable backends (S3 via `minio-go/v7`, GitHub via `go-github/v69`). TUI in `internal/tui/` gated by `//go:build tui`, wired via `main_tui.go`/`main_notui.go`.

## Architecture Decisions

| Decision | Options | Choice | Why |
|----------|---------|--------|-----|
| Transport shape | `Push(reader)` vs `Push(bytes)` | `Push(ctx, io.Reader, key)` | Streams through gzip; no full-vault buffering |
| S3 SDK | `minio-go/v7` vs `aws-sdk-go-v2` | `minio-go/v7` | ~5 pure-Go deps vs ~100 for aws-sdk-go-v2; covers AWS/MinIO/R2/B2 |
| GitHub client | `go-github/v69` + `oauth2` | `go-github/v69` | Canonical SDK; token auth via oauth2 |
| Config store | env-only vs `config.yaml` | `~/.skillvault/config.yaml` + env override | Env works for CI; file for persistence. `yaml.v3` already transitively available |
| TUI isolation | Separate binary vs build tag | `//go:build tui` on all `internal/tui/*.go` | Zero binary cost, no service wiring duplication |
| Subcommand dispatch | `"sync-push"` vs `"sync"`+"push" | `"sync-push"`/`"sync-pull"` canonical commands | Matches existing `"memory-index"` pattern in `ParseCommand` |

## Data Flow

```
Push: ExportJSON() → gzip.Writer → transport.Push(ctx, reader, key)
Pull: transport.Pull(ctx, writer, key) → gzip.Reader → json.Unmarshal → ImportVault()
```

`GzipTransport` wraps any `Transport`: compresses on push, decompresses on pull. Dry-run: export + compress to measure, print timestamp diff + payload size, skip transfer.

## File Changes

| File | Action | Purpose |
|------|--------|---------|
| `internal/sync/transport.go` | Create | `Transport` interface + `GzipTransport` decorator |
| `internal/sync/s3.go` | Create | `S3Transport` — bucket config, minio client init from env/config |
| `internal/sync/github.go` | Create | `GitHubTransport` — owner/repo, release asset upload/download |
| `internal/sync/config.go` | Create | `LoadConfig()` — reads `config.yaml`, env override |
| `internal/sync/sanitizer.go` | Create | `io.Writer` that masks `key`/`secret`/`token` regex patterns |
| `internal/app/sync.go` | Create | `SyncService{Push,Pull}` — wires export/import, dry-run, timestamp check |
| `internal/tui/model.go`, `views.go`, `run.go` | Create | Bubble Tea model/view/update; entry list, search bar, detail pane (read-only). All `//go:build tui` |
| `cmd/skillvault/main_tui.go` | Create | `//go:build tui` — `runTUI(svc)` launches `tui.Run()` |
| `cmd/skillvault/main_notui.go` | Create | `//go:build !tui` — prints rebuild message, exits 1 |
| `internal/cli/commands.go` | Modify | Add `"sync"`→subcommand dispatch + `"tui"` case; `ParseSyncFlags()` |
| `cmd/skillvault/main.go` | Modify | Add `syncSvc` to `vaultServices`/`openVault()`; wire `case "sync-push"/"sync-pull"` in `runCLI` |
| `go.mod` | Modify | Add `minio-go/v7`, `go-github/v69`, `oauth2`, `bubbletea`, `yaml.v3` |
| `Makefile` | Modify | `build-tui: go build -tags tui -o $(BINARY) ./cmd/skillvault` |

## Key Interfaces

```go
// internal/sync/transport.go
type Transport interface {
    Push(ctx context.Context, reader io.Reader, key string) error
    Pull(ctx context.Context, writer io.Writer, key string) error
}

// internal/sync/config.go
type Config struct {
    Transport  string        `yaml:"transport"`
    RemotePath string        `yaml:"remote_path"`
    S3         *S3Config     `yaml:"s3,omitempty"`
    GitHub     *GitHubConfig `yaml:"github,omitempty"`
}
type S3Config struct {
    Bucket, Region, Endpoint, AccessKeyID, SecretAccessKey string
}
type GitHubConfig struct {
    Owner, Repo, Token string
}
```

## CLI Command Spec

```
skillvault sync push  --transport s3|github --remote-path <path> [--dry-run]
skillvault sync pull  --transport s3|github --remote-path <path> [--dry-run]
```

`ParseCommand` maps `"sync"` → parse `args[2]` as subcommand returning `"sync-push"` or `"sync-pull"` (same pattern as `"memory"`→`"memory-index"`). `ParseSyncFlags` extracts `--transport`, `--remote-path`, `--dry-run`. Falls back to `config.yaml` when flags absent.

Credentials priority: env var > config file. Env names: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `AWS_ENDPOINT` (MinIO), `GITHUB_TOKEN`.

## Build Tags

- `internal/tui/*.go`: `//go:build tui`
- `cmd/skillvault/main_tui.go`: `//go:build tui` — `func runTUI(svc *vaultServices)` launches Bubble Tea
- `cmd/skillvault/main_notui.go`: `//go:build !tui` — prints rebuild message, exits 1
- `Makefile`: `build-tui: go build -tags tui -o $(BINARY) ./cmd/skillvault`

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Missing/invalid credentials | Transport constructor returns error; CLI exits before network call |
| Remote not found (S3 404, GitHub no release) | `Pull` returns wrapped error; CLI prints to stderr |
| Remote older than local | Warn via stderr; proceed (snapshot = last-write-wins). Timestamp diff shown in dry-run |
| Corrupt gzip on pull | `Pull` returns decompress error; import never attempted |
| `go build` (no tag) + `skillvault tui` | `main_notui.go` prints rebuild message to stderr, exits 1 |
| Non-TTY + Bubble Tea | Bubble Tea built-in TTY detection; exits with "not a terminal" |

## Testing Strategy

| Layer | What | How |
|-------|------|-----|
| `internal/sync/` | Transport + gzip + sanitizer | `bytes.Buffer` as mock; table-driven |
| `internal/sync/` | Config loading | Temp YAML files; verify env > file precedence |
| `internal/app/` | SyncService | In-memory SQLite + mock Transport; round-trip, dry-run, timestamp comparison |
| CLI | Flag parsing | Table-driven `TestParseSyncFlags` — matches existing pattern |
| TUI | Model state | Bubble Tea `teatest` for update/view state transitions |

Default binary grows by `minio-go` + `go-github` + `oauth2` + `yaml.v3` (<500KB per proposal). No migration required.
