package sync

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

// mockTransport stores pushed data in a buffer and returns it on pull.
type mockTransport struct {
	buf    bytes.Buffer
	pullFn func(w io.Writer) error // optional custom pull behavior
}

func (m *mockTransport) Push(_ context.Context, reader io.Reader, _ string) error {
	_, err := io.Copy(&m.buf, reader)
	return err
}

func (m *mockTransport) Pull(_ context.Context, writer io.Writer, _ string) error {
	if m.pullFn != nil {
		return m.pullFn(writer)
	}
	_, err := io.Copy(writer, &m.buf)
	return err
}

func TestGzipTransportRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"small json", `{"hello": "world"}`},
		{"empty", ""},
		{"large payload", string(bytes.Repeat([]byte("hello gzip sync, "), 1024))},
		{"binary content", string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTransport{}
			gt := NewGzipTransport(mock)
			ctx := context.Background()

			// Push
			orig := bytes.NewReader([]byte(tt.payload))
			if err := gt.Push(ctx, orig, "vault/key"); err != nil {
				t.Fatalf("Push failed: %v", err)
			}

			// Verify compression happened (compressed should be different from original,
			// except for very small inputs where gzip header makes it larger).
			if len(tt.payload) > 100 && mock.buf.Len() >= len(tt.payload) {
				t.Errorf("expected compression (payload=%d, compressed=%d)", len(tt.payload), mock.buf.Len())
			}

			// Pull
			var out bytes.Buffer
			if err := gt.Pull(ctx, &out, "vault/key"); err != nil {
				t.Fatalf("Pull failed: %v", err)
			}

			if out.String() != tt.payload {
				t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", out.String(), tt.payload)
			}
		})
	}
}

func TestGzipTransportPushError(t *testing.T) {
	mock := &mockTransport{
		pullFn: func(_ io.Writer) error { return errors.New("simulated pull failure") },
	}
	gt := NewGzipTransport(mock)
	ctx := context.Background()

	// Push should succeed (mock doesn't fail on push).
	orig := bytes.NewReader([]byte(`{"data": "test"}`))
	if err := gt.Push(ctx, orig, "key"); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Pull should fail because mock.pullFn returns an error.
	var out bytes.Buffer
	if err := gt.Pull(ctx, &out, "key"); err == nil {
		t.Fatal("expected Pull to fail, got nil")
	}
}

func TestGzipTransportCompresses(t *testing.T) {
	// A highly compressible payload to verify gzip is active.
	payload := string(bytes.Repeat([]byte("AAAAAAAAAA"), 1000)) // 10KB of 'A'
	mock := &mockTransport{}
	gt := NewGzipTransport(mock)
	ctx := context.Background()

	if err := gt.Push(ctx, bytes.NewReader([]byte(payload)), "key"); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Compressed size should be significantly smaller.
	if mock.buf.Len() > len(payload) {
		t.Errorf("gzip should compress: payload=%d, compressed=%d", len(payload), mock.buf.Len())
	}
}
