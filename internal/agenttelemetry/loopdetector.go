package agenttelemetry

import (
	"sync"
	"time"
)

// LoopDetector detects repeated identical tool calls within a time window.
type LoopDetector struct {
	mu       sync.Mutex
	window   time.Duration
	maxSlots int
	calls    map[string][]time.Time // args_hash → timestamps
}

// LoopSignal is emitted when a loop is detected.
type LoopSignal struct {
	ArgsHash    string    `json:"args_hash"`
	ToolName    string    `json:"tool_name"`
	Count       int       `json:"count"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

// NewLoopDetector creates a LoopDetector with a 60s window and 1000 max slots.
func NewLoopDetector() *LoopDetector {
	return &LoopDetector{
		window:   60 * time.Second,
		maxSlots: 1000,
		calls:    make(map[string][]time.Time),
	}
}

// Check records a tool call and returns a LoopSignal if a loop is detected.
// Loop detection: 3+ identical args_hash within the 60s window.
func (ld *LoopDetector) Check(call ToolCallRecord) *LoopSignal {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	now := time.Now()
	key := call.ArgsHash

	timestamps, exists := ld.calls[key]

	// Evict old timestamps outside the window.
	cutoff := now.Add(-ld.window)
	valid := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	valid = append(valid, now)
	ld.calls[key] = valid

	// LRU eviction: if over maxSlots and this is a new key, evict oldest.
	if !exists && len(ld.calls) > ld.maxSlots {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range ld.calls {
			latest := v[len(v)-1]
			if oldestKey == "" || latest.Before(oldestTime) {
				oldestKey = k
				oldestTime = latest
			}
		}
		delete(ld.calls, oldestKey)
	}

	if len(valid) >= 3 {
		return &LoopSignal{
			ArgsHash:    call.ArgsHash,
			ToolName:    call.ToolName,
			Count:       len(valid),
			WindowStart: valid[0],
			WindowEnd:   now,
		}
	}

	return nil
}
