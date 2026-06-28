package sync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

const snapshotTag = "vault-snapshot"

// githubReposAPI is the subset of *github.RepositoriesService used by GitHubTransport.
// It uses io.Reader for UploadReleaseAsset instead of *os.File so tests can pass
// in-memory buffers without touching the filesystem.
type githubReposAPI interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
	CreateRelease(ctx context.Context, owner, repo string, release *github.RepositoryRelease) (*github.RepositoryRelease, *github.Response, error)
	GetLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error)
	ListReleaseAssets(ctx context.Context, owner, repo string, releaseID int64, opts *github.ListOptions) ([]*github.ReleaseAsset, *github.Response, error)
	UploadReleaseAsset(ctx context.Context, owner, repo string, releaseID int64, opts *github.UploadOptions, file io.Reader) (*github.ReleaseAsset, *github.Response, error)
	DeleteReleaseAsset(ctx context.Context, owner, repo string, assetID int64) (*github.Response, error)
	DownloadReleaseAsset(ctx context.Context, owner, repo string, assetID int64, followRedirectsClient *http.Client) (io.ReadCloser, string, error)
}

// githubReposAdapter wraps *github.RepositoriesService and adapts UploadReleaseAsset
// from io.Reader (the interface our code uses) to *os.File (what the SDK requires).
type githubReposAdapter struct {
	svc *github.RepositoriesService
}

func (a *githubReposAdapter) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error) {
	return a.svc.GetReleaseByTag(ctx, owner, repo, tag)
}

func (a *githubReposAdapter) CreateRelease(ctx context.Context, owner, repo string, release *github.RepositoryRelease) (*github.RepositoryRelease, *github.Response, error) {
	return a.svc.CreateRelease(ctx, owner, repo, release)
}

func (a *githubReposAdapter) GetLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
	return a.svc.GetLatestRelease(ctx, owner, repo)
}

func (a *githubReposAdapter) ListReleaseAssets(ctx context.Context, owner, repo string, releaseID int64, opts *github.ListOptions) ([]*github.ReleaseAsset, *github.Response, error) {
	return a.svc.ListReleaseAssets(ctx, owner, repo, releaseID, opts)
}

func (a *githubReposAdapter) UploadReleaseAsset(ctx context.Context, owner, repo string, releaseID int64, opts *github.UploadOptions, file io.Reader) (*github.ReleaseAsset, *github.Response, error) {
	// The real SDK requires *os.File. Buffer to a temp file.
	tmp, err := os.CreateTemp("", "skillvault-gh-upload-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, file); err != nil {
		return nil, nil, fmt.Errorf("write temp file: %w", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		return nil, nil, fmt.Errorf("seek temp file: %w", err)
	}

	return a.svc.UploadReleaseAsset(ctx, owner, repo, releaseID, opts, tmp)
}

func (a *githubReposAdapter) DeleteReleaseAsset(ctx context.Context, owner, repo string, assetID int64) (*github.Response, error) {
	return a.svc.DeleteReleaseAsset(ctx, owner, repo, assetID)
}

func (a *githubReposAdapter) DownloadReleaseAsset(ctx context.Context, owner, repo string, assetID int64, followRedirectsClient *http.Client) (io.ReadCloser, string, error) {
	return a.svc.DownloadReleaseAsset(ctx, owner, repo, assetID, followRedirectsClient)
}

// GitHubTransport uploads to and downloads from GitHub Releases.
type GitHubTransport struct {
	client githubReposAPI
	owner  string
	repo   string
}

// NewGitHubTransport creates a GitHubTransport validated against cfg.
// Uses OAuth2 token authentication.
func NewGitHubTransport(cfg *GitHubConfig) (*GitHubTransport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("github config is nil")
	}
	if cfg.Owner == "" {
		return nil, fmt.Errorf("github owner is required")
	}
	if cfg.Repo == "" {
		return nil, fmt.Errorf("github repo is required")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	tc := oauth2.NewClient(context.Background(), ts)
	gh := github.NewClient(tc)

	return &GitHubTransport{
		client: &githubReposAdapter{svc: gh.Repositories},
		owner:  cfg.Owner,
		repo:   cfg.Repo,
	}, nil
}

// Push creates or updates a "vault-snapshot" release and uploads assetName as a release asset.
// If an existing asset with the same name exists, it is deleted first.
func (g *GitHubTransport) Push(ctx context.Context, reader io.Reader, assetName string) error {
	release, resp, err := g.client.GetReleaseByTag(ctx, g.owner, g.repo, snapshotTag)
	if err != nil {
		// If the release doesn't exist (404), create it.
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			release, _, err = g.client.CreateRelease(ctx, g.owner, g.repo, &github.RepositoryRelease{
				TagName: github.Ptr(snapshotTag),
				Name:    github.Ptr("Vault Snapshot"),
				Body:    github.Ptr("Automated vault snapshot backup."),
			})
			if err != nil {
				return fmt.Errorf("github create release: %w", err)
			}
		} else {
			return fmt.Errorf("github get release %q: %w", snapshotTag, err)
		}
	}

	// Delete any existing asset with the same name before uploading.
	releaseID := release.GetID()
	assets, _, err := g.client.ListReleaseAssets(ctx, g.owner, g.repo, releaseID, &github.ListOptions{PerPage: 100})
	if err != nil {
		return fmt.Errorf("github list assets: %w", err)
	}
	for _, a := range assets {
		if a.GetName() == assetName {
			if _, err := g.client.DeleteReleaseAsset(ctx, g.owner, g.repo, a.GetID()); err != nil {
				return fmt.Errorf("github delete old asset %q: %w", assetName, err)
			}
			break
		}
	}

	// Upload the new asset.
	_, _, err = g.client.UploadReleaseAsset(ctx, g.owner, g.repo, releaseID, &github.UploadOptions{
		Name: assetName,
	}, reader)
	if err != nil {
		return fmt.Errorf("github upload asset: %w", err)
	}
	return nil
}

// Pull downloads the assetName from the latest release's assets and writes its contents to writer.
func (g *GitHubTransport) Pull(ctx context.Context, writer io.Writer, assetName string) error {
	release, _, err := g.client.GetLatestRelease(ctx, g.owner, g.repo)
	if err != nil {
		return fmt.Errorf("github get latest release: %w", err)
	}

	releaseID := release.GetID()
	assets, _, err := g.client.ListReleaseAssets(ctx, g.owner, g.repo, releaseID, &github.ListOptions{PerPage: 100})
	if err != nil {
		return fmt.Errorf("github list assets: %w", err)
	}

	for _, a := range assets {
		if a.GetName() == assetName {
			rc, redirectURL, err := g.client.DownloadReleaseAsset(ctx, g.owner, g.repo, a.GetID(), http.DefaultClient)
			if err != nil {
				return fmt.Errorf("github download asset: %w", err)
			}
			if rc == nil {
				return fmt.Errorf("github download asset returned nil reader (redirect to %s)", redirectURL)
			}
			defer rc.Close()
			if _, err := io.Copy(writer, rc); err != nil {
				return fmt.Errorf("github read asset: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("github asset %q not found in latest release", assetName)
}
