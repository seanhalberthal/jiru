package config

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ProfileStore manages ~/.config/jiru/profiles.yaml.
type ProfileStore struct {
	Active   string            `yaml:"active"`
	Profiles map[string]Config `yaml:"profiles"`
}

// ProfileEntry pairs a profile name with its config.
type ProfileEntry struct {
	Name   string
	Config Config
}

// profilesPath returns the path to the profiles YAML file.
func profilesPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.yaml"), nil
}

// LoadProfiles reads ~/.config/jiru/profiles.yaml.
// Returns nil (not an error) if the file does not exist.
func LoadProfiles() (*ProfileStore, error) {
	path, err := profilesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var store ProfileStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// saveProfiles writes the profile store to disk.
func saveProfiles(store *ProfileStore) error {
	path, err := profilesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(store)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ActiveProfile returns the active profile entry.
// If profiles.yaml doesn't exist, returns nil.
func ActiveProfile() (*ProfileEntry, error) {
	store, err := LoadProfiles()
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, nil
	}
	name := store.Active
	if name == "" {
		name = "default"
	}
	cfg, ok := store.Profiles[name]
	if !ok {
		return nil, nil
	}
	return &ProfileEntry{Name: name, Config: cfg}, nil
}

// SwitchProfile updates the active profile in profiles.yaml.
func SwitchProfile(name string) error {
	store, err := LoadProfiles()
	if err != nil {
		return err
	}
	if store == nil {
		return nil
	}
	if _, ok := store.Profiles[name]; !ok {
		return nil
	}
	store.Active = name
	return saveProfiles(store)
}

// SaveProfile adds or updates a named profile.
func SaveProfile(name string, cfg Config) error {
	store, err := LoadProfiles()
	if err != nil {
		return err
	}
	if store == nil {
		store = &ProfileStore{
			Active:   name,
			Profiles: make(map[string]Config),
		}
	}
	if store.Profiles == nil {
		store.Profiles = make(map[string]Config)
	}
	store.Profiles[name] = cfg
	return saveProfiles(store)
}

// ListProfileNames returns sorted profile names.
func ListProfileNames() ([]string, error) {
	store, err := LoadProfiles()
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, nil
	}
	names := make([]string, 0, len(store.Profiles))
	for name := range store.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// DeleteProfile removes a profile and its keyring entry.
func DeleteProfile(name string) error {
	store, err := LoadProfiles()
	if err != nil {
		return err
	}
	if store == nil {
		return nil
	}
	delete(store.Profiles, name)
	deleteKeyringTokenForProfile(name)
	if store.Active == name {
		store.Active = "default"
	}
	return saveProfiles(store)
}

// MigrateToProfiles creates a default profile from existing config.env on first run.
// Idempotent: skips if profiles.yaml already exists.
func MigrateToProfiles() error {
	store, err := LoadProfiles()
	if err != nil {
		return err
	}
	if store != nil {
		return nil // Already migrated.
	}

	// Load existing config the old way.
	cfg := &Config{AuthType: "basic"}
	cfg.applyConfigFile()
	if cfg.Domain == "" && cfg.User == "" {
		return nil // No existing config to migrate.
	}

	// Don't store the API token in the profile YAML — it stays in the keyring.
	// The default profile uses the existing keyring key "api-token".
	apiToken := cfg.APIToken
	cfg.APIToken = ""

	store = &ProfileStore{
		Active:   "default",
		Profiles: map[string]Config{"default": *cfg},
	}

	if err := saveProfiles(store); err != nil {
		return err
	}

	// Ensure the token is in the keyring under the default profile key.
	if apiToken != "" {
		_ = setKeyringTokenForProfile("default", apiToken)
	}

	// Clean up legacy config.env now that migration is complete.
	if path, err := configEnvPath(); err == nil {
		_ = os.Remove(path)
	}

	return nil
}

// configEnvPath returns the path to the legacy config.env file.
func configEnvPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.env"), nil
}

// ActiveProfileName returns just the active profile name, or "" if no profiles exist.
func ActiveProfileName() string {
	store, _ := LoadProfiles()
	if store == nil {
		return ""
	}
	if store.Active == "" {
		return "default"
	}
	return store.Active
}
