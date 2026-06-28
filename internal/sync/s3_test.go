package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
)

// mockS3Client implements s3Client with in-memory storage for unit tests.
type mockS3Client struct {
	objects map[string][]byte // key: "bucket/object"
}

func (m *mockS3Client) PutObject(_ context.Context, bucket, object string, reader io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	key := bucket + "/" + object
	m.objects[key] = data
	return minio.UploadInfo{Size: int64(len(data))}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, bucket, object string, _ minio.GetObjectOptions) (io.ReadCloser, error) {
	key := bucket + "/" + object
	data, ok := m.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func newMockS3Transport() *S3Transport {
	return &S3Transport{
		client: &mockS3Client{objects: make(map[string][]byte)},
		bucket: "test-bucket",
	}
}

func TestS3TransportRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"small json", `{"hello": "world"}`},
		{"empty payload", ""},
		{"large payload", string(bytes.Repeat([]byte("hello s3 sync, "), 1024))},
		{"binary content", string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3 := newMockS3Transport()
			ctx := context.Background()
			key := "vault/snapshot.json.gz"

			// Push
			if err := s3.Push(ctx, bytes.NewReader([]byte(tt.payload)), key); err != nil {
				t.Fatalf("Push failed: %v", err)
			}

			// Verify data is stored.
			storageKey := "test-bucket/" + key
			if _, ok := s3.client.(*mockS3Client).objects[storageKey]; !ok {
				t.Fatal("object not stored in mock")
			}

			// Pull
			var out bytes.Buffer
			if err := s3.Pull(ctx, &out, key); err != nil {
				t.Fatalf("Pull failed: %v", err)
			}

			if out.String() != tt.payload {
				t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", out.String(), tt.payload)
			}
		})
	}
}

func TestS3TransportPullMissingObject(t *testing.T) {
	s3 := newMockS3Transport()
	ctx := context.Background()

	var out bytes.Buffer
	err := s3.Pull(ctx, &out, "missing/key")
	if err == nil {
		t.Fatal("expected error pulling missing object, got nil")
	}
}

func TestS3TransportMultipleObjects(t *testing.T) {
	s3 := newMockS3Transport()
	ctx := context.Background()

	payloads := map[string]string{
		"key-a": "data for A",
		"key-b": "data for B",
		"key-c": "data for C",
	}

	// Push all.
	for k, v := range payloads {
		if err := s3.Push(ctx, bytes.NewReader([]byte(v)), k); err != nil {
			t.Fatalf("Push(%q) failed: %v", k, err)
		}
	}

	// Pull all and verify.
	for k, want := range payloads {
		var out bytes.Buffer
		if err := s3.Pull(ctx, &out, k); err != nil {
			t.Fatalf("Pull(%q) failed: %v", k, err)
		}
		if out.String() != want {
			t.Errorf("Pull(%q): got %q, want %q", k, out.String(), want)
		}
	}
}

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		raw          string
		wantEndpoint string
		wantSecure   bool
	}{
		{"https://s3.amazonaws.com", "s3.amazonaws.com", true},
		{"https://play.min.io", "play.min.io", true},
		{"http://localhost:9000", "localhost:9000", false},
		{"http://127.0.0.1:9000", "127.0.0.1:9000", false},
		{"play.min.io", "play.min.io", true},
		{"s3.us-east-1.amazonaws.com", "s3.us-east-1.amazonaws.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			endpoint, secure := parseEndpoint(tt.raw)
			if endpoint != tt.wantEndpoint {
				t.Errorf("endpoint = %q, want %q", endpoint, tt.wantEndpoint)
			}
			if secure != tt.wantSecure {
				t.Errorf("secure = %v, want %v", secure, tt.wantSecure)
			}
		})
	}
}

func TestNewS3TransportValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *S3Config
		wantErr string
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "s3 config is nil",
		},
		{
			name:    "missing bucket",
			cfg:     &S3Config{Endpoint: "https://s3.amazonaws.com"},
			wantErr: "s3 bucket is required",
		},
		{
			name:    "missing endpoint",
			cfg:     &S3Config{Bucket: "my-bucket"},
			wantErr: "s3 endpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewS3Transport(tt.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// contains reports whether substr is within s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// newFailingS3Transport returns an S3Transport with a mock that fails on every operation.
func newFailingS3Transport(err error) *S3Transport {
	return &S3Transport{
		client: &failingS3Client{err: err},
		bucket: "test-bucket",
	}
}

type failingS3Client struct {
	err error
}

func (f *failingS3Client) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{}, f.err
}

func (f *failingS3Client) GetObject(_ context.Context, _, _ string, _ minio.GetObjectOptions) (io.ReadCloser, error) {
	return nil, f.err
}

func TestS3TransportPushError(t *testing.T) {
	wantErr := errors.New("simulated S3 outage")
	s3 := newFailingS3Transport(wantErr)
	ctx := context.Background()

	err := s3.Push(ctx, bytes.NewReader([]byte("data")), "key")
	if err == nil {
		t.Fatal("expected push error, got nil")
	}
	if !contains(err.Error(), wantErr.Error()) {
		t.Errorf("error = %q, want substring %q", err.Error(), wantErr.Error())
	}
}

func TestS3TransportPullError(t *testing.T) {
	wantErr := errors.New("simulated S3 outage")
	s3 := newFailingS3Transport(wantErr)
	ctx := context.Background()

	var out bytes.Buffer
	err := s3.Pull(ctx, &out, "key")
	if err == nil {
		t.Fatal("expected pull error, got nil")
	}
	if !contains(err.Error(), wantErr.Error()) {
		t.Errorf("error = %q, want substring %q", err.Error(), wantErr.Error())
	}
}

// Verify S3Transport satisfies Transport interface.
var _ Transport = (*S3Transport)(nil)

// Verify mockS3Client satisfies s3Client interface.
var _ s3Client = (*mockS3Client)(nil)

// Avoid unused import for fmt when not used in production tests.
var _ = fmt.Sprintf
