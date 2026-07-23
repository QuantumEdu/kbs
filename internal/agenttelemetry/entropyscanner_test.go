package agenttelemetry

import "testing"

func TestEntropyScannerDetectsHighEntropy(t *testing.T) {
	s := NewEntropyScanner()

	// "dGhpc2lzYXRlc3RzZWNyZXRrZXk=" = base64 of "thisisatestsecretkey" — 28 chars.
	if !s.Scan("dGhpc2lzYXRlc3RzZWNyZXRrZXk=") {
		t.Error("expected high-entropy detection for base64 string")
	}
}

func TestEntropyScannerIgnoresShort(t *testing.T) {
	s := NewEntropyScanner()

	if s.Scan("abc123") {
		t.Error("short string should not trigger entropy scan")
	}
}

func TestEntropyScannerIgnoresLowEntropy(t *testing.T) {
	s := NewEntropyScanner()

	if s.Scan("hello world this is a normal sentence with spaces and punctuation!") {
		t.Error("normal text should not trigger entropy scan")
	}
}

func TestEntropyScannerDetectsInContext(t *testing.T) {
	s := NewEntropyScanner()

	input := `{"key": "dGhpc2lzYXRlc3RzZWNyZXRrZXk=", "type": "secret"}`
	if !s.Scan(input) {
		t.Error("should detect high-entropy base64 string in JSON context")
	}
}

func TestEntropyScannerLongNormalText(t *testing.T) {
	s := NewEntropyScanner()

	long := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
	if s.Scan(long) {
		t.Error("long normal text should not trigger entropy scan")
	}
}

func TestEntropyScannerCustomThresholds(t *testing.T) {
	s := &EntropyScanner{minLength: 5, minRatio: 0.5}

	// Short string (8 chars) with high ratio — should flag with custom thresholds.
	if !s.Scan("abcDEF12") {
		t.Error("custom thresholds should flag short high-entropy string")
	}
}
