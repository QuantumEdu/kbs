package sync

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// s3Client is the subset of *minio.Client used by S3Transport.
// Exists as an interface so tests can supply a mock.
type s3Client interface {
	PutObject(ctx context.Context, bucket, object string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (io.ReadCloser, error)
}

// minioAdapter makes *minio.Client satisfy s3Client.
type minioAdapter struct{ *minio.Client }

func (a *minioAdapter) GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
	return a.Client.GetObject(ctx, bucket, object, opts)
}

// S3Transport uploads to and downloads from an S3-compatible object store.
type S3Transport struct {
	client s3Client
	bucket string
}

// NewS3Transport creates an S3Transport validated against cfg.
// Supports any S3-compatible endpoint (AWS, MinIO, R2, B2, etc.).
func NewS3Transport(cfg *S3Config) (*S3Transport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("s3 config is nil")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}

	endpoint, secure := parseEndpoint(cfg.Endpoint)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	return &S3Transport{
		client: &minioAdapter{client},
		bucket: cfg.Bucket,
	}, nil
}

// Push uploads reader to the S3 bucket under the given key with content-type application/gzip.
func (s *S3Transport) Push(ctx context.Context, reader io.Reader, key string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, -1, minio.PutObjectOptions{
		ContentType: "application/gzip",
	})
	if err != nil {
		return fmt.Errorf("s3 put object %q: %w", key, err)
	}
	return nil
}

// Pull downloads the object identified by key from the S3 bucket and writes its contents to writer.
func (s *S3Transport) Pull(ctx context.Context, writer io.Writer, key string) error {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("s3 get object %q: %w", key, err)
	}
	defer obj.Close()

	if _, err := io.Copy(writer, obj); err != nil {
		return fmt.Errorf("s3 read object %q: %w", key, err)
	}
	return nil
}

// parseEndpoint splits an endpoint URL into the host part and whether TLS should be used.
//   - "https://s3.amazonaws.com" → "s3.amazonaws.com", true
//   - "http://localhost:9000"    → "localhost:9000", false
//   - "play.min.io"              → "play.min.io", true (default secure)
func parseEndpoint(raw string) (endpoint string, secure bool) {
	switch {
	case strings.HasPrefix(raw, "https://"):
		return strings.TrimPrefix(raw, "https://"), true
	case strings.HasPrefix(raw, "http://"):
		return strings.TrimPrefix(raw, "http://"), false
	default:
		return raw, true
	}
}
