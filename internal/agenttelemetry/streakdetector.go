package agenttelemetry

import "sync"

// StreakDetector detects unproductive streaks of consecutive failures or no-change steps.
type StreakDetector struct {
	mu             sync.Mutex
	maxFails       int
	maxNoChange    int
	failStreak     map[string]int // runID → consecutive fails
	noChangeStreak map[string]int // runID → consecutive no-file-change
}

// StreakSignal is emitted when an unproductive streak reaches threshold.
type StreakSignal struct {
	RunID string `json:"run_id"`
	Type  string `json:"type"` // "fail-streak" or "no-change-streak"
	Count int    `json:"count"`
}

// NewStreakDetector creates a StreakDetector with defaults: max 5 consecutive fails, 3 no-change.
func NewStreakDetector() *StreakDetector {
	return &StreakDetector{
		maxFails:       5,
		maxNoChange:    3,
		failStreak:     make(map[string]int),
		noChangeStreak: make(map[string]int),
	}
}

// RecordFail increments the fail streak for a run.
// Returns StreakSignal if the fail streak reaches the threshold (>= 5).
func (sd *StreakDetector) RecordFail(runID string) *StreakSignal {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.failStreak[runID]++
	count := sd.failStreak[runID]

	if count >= sd.maxFails {
		return &StreakSignal{
			RunID: runID,
			Type:  "fail-streak",
			Count: count,
		}
	}
	return nil
}

// RecordNoChange increments the no-change streak.
// Returns StreakSignal if the no-change streak reaches the threshold (>= 3).
func (sd *StreakDetector) RecordNoChange(runID string) *StreakSignal {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.noChangeStreak[runID]++
	count := sd.noChangeStreak[runID]

	if count >= sd.maxNoChange {
		return &StreakSignal{
			RunID: runID,
			Type:  "no-change-streak",
			Count: count,
		}
	}
	return nil
}

// RecordSuccess resets both streaks for a run.
func (sd *StreakDetector) RecordSuccess(runID string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	delete(sd.failStreak, runID)
	delete(sd.noChangeStreak, runID)
}
