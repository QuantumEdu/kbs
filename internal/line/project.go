package line

import "time"

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RepoPath    string    `json:"repo_path"`
	DefaultExec string    `json:"default_exec"`
	ReviewExec  string    `json:"review_exec"`
	NtfyTopic   string    `json:"ntfy_topic"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
