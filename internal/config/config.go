package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"git-mirror-sync/internal/model"
)

// Config is the root configuration loaded from config.toml.
type Config struct {
	Source  SourceConfig   `toml:"source"`
	Targets []TargetConfig `toml:"targets"`
	Policy  PolicyConfig   `toml:"policy"`
}

// SourceConfig describes the GitHub source.
type SourceConfig struct {
	Provider        string `toml:"provider"`
	TokenEnv        string `toml:"token_env"`
	IncludeForks    *bool  `toml:"include_forks"`
	IncludeArchived *bool  `toml:"include_archived"`
}

// ShouldIncludeForks defaults to true when unset.
func (s SourceConfig) ShouldIncludeForks() bool {
	if s.IncludeForks == nil {
		return true
	}
	return *s.IncludeForks
}

// ShouldIncludeArchived defaults to true when unset.
func (s SourceConfig) ShouldIncludeArchived() bool {
	if s.IncludeArchived == nil {
		return true
	}
	return *s.IncludeArchived
}

// TargetConfig describes one backup destination.
type TargetConfig struct {
	Provider string `toml:"provider"`
	TokenEnv string `toml:"token_env"`
	Owner    string `toml:"owner"`
	BaseURL  string `toml:"base_url"`
	APIURL   string `toml:"api_url"`
}

// PolicyConfig controls sync behaviour.
type PolicyConfig struct {
	Visibility    string `toml:"visibility"`
	CreateMissing *bool  `toml:"create_missing"`
	DeleteMissing bool   `toml:"delete_missing"`
	Concurrency   int    `toml:"concurrency"`
	// MaxRetries is how many times to retry a repo after a transient failure.
	// Default 3 (first attempt + up to 3 retries). Set 0 to disable retries.
	MaxRetries *int `toml:"max_retries"`
}

// RetryLimit returns how many retries are allowed after the first attempt.
func (p PolicyConfig) RetryLimit() int {
	if p.MaxRetries == nil {
		return 3
	}
	if *p.MaxRetries < 0 {
		return 0
	}
	return *p.MaxRetries
}

// ShouldCreateMissing returns whether missing target repos should be created.
func (p PolicyConfig) ShouldCreateMissing() bool {
	if p.CreateMissing == nil {
		return true
	}
	return *p.CreateMissing
}

// Load reads and validates a TOML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Source.Provider == "" {
		c.Source.Provider = "github"
	}
	if c.Source.TokenEnv == "" {
		c.Source.TokenEnv = "GITHUB_TOKEN"
	}
	if c.Policy.Visibility == "" {
		c.Policy.Visibility = string(model.VisibilityFollow)
	}
	if c.Policy.Concurrency <= 0 {
		c.Policy.Concurrency = 2
	}
	for i := range c.Targets {
		t := &c.Targets[i]
		t.Provider = strings.ToLower(strings.TrimSpace(t.Provider))
		switch t.Provider {
		case "gitlab":
			if t.BaseURL == "" {
				t.BaseURL = "https://gitlab.com"
			}
		case "gitee":
			if t.BaseURL == "" {
				t.BaseURL = "https://gitee.com"
			}
			if t.APIURL == "" {
				t.APIURL = "https://gitee.com/api/v5"
			}
		case "cnb":
			if t.BaseURL == "" {
				t.BaseURL = "https://cnb.cool"
			}
			if t.APIURL == "" {
				t.APIURL = "https://api.cnb.cool"
			}
		case "codeberg":
			if t.BaseURL == "" {
				t.BaseURL = "https://codeberg.org"
			}
			if t.APIURL == "" {
				t.APIURL = "https://codeberg.org/api/v1"
			}
		}
	}
}

// Validate checks required fields.
func (c *Config) Validate() error {
	if strings.ToLower(c.Source.Provider) != "github" {
		return fmt.Errorf("source.provider must be \"github\", got %q", c.Source.Provider)
	}
	if c.Source.TokenEnv == "" {
		return fmt.Errorf("source.token_env is required")
	}
	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one [[targets]] entry is required")
	}

	vis := model.VisibilityPolicy(strings.ToLower(c.Policy.Visibility))
	switch vis {
	case model.VisibilityPrivate, model.VisibilityPublic, model.VisibilityFollow:
	default:
		return fmt.Errorf("policy.visibility must be private|public|follow, got %q", c.Policy.Visibility)
	}
	c.Policy.Visibility = string(vis)

	for i, t := range c.Targets {
		if t.Provider == "" {
			return fmt.Errorf("targets[%d].provider is required", i)
		}
		switch t.Provider {
		case "gitlab", "gitee", "cnb", "codeberg":
		default:
			return fmt.Errorf("targets[%d].provider unsupported: %q", i, t.Provider)
		}
		if t.TokenEnv == "" {
			return fmt.Errorf("targets[%d].token_env is required", i)
		}
		if t.Owner == "" {
			return fmt.Errorf("targets[%d].owner is required (user/group/org namespace)", i)
		}
	}
	return nil
}

// VisibilityPolicy returns the typed visibility policy.
func (c *Config) VisibilityPolicy() model.VisibilityPolicy {
	return model.VisibilityPolicy(c.Policy.Visibility)
}

// TokenFromEnv reads a token from the named environment variable.
func TokenFromEnv(envName string) (string, error) {
	token := strings.TrimSpace(os.Getenv(envName))
	if token == "" {
		return "", fmt.Errorf("environment variable %s is empty or unset", envName)
	}
	return token, nil
}
