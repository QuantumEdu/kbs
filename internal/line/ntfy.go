package line

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Notification struct {
	Title    string
	Message  string
	Click    string
	Priority string
	Actions  string
}

type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

type NopNotifier struct{}

func (NopNotifier) Notify(context.Context, Notification) error { return nil }

type HTTPNotifier struct {
	Server string
	Topic  string
	Client *http.Client
}

func (n HTTPNotifier) Notify(ctx context.Context, note Notification) error {
	topic := strings.TrimSpace(n.Topic)
	if topic == "" {
		return nil
	}
	if strings.Contains(topic, "/") {
		return fmt.Errorf("ntfy topic must not contain /")
	}
	server := strings.TrimSpace(n.Server)
	if server == "" {
		server = "https://ntfy.sh"
	}
	endpoint, err := url.JoinPath(server, topic)
	if err != nil {
		return fmt.Errorf("ntfy url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(note.Message))
	if err != nil {
		return fmt.Errorf("ntfy request: %w", err)
	}
	if note.Title != "" {
		req.Header.Set("Title", note.Title)
	}
	if note.Click != "" {
		req.Header.Set("Click", note.Click)
	}
	if note.Priority != "" {
		req.Header.Set("Priority", note.Priority)
	}
	if note.Actions != "" {
		req.Header.Set("Actions", note.Actions)
	}
	client := n.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy post: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy status %s", resp.Status)
	}
	return nil
}

func notificationFor(job Job) (Notification, bool) {
	switch job.Status {
	case StatusNeedsHuman:
		actions := fmt.Sprintf("view, Open Issue, %s; view, Open UI, http://127.0.0.1:7340/jobs/%s", job.IssueURL, job.ID)
		return Notification{
			Title:    "line needs_human",
			Message:  job.Question,
			Click:    job.IssueURL,
			Priority: "high",
			Actions:  actions,
		}, true
	case StatusBlocked:
		return Notification{
			Title:    "line blocked",
			Message:  job.Summary,
			Click:    job.IssueURL,
			Priority: "high",
		}, true
	case StatusReady:
		return Notification{
			Title:    "line ready",
			Message:  job.Summary,
			Click:    job.IssueURL,
			Priority: "default",
		}, true
	default:
		return Notification{}, false
	}
}
