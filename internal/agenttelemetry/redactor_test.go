package agenttelemetry

import (
	"strings"
	"testing"
)

func TestRedactorOpenAIKey(t *testing.T) {
	r, errs := NewRedactor(nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	result := r.Redact("API key: sk-proj-abc123def456ghi789jkl012mno345pqr")
	if strings.Contains(result, "sk-proj-abc123def456ghi789jkl012mno345pqr") {
		t.Error("OpenAI key was not redacted")
	}
	if !strings.Contains(result, "sk-***REDACTED***") {
		t.Errorf("expected sk-***REDACTED***, got %q", result)
	}
}

func TestRedactorBearerToken(t *testing.T) {
	r, errs := NewRedactor(nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Standalone Bearer token (no Authorization: prefix to avoid Auth header overlap).
	result := r.Redact("token: Bearer eyJhbGciOiJIUzI1NiJ9.xyz")
	if strings.Contains(result, "eyJhbGci") {
		t.Error("Bearer token was not redacted")
	}
	if !strings.Contains(result, "Bearer ***REDACTED***") {
		t.Errorf("expected Bearer ***REDACTED***, got %q", result)
	}
}

func TestRedactorAuthHeader(t *testing.T) {
	r, errs := NewRedactor(nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	result := r.Redact("Authorization: Basic dXNlcjpwYXNz")
	if strings.Contains(result, "dXNlcjpwYXNz") {
		t.Error("Authorization header was not redacted")
	}
	if !strings.Contains(result, "Authorization: ***REDACTED***") {
		t.Errorf("expected Authorization: ***REDACTED***, got %q", result)
	}
}

func TestRedactorAPIKeyFlag(t *testing.T) {
	r, errs := NewRedactor(nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	result := r.Redact("some-tool --api-key secret123 --verbose")
	if strings.Contains(result, "secret123") {
		t.Error("--api-key value was not redacted")
	}
	if !strings.Contains(result, "--api-key ***REDACTED***") {
		t.Errorf("expected --api-key ***REDACTED***, got %q", result)
	}
}

func TestRedactorCustomPattern(t *testing.T) {
	r, errs := NewRedactor([]string{`ghp_[a-zA-Z0-9]{36}`})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	result := r.Redact("token: ghp_abcdefghijklmnopqrstuvwxyz1234567890")
	if strings.Contains(result, "ghp_abcdefghijklmnopqrstuvwxyz1234567890") {
		t.Error("custom pattern did not redact GitHub token")
	}
}

func TestRedactorInvalidCustomPattern(t *testing.T) {
	_, errs := NewRedactor([]string{`[invalid`})
	if len(errs) == 0 {
		t.Error("expected compile error for invalid pattern, got none")
	}
}

func TestRedactorMultipleRedactions(t *testing.T) {
	r, errs := NewRedactor(nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	input := "key: sk-abc123def456ghi789jkl012mno345pqr and token: Bearer xyz.example.token"
	result := r.Redact(input)

	if strings.Contains(result, "sk-abc123def456ghi789jkl012mno345pqr") {
		t.Error("OpenAI key was not redacted")
	}
	if strings.Contains(result, "xyz.example.token") {
		t.Error("Bearer token was not redacted")
	}
	if !strings.Contains(result, "Bearer ***REDACTED***") {
		t.Errorf("expected Bearer ***REDACTED***, got %q", result)
	}
}

func TestRedactorNoMatchUnchanged(t *testing.T) {
	r, errs := NewRedactor(nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	input := "hello world, nothing to redact here"
	result := r.Redact(input)
	if result != input {
		t.Errorf("unexpected redaction on clean input: %q", result)
	}
}

func TestRedactorPatterns(t *testing.T) {
	r, errs := NewRedactor([]string{`custom-\d+`})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	patterns := r.Patterns()
	if len(patterns) != 5 {
		t.Errorf("Patterns count = %d, want 5 (4 built-in + 1 custom)", len(patterns))
	}
}
