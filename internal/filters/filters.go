package filters

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/seanhalberthal/jiru/internal/jira"
)

// activeProfile holds the current profile name for filter file paths.
var activeProfile string

// SetProfile sets the active profile for filter file paths.
func SetProfile(profile string) {
	activeProfile = profile
}

// filtersPath returns the path to the filters YAML file.
func filtersPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if activeProfile == "" || activeProfile == "default" {
		return filepath.Join(home, ".config", "jiru", "filters.yaml"), nil
	}
	return filepath.Join(home, ".config", "jiru", "filters-"+activeProfile+".yaml"), nil
}

// Load reads all saved filters from disk.
// Returns an empty slice (not an error) if the file does not exist yet.
func Load() ([]jira.SavedFilter, error) {
	path, err := filtersPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []jira.SavedFilter{}, nil
	}
	if err != nil {
		return nil, err
	}
	var filters []jira.SavedFilter
	if err := yaml.Unmarshal(data, &filters); err != nil {
		return nil, err
	}
	return filters, nil
}

// save writes all filters to disk atomically.
func save(filters []jira.SavedFilter) error {
	path, err := filtersPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(filters)
	if err != nil {
		return err
	}
	// Write to a temp file then rename for atomicity.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// newID generates a short random hex identifier.
func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// MaxFilterNameLen is the maximum length of a filter name.
// Shared with the UI input's CharLimit.
const MaxFilterNameLen = 100

// Duplicate creates a copy of an existing filter with "(copy)" appended to the name.
// The duplicate is never marked as a favourite regardless of the source.
func Duplicate(id string) (jira.SavedFilter, error) {
	all, err := Load()
	if err != nil {
		return jira.SavedFilter{}, err
	}
	var source *jira.SavedFilter
	for i, f := range all {
		if f.ID == id {
			source = &all[i]
			break
		}
	}
	if source == nil {
		return jira.SavedFilter{}, nil // Not found — no-op.
	}
	now := time.Now()
	name := source.Name + " (copy)"
	if len(name) > MaxFilterNameLen {
		name = name[:MaxFilterNameLen]
	}
	dup := jira.SavedFilter{
		ID:        newID(),
		Name:      name,
		JQL:       source.JQL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	all = append(all, dup)
	return dup, save(all)
}

// Add creates a new saved filter and persists it.
func Add(name, jql string) (jira.SavedFilter, error) {
	filters, err := Load()
	if err != nil {
		return jira.SavedFilter{}, err
	}
	now := time.Now()
	f := jira.SavedFilter{
		ID:        newID(),
		Name:      name,
		JQL:       jql,
		CreatedAt: now,
		UpdatedAt: now,
	}
	filters = append(filters, f)
	return f, save(filters)
}

// Update replaces a filter's name and/or JQL by ID and persists the change.
func Update(id, name, jql string) error {
	filters, err := Load()
	if err != nil {
		return err
	}
	for i, f := range filters {
		if f.ID == id {
			filters[i].Name = name
			filters[i].JQL = jql
			filters[i].UpdatedAt = time.Now()
			return save(filters)
		}
	}
	return nil // Not found — treat as a no-op.
}

// ToggleFavourite flips the Favourite flag on the filter with the given ID.
func ToggleFavourite(id string) error {
	filters, err := Load()
	if err != nil {
		return err
	}
	for i, f := range filters {
		if f.ID == id {
			filters[i].Favourite = !f.Favourite
			filters[i].UpdatedAt = time.Now()
			return save(filters)
		}
	}
	return nil
}

// Delete removes the filter with the given ID.
func Delete(id string) error {
	filters, err := Load()
	if err != nil {
		return err
	}
	filters = slices.DeleteFunc(filters, func(f jira.SavedFilter) bool {
		return f.ID == id
	})
	return save(filters)
}

// Sorted returns the filters slice with favourites first, then by creation time descending.
func Sorted(filters []jira.SavedFilter) []jira.SavedFilter {
	out := make([]jira.SavedFilter, len(filters))
	copy(out, filters)
	slices.SortStableFunc(out, func(a, b jira.SavedFilter) int {
		if a.Favourite == b.Favourite {
			// Newer first.
			if b.CreatedAt.Before(a.CreatedAt) {
				return -1
			}
			if a.CreatedAt.Before(b.CreatedAt) {
				return 1
			}
			return 0
		}
		if a.Favourite {
			return -1
		}
		return 1
	})
	return out
}
