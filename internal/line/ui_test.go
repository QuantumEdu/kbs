package line

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestUIListsJobsAndContinues(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := openTestStore(t)
	exec := &scriptedExecutor{replies: []string{
		`{"status":"needs_human","question":"Public?"}`,
		`{"status":"ok","summary":"public"}`,
	}}
	engine := NewEngine(store, exec, "")
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/30", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	server := httptest.NewServer(NewUI(engine))
	t.Cleanup(server.Close)

	home, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer home.Body.Close()
	body, _ := io.ReadAll(home.Body)
	if home.StatusCode != 200 || !strings.Contains(string(body), job.ID) {
		t.Fatalf("home %d %s", home.StatusCode, body)
	}

	detail, err := http.Get(server.URL + "/jobs/" + job.ID)
	if err != nil {
		t.Fatalf("GET job: %v", err)
	}
	defer detail.Body.Close()
	page, _ := io.ReadAll(detail.Body)
	if !strings.Contains(string(page), "Public?") || !strings.Contains(string(page), `name="answer"`) {
		t.Fatalf("detail %s", page)
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.PostForm(server.URL+"/jobs/"+job.ID+"/continue", url.Values{"answer": {"yes"}})
	if err != nil {
		t.Fatalf("POST continue: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("continue status %d", resp.StatusCode)
	}
	got, _, err := engine.Status(ctx, job.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.Status != StatusAwaitingNext {
		t.Fatalf("after continue %+v", got)
	}
}

func TestUIAdvance(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"spec"}`,
		`{"status":"ok","summary":"built"}`,
	}}
	engine := NewEngine(openTestStore(t), exec, "")
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/31", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	server := httptest.NewServer(NewUI(engine))
	t.Cleanup(server.Close)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.PostForm(server.URL+"/jobs/"+job.ID+"/advance", url.Values{})
	if err != nil {
		t.Fatalf("POST advance: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", resp.StatusCode)
	}
	got, _, err := engine.Status(ctx, job.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.Phase != PhaseReview || got.Status != StatusAwaitingNext {
		t.Fatalf("got %+v", got)
	}
}
