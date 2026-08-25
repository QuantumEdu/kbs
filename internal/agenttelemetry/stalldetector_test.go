package agenttelemetry

import (
	"testing"
	"time"
)

func TestStallDetectorInactivityViolation(t *testing.T) {
	sd := NewStallDetector()
	sd.threshold = 50 * time.Millisecond // Short threshold for testing.
	runID := "run-001"

	// Record an event.
	sd.Record(runID, time.Now())

	// Immediately after, no violation.
	sig := sd.Check(runID)
	if sig != nil {
		t.Fatal("expected no stall immediately after recording")
	}

	// Wait past threshold.
	time.Sleep(60 * time.Millisecond)

	sig = sd.Check(runID)
	if sig == nil {
		t.Fatal("expected StallSignal after inactivity > threshold")
	}
	if sig.RunID != runID {
		t.Errorf("RunID = %q, want %q", sig.RunID, runID)
	}
	if sig.InactiveSince < sd.threshold {
		t.Errorf("InactiveSince = %v, want >= %v", sig.InactiveSince, sd.threshold)
	}
}

func TestStallDetectorRecentActivity(t *testing.T) {
	sd := NewStallDetector()
	sd.threshold = 100 * time.Millisecond
	runID := "run-001"

	// Record an event.
	sd.Record(runID, time.Now())

	// Wait some time, but less than threshold.
	time.Sleep(30 * time.Millisecond)

	// Record another event (activity).
	sd.Record(runID, time.Now())

	sig := sd.Check(runID)
	if sig != nil {
		t.Fatal("expected no stall with recent activity")
	}
}

func TestStallDetectorUnknownRun(t *testing.T) {
	sd := NewStallDetector()

	sig := sd.Check("nonexistent-run")
	if sig != nil {
		t.Fatal("expected no stall for unrecorded run")
	}
}

func TestStallDetectorRecordAndCheck(t *testing.T) {
	cases := []struct {
		name        string
		threshold   time.Duration
		steps       []time.Duration // event offsets from t0 for one run
		wantSignals []bool          // whether each event must yield a signal
	}{
		{
			name:        "first ever event never signals",
			threshold:   60 * time.Second,
			steps:       []time.Duration{0},
			wantSignals: []bool{false},
		},
		{
			name:        "events ten seconds apart stay quiet",
			threshold:   60 * time.Second,
			steps:       []time.Duration{0, 10 * time.Second},
			wantSignals: []bool{false, false},
		},
		{
			name:        "gap past threshold signals once then resets",
			threshold:   60 * time.Second,
			steps:       []time.Duration{0, 61 * time.Second, 62 * time.Second},
			wantSignals: []bool{false, true, false},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sd := NewStallDetector()
			sd.threshold = tt.threshold
			runID := "run-rac"
			t0 := time.Now()

			for i, step := range tt.steps {
				sig := sd.RecordAndCheck(runID, t0.Add(step))
				if got := sig != nil; got != tt.wantSignals[i] {
					t.Fatalf("event %d: signal = %v, want %v (sig=%v)", i, got, tt.wantSignals[i], sig)
				}
				if sig == nil {
					continue
				}
				prev := t0.Add(tt.steps[i-1])
				if !sig.LastEvent.Equal(prev) {
					t.Errorf("event %d: LastEvent = %v, want %v", i, sig.LastEvent, prev)
				}
				if want := tt.steps[i] - tt.steps[i-1]; sig.InactiveSince != want {
					t.Errorf("event %d: InactiveSince = %v, want %v", i, sig.InactiveSince, want)
				}
			}
		})
	}
}

func TestStallDetectorForgetRemovesEntry(t *testing.T) {
	sd := NewStallDetector()
	runID := "run-fg"
	t0 := time.Now()

	// Prime the tracker and cross the threshold so a signal exists pre-Forget.
	if sig := sd.RecordAndCheck(runID, t0); sig != nil {
		t.Fatalf("first event: unexpected signal %v", sig)
	}
	if sig := sd.RecordAndCheck(runID, t0.Add(61*time.Second)); sig == nil {
		t.Fatal("second event: expected stall signal past threshold")
	}

	sd.Forget(runID)

	// The map no longer contains the run.
	if got := sd.size(); got != 0 {
		t.Fatalf("sd.size() = %d, want 0 after Forget", got)
	}

	// The entry is gone: the next event is treated as first-ever. (Note this
	// re-adds the run to the tracker — RecordAndCheck always records.)
	if sig := sd.RecordAndCheck(runID, t0.Add(120*time.Second)); sig != nil {
		t.Fatalf("post-forget event: unexpected signal %v", sig)
	}

	// Forgetting an unknown run is a no-op.
	before := sd.size()
	sd.Forget("never-seen")
	if got := sd.size(); got != before {
		t.Errorf("sd.size() = %d after unknown-run Forget, want %d", got, before)
	}
}
