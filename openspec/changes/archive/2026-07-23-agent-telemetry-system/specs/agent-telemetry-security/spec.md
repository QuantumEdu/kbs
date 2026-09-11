# agent-telemetry-security Specification

## Purpose

Protect secrets and PII in telemetry events through hash-by-default arguments, regex redaction, and opt-in prompt body storage.

## Requirements

### Requirement: REQ-SEC-01 — Hash-By-Default for Arguments

The system MUST hash all command arguments and tool arguments before storage. Plaintext args MUST NOT be stored in `tool_calls.args_hash` or `command_executions.command_hash`. Hashing function: SHA-256 with project-specific salt from `~/.telemetry/salt` (generated on first daemon start, 0600 permissions).

```go
type ArgHasher interface {
    Hash(args []string) string     // SHA-256(salt + strings.Join(args, "\x00"))
    Verify(args []string, hash string) bool
}
```

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Tool args hashed** | tool `bash` called with `["curl", "-H", "Authorization: Bearer sk-abc123"]` | event emitted | `args_hash` = sha256(salt + args), plaintext NOT in DB |
| **Salt generation** | first daemon start, no `~/.telemetry/salt` | daemon initializes | 32-byte random salt written with 0600 perms, logged as info |
| **Salt tampered** | salt file has wrong perms (e.g., 0644) | daemon starts | daemon exits with code 3, logs "salt file permissions too open" |

### Requirement: REQ-SEC-02 — Regex Redaction

The system MUST apply configurable regex patterns to redact secrets before event storage. Built-in patterns (always active):

| Pattern | Matches | Replacement |
|---|---|---|
| `sk-[a-zA-Z0-9]{32,}` | OpenAI keys | `sk-***REDACTED***` |
| `Bearer\s+[a-zA-Z0-9._\-]+` | Bearer tokens | `Bearer ***REDACTED***` |
| `Authorization:\s*[^\s]+` | Auth headers | `Authorization: ***REDACTED***` |
| `--api-key\s+\S+` | CLI API key flags | `--api-key ***REDACTED***` |

Users MAY add custom patterns via `TELEMETRY_REDACTION_PATTERNS` (newline-separated regexes).

**Scenario: API key redacted from prompt**
- GIVEN user prompt contains `sk-proj-abc123def456ghi789jkl012mno345pqr`
- WHEN `prompt.submitted` emitted with `TELEMETRY_STORE_PROMPTS=true`
- THEN stored payload has `"sk-***REDACTED***"` replacing the key

### Requirement: REQ-SEC-03 — Opt-In Prompt Storage

The system MUST NOT store prompt or response bodies by default. Only event hashes (`prompt_hash`, `response_hash`) are persisted. Full prompt/response body storage requires `TELEMETRY_STORE_PROMPTS=true` (explicit user opt-in).

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Default: hash only** | `TELEMETRY_STORE_PROMPTS` unset | prompt submitted | `prompt.submitted.payload` = `{prompt_hash, prompt_char_count, model}`, NO `prompt_body` |
| **Opt-in: store body** | `TELEMETRY_STORE_PROMPTS=true` | prompt submitted | `payload` includes `prompt_body` (redacted, max 100KB) |
| **Refuse oversized** | `TELEMETRY_STORE_PROMPTS=true`, prompt > 100KB | prompt submitted | `prompt_body` truncated to 100KB, `prompt_truncated: true` in payload |

### Requirement: REQ-SEC-04 — Secret Scanning on Write

The system SHOULD scan event payloads for high-entropy strings (base64 ratio > 0.75, length > 20) before storage. Flagged events set `redaction_policy: scanned-warning` and log a warning. This is a best-effort safety net, not a guarantee.

**Scenario: High-entropy string detected**
- GIVEN event payload contains `dGhpc2lzYXRlc3RzZWNyZXRrZXk=` (base64-like, 28 chars, ratio 0.82)
- WHEN event processed by daemon
- THEN event stored with `redaction_policy: scanned-warning`, daemon logs "high-entropy field detected in payload"

## Error Handling

| Error | Behavior |
|---|---|
| Salt file missing on restart | Daemon exits with code 3, asks user to restore from backup or delete DB |
| Redaction regex compile error (custom) | Log error, skip that pattern, use built-in patterns only |
| Prompt too large for storage (>1MB) | Reject, store hash only, log warning |

## Non-Functional Requirements

| NFR | Requirement |
|---|---|
| **Privacy** | Plaintext secrets MUST NOT appear in logs, DB, or CLI output |
| **Transparency** | `telemetryctl status` MUST show: prompt storage enabled?, redaction patterns active, salt fingerprint |
| **Compliance** | Opt-in model respects user consent; default is zero plaintext prompt storage |
