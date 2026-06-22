package recents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/undont/jiru/internal/config"
)

// activeProfile holds the current profile name for recents file paths.
var activeProfile string

// SetProfile sets the active profile for recents file paths.
func SetProfile(profile string) {
	activeProfile = profile
}

// Entry is a recently viewed Confluence page.
type Entry struct {
	PageID   string    `json:"page_id"`
	Title    string    `json:"title"`
	SpaceKey string    `json:"space_key"`
	ViewedAt time.Time `json:"viewed_at"`
}

// MaxEntries is the maximum number of recent entries to keep.
const MaxEntries = 20

// sanitiseProfile removes path separators and leading dots from a profile name.
func sanitiseProfile(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '_'
		}
		return r
	}, strings.TrimLeft(name, "."))
}

// profileFilePath returns the recents file path for an arbitrary profile name.
func profileFilePath(profile string) (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	if profile == "" || profile == "default" {
		return filepath.Join(dir, "recents.json"), nil
	}
	return filepath.Join(dir, "recents-"+sanitiseProfile(profile)+".json"), nil
}

// recentsPath returns the path to the active profile's recents JSON file.
func recentsPath() (string, error) {
	return profileFilePath(activeProfile)
}

// RenameProfileFiles moves the recents file from oldProfile to newProfile.
// No-op if the source file does not exist.
func RenameProfileFiles(oldProfile, newProfile string) error {
	if oldProfile == newProfile {
		return nil
	}
	oldPath, err := profileFilePath(oldProfile)
	if err != nil {
		return err
	}
	newPath, err := profileFilePath(newProfile)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil // nothing to migrate
	} else if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

// DeleteProfileFiles removes the recents file for a profile.
// No-op if the file does not exist.
func DeleteProfileFiles(profile string) error {
	path, err := profileFilePath(profile)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Load reads all recent entries from disk.
// Returns an empty slice (not an error) if the file does not exist yet.
func Load() ([]Entry, error) {
	path, err := recentsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return []Entry{}, nil
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// save writes all entries to disk atomically.
func save(entries []Entry) error {
	path, err := recentsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
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

// Add inserts or bumps a page to the top of the recents list.
// If the page already exists, updates its title/space and moves it to the top.
// Trims to MaxEntries.
func Add(pageID, title, spaceKey string) error {
	entries, err := Load()
	if err != nil {
		return err
	}

	// Remove existing entry if present (will be re-added at top).
	entries = slices.DeleteFunc(entries, func(e Entry) bool {
		return e.PageID == pageID
	})

	// Prepend new entry.
	entry := Entry{
		PageID:   pageID,
		Title:    title,
		SpaceKey: spaceKey,
		ViewedAt: time.Now(),
	}
	entries = append([]Entry{entry}, entries...)

	// Trim to max.
	if len(entries) > MaxEntries {
		entries = entries[:MaxEntries]
	}

	return save(entries)
}

// Sorted returns entries ordered by ViewedAt descending (most recent first).
func Sorted(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	copy(out, entries)
	slices.SortStableFunc(out, func(a, b Entry) int {
		if b.ViewedAt.Before(a.ViewedAt) {
			return -1
		}
		if a.ViewedAt.Before(b.ViewedAt) {
			return 1
		}
		return 0
	})
	return out
}
