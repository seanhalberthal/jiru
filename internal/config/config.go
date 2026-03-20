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
	Domain          string
	User            string
	APIToken        string
	AuthType        string // "basic" or "bearer"
	BoardID         int
	Project         string // Project key for filtering boards
	RepoPath        string // Optional path to a local git repository for branch creation
	BranchUppercase bool   // Use uppercase branch names (e.g., PROJ-123-FIX-BUG)
	BranchMode      string // "local", "remote", or "both" (default: "local")
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
	return LoadProfile("")
}

// LoadProfile loads configuration for a specific profile.
// If name is empty, uses the active profile (or falls back to config.env).
func LoadProfile(name string) (*Config, error) {
	cfg := &Config{
		AuthType: "basic",
	}

	// 1. Environment variables take priority.
	if err := cfg.applyEnvVars(); err != nil {
		return nil, err
	}

	// 1.5. Load from profile.
	cfg.applyProfile(name)

	// 2. Fill gaps from zsh config files (e.g. ~/.zshrc, ~/.secrets.zsh).
	if cfg.Domain == "" || cfg.User == "" || cfg.APIToken == "" {
		cfg.applyZshCredentials()
	}

	// 3. Fall back to jira-cli config for missing values.
	if cfg.Domain == "" || cfg.User == "" {
		_ = cfg.loadJiraCliConfig()
	}

	// Default branch mode.
	if cfg.BranchMode == "" {
		cfg.BranchMode = "local"
	}

	// Validate auth type.
	switch cfg.AuthType {
	case "basic", "bearer":
		// valid
	default:
		return nil, fmt.Errorf("invalid JIRA_AUTH_TYPE %q: must be 'basic' or 'bearer'", cfg.AuthType)
	}

	// Validate branch mode.
	switch cfg.BranchMode {
	case "local", "remote", "both":
		// valid
	default:
		return nil, fmt.Errorf("invalid JIRA_BRANCH_MODE %q: must be 'local', 'remote', or 'both'", cfg.BranchMode)
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

// applyEnvVars fills config from environment variables.
func (c *Config) applyEnvVars() error {
	c.Domain = os.Getenv("JIRA_DOMAIN")
	if c.Domain == "" {
		if u := os.Getenv("JIRA_URL"); u != "" {
			c.Domain = stripProtocol(u)
		}
	}
	c.User = os.Getenv("JIRA_USER")
	if c.User == "" {
		c.User = os.Getenv("JIRA_USERNAME")
	}
	c.APIToken = os.Getenv("JIRA_API_TOKEN")

	if at := os.Getenv("JIRA_AUTH_TYPE"); at != "" {
		c.AuthType = at
	}

	if bid := os.Getenv("JIRA_BOARD_ID"); bid != "" {
		id, err := strconv.Atoi(bid)
		if err != nil {
			return fmt.Errorf("invalid JIRA_BOARD_ID: %w", err)
		}
		c.BoardID = id
	}

	c.Project = os.Getenv("JIRA_PROJECT")
	c.RepoPath = expandTilde(os.Getenv("JIRA_REPO_PATH"))
	c.BranchUppercase = os.Getenv("JIRA_BRANCH_UPPERCASE") == "true"
	c.BranchMode = os.Getenv("JIRA_BRANCH_MODE")
	return nil
}

// applyProfile loads config from profiles.yaml for the given profile name.
// Returns true if a profile was found and applied.
func (c *Config) applyProfile(name string) bool {
	store, err := LoadProfiles()
	if err != nil || store == nil {
		return false
	}

	profileName := name
	if profileName == "" {
		profileName = store.Active
		if profileName == "" {
			profileName = "default"
		}
	}

	p, ok := store.Profiles[profileName]
	if !ok {
		return false
	}

	if c.Domain == "" {
		c.Domain = p.Domain
	}
	if c.User == "" {
		c.User = p.User
	}
	if c.AuthType == "basic" && p.AuthType != "" {
		c.AuthType = p.AuthType
	}
	if c.BoardID == 0 {
		c.BoardID = p.BoardID
	}
	if c.Project == "" {
		c.Project = p.Project
	}
	if c.RepoPath == "" {
		c.RepoPath = p.RepoPath
	}
	if !c.BranchUppercase {
		c.BranchUppercase = p.BranchUppercase
	}
	if c.BranchMode == "" {
		c.BranchMode = p.BranchMode
	}

	// Load API token from keyring for this profile.
	if c.APIToken == "" {
		if token, err := getKeyringTokenForProfile(profileName); err == nil && token != "" {
			c.APIToken = token
		}
	}

	return true
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
	return PartialLoadProfile("")
}

// PartialLoadProfile attempts to load config for a specific profile.
func PartialLoadProfile(name string) (*Config, []string) {
	cfg := &Config{AuthType: "basic"}

	// 1. Environment variables take priority (ignore errors for partial load).
	_ = cfg.applyEnvVars()

	// 1.5. Load from profile.
	cfg.applyProfile(name)

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

	// Default branch mode silently.
	switch cfg.BranchMode {
	case "local", "remote", "both":
	default:
		cfg.BranchMode = "local"
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

// configDir returns the jiru config directory path.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jiru"), nil
}

// WriteConfigProfile saves config to a named profile in profiles.yaml.
// The API token is stored in the OS keychain, not in the YAML file.
func WriteConfigProfile(profile string, cfg *Config) error {
	if profile == "" {
		profile = "default"
	}

	// Store token in profile-aware keyring.
	if cfg.APIToken != "" {
		if err := setKeyringTokenForProfile(profile, cfg.APIToken); err != nil {
			return fmt.Errorf("failed to store API token in keychain: %w", err)
		}
	}

	// Save to profiles.yaml (without token).
	profileCfg := *cfg
	profileCfg.APIToken = ""
	if err := SaveProfile(profile, profileCfg); err != nil {
		return err
	}

	// Set in current process.
	setConfigEnv(cfg)
	return nil
}

// setConfigEnv sets config values in the current process environment.
func setConfigEnv(cfg *Config) {
	_ = os.Setenv("JIRA_DOMAIN", cfg.Domain)
	_ = os.Setenv("JIRA_USER", cfg.User)
	_ = os.Setenv("JIRA_AUTH_TYPE", cfg.AuthType)
	if cfg.Project != "" {
		_ = os.Setenv("JIRA_PROJECT", cfg.Project)
	}
	if cfg.BoardID != 0 {
		_ = os.Setenv("JIRA_BOARD_ID", strconv.Itoa(cfg.BoardID))
	}
	if cfg.RepoPath != "" {
		_ = os.Setenv("JIRA_REPO_PATH", cfg.RepoPath)
	}
	if cfg.BranchUppercase {
		_ = os.Setenv("JIRA_BRANCH_UPPERCASE", "true")
	}
	if cfg.BranchMode != "" && cfg.BranchMode != "local" {
		_ = os.Setenv("JIRA_BRANCH_MODE", cfg.BranchMode)
	}
}

// ResetConfig removes profiles.yaml, all profile keyring entries,
// and any legacy config.env file.
func ResetConfig() error {
	// Delete keyring entries for all known profiles.
	store, _ := LoadProfiles()
	if store != nil {
		for name := range store.Profiles {
			deleteKeyringTokenForProfile(name)
		}
	}
	// Also delete the legacy generic keyring token.
	deleteKeyringToken()

	dir, err := configDir()
	if err != nil {
		return err
	}

	// Delete profiles.yaml.
	if err := os.Remove(filepath.Join(dir, "profiles.yaml")); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Delete legacy config.env.
	if err := os.Remove(filepath.Join(dir, "config.env")); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Clear env vars so the wizard starts fresh.
	for _, k := range []string{
		"JIRA_DOMAIN", "JIRA_URL", "JIRA_USER", "JIRA_USERNAME",
		"JIRA_API_TOKEN", "JIRA_AUTH_TYPE", "JIRA_BOARD_ID",
		"JIRA_PROJECT", "JIRA_REPO_PATH", "JIRA_BRANCH_UPPERCASE",
		"JIRA_BRANCH_MODE",
	} {
		_ = os.Unsetenv(k)
	}
	return nil
}

// applyConfigFile fills missing config values from the legacy ~/.config/jiru/config.env.
// Only used by MigrateToProfiles for backward-compatible migration.
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
	if c.RepoPath == "" {
		c.RepoPath = expandTilde(exports["JIRA_REPO_PATH"])
	}
	if !c.BranchUppercase {
		c.BranchUppercase = exports["JIRA_BRANCH_UPPERCASE"] == "true"
	}
	if c.BranchMode == "" {
		c.BranchMode = exports["JIRA_BRANCH_MODE"]
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
	for line := range strings.SplitSeq(string(data), "\n") {
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

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(s string) string {
	if !strings.HasPrefix(s, "~") {
		return s
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return s
	}
	return filepath.Join(home, s[1:])
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
