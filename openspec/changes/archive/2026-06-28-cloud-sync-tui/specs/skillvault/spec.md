# Delta for SkillVault

## ADDED Requirements

### Requirement: REQ-SYNC-01 — Pluggable Transport and Sync Service

The system MUST define a pluggable `Transport` interface (`Push`/`Pull` byte streams) in `internal/sync/`. Implementations: S3-compatible (minio-go v7, supporting AWS S3, MinIO, Cloudflare R2, Backblaze B2) and GitHub Releases. Gzip MUST compress payload before wire transfer; decompress on pull. Sync service SHALL reuse `ExportAll()`/`ImportAll()` unchanged. Snapshot semantics: last-write-wins via timestamp comparison.

#### Scenario: Round-trip via S3

- GIVEN vault with entries and configured S3 credentials
- WHEN `sync push --transport s3` then `sync pull --transport s3`
- THEN all entries, projects, workflows, and tags are preserved identically

#### Scenario: Dry-run and failure

- GIVEN `--dry-run` flag, WHEN `sync push` runs, THEN timestamp diff and payload size shown, no transfer
- GIVEN invalid credentials, WHEN push executes, THEN error reported with sanitized logs (no raw keys)

### Requirement: REQ-SYNC-02 — Sync CLI and Credentials

The CLI MUST support `skillvault sync push` and `skillvault sync pull`. Credentials MUST load from env vars (`AWS_ACCESS_KEY_ID`, `GITHUB_TOKEN`, etc.) and `~/.skillvault/config.yaml`. Transport logs MUST sanitize credentials.

#### Scenario: Push pull cycle

- GIVEN configured transport, WHEN `sync push` runs, THEN vault exported, compressed, uploaded as dated snapshot
- GIVEN remote newer than local, WHEN `sync pull` runs, THEN downloaded, decompressed, imported

### Requirement: REQ-TUI-01 — Build Tag, Views, and Constraints

All `internal/tui/` files MUST be gated by `//go:build tui`. CLI SHALL register `tui` via build-tag-gated `main_tui.go`. Without tag, `main_notui.go` SHALL print: "TUI not available. Rebuild with: go build -tags tui ./cmd/skillvault" to stderr, exit 1. TUI MUST provide browse (entry list), FTS5 search, and entry detail views. TUI SHALL be read-only — no creation, editing, or mutation. A `make build-tui` target SHOULD build with `-tags tui`. Default binary (no tag) SHOULD grow <500KB over baseline.

#### Scenario: TUI with and without build tag

- GIVEN `go build -tags tui`, WHEN `skillvault tui` runs, THEN Bubble Tea TUI launches with browse/search/detail views
- GIVEN build without tag, WHEN `skillvault tui` runs, THEN stderr prints rebuild message, exit code 1

#### Scenario: TUI views and build target

- GIVEN TUI launched with populated vault, WHEN user browses or searches, THEN entries display title/type/status
- GIVEN user selects entry, WHEN detail view opens, THEN metadata/tags/body shown without edit capability
- GIVEN project root, WHEN `make build-tui` runs, THEN binary includes TUI and passes existing tests

## MODIFIED Requirements

### Requirement: REQ-HYB-07

The vault MUST remain local-first by default. Cloud sync via pluggable transports is OPTIONAL and user-initiated — no daemon, background sync, or automatic network calls.
(Previously: No cloud sync, no daemon, no vector DB in v2 — local-first only)

#### Scenario: Init unchanged, sync explicit

- GIVEN no vault exists, WHEN `skillvault init` runs, THEN `vault.db` and subdirectories created — no network calls
- GIVEN running vault, WHEN no `sync` command issued, THEN zero network calls occur

### Requirement: REQ-SEC-05

The system MUST NOT make network calls except during explicit `sync push` or `sync pull`. All other operations remain local-only.
(Previously: No network calls in core v2 — local-first only)

#### Scenario: Local vs network operations

- GIVEN any operation other than sync (search, get-context, save-entry, etc.), WHEN invoked, THEN only local SQLite/filesystem accessed
- GIVEN configured transport, WHEN `sync push` executes, THEN network calls transfer snapshot

### Requirement: REQ-CLI-02

The CLI MUST provide 16 commands: `init`, `add-entry`, `search`, `get`, `save-artifact`, `get-context`, `add-project`, `list-projects`, `archive`, `add-workflow`, `render-workflow`, `session-wrap`, `export`, `import`, `sync`, `tui`.
(Previously: 14 commands — `sync` and `tui` are new)

#### Scenario: Sync and TUI dispatch

- GIVEN vault configured, WHEN `skillvault sync push` runs, THEN snapshot uploaded via transport
- GIVEN non-tui build, WHEN `skillvault tui` runs, THEN rebuild message printed to stderr
