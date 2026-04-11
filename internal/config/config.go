package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	Domain          string `yaml:"domain,omitempty"`
	User            string `yaml:"user,omitempty"`
	APIToken        string `yaml:"-"`
	AuthType        string `yaml:"auth_type,omitempty"`
	BoardID         int    `yaml:"board_id,omitempty"`
	Project         string `yaml:"project,omitempty"`
	RepoPath        string `yaml:"repo_path,omitempty"`
	BranchUppercase bool   `yaml:"branch_uppercase,omitempty"`
	BranchMode      string `yaml:"branch_mode,omitempty"`
	BranchCopyKey   bool   `yaml:"branch_copy_key,omitempty"`
}

// Load reads configuration from environment variables, falling back to
// profiles.yml written by the setup wizard.
func Load() (*Config, error) {
	return LoadProfile("")
}

// LoadProfile loads configuration for a specific profile.
// If name is empty, uses the active profile.
func LoadProfile(name string) (*Config, error) {
	cfg := &Config{
		AuthType: "basic",
	}

	// 1. Environment variables take priority.
	if err := cfg.applyEnvVars(); err != nil {
		return nil, err
	}

	// 2. Load from profile.
	cfg.applyProfile(name)

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
		return nil, fmt.Errorf("JIRA_DOMAIN is required (set env var or run the setup wizard)")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("JIRA_USER is required (set env var or run the setup wizard)")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("JIRA_API_TOKEN is required (set env var or run the setup wizard)")
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
	c.BranchCopyKey = os.Getenv("JIRA_BRANCH_COPY_KEY") == "true"
	return nil
}

// ClearSensitiveEnv removes API tokens from the process environment to prevent
// leakage to child processes (e.g. git). Call once after config loading is complete.
func ClearSensitiveEnv() {
	_ = os.Unsetenv("JIRA_API_TOKEN")
}

// applyProfile loads config from profiles.yml for the given profile name.
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
	if !c.BranchCopyKey {
		c.BranchCopyKey = p.BranchCopyKey
	}

	// Load API token from keyring for this profile.
	if c.APIToken == "" {
		if token, err := getKeyringTokenForProfile(profileName); err == nil && token != "" {
			c.APIToken = token
		}
	} else {
		// Env var provided a token — sync it to the keychain so it stays
		// current even when the env var isn't set (e.g. different terminal).
		if stored, err := getKeyringTokenForProfile(profileName); err == nil && stored != c.APIToken {
			_ = setKeyringTokenForProfile(profileName, c.APIToken)
		}
	}

	return true
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

	// 2. Load from profile.
	cfg.applyProfile(name)

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
// Respects XDG_CONFIG_HOME; defaults to ~/.config/jiru.
func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "jiru"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jiru"), nil
}

// WriteConfigProfile saves config to a named profile in profiles.yml.
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

	// Save to profiles.yml (without token).
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
	if cfg.BranchCopyKey {
		_ = os.Setenv("JIRA_BRANCH_COPY_KEY", "true")
	}
}

// ResetConfig removes profiles.yml, all profile keyring entries,
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

	// Delete profiles.yml.
	if err := os.Remove(filepath.Join(dir, "profiles.yml")); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Delete legacy config.env if it still exists.
	if err := os.Remove(filepath.Join(dir, "config.env")); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Clear env vars so the wizard starts fresh.
	for _, k := range []string{
		"JIRA_DOMAIN", "JIRA_URL", "JIRA_USER", "JIRA_USERNAME",
		"JIRA_API_TOKEN", "JIRA_AUTH_TYPE", "JIRA_BOARD_ID",
		"JIRA_PROJECT", "JIRA_REPO_PATH", "JIRA_BRANCH_UPPERCASE",
		"JIRA_BRANCH_MODE", "JIRA_BRANCH_COPY_KEY",
	} {
		_ = os.Unsetenv(k)
	}
	return nil
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
