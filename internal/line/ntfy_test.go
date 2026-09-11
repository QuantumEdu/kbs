package line

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPNotifierPostsTopic(t *testing.T) {
	t.Parallel()
	var gotPath, gotTitle, gotClick, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTitle = r.Header.Get("Title")
		gotClick = r.Header.Get("Click")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	n := HTTPNotifier{Server: server.URL, Topic: "factory-test", Client: server.Client()}
	err := n.Notify(context.Background(), Notification{
		Title:   "line needs_human",
		Message: "Is the API public?",
		Click:   "https://github.com/o/r/issues/1",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if gotPath != "/factory-test" || gotTitle != "line needs_human" {
		t.Fatalf("path=%s title=%s", gotPath, gotTitle)
	}
	if gotClick != "https://github.com/o/r/issues/1" || gotBody != "Is the API public?" {
		t.Fatalf("click=%s body=%s", gotClick, gotBody)
	}
}

func TestHTTPNotifierNoTopicIsNoop(t *testing.T) {
	t.Parallel()
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	t.Cleanup(server.Close)
	n := HTTPNotifier{Server: server.URL, Client: server.Client()}
	if err := n.Notify(context.Background(), Notification{Message: "x"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if called {
		t.Fatal("expected no request without topic")
	}
}

func TestHTTPNotifierHTTPError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)
	n := HTTPNotifier{Server: server.URL, Topic: "t", Client: server.Client()}
	if err := n.Notify(context.Background(), Notification{Message: "x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNotificationActions(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:       "job-123",
		IssueURL: "https://github.com/o/r/issues/77",
		Status:   StatusNeedsHuman,
		Question: "Confirm scope?",
	}
	note, ok := notificationFor(job)
	if !ok {
		t.Fatal("expected notification for needs_human")
	}
	if note.Actions == "" {
		t.Fatal("expected non-empty Actions for needs_human")
	}

	var gotActions string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotActions = r.Header.Get("Actions")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	n := HTTPNotifier{Server: server.URL, Topic: "actions-test", Client: server.Client()}
	if err := n.Notify(context.Background(), note); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if gotActions != note.Actions {
		t.Fatalf("got Actions header %q, want %q", gotActions, note.Actions)
	}
}
