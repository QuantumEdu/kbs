package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-github/v69/github"
)

// mockGitHubReposAPI implements githubReposAPI with in-memory storage for unit tests.
type mockGitHubReposAPI struct {
	releases map[string]*github.RepositoryRelease // keyed by tag
	assets   map[int64]map[string][]byte           // releaseID → assetName → data
	nextID   int64
}

func newMockGitHubReposAPI() *mockGitHubReposAPI {
	return &mockGitHubReposAPI{
		releases: make(map[string]*github.RepositoryRelease),
		assets:   make(map[int64]map[string][]byte),
		nextID:   1,
	}
}

func (m *mockGitHubReposAPI) GetReleaseByTag(_ context.Context, _, _ string, tag string) (*github.RepositoryRelease, *github.Response, error) {
	rel, ok := m.releases[tag]
	if !ok {
		return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("not found")
	}
	return rel, &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil
}

func (m *mockGitHubReposAPI) CreateRelease(_ context.Context, _, _ string, release *github.RepositoryRelease) (*github.RepositoryRelease, *github.Response, error) {
	id := m.nextID
	m.nextID++
	release.ID = github.Ptr(id)
	if release.TagName != nil {
		m.releases[*release.TagName] = release
	}
	m.assets[id] = make(map[string][]byte)
	return release, nil, nil
}

func (m *mockGitHubReposAPI) GetLatestRelease(_ context.Context, _, _ string) (*github.RepositoryRelease, *github.Response, error) {
	// Return the last release created — simple approximation.
	var latest *github.RepositoryRelease
	for _, rel := range m.releases {
		if latest == nil || rel.GetID() > latest.GetID() {
			latest = rel
		}
	}
	if latest == nil {
		return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("no releases")
	}
	return latest, nil, nil
}

func (m *mockGitHubReposAPI) ListReleaseAssets(_ context.Context, _, _ string, releaseID int64, _ *github.ListOptions) ([]*github.ReleaseAsset, *github.Response, error) {
	assetMap, ok := m.assets[releaseID]
	if !ok {
		return nil, nil, fmt.Errorf("release %d not found", releaseID)
	}
	assets := make([]*github.ReleaseAsset, 0, len(assetMap))
	assetID := int64(1000)
	for name := range assetMap {
		assetID++
		a := &github.ReleaseAsset{
			ID:   github.Ptr(assetID),
			Name: github.Ptr(name),
		}
		assets = append(assets, a)
	}
	return assets, nil, nil
}

func (m *mockGitHubReposAPI) UploadReleaseAsset(_ context.Context, _, _ string, releaseID int64, opts *github.UploadOptions, file io.Reader) (*github.ReleaseAsset, *github.Response, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	assetMap, ok := m.assets[releaseID]
	if !ok {
		assetMap = make(map[string][]byte)
		m.assets[releaseID] = assetMap
	}
	name := opts.Name
	assetMap[name] = data

	assetID := m.nextID
	m.nextID++
	return &github.ReleaseAsset{ID: github.Ptr(assetID), Name: github.Ptr(name)}, nil, nil
}

func (m *mockGitHubReposAPI) DeleteReleaseAsset(_ context.Context, _, _ string, assetID int64) (*github.Response, error) {
	_ = assetID // We match by name in the mock, so this is a no-op in storage.
	return nil, nil
}

func (m *mockGitHubReposAPI) DownloadReleaseAsset(_ context.Context, _, _ string, assetID int64, _ *http.Client) (io.ReadCloser, string, error) {
	_ = assetID // Simplified: just return first asset data found.
	for _, assetMap := range m.assets {
		for _, data := range assetMap {
			return io.NopCloser(bytes.NewReader(data)), "", nil
		}
	}
	return nil, "", errors.New("asset not found")
}

func newMockGitHubTransport() *GitHubTransport {
	return &GitHubTransport{
		client: newMockGitHubReposAPI(),
		owner:  "test-owner",
		repo:   "test-repo",
	}
}

func TestGitHubTransportRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"small json", `{"hello": "world"}`},
		{"empty payload", ""},
		{"large payload", string(bytes.Repeat([]byte("hello github sync, "), 1024))},
		{"binary content", string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gh := newMockGitHubTransport()
			ctx := context.Background()
			assetName := "vault-snapshot.json.gz"

			// Push
			if err := gh.Push(ctx, bytes.NewReader([]byte(tt.payload)), assetName); err != nil {
				t.Fatalf("Push failed: %v", err)
			}

			// Pull
			var out bytes.Buffer
			if err := gh.Pull(ctx, &out, assetName); err != nil {
				t.Fatalf("Pull failed: %v", err)
			}

			if out.String() != tt.payload {
				t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", out.String(), tt.payload)
			}
		})
	}
}

func TestGitHubTransportPushCreatesRelease(t *testing.T) {
	gh := newMockGitHubTransport()
	ctx := context.Background()
	payload := `{"vault": "data"}`
	assetName := "backup.json.gz"

	err := gh.Push(ctx, bytes.NewReader([]byte(payload)), assetName)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Verify the release was created.
	mock := gh.client.(*mockGitHubReposAPI)
	rel, ok := mock.releases[snapshotTag]
	if !ok {
		t.Fatal("release was not created")
	}
	if rel.GetID() <= 0 {
		t.Error("release ID not set")
	}

	// Verify the asset was stored.
	assetData, ok := mock.assets[rel.GetID()][assetName]
	if !ok {
		t.Fatal("asset not found in mock storage")
	}
	if string(assetData) != payload {
		t.Errorf("stored asset = %q, want %q", string(assetData), payload)
	}
}

func TestGitHubTransportPushReplacesExistingAsset(t *testing.T) {
	gh := newMockGitHubTransport()
	ctx := context.Background()
	assetName := "snapshot.json.gz"

	// First push
	if err := gh.Push(ctx, bytes.NewReader([]byte("v1")), assetName); err != nil {
		t.Fatalf("Push 1 failed: %v", err)
	}

	// Second push with different data (same release, same asset name).
	if err := gh.Push(ctx, bytes.NewReader([]byte("v2")), assetName); err != nil {
		t.Fatalf("Push 2 failed: %v", err)
	}

	// Pull should get v2.
	var out bytes.Buffer
	if err := gh.Pull(ctx, &out, assetName); err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
	if out.String() != "v2" {
		t.Errorf("Pull = %q, want v2", out.String())
	}
}

func TestGitHubTransportPullNoRelease(t *testing.T) {
	gh := &GitHubTransport{
		client: &noReleaseGitHubMock{},
		owner:  "test-owner",
		repo:   "test-repo",
	}
	ctx := context.Background()

	var out bytes.Buffer
	err := gh.Pull(ctx, &out, "snapshot.json.gz")
	if err == nil {
		t.Fatal("expected error pulling from repo with no releases")
	}
	if !contains(err.Error(), "no releases") {
		t.Errorf("error = %q, want 'no releases'", err.Error())
	}
}

func TestGitHubTransportPushGetError(t *testing.T) {
	mock := &errorOnGetReleaseMock{}
	gh := &GitHubTransport{
		client: mock,
		owner:  "test-owner",
		repo:   "test-repo",
	}
	ctx := context.Background()

	// Push should fail because GetReleaseByTag returns a non-404 error.
	err := gh.Push(ctx, bytes.NewReader([]byte("data")), "asset.gz")
	if err == nil {
		t.Fatal("expected push error, got nil")
	}
}

func TestGitHubTransportNewValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *GitHubConfig
		wantErr string
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "github config is nil",
		},
		{
			name:    "missing owner",
			cfg:     &GitHubConfig{Repo: "my-repo"},
			wantErr: "github owner is required",
		},
		{
			name:    "missing repo",
			cfg:     &GitHubConfig{Owner: "my-org"},
			wantErr: "github repo is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitHubTransport(tt.cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// --- specialized mocks for error paths ---

type noReleaseGitHubMock struct {
	fallbackGitHubMock
}

func (m *noReleaseGitHubMock) GetLatestRelease(_ context.Context, _, _ string) (*github.RepositoryRelease, *github.Response, error) {
	return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("no releases")
}

type errorOnGetReleaseMock struct {
	fallbackGitHubMock
}

func (m *errorOnGetReleaseMock) GetReleaseByTag(_ context.Context, _, _ string, _ string) (*github.RepositoryRelease, *github.Response, error) {
	return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}, errors.New("internal server error")
}

// fallbackGitHubMock provides stubs for all githubReposAPI methods so specialized mocks
// only need to override the methods relevant to their test scenario.
type fallbackGitHubMock struct{}

func (f *fallbackGitHubMock) GetReleaseByTag(context.Context, string, string, string) (*github.RepositoryRelease, *github.Response, error) {
	return nil, nil, errors.New("unexpected call")
}
func (f *fallbackGitHubMock) CreateRelease(context.Context, string, string, *github.RepositoryRelease) (*github.RepositoryRelease, *github.Response, error) {
	return nil, nil, errors.New("unexpected call")
}
func (f *fallbackGitHubMock) GetLatestRelease(context.Context, string, string) (*github.RepositoryRelease, *github.Response, error) {
	return nil, nil, errors.New("unexpected call")
}
func (f *fallbackGitHubMock) ListReleaseAssets(context.Context, string, string, int64, *github.ListOptions) ([]*github.ReleaseAsset, *github.Response, error) {
	return nil, nil, errors.New("unexpected call")
}
func (f *fallbackGitHubMock) UploadReleaseAsset(context.Context, string, string, int64, *github.UploadOptions, io.Reader) (*github.ReleaseAsset, *github.Response, error) {
	return nil, nil, errors.New("unexpected call")
}
func (f *fallbackGitHubMock) DeleteReleaseAsset(context.Context, string, string, int64) (*github.Response, error) {
	return nil, errors.New("unexpected call")
}
func (f *fallbackGitHubMock) DownloadReleaseAsset(context.Context, string, string, int64, *http.Client) (io.ReadCloser, string, error) {
	return nil, "", errors.New("unexpected call")
}

// Verify GitHubTransport satisfies Transport interface.
var _ Transport = (*GitHubTransport)(nil)

// Verify mockGitHubReposAPI satisfies githubReposAPI interface.
var _ githubReposAPI = (*mockGitHubReposAPI)(nil)

// Avoid unused import.
var _ = fmt.Sprintf
