package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FileNotExist(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadConfig with missing file should not error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// Should be empty (no file, no env set in test).
	if cfg.Transport != "" {
		t.Errorf("Transport should be empty, got %q", cfg.Transport)
	}
}

func TestLoadConfig_YamlFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlContent := []byte(`
transport: s3
remote_path: my-vault/snapshot.json.gz
s3:
  bucket: my-bucket
  region: us-east-1
  endpoint: https://s3.amazonaws.com
  access_key_id: AKIA123456
  secret_access_key: secret123
github:
  owner: my-org
  repo: my-repo
  token: ghp_token123
`)
	if err := os.WriteFile(path, yamlContent, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Transport != "s3" {
		t.Errorf("Transport = %q, want s3", cfg.Transport)
	}
	if cfg.RemotePath != "my-vault/snapshot.json.gz" {
		t.Errorf("RemotePath = %q, want my-vault/snapshot.json.gz", cfg.RemotePath)
	}
	if cfg.S3 == nil {
		t.Fatal("S3 config should not be nil")
	}
	if cfg.S3.Bucket != "my-bucket" {
		t.Errorf("S3.Bucket = %q, want my-bucket", cfg.S3.Bucket)
	}
	if cfg.S3.Region != "us-east-1" {
		t.Errorf("S3.Region = %q, want us-east-1", cfg.S3.Region)
	}
	if cfg.S3.Endpoint != "https://s3.amazonaws.com" {
		t.Errorf("S3.Endpoint = %q, want https://s3.amazonaws.com", cfg.S3.Endpoint)
	}
	if cfg.S3.AccessKeyID != "AKIA123456" {
		t.Errorf("S3.AccessKeyID = %q, want AKIA123456", cfg.S3.AccessKeyID)
	}
	if cfg.S3.SecretAccessKey != "secret123" {
		t.Errorf("S3.SecretAccessKey = %q, want secret123", cfg.S3.SecretAccessKey)
	}
	if cfg.GitHub == nil {
		t.Fatal("GitHub config should not be nil")
	}
	if cfg.GitHub.Owner != "my-org" {
		t.Errorf("GitHub.Owner = %q, want my-org", cfg.GitHub.Owner)
	}
	if cfg.GitHub.Repo != "my-repo" {
		t.Errorf("GitHub.Repo = %q, want my-repo", cfg.GitHub.Repo)
	}
	if cfg.GitHub.Token != "ghp_token123" {
		t.Errorf("GitHub.Token = %q, want ghp_token123", cfg.GitHub.Token)
	}
}

func TestLoadConfig_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlContent := []byte(`
transport: github
s3:
  access_key_id: file-key
  secret_access_key: file-secret
  region: us-west-1
github:
  token: file-token
`)
	if err := os.WriteFile(path, yamlContent, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set environment variables that should override file values.
	t.Setenv("AWS_ACCESS_KEY_ID", "env-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")
	t.Setenv("AWS_REGION", "eu-central-1")
	t.Setenv("GITHUB_TOKEN", "env-ghp-token")
	t.Setenv("TRANSPORT", "s3")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Env should override file values.
	if cfg.Transport != "s3" {
		t.Errorf("Transport = %q, want s3 (env should override file value github)", cfg.Transport)
	}
	if cfg.S3.AccessKeyID != "env-key" {
		t.Errorf("S3.AccessKeyID = %q, want env-key", cfg.S3.AccessKeyID)
	}
	if cfg.S3.SecretAccessKey != "env-secret" {
		t.Errorf("S3.SecretAccessKey = %q, want env-secret", cfg.S3.SecretAccessKey)
	}
	if cfg.S3.Region != "eu-central-1" {
		t.Errorf("S3.Region = %q, want eu-central-1", cfg.S3.Region)
	}
	if cfg.GitHub.Token != "env-ghp-token" {
		t.Errorf("GitHub.Token = %q, want env-ghp-token", cfg.GitHub.Token)
	}
}

func TestLoadConfig_PartialEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// File sets region, env sets only key_id — region should stay from file.
	yamlContent := []byte(`
s3:
  region: ap-southeast-1
`)
	if err := os.WriteFile(path, yamlContent, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("AWS_ACCESS_KEY_ID", "env-key-only")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.S3.Region != "ap-southeast-1" {
		t.Errorf("S3.Region = %q, want ap-southeast-1 (file value should persist)", cfg.S3.Region)
	}
	if cfg.S3.AccessKeyID != "env-key-only" {
		t.Errorf("S3.AccessKeyID = %q, want env-key-only", cfg.S3.AccessKeyID)
	}
}

func TestLoadConfig_InvalidYaml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("invalid: yaml: [unclosed"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadConfig_NonexistentDir(t *testing.T) {
	path := "/nonexistent/deadbeef/config.yaml"
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig should return empty config for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	p := DefaultConfigPath()
	if p == "" {
		t.Error("DefaultConfigPath should not be empty")
	}
	if filepath.Base(p) != "config.yaml" {
		t.Errorf("DefaultConfigPath should end with config.yaml, got %q", p)
	}
}
