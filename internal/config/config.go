package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	Domain   string
	User     string
	APIToken string
	AuthType string // "basic" or "bearer"
	BoardID  int
	Project  string // Project key for filtering boards
}

// jiraCliConfig mirrors the relevant fields from jira-cli's config.yml.
type jiraCliConfig struct {
	Server string `yaml:"server"`
	Login  string `yaml:"login"`
	Board  *struct {
		ID int `yaml:"id"`
	} `yaml:"board"`
}

// Load reads configuration from environment variables, falling back to
// zsh config files and then jira-cli's config file at ~/.config/.jira/.config.yml.
func Load() (*Config, error) {
	cfg := &Config{
		AuthType: "basic",
	}

	// 1. Environment variables take priority.
	cfg.Domain = os.Getenv("JIRA_DOMAIN")
	if cfg.Domain == "" {
		if u := os.Getenv("JIRA_URL"); u != "" {
			cfg.Domain = stripProtocol(u)
		}
	}
	cfg.User = os.Getenv("JIRA_USER")
	if cfg.User == "" {
		cfg.User = os.Getenv("JIRA_USERNAME")
	}
	cfg.APIToken = os.Getenv("JIRA_API_TOKEN")

	if at := os.Getenv("JIRA_AUTH_TYPE"); at != "" {
		cfg.AuthType = at
	}

	if bid := os.Getenv("JIRA_BOARD_ID"); bid != "" {
		id, err := strconv.Atoi(bid)
		if err != nil {
			return nil, fmt.Errorf("invalid JIRA_BOARD_ID: %w", err)
		}
		cfg.BoardID = id
	}

	cfg.Project = os.Getenv("JIRA_PROJECT")

	// 2. Fill gaps from zsh config files (e.g. ~/.zshrc, ~/.secrets.zsh).
	if cfg.Domain == "" || cfg.User == "" || cfg.APIToken == "" {
		cfg.applyZshCredentials()
	}

	// 3. Fall back to jira-cli config for missing values.
	if cfg.Domain == "" || cfg.User == "" {
		_ = cfg.loadJiraCliConfig()
	}

	// Validate auth type.
	switch cfg.AuthType {
	case "basic", "bearer":
		// valid
	default:
		return nil, fmt.Errorf("invalid JIRA_AUTH_TYPE %q: must be 'basic' or 'bearer'", cfg.AuthType)
	}

	// Validate required fields.
	if cfg.Domain == "" {
		return nil, fmt.Errorf("JIRA_DOMAIN or JIRA_URL is required (set env var, add to ~/.zshrc, or configure jira-cli)")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("JIRA_USER or JIRA_USERNAME is required (set env var, add to ~/.zshrc, or configure jira-cli)")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("JIRA_API_TOKEN is required (set env var or add to ~/.zshrc / ~/.secrets.zsh)")
	}

	return cfg, nil
}

// applyZshCredentials fills missing config values from zsh config files.
// Supports aliases: JIRA_URL → Domain (with protocol stripping), JIRA_USERNAME → User.
func (c *Config) applyZshCredentials() {
	creds := loadZshCredentials()

	if c.Domain == "" {
		if d := creds["JIRA_DOMAIN"]; d != "" {
			c.Domain = d
		} else if u := creds["JIRA_URL"]; u != "" {
			c.Domain = stripProtocol(u)
		}
	}
	if c.User == "" {
		if u := creds["JIRA_USER"]; u != "" {
			c.User = u
		} else if u := creds["JIRA_USERNAME"]; u != "" {
			c.User = u
		}
	}
	if c.APIToken == "" {
		c.APIToken = creds["JIRA_API_TOKEN"]
	}
	if c.AuthType == "basic" {
		if at, ok := creds["JIRA_AUTH_TYPE"]; ok {
			c.AuthType = at
		}
	}
	if c.BoardID == 0 {
		if bid, ok := creds["JIRA_BOARD_ID"]; ok {
			if id, err := strconv.Atoi(bid); err == nil {
				c.BoardID = id
			}
		}
	}
}

// stripProtocol removes http:// or https:// prefix from a URL.
func stripProtocol(url string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}

// ServerURL returns the full Jira server URL.
func (c *Config) ServerURL() string {
	return "https://" + c.Domain
}

func (c *Config) loadJiraCliConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	path := filepath.Join(home, ".config", ".jira", ".config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var jcfg jiraCliConfig
	if err := yaml.Unmarshal(data, &jcfg); err != nil {
		return err
	}

	if c.Domain == "" && jcfg.Server != "" {
		c.Domain = stripProtocol(jcfg.Server)
	}

	if c.User == "" && jcfg.Login != "" {
		c.User = jcfg.Login
	}

	if c.BoardID == 0 && jcfg.Board != nil {
		c.BoardID = jcfg.Board.ID
	}

	return nil
}
