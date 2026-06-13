package security

import "testing"

func TestScanOpenAIKey(t *testing.T) {
	s := New()
	content := "sk-abc" + "123XYZ_-abc123XYZ_-abc123XYZ_-abc12"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}
	if result.Matches[0].Type != "openai_api_key" {
		t.Errorf("Type=%q, want openai_api_key", result.Matches[0].Type)
	}
}

func TestScanPrivateKeyRSA(t *testing.T) {
	s := New()
	content := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
	if result.Matches[0].Type != "private_key" {
		t.Errorf("Type=%q, want private_key", result.Matches[0].Type)
	}
}

func TestScanPrivateKeyEC(t *testing.T) {
	s := New()
	content := "-----BEGIN EC PRIVATE KEY-----\nMHQCAQEEIIm3V"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
}

func TestScanPrivateKeyOpenSSH(t *testing.T) {
	s := New()
	content := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
}

func TestScanPrivateKeyGeneric(t *testing.T) {
	s := New()
	content := "-----BEGIN PRIVATE KEY-----\nMIICdgIBADANBgkqhkiG"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
}

func TestScanGitHubToken(t *testing.T) {
	s := New()
	content := "ghp_abc" + "123XYZ_abc123XYZ_abc123XYZ_abc1"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
	if result.Matches[0].Type != "github_token" {
		t.Errorf("Type=%q, want github_token", result.Matches[0].Type)
	}
}

func TestScanSlackTokenBot(t *testing.T) {
	s := New()
	content := "xoxb" + "-1234567890-1234567890-abcdefghijklmnopqrst"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
	if result.Matches[0].Type != "slack_token" {
		t.Errorf("Type=%q, want slack_token", result.Matches[0].Type)
	}
}

func TestScanSlackTokenApp(t *testing.T) {
	s := New()
	content := "xoxa" + "-1234567890-1234567890-abcdefghijklmnopqrst"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
}

func TestScanSlackTokenUser(t *testing.T) {
	s := New()
	content := "xoxp" + "-1234567890-1234567890-abcdefghijklmnopqrst"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
}

func TestScanSlackTokenWebhook(t *testing.T) {
	s := New()
	content := "xoxr" + "-1234567890-1234567890-abcdefghijklmnopqrst"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
}

func TestScanCleanContent(t *testing.T) {
	s := New()
	content := "This is a perfectly safe prompt for a coding agent."
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasSecret {
		t.Fatalf("expected HasSecret=false, got matches: %+v", result.Matches)
	}
	if len(result.Matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(result.Matches))
	}
}

func TestScanEmptyContent(t *testing.T) {
	s := New()
	result, err := s.Scan("")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasSecret {
		t.Fatal("expected HasSecret=false for empty content")
	}
}

func TestScanMultipleSecrets(t *testing.T) {
	s := New()
	content := "key=sk-abc" + "123XYZ_-abc123XYZ_-abc123XYZ_-abc12 token=ghp_abc" + "123XYZ_abc123XYZ_abc123XYZ_abc1"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSecret {
		t.Fatal("expected HasSecret=true")
	}
	if len(result.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(result.Matches))
	}
}

func TestRedactCleanContent(t *testing.T) {
	s := New()
	content := "safe content"
	redacted, matches := s.Redact(content)
	if redacted != content {
		t.Errorf("expected unchanged, got %q", redacted)
	}
	if matches != nil {
		t.Fatalf("expected nil matches, got %+v", matches)
	}
}

func TestRedactOpenAIKey(t *testing.T) {
	s := New()
	content := "key is sk-abc" + "123XYZ_-abc123XYZ_-abc123XYZ_-abc12 here"
	redacted, matches := s.Redact(content)
	if redacted != "key is [REDACTED] here" {
		t.Errorf("got %q", redacted)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Type != "openai_api_key" {
		t.Errorf("Type=%q, want openai_api_key", matches[0].Type)
	}
}

func TestRedactMultipleSecrets(t *testing.T) {
	s := New()
	content := "sk-abc" + "123XYZ_-abc123XYZ_-abc123XYZ_-abc12 and ghp_abc" + "123XYZ_abc123XYZ_abc123XYZ_abc1"
	redacted, matches := s.Redact(content)
	expected := "[REDACTED] and [REDACTED]"
	if redacted != expected {
		t.Errorf("got %q, want %q", redacted, expected)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestScanPositionTracking(t *testing.T) {
	s := New()
	content := "before sk-abc" + "123XYZ_-abc123XYZ_-abc123XYZ_-abc12 after"
	result, err := s.Scan(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}
	m := result.Matches[0]
	if m.Start != 7 {
		t.Errorf("Start=%d, want 7", m.Start)
	}
	wantEnd := m.Start + len(m.Pattern)
	if m.End != wantEnd {
		t.Errorf("End=%d, want %d", m.End, wantEnd)
	}
	if content[m.Start:m.End] != m.Pattern {
		t.Errorf("pattern slice mismatch: got %q from slice, m.Pattern=%q", content[m.Start:m.End], m.Pattern)
	}
}
