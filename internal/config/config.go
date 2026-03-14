package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

	// 1.5. Fill gaps from jiratui config file and load keychain token.
	cfg.applyConfigFile()

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

// PartialLoad attempts to load configuration, returning whatever values are
// available without erroring on missing required fields.
// Returns the partial config and a slice of missing required field names.
func PartialLoad() (*Config, []string) {
	cfg := &Config{AuthType: "basic"}

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
		if id, err := strconv.Atoi(bid); err == nil {
			cfg.BoardID = id
		}
	}
	cfg.Project = os.Getenv("JIRA_PROJECT")

	// 1.5. Fill gaps from jiratui config file and load keychain token.
	cfg.applyConfigFile()

	// 2. Fill gaps from zsh config files.
	if cfg.Domain == "" || cfg.User == "" || cfg.APIToken == "" {
		cfg.applyZshCredentials()
	}

	// 3. Fall back to jira-cli config.
	if cfg.Domain == "" || cfg.User == "" {
		_ = cfg.loadJiraCliConfig()
	}

	// Validate auth type silently.
	switch cfg.AuthType {
	case "basic", "bearer":
	default:
		cfg.AuthType = "basic"
	}

	var missing []string
	if cfg.Domain == "" {
		missing = append(missing, "domain")
	}
	if cfg.User == "" {
		missing = append(missing, "user")
	}
	if cfg.APIToken == "" {
		missing = append(missing, "api_token")
	}
	return cfg, missing
}

// configDir returns the jiratui config directory path.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jiratui"), nil
}

// WriteConfig writes the given config values to ~/.config/jiratui/config.env
// as export statements, and sets them in the current process environment.
// The API token is stored in the OS keychain when available; if the keychain
// is unavailable, it falls back to writing the token in the config file.
func WriteConfig(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	// Store the API token in the OS keychain — never write it to disk.
	if err := setKeyringToken(cfg.APIToken); err != nil {
		return fmt.Errorf("failed to store API token in keychain: %w", err)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("export JIRA_DOMAIN=%q", cfg.Domain))
	lines = append(lines, fmt.Sprintf("export JIRA_USER=%q", cfg.User))
	lines = append(lines, fmt.Sprintf("export JIRA_AUTH_TYPE=%q", cfg.AuthType))
	if cfg.Project != "" {
		lines = append(lines, fmt.Sprintf("export JIRA_PROJECT=%q", cfg.Project))
	}
	if cfg.BoardID != 0 {
		lines = append(lines, fmt.Sprintf("export JIRA_BOARD_ID=%q", strconv.Itoa(cfg.BoardID)))
	}

	path := filepath.Join(dir, "config.env")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}

	// Also set in current process so Load() will find them.
	_ = os.Setenv("JIRA_DOMAIN", cfg.Domain)
	_ = os.Setenv("JIRA_USER", cfg.User)
	_ = os.Setenv("JIRA_API_TOKEN", cfg.APIToken)
	_ = os.Setenv("JIRA_AUTH_TYPE", cfg.AuthType)
	if cfg.Project != "" {
		_ = os.Setenv("JIRA_PROJECT", cfg.Project)
	}
	if cfg.BoardID != 0 {
		_ = os.Setenv("JIRA_BOARD_ID", strconv.Itoa(cfg.BoardID))
	}

	return nil
}

// applyConfigFile fills missing config values from ~/.config/jiratui/config.env.
// If the API token is not in the file, it attempts to load it from the OS keychain.
func (c *Config) applyConfigFile() {
	dir, err := configDir()
	if err != nil {
		// Config dir unavailable — still try the keychain for the token.
		if c.APIToken == "" {
			if token, err := getKeyringToken(); err == nil && token != "" {
				c.APIToken = token
			}
		}
		return
	}
	path := filepath.Join(dir, "config.env")
	exports, err := parseZshExports(path) // Same format — reuse parser.
	if err != nil {
		// Config file doesn't exist — still try the keychain for the token.
		if c.APIToken == "" {
			if token, err := getKeyringToken(); err == nil && token != "" {
				c.APIToken = token
			}
		}
		return
	}
	if c.Domain == "" {
		if d := exports["JIRA_DOMAIN"]; d != "" {
			c.Domain = d
		}
	}
	if c.User == "" {
		if u := exports["JIRA_USER"]; u != "" {
			c.User = u
		}
	}
	fileToken := exports["JIRA_API_TOKEN"]
	if c.APIToken == "" {
		c.APIToken = fileToken
	}
	if c.AuthType == "basic" {
		if at, ok := exports["JIRA_AUTH_TYPE"]; ok {
			c.AuthType = at
		}
	}
	if c.BoardID == 0 {
		if bid, ok := exports["JIRA_BOARD_ID"]; ok {
			if id, err := strconv.Atoi(bid); err == nil {
				c.BoardID = id
			}
		}
	}
	if c.Project == "" {
		c.Project = exports["JIRA_PROJECT"]
	}

	// If the token was not in the config file, try the OS keychain.
	if c.APIToken == "" {
		if token, err := getKeyringToken(); err == nil && token != "" {
			c.APIToken = token
		}
	}

	// Migrate: if the token is in the config file but not in the keychain,
	// move it to the keychain and rewrite the file without the token.
	if fileToken != "" {
		if err := migrateTokenToKeyring(path, fileToken); err == nil {
			c.APIToken = fileToken
		}
	}
}

// migrateTokenToKeyring moves an API token from the config file into the OS
// keychain and rewrites the config file without the token line.
func migrateTokenToKeyring(configPath, token string) error {
	if err := setKeyringToken(token); err != nil {
		return err // Keychain unavailable — leave the file as-is.
	}

	// Rewrite the config file without the JIRA_API_TOKEN line.
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "JIRA_API_TOKEN") {
			continue
		}
		kept = append(kept, line)
	}
	return os.WriteFile(configPath, []byte(strings.Join(kept, "\n")), 0o600)
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
