# line

`line` is a sibling CLI in the kbs suite: a deterministic issue-to-PR factory.

SkillVault still owns prompts, skills, and retrieval. `line` owns the job state machine, agent execution, GitHub check polling, optional ntfy, and a loopback UI. It does **not** extend `skillvault run`.

## Commands

```sh
cd /path/to/kbs
make build-line

line run --repo /absolute/path/to/repo https://github.com/owner/repo/issues/123
line status
line continue --answer "the API is public"
line next
line serve --listen 127.0.0.1:7340
```

State: `~/.line/line.db` (`--db`).

## Phases

```text
plan → build → review → wait_ci → ready
                         ↑     |
                         repair
```

Each LLM phase and each CI poll parks on `awaiting_next` until `line next` or the UI **advance** button.

| Phase | Who | Next on ok |
|---|---|---|
| `plan` | agent | `build` |
| `build` | agent | `review` |
| `review` | agent | `wait_ci` |
| `wait_ci` | `gh pr view` + `gh pr checks` (no LLM) | `ready` if pass; stay `wait_ci` if pending; `repair` if fail |
| `repair` | agent | `review` again |
| `ready` | stop | human merge |

Repair budget: 3 CI-fail repairs, then `blocked`. `needs_human` still waits for `line continue`.

`wait_ci` requires authenticated `gh` in `--repo` with an open pull request. One poll per `next`; pending CI is not a long sleep.

## ntfy

Optional. Empty topic means no push.

```sh
export LINE_NTFY_TOPIC=line-yourname-unguessable
# optional: LINE_NTFY_SERVER=https://ntfy.sh
line run --ntfy-topic "$LINE_NTFY_TOPIC" --repo /repo https://github.com/owner/repo/issues/1
```

On the phone: ntfy app → subscribe to that topic. Pushes fire on `needs_human`, `blocked`, and `ready`. Tapping opens the GitHub issue (`Click`). A failed ntfy post does not fail the job; it is recorded as event `ntfy_error`.

Do not put secrets in the topic name if you use the public `ntfy.sh` server.

## UI

`line serve` binds loopback only (`127.0.0.1:7340`). It lists jobs, shows events, continues `needs_human`, and advances `awaiting_next` (including `wait_ci` and `repair`). Do not expose the port.

## Prompts

Embedded in `internal/line/prompts/{plan,build,review,repair}.md`. Override with `--prompts <dir>`.

Default executor: `claude --print`.
