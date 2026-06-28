package sync

import (
	"bytes"
	"strings"
	"testing"
)

func TestSanitizingWriter_MasksCredentials(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, output string)
	}{
		{
			name:  "aws access key",
			input: "aws_access_key_id=AKIA1234567890ABCDEF\n",
			check: func(t *testing.T, output string) {
				if strings.Contains(output, "AKIA1234567890ABCDEF") {
					t.Error("access key should be redacted")
				}
				if !strings.Contains(output, "***REDACTED***") {
					t.Error("output should contain REDACTED marker")
				}
				if !strings.Contains(output, "aws_access_key_id=") {
					t.Error("key name should be preserved")
				}
			},
		},
		{
			name:  "secret key",
			input: "secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\n",
			check: func(t *testing.T, output string) {
				if strings.Contains(output, "wJalrXUtnFEMI") {
					t.Error("secret key should be redacted")
				}
			},
		},
		{
			name:  "github token",
			input: "GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrstuv\n",
			check: func(t *testing.T, output string) {
				if strings.Contains(output, "ghp_1234567890") {
					t.Error("github token should be redacted")
				}
			},
		},
		{
			name:  "token with colon",
			input: "x-api-key: abc-secret-123456\n",
			check: func(t *testing.T, output string) {
				if strings.Contains(output, "abc-secret-123456") {
					t.Error("API key should be redacted")
				}
			},
		},
		{
			name:  "multiple credentials in one write",
			input: "aws_access_key_id=AKIA1234 secret=supersecret token=abc123\n",
			check: func(t *testing.T, output string) {
				if strings.Contains(output, "AKIA1234") {
					t.Error("access key should be redacted")
				}
				if strings.Contains(output, "supersecret") {
					t.Error("secret value should be redacted")
				}
				if strings.Contains(output, "abc123") {
					t.Error("token value should be redacted")
				}
			},
		},
		{
			name:  "safe content unchanged",
			input: "name=myapp version=1.0.0 status=active\n",
			check: func(t *testing.T, output string) {
				if output != "name=myapp version=1.0.0 status=active\n" {
					t.Errorf("safe content should be unchanged, got %q", output)
				}
			},
		},
		{
			name:  "json with credentials",
			input: `{"access_key_id":"AKIA123456","region":"us-east-1","endpoint":"https://s3.example.com"}`,
			check: func(t *testing.T, output string) {
				if strings.Contains(output, "AKIA123456") {
					t.Error("access key in JSON should be redacted")
				}
				if !strings.Contains(output, "us-east-1") {
					t.Error("safe value (region) should be preserved")
				}
				if !strings.Contains(output, "https://s3.example.com") {
					t.Error("safe value (endpoint) should be preserved")
				}
			},
		},
		{
			name:  "empty input",
			input: "",
			check: func(t *testing.T, output string) {
				if output != "" {
					t.Errorf("empty input should produce empty output, got %q", output)
				}
			},
		},
		{
			name:  "short value is redacted too",
			input: "key=se",
			check: func(t *testing.T, output string) {
				// Even short values after key= should be redacted.
				if strings.Contains(output, "key=se") && !strings.Contains(output, "***REDACTED***") {
					t.Errorf("key=se should be redacted, got %q", output)
				}
			},
		},
		{
			name:  "key in middle of word",
			input: "api_key_id=abcdef\n",
			check: func(t *testing.T, output string) {
				if strings.Contains(output, "abcdef") {
					t.Error("api_key_id value should be redacted")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			sw := NewSanitizingWriter(&buf)

			n, err := sw.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write returned n=%d, want %d", n, len(tt.input))
			}

			tt.check(t, buf.String())
		})
	}
}

func TestSanitizingWriter_ReturnsOriginalLength(t *testing.T) {
	var buf bytes.Buffer
	sw := NewSanitizingWriter(&buf)

	input := []byte("secret=myrealvalue")
	n, err := sw.Write(input)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(input) {
		t.Errorf("Write should return original length %d, got %d", len(input), n)
	}
	// Value should be redacted (different from original).
	if buf.String() == string(input) {
		t.Errorf("sanitized output should be different from input, got identical")
	}
	if !strings.Contains(buf.String(), "***REDACTED***") {
		t.Errorf("sanitized output should contain REDACTED marker, got %q", buf.String())
	}
}
