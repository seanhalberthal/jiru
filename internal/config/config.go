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
// jira-cli's config file at ~/.config/.jira/.config.yml.
func Load() (*Config, error) {
	cfg := &Config{
		AuthType: "basic",
	}

	// Environment variables take priority.
	cfg.Domain = os.Getenv("JIRA_DOMAIN")
	cfg.User = os.Getenv("JIRA_USER")
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

	// Fall back to jira-cli config for missing values.
	if cfg.Domain == "" || cfg.User == "" || cfg.BoardID == 0 {
		_ = cfg.loadJiraCliConfig()
	}

	// Validate required fields.
	if cfg.Domain == "" {
		return nil, fmt.Errorf("JIRA_DOMAIN is required (set env var or configure jira-cli)")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("JIRA_USER is required (set env var or configure jira-cli)")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("JIRA_API_TOKEN is required")
	}
	if cfg.BoardID == 0 {
		return nil, fmt.Errorf("JIRA_BOARD_ID is required")
	}

	return cfg, nil
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
		// Strip protocol prefix if present.
		domain := jcfg.Server
		for _, prefix := range []string{"https://", "http://"} {
			if len(domain) > len(prefix) && domain[:len(prefix)] == prefix {
				domain = domain[len(prefix):]
				break
			}
		}
		c.Domain = domain
	}

	if c.User == "" && jcfg.Login != "" {
		c.User = jcfg.Login
	}

	if c.BoardID == 0 && jcfg.Board != nil {
		c.BoardID = jcfg.Board.ID
	}

	return nil
}
