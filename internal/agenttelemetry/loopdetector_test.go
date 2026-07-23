package agenttelemetry

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLoopDetectorThreeIdentical(t *testing.T) {
	ld := NewLoopDetector()
	hash := "abc123"
	tool := "bash"

	// Two identical calls — no detection yet.
	for i := 0; i < 2; i++ {
		sig := ld.Check(ToolCallRecord{ArgsHash: hash, ToolName: tool})
		if sig != nil {
			t.Fatalf("unexpected signal after %d calls", i+1)
		}
	}

	// Third identical call — should detect.
	sig := ld.Check(ToolCallRecord{ArgsHash: hash, ToolName: tool})
	if sig == nil {
		t.Fatal("expected LoopSignal after 3 identical calls, got nil")
	}
	if sig.ArgsHash != hash {
		t.Errorf("ArgsHash = %q, want %q", sig.ArgsHash, hash)
	}
	if sig.ToolName != tool {
		t.Errorf("ToolName = %q, want %q", sig.ToolName, tool)
	}
	if sig.Count < 3 {
		t.Errorf("Count = %d, want >= 3", sig.Count)
	}
}

func TestLoopDetectorDifferentArgsHash(t *testing.T) {
	ld := NewLoopDetector()

	ld.Check(ToolCallRecord{ArgsHash: "aaa", ToolName: "bash"})
	ld.Check(ToolCallRecord{ArgsHash: "bbb", ToolName: "bash"})
	ld.Check(ToolCallRecord{ArgsHash: "aaa", ToolName: "bash"})
	ld.Check(ToolCallRecord{ArgsHash: "bbb", ToolName: "bash"})
	sig := ld.Check(ToolCallRecord{ArgsHash: "ccc", ToolName: "bash"})

	if sig != nil {
		t.Fatal("expected no signal with different args hashes")
	}
}

func TestLoopDetectorWindowExpiry(t *testing.T) {
	ld := NewLoopDetector()
	ld.window = 100 * time.Millisecond // Short window for testing.
	hash := "abc123"

	sig := ld.Check(ToolCallRecord{ArgsHash: hash, ToolName: "bash"})
	if sig != nil {
		t.Fatal("unexpected signal for first call")
	}

	time.Sleep(50 * time.Millisecond)
	sig = ld.Check(ToolCallRecord{ArgsHash: hash, ToolName: "bash"})
	if sig != nil {
		t.Fatal("unexpected signal for second call")
	}

	// Wait past the window so the first call expires.
	time.Sleep(60 * time.Millisecond)

	// Third call — but only two within the window.
	sig = ld.Check(ToolCallRecord{ArgsHash: hash, ToolName: "bash"})
	if sig != nil {
		t.Fatal("expected no signal after window expiry of first call")
	}
}

func TestLoopDetectorConcurrent(t *testing.T) {
	ld := NewLoopDetector()
	var wg sync.WaitGroup

	// Concurrently send 3 identical calls.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ld.Check(ToolCallRecord{ArgsHash: "concurrent", ToolName: "bash"})
		}()
	}
	wg.Wait()

	// After all goroutines finish, a 4th call should detect a loop.
	sig := ld.Check(ToolCallRecord{ArgsHash: "concurrent", ToolName: "bash"})
	if sig == nil {
		t.Fatal("expected LoopSignal after concurrent access")
	}
}

func TestLoopDetectorLRUEviction(t *testing.T) {
	ld := NewLoopDetector()
	ld.maxSlots = 3

	// Fill 4 slots to trigger eviction.
	for i := 0; i < 4; i++ {
		hash := fmt.Sprintf("hash-%d", i)
		ld.Check(ToolCallRecord{ArgsHash: hash, ToolName: "bash"})
	}

	// Only 3 entries should remain.
	ld.mu.Lock()
	slotCount := len(ld.calls)
	ld.mu.Unlock()

	if slotCount > 3 {
		t.Errorf("expected <= 3 slots after LRU eviction, got %d", slotCount)
	}
}
