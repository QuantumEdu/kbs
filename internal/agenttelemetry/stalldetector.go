package agenttelemetry

import (
	"sync"
	"time"
)

// StallDetector checks for wall-clock inactivity on active runs.
type StallDetector struct {
	mu        sync.Mutex
	threshold time.Duration
	lastEvent map[string]time.Time // runID → last event timestamp
}

// StallSignal is emitted when a run has been inactive past the threshold.
type StallSignal struct {
	RunID          string        `json:"run_id"`
	LastEvent      time.Time     `json:"last_event"`
	InactiveSince  time.Duration `json:"inactive_seconds"`
}

// NewStallDetector creates a StallDetector with a 60s threshold.
func NewStallDetector() *StallDetector {
	return &StallDetector{
		threshold: 60 * time.Second,
		lastEvent: make(map[string]time.Time),
	}
}

// Record updates the last activity timestamp for a run.
func (sd *StallDetector) Record(runID string, t time.Time) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.lastEvent[runID] = t
}

// Check returns a StallSignal if a run has been inactive past the threshold.
// Returns nil if the run has recent activity or has never been recorded.
func (sd *StallDetector) Check(runID string) *StallSignal {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	last, exists := sd.lastEvent[runID]
	if !exists {
		return nil
	}

	inactiveFor := time.Since(last)
	if inactiveFor > sd.threshold {
		return &StallSignal{
			RunID:         runID,
			LastEvent:     last,
			InactiveSince: inactiveFor,
		}
	}

	return nil
}
