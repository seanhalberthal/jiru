package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProfileStore manages ~/.config/jiru/profiles.json.
type ProfileStore struct {
	Active   string            `json:"active"`
	Profiles map[string]Config `json:"profiles"`
}

// ProfileEntry pairs a profile name with its config.
type ProfileEntry struct {
	Name   string
	Config Config
}

// profilesPath returns the path to the profiles JSON file.
func profilesPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

// LoadProfiles reads ~/.config/jiru/profiles.json.
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
	// Skip empty files — treat as "no profiles" rather than an unmarshal error.
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var store ProfileStore
	if err := json.Unmarshal(data, &store); err != nil {
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
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ActiveProfile returns the active profile entry.
// If profiles.json doesn't exist, returns nil.
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

// SwitchProfile updates the active profile in profiles.json.
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
