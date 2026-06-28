package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the cloud sync configuration.
type Config struct {
	// Transport is the backend identifier ("s3" or "github").
	Transport string `yaml:"transport"`
	// RemotePath is the object key (S3) or asset name (GitHub).
	RemotePath string `yaml:"remote_path"`
	// S3 holds S3-specific configuration (present when transport is "s3").
	S3 *S3Config `yaml:"s3,omitempty"`
	// GitHub holds GitHub-specific configuration (present when transport is "github").
	GitHub *GitHubConfig `yaml:"github,omitempty"`
}

// S3Config holds S3-compatible storage credentials and endpoint.
type S3Config struct {
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
}

// GitHubConfig holds GitHub Releases credentials and target.
type GitHubConfig struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
	Token string `yaml:"token"`
}

// DefaultConfigPath returns the default location for the sync configuration file.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".skillvault", "config.yaml")
	}
	return filepath.Join(home, ".skillvault", "config.yaml")
}

// LoadConfig reads the YAML config from file (if it exists) and applies
// environment variable overrides. Env vars take precedence over file values.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No file is fine — use env-only config.
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

// applyEnvOverrides applies environment variables with higher priority than file config.
func applyEnvOverrides(cfg *Config) {
	if cfg.S3 == nil {
		cfg.S3 = &S3Config{}
	}
	if v := os.Getenv("AWS_ACCESS_KEY_ID"); v != "" {
		cfg.S3.AccessKeyID = v
	}
	if v := os.Getenv("AWS_SECRET_ACCESS_KEY"); v != "" {
		cfg.S3.SecretAccessKey = v
	}
	if v := os.Getenv("AWS_REGION"); v != "" {
		cfg.S3.Region = v
	}
	if v := os.Getenv("AWS_ENDPOINT"); v != "" {
		cfg.S3.Endpoint = v
	}
	// S3 bucket can also be set via env.
	if v := os.Getenv("S3_BUCKET"); v != "" {
		cfg.S3.Bucket = v
	}
	// Remote path override.
	if v := os.Getenv("REMOTE_PATH"); v != "" {
		cfg.RemotePath = v
	}
	// Transport override.
	if v := os.Getenv("TRANSPORT"); v != "" {
		cfg.Transport = v
	}

	if cfg.GitHub == nil {
		cfg.GitHub = &GitHubConfig{}
	}
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		cfg.GitHub.Token = v
	}
	if v := os.Getenv("GITHUB_OWNER"); v != "" {
		cfg.GitHub.Owner = v
	}
	if v := os.Getenv("GITHUB_REPO"); v != "" {
		cfg.GitHub.Repo = v
	}
}
