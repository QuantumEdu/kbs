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
