// Package sync defines pluggable transport backends for vault snapshot upload/download.
package sync

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
)

// Transport is the interface for pushing/pulling byte streams to/from a remote.
type Transport interface {
	// Push uploads the contents of reader to the remote identified by key.
	Push(ctx context.Context, reader io.Reader, key string) error
	// Pull downloads the remote identified by key and writes its contents to writer.
	Pull(ctx context.Context, writer io.Writer, key string) error
}

// GzipTransport wraps a Transport and applies gzip compression on push
// and decompression on pull.
type GzipTransport struct {
	inner Transport
}

// NewGzipTransport creates a gzip-decorated transport.
func NewGzipTransport(inner Transport) *GzipTransport {
	return &GzipTransport{inner: inner}
}

// Push compresses reader through gzip and pushes the compressed stream via the inner transport.
func (g *GzipTransport) Push(ctx context.Context, reader io.Reader, key string) error {
	pr, pw := io.Pipe()
	go func() {
		gw := gzip.NewWriter(pw)
		if _, err := io.Copy(gw, reader); err != nil {
			gw.Close()
			pw.CloseWithError(fmt.Errorf("gzip compress: %w", err))
			return
		}
		if err := gw.Close(); err != nil {
			pw.CloseWithError(fmt.Errorf("gzip close: %w", err))
			return
		}
		pw.Close()
	}()
	return g.inner.Push(ctx, pr, key)
}

// Pull downloads the compressed stream via the inner transport, decompresses it,
// and writes the result to writer.
func (g *GzipTransport) Pull(ctx context.Context, writer io.Writer, key string) error {
	pr, pw := io.Pipe()
	go func() {
		err := g.inner.Pull(ctx, pw, key)
		pw.CloseWithError(err)
	}()
	gr, err := gzip.NewReader(pr)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()
	if _, err := io.Copy(writer, gr); err != nil {
		return fmt.Errorf("gzip decompress: %w", err)
	}
	return nil
}
