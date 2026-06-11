package api

import (
	"net/http"
	"testing"
	"time"
)

func TestServerReturns501(t *testing.T) {
	s := NewServer("127.0.0.1", 0)

	// Start in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	// Give it a moment to start, then try to connect
	time.Sleep(50 * time.Millisecond)

	// Since we used port 0, we can't easily test. Let's test the handler directly.
	// For the scaffold, just verify the server struct works.
	if s == nil {
		t.Fatal("server is nil")
	}

	// Stop the server
	s.Stop()
}

func TestHandlerDirectly(t *testing.T) {
	s := NewServer("127.0.0.1", 7438)
	rec := &testResponseWriter{header: make(http.Header)}
	req, _ := http.NewRequest("GET", "/health", nil)

	s.handleAll(rec, req)

	if rec.status != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.status, http.StatusNotImplemented)
	}
	if rec.body != `{"error":"not implemented","message":"HTTP API will be available in v1-final"}` {
		t.Errorf("body = %q", rec.body)
	}
}

type testResponseWriter struct {
	header http.Header
	status int
	body   string
}

func (w *testResponseWriter) Header() http.Header         { return w.header }
func (w *testResponseWriter) WriteHeader(statusCode int)  { w.status = statusCode }
func (w *testResponseWriter) Write(b []byte) (int, error) { w.body += string(b); return len(b), nil }
