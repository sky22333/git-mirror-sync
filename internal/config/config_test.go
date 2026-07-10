package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"git-mirror-sync/internal/config"
	"git-mirror-sync/internal/model"
)

func TestLoadAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[source]
provider = "github"
token_env = "GITHUB_TOKEN"

[[targets]]
provider = "gitlab"
token_env = "GITLAB_TOKEN"
owner = "mygroup"

[[targets]]
provider = "codeberg"
token_env = "CODEBERG_TOKEN"
owner = "myuser"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Source.ShouldIncludeForks() {
		t.Fatal("include_forks should default to true")
	}
	if !cfg.Source.ShouldIncludeArchived() {
		t.Fatal("include_archived should default to true")
	}
	if !cfg.Policy.ShouldCreateMissing() {
		t.Fatal("create_missing should default to true")
	}
	if cfg.Policy.Concurrency != 2 {
		t.Fatalf("concurrency default = %d", cfg.Policy.Concurrency)
	}
	if cfg.Policy.RetryLimit() != 3 {
		t.Fatalf("max_retries default = %d", cfg.Policy.RetryLimit())
	}
	if cfg.VisibilityPolicy() != model.VisibilityFollow {
		t.Fatalf("visibility default = %s", cfg.Policy.Visibility)
	}
	if cfg.Targets[0].BaseURL != "https://gitlab.com" {
		t.Fatalf("gitlab base_url = %s", cfg.Targets[0].BaseURL)
	}
	if cfg.Targets[1].BaseURL != "https://codeberg.org" {
		t.Fatalf("codeberg base_url = %s", cfg.Targets[1].BaseURL)
	}
	if cfg.Targets[1].APIURL != "https://codeberg.org/api/v1" {
		t.Fatalf("codeberg api_url = %s", cfg.Targets[1].APIURL)
	}
}

func TestSourceIncludeFlagsCanDisable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[source]
provider = "github"
token_env = "GITHUB_TOKEN"
include_forks = false
include_archived = false

[[targets]]
provider = "gitlab"
token_env = "GITLAB_TOKEN"
owner = "mygroup"

[policy]
visibility = "private"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Source.ShouldIncludeForks() {
		t.Fatal("include_forks=false should stick")
	}
	if cfg.Source.ShouldIncludeArchived() {
		t.Fatal("include_archived=false should stick")
	}
	if cfg.VisibilityPolicy() != model.VisibilityPrivate {
		t.Fatalf("visibility = %s", cfg.Policy.Visibility)
	}
}

func TestValidateRejectsBadProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[source]
provider = "github"
token_env = "GITHUB_TOKEN"

[[targets]]
provider = "bitbucket"
token_env = "X"
owner = "o"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path); err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
