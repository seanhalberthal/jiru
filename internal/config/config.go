package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Environment variable names. Centralised so a rename only touches one place.
const (
	envDomain          = "JIRA_DOMAIN"
	envURL             = "JIRA_URL"
	envUser            = "JIRA_USER"
	envUsername        = "JIRA_USERNAME"
	envAPIToken        = "JIRA_API_TOKEN"
	envAuthType        = "JIRA_AUTH_TYPE"
	envBoardID         = "JIRA_BOARD_ID"
	envProject         = "JIRA_PROJECT"
	envRepoPath        = "JIRA_REPO_PATH"
	envBranchUppercase = "JIRA_BRANCH_UPPERCASE"
	envBranchMode      = "JIRA_BRANCH_MODE"
	envBranchCopyName  = "JIRA_BRANCH_COPY_NAME"
)

// allEnvVars lists every jiru-specific env var the config layer reads. Used by
// ResetConfig to clear the process environment when starting fresh.
var allEnvVars = []string{
	envDomain, envURL, envUser, envUsername,
	envAPIToken, envAuthType, envBoardID,
	envProject, envRepoPath, envBranchUppercase,
	envBranchMode, envBranchCopyName,
}

// Config holds the application configuration.
type Config struct {
	Domain          string `json:"domain,omitempty"`
	User            string `json:"user,omitempty"`
	APIToken        string `json:"-"`
	AuthType        string `json:"auth_type,omitempty"`
	BoardID         int    `json:"board_id,omitempty"`
	Project         string `json:"project,omitempty"`
	RepoPath        string `json:"repo_path,omitempty"`
	BranchUppercase bool   `json:"branch_uppercase,omitempty"`
	BranchMode      string `json:"branch_mode,omitempty"`
	BranchCopyName  bool   `json:"branch_copy_name,omitempty"`
}

// Load reads configuration from environment variables, falling back to
// profiles.json written by the setup wizard.
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
		return nil, fmt.Errorf("invalid %s %q: must be 'basic' or 'bearer'", envAuthType, cfg.AuthType)
	}

	// Validate branch mode.
	switch cfg.BranchMode {
	case "local", "remote", "both":
		// valid
	default:
		return nil, fmt.Errorf("invalid %s %q: must be 'local', 'remote', or 'both'", envBranchMode, cfg.BranchMode)
	}

	// Validate required fields.
	if cfg.Domain == "" {
		return nil, fmt.Errorf("%s is required (set env var or run the setup wizard)", envDomain)
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("%s is required (set env var or run the setup wizard)", envUser)
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("%s is required (set env var or run the setup wizard)", envAPIToken)
	}

	return cfg, nil
}

// applyEnvVars fills config from environment variables.
func (c *Config) applyEnvVars() error {
	c.Domain = os.Getenv(envDomain)
	if c.Domain == "" {
		if u := os.Getenv(envURL); u != "" {
			c.Domain = stripProtocol(u)
		}
	}
	c.User = os.Getenv(envUser)
	if c.User == "" {
		c.User = os.Getenv(envUsername)
	}
	c.APIToken = os.Getenv(envAPIToken)

	if at := os.Getenv(envAuthType); at != "" {
		c.AuthType = at
	}

	if bid := os.Getenv(envBoardID); bid != "" {
		id, err := strconv.Atoi(bid)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", envBoardID, err)
		}
		c.BoardID = id
	}

	c.Project = os.Getenv(envProject)
	c.RepoPath = expandTilde(os.Getenv(envRepoPath))
	c.BranchUppercase = os.Getenv(envBranchUppercase) == "true"
	c.BranchMode = os.Getenv(envBranchMode)
	c.BranchCopyName = os.Getenv(envBranchCopyName) == "true"
	return nil
}

// ClearSensitiveEnv removes API tokens from the process environment to prevent
// leakage to child processes (e.g. git). Call once after config loading is complete.
func ClearSensitiveEnv() {
	_ = os.Unsetenv(envAPIToken)
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
	if !c.BranchCopyName {
		c.BranchCopyName = p.BranchCopyName
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

// ConfigDir returns the jiru config directory path.
// Respects XDG_CONFIG_HOME; defaults to ~/.config/jiru.
func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "jiru"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jiru"), nil
}

// WriteConfigProfile saves config to a named profile in profiles.json.
// The API token is stored in the OS keychain, not in the JSON file.
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

	// Save to profiles.json (without token).
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
	_ = os.Setenv(envDomain, cfg.Domain)
	_ = os.Setenv(envUser, cfg.User)
	_ = os.Setenv(envAuthType, cfg.AuthType)
	if cfg.Project != "" {
		_ = os.Setenv(envProject, cfg.Project)
	}
	if cfg.BoardID != 0 {
		_ = os.Setenv(envBoardID, strconv.Itoa(cfg.BoardID))
	}
	if cfg.RepoPath != "" {
		_ = os.Setenv(envRepoPath, cfg.RepoPath)
	}
	if cfg.BranchUppercase {
		_ = os.Setenv(envBranchUppercase, "true")
	}
	if cfg.BranchMode != "" && cfg.BranchMode != "local" {
		_ = os.Setenv(envBranchMode, cfg.BranchMode)
	}
	if cfg.BranchCopyName {
		_ = os.Setenv(envBranchCopyName, "true")
	}
}

// ResetConfig removes profiles.json, all profile keyring entries,
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

	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	// Delete profiles.json (and legacy profiles.yml if present).
	for _, name := range []string{"profiles.json", "profiles.yml"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	// Delete legacy config.env if it still exists.
	if err := os.Remove(filepath.Join(dir, "config.env")); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Clear env vars so the wizard starts fresh.
	for _, k := range allEnvVars {
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
