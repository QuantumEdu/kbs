# agent-telemetry-quality Specification

## Purpose

Detect quality/inefficiency signals from event streams: infinite loops, stalls, unproductive streaks. Computed on write, not in a separate process.

## Requirements

### Requirement: REQ-QUAL-01 — Loop Detection

The system MUST detect when an agent repeats the same tool call ≥ 3 times within a rolling 60-second window and emit `loop.detected`.

```go
type LoopDetector interface {
    Check(call ToolCallRecord) *LoopSignal  // nil if no loop detected
}

type LoopSignal struct {
    Pattern     string // "identical-call"
    ToolName    string
    RepeatCount int
    WindowMs    int64
    CallHashes  []string // last N call args_hash values
}
```

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Loop detected: 3 identical bash calls** | step "fix-lint" issues 3 `bash("eslint --fix")` calls within 30s | 3rd call arrives | `loop.detected` event emitted with `pattern: identical-call, repeat_count: 3` |
| **No false positive: different args** | 3 `bash` calls with DIFFERENT args within 10s | all 3 arrive | NO `loop.detected` event emitted |
| **Window expires** | 2 identical calls at t=0s and t=30s; 3rd at t=65s | 3rd call arrives | NO detection (window expired) |
| **Loop detection bypass** | agent uses different-but-equivalent tool calls (e.g., different path syntax) | — | **Known limitation**: semantic equivalence NOT detected (documented) |

### Requirement: REQ-QUAL-02 — Stall Detection

The system MUST detect stalls: no event received from any source for > 60 seconds during an active run. Emits `policy.violation` with `policy_name: stall-detected` and `severity: warning`.

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Stall detected** | active run `run-01`, last event at t=0s | no events by t=65s | `policy.violation` emitted with `detail: "no event for 65s"` |
| **Stall not detected (idle run)** | run `run-01` status = `completed` | 60s elapses | NO violation (run is not active) |

### Requirement: REQ-QUAL-03 — Unproductive Streak Detection

The system SHOULD detect unproductive streaks: ≥ 5 consecutive tool calls where `success=false` OR ≥ 3 consecutive tool calls with `success=true` but zero file changes detected. Emits `policy.violation` with `policy_name: unproductive-streak`, `severity: info`.

**Scenario: Failed tool streak**
- GIVEN 5 consecutive `tool.completed` events with `success: false`
- WHEN 5th arrives
- THEN `policy.violation` emitted with `detail: "5 consecutive failed tool calls"`

**Scenario: No-change streak**
- GIVEN 3 `tool.completed` events with `success: true` and zero `file.*` events between them
- WHEN 3rd arrives
- THEN `policy.violation` emitted with `detail: "3 successful tool calls with no file changes"`

### Requirement: REQ-QUAL-04 — Token Efficiency Signal

The system SHOULD compute a token efficiency ratio per step: `output_tokens / (input_tokens + output_tokens)`. Store in `token_usage`. Ratio < 0.05 (95%+ input tokens) flags `policy.violation` with `policy_name: low-output-ratio`, `severity: info`.

| Scenario | GIVEN | WHEN | THEN |
|---|---|---|---|
| **Low output ratio** | step used 10000 input tokens, 200 output tokens (ratio 0.02) | step.completed | `policy.violation` emitted, ratio stored in `token_usage` |
| **Normal ratio** | step used 2000 input, 800 output (ratio 0.28) | step.completed | NO violation |

### Requirement: REQ-QUAL-05 — Signal Storage

Quality signals MUST be stored as `events` rows with `event_type: loop.detected` or `policy.violation` and linked to the active `run_id` and `step_id`. `telemetryctl run show ID` MUST display quality signal summary.

**Scenario: Quality signals in CLI**
- GIVEN run `run-abc` has 2 `loop.detected` events and 1 `policy.violation`
- WHEN `telemetryctl run show run-abc`
- THEN output includes "Quality Signals: 2 loops, 1 policy violation"

## Error Handling

| Error | Behavior |
|---|---|
| Detector state loss (daemon restart) | Reset counters; known limitation — streaks spanning restarts not detected |
| Event reordering (out-of-order delivery) | Detectors use `timestamp` field, not insertion order; skip events with `timestamp` < last processed for that run |

## Non-Functional Requirements

| NFR | Requirement |
|---|---|
| **Latency** | Detection MUST complete < 10ms per event; MUST NOT block event ingestion |
| **Memory** | Detectors MUST bound state: max 1000 recent tool calls per run in memory; evict LRU |
| **Accuracy** | Loop detection false positive rate SHOULD be < 5% |
