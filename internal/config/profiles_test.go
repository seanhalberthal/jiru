package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

// --- keyringUserForProfile ---

func TestKeyringUserForProfile_EmptyProfile(t *testing.T) {
	got := keyringUserForProfile("")
	if got != "api-token" {
		t.Errorf("keyringUserForProfile(%q) = %q, want %q", "", got, "api-token")
	}
}

func TestKeyringUserForProfile_DefaultProfile(t *testing.T) {
	got := keyringUserForProfile("default")
	if got != "api-token" {
		t.Errorf("keyringUserForProfile(%q) = %q, want %q", "default", got, "api-token")
	}
}

func TestKeyringUserForProfile_NamedProfile(t *testing.T) {
	got := keyringUserForProfile("staging")
	if got != "api-token-staging" {
		t.Errorf("keyringUserForProfile(%q) = %q, want %q", "staging", got, "api-token-staging")
	}
}

func TestKeyringUserForProfile_AnotherNamedProfile(t *testing.T) {
	got := keyringUserForProfile("production")
	if got != "api-token-production" {
		t.Errorf("keyringUserForProfile(%q) = %q, want %q", "production", got, "api-token-production")
	}
}

// --- ProfileStore JSON marshalling/unmarshalling ---

func TestProfileStore_JSONRoundTrip(t *testing.T) {
	store := ProfileStore{
		Active: "staging",
		Profiles: map[string]Config{
			"default": {
				Domain:   "default.atlassian.net",
				User:     "default@example.com",
				AuthType: "basic",
				Project:  "DEF",
				BoardID:  10,
			},
			"staging": {
				Domain:          "staging.atlassian.net",
				User:            "staging@example.com",
				AuthType:        "bearer",
				Project:         "STG",
				BoardID:         20,
				RepoPath:        "/repos/staging",
				BranchUppercase: true,
				BranchMode:      "remote",
			},
		},
	}

	data, err := json.Marshal(&store)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got ProfileStore
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.Active != store.Active {
		t.Errorf("Active = %q, want %q", got.Active, store.Active)
	}
	if len(got.Profiles) != len(store.Profiles) {
		t.Fatalf("Profiles length = %d, want %d", len(got.Profiles), len(store.Profiles))
	}

	for name, wantCfg := range store.Profiles {
		gotCfg, ok := got.Profiles[name]
		if !ok {
			t.Errorf("profile %q missing after round trip", name)
			continue
		}
		if gotCfg.Domain != wantCfg.Domain {
			t.Errorf("profile %q Domain = %q, want %q", name, gotCfg.Domain, wantCfg.Domain)
		}
		if gotCfg.User != wantCfg.User {
			t.Errorf("profile %q User = %q, want %q", name, gotCfg.User, wantCfg.User)
		}
		if gotCfg.AuthType != wantCfg.AuthType {
			t.Errorf("profile %q AuthType = %q, want %q", name, gotCfg.AuthType, wantCfg.AuthType)
		}
		if gotCfg.Project != wantCfg.Project {
			t.Errorf("profile %q Project = %q, want %q", name, gotCfg.Project, wantCfg.Project)
		}
		if gotCfg.BoardID != wantCfg.BoardID {
			t.Errorf("profile %q BoardID = %d, want %d", name, gotCfg.BoardID, wantCfg.BoardID)
		}
		if gotCfg.RepoPath != wantCfg.RepoPath {
			t.Errorf("profile %q RepoPath = %q, want %q", name, gotCfg.RepoPath, wantCfg.RepoPath)
		}
		if gotCfg.BranchUppercase != wantCfg.BranchUppercase {
			t.Errorf("profile %q BranchUppercase = %v, want %v", name, gotCfg.BranchUppercase, wantCfg.BranchUppercase)
		}
		if gotCfg.BranchMode != wantCfg.BranchMode {
			t.Errorf("profile %q BranchMode = %q, want %q", name, gotCfg.BranchMode, wantCfg.BranchMode)
		}
	}
}

func TestProfileStore_UnmarshalFromJSON(t *testing.T) {
	jsonData := `{
  "active": "prod",
  "profiles": {
    "prod": {
      "domain": "prod.atlassian.net",
      "user": "prod@example.com",
      "auth_type": "bearer",
      "board_id": 5,
      "project": "PROD"
    }
  }
}`
	var store ProfileStore
	if err := json.Unmarshal([]byte(jsonData), &store); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if store.Active != "prod" {
		t.Errorf("Active = %q, want %q", store.Active, "prod")
	}
	cfg, ok := store.Profiles["prod"]
	if !ok {
		t.Fatal("profile 'prod' not found")
	}
	if cfg.Domain != "prod.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "prod.atlassian.net")
	}
	if cfg.User != "prod@example.com" {
		t.Errorf("User = %q, want %q", cfg.User, "prod@example.com")
	}
	if cfg.AuthType != "bearer" {
		t.Errorf("AuthType = %q, want %q", cfg.AuthType, "bearer")
	}
	if cfg.BoardID != 5 {
		t.Errorf("BoardID = %d, want 5", cfg.BoardID)
	}
}

// --- LoadProfiles ---

func TestLoadProfiles_NoFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store != nil {
		t.Error("expected nil store when profiles.json does not exist")
	}
}

func TestLoadProfiles_ReadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	cfgDir := filepath.Join(dir, ".config", "jiru")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}

	jsonContent := `{
  "active": "staging",
  "profiles": {
    "default": {
      "domain": "default.atlassian.net",
      "user": "default@example.com"
    },
    "staging": {
      "domain": "staging.atlassian.net",
      "user": "staging@example.com"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(cfgDir, "profiles.json"), []byte(jsonContent), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.Active != "staging" {
		t.Errorf("Active = %q, want %q", store.Active, "staging")
	}
	if len(store.Profiles) != 2 {
		t.Errorf("Profiles count = %d, want 2", len(store.Profiles))
	}
}

func TestLoadProfiles_InvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	cfgDir := filepath.Join(dir, ".config", "jiru")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "profiles.json"), []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProfiles()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- ActiveProfileName ---

func TestActiveProfileName_NoProfilesReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	name := ActiveProfileName()
	if name != "" {
		t.Errorf("ActiveProfileName() = %q, want %q", name, "")
	}
}

func TestActiveProfileName_ReturnsDefaultWhenActiveEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "",
		Profiles: map[string]Config{
			"default": {Domain: "test.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	name := ActiveProfileName()
	if name != "default" {
		t.Errorf("ActiveProfileName() = %q, want %q", name, "default")
	}
}

func TestActiveProfileName_ReturnsActiveName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "staging",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	name := ActiveProfileName()
	if name != "staging" {
		t.Errorf("ActiveProfileName() = %q, want %q", name, "staging")
	}
}

// --- ActiveProfile ---

func TestActiveProfile_NoProfilesReturnsNil(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	entry, err := ActiveProfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry != nil {
		t.Error("expected nil entry when no profiles exist")
	}
}

func TestActiveProfile_ReturnsActiveEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "staging",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net", User: "stg@example.com"},
		},
	}
	writeTestProfiles(t, dir, store)

	entry, err := ActiveProfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Name != "staging" {
		t.Errorf("Name = %q, want %q", entry.Name, "staging")
	}
	if entry.Config.Domain != "staging.atlassian.net" {
		t.Errorf("Domain = %q, want %q", entry.Config.Domain, "staging.atlassian.net")
	}
}

func TestActiveProfile_FallsBackToDefaultWhenActiveEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	entry, err := ActiveProfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Name != "default" {
		t.Errorf("Name = %q, want %q", entry.Name, "default")
	}
}

func TestActiveProfile_ReturnsNilWhenActiveProfileMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "nonexistent",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	entry, err := ActiveProfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry != nil {
		t.Error("expected nil entry when active profile does not exist in map")
	}
}

// --- SaveProfile ---

func TestSaveProfile_CreatesProfilesJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	cfg := Config{
		Domain:   "new.atlassian.net",
		User:     "new@example.com",
		AuthType: "basic",
		Project:  "NEW",
	}

	if err := SaveProfile("myprofile", cfg); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Verify the file was created.
	path := filepath.Join(dir, ".config", "jiru", "profiles.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("profiles.json was not created")
	}

	// Verify content.
	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store after SaveProfile")
	}
	if store.Active != "myprofile" {
		t.Errorf("Active = %q, want %q (first profile becomes active)", store.Active, "myprofile")
	}
	savedCfg, ok := store.Profiles["myprofile"]
	if !ok {
		t.Fatal("profile 'myprofile' not found")
	}
	if savedCfg.Domain != "new.atlassian.net" {
		t.Errorf("Domain = %q, want %q", savedCfg.Domain, "new.atlassian.net")
	}
	if savedCfg.User != "new@example.com" {
		t.Errorf("User = %q, want %q", savedCfg.User, "new@example.com")
	}
}

func TestSaveProfile_AddsToExistingProfiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	// Create an initial profile.
	initial := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net", User: "default@example.com"},
		},
	}
	writeTestProfiles(t, dir, initial)

	// Add a new profile.
	cfg := Config{Domain: "staging.atlassian.net", User: "staging@example.com"}
	if err := SaveProfile("staging", cfg); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if len(store.Profiles) != 2 {
		t.Errorf("Profiles count = %d, want 2", len(store.Profiles))
	}
	// Active should remain "default" since it was already set.
	if store.Active != "default" {
		t.Errorf("Active = %q, want %q (should not change when adding)", store.Active, "default")
	}
}

func TestSaveProfile_OverwritesExistingProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	initial := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "old.atlassian.net", User: "old@example.com"},
		},
	}
	writeTestProfiles(t, dir, initial)

	cfg := Config{Domain: "updated.atlassian.net", User: "updated@example.com"}
	if err := SaveProfile("default", cfg); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	updatedCfg := store.Profiles["default"]
	if updatedCfg.Domain != "updated.atlassian.net" {
		t.Errorf("Domain = %q, want %q", updatedCfg.Domain, "updated.atlassian.net")
	}
}

// --- ListProfileNames ---

func TestListProfileNames_NoProfilesReturnsNil(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	names, err := ListProfileNames()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestListProfileNames_ReturnsSortedNames(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"staging":    {Domain: "staging.atlassian.net"},
			"default":    {Domain: "default.atlassian.net"},
			"production": {Domain: "production.atlassian.net"},
			"alpha":      {Domain: "alpha.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	names, err := ListProfileNames()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"alpha", "default", "production", "staging"}
	if len(names) != len(want) {
		t.Fatalf("names count = %d, want %d", len(names), len(want))
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestListProfileNames_SingleProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "only",
		Profiles: map[string]Config{
			"only": {Domain: "only.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	names, err := ListProfileNames()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "only" {
		t.Errorf("names = %v, want [only]", names)
	}
}

// --- SwitchProfile ---

func TestSwitchProfile_UpdatesActive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	if err := SwitchProfile("staging"); err != nil {
		t.Fatalf("SwitchProfile failed: %v", err)
	}

	reloaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if reloaded.Active != "staging" {
		t.Errorf("Active = %q, want %q", reloaded.Active, "staging")
	}
}

func TestSwitchProfile_NonexistentProfileNoOp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	if err := SwitchProfile("nonexistent"); err != nil {
		t.Fatalf("SwitchProfile failed: %v", err)
	}

	reloaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	// Should remain "default" since "nonexistent" doesn't exist.
	if reloaded.Active != "default" {
		t.Errorf("Active = %q, want %q (should not change for nonexistent profile)", reloaded.Active, "default")
	}
}

func TestSwitchProfile_NoProfilesFileNoOp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	// No profiles.json exists — should be a no-op, no error.
	if err := SwitchProfile("anything"); err != nil {
		t.Fatalf("SwitchProfile failed: %v", err)
	}
}

// --- DeleteProfile ---

func TestDeleteProfile_RemovesProfile(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	if err := DeleteProfile("staging"); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	reloaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if _, ok := reloaded.Profiles["staging"]; ok {
		t.Error("profile 'staging' should have been deleted")
	}
	if len(reloaded.Profiles) != 1 {
		t.Errorf("Profiles count = %d, want 1", len(reloaded.Profiles))
	}
}

func TestDeleteProfile_ResetsActiveToDefault(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "staging",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	if err := DeleteProfile("staging"); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	reloaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if reloaded.Active != "default" {
		t.Errorf("Active = %q, want %q (should reset to default when active profile deleted)", reloaded.Active, "default")
	}
}

func TestDeleteProfile_DeletingNonActivePreservesActive(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	if err := DeleteProfile("staging"); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	reloaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if reloaded.Active != "default" {
		t.Errorf("Active = %q, want %q (should not change when deleting non-active profile)", reloaded.Active, "default")
	}
}

func TestDeleteProfile_NoProfilesFileNoOp(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	// No profiles.json — should be a no-op, no error.
	if err := DeleteProfile("anything"); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}
}

func TestDeleteProfile_CleansUpKeyring(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	// Set a keyring token for the profile.
	_ = keyring.Set(keyringService, keyringUserForProfile("staging"), "secret-token")

	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	if err := DeleteProfile("staging"); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	// Keyring entry should be gone.
	_, err := keyring.Get(keyringService, keyringUserForProfile("staging"))
	if err == nil {
		t.Error("expected keyring entry to be deleted after DeleteProfile")
	}
}

// --- Profile-aware keyring functions ---

func TestSetAndGetKeyringTokenForProfile(t *testing.T) {
	keyring.MockInit()

	if err := setKeyringTokenForProfile("staging", "staging-secret"); err != nil {
		t.Fatalf("setKeyringTokenForProfile failed: %v", err)
	}

	token, err := getKeyringTokenForProfile("staging")
	if err != nil {
		t.Fatalf("getKeyringTokenForProfile failed: %v", err)
	}
	if token != "staging-secret" {
		t.Errorf("token = %q, want %q", token, "staging-secret")
	}
}

func TestSetAndGetKeyringTokenForProfile_Default(t *testing.T) {
	keyring.MockInit()

	if err := setKeyringTokenForProfile("default", "default-secret"); err != nil {
		t.Fatalf("setKeyringTokenForProfile failed: %v", err)
	}

	// Should use "api-token" key (backward compat).
	token, err := keyring.Get(keyringService, "api-token")
	if err != nil {
		t.Fatalf("keyring.Get failed: %v", err)
	}
	if token != "default-secret" {
		t.Errorf("token = %q, want %q", token, "default-secret")
	}
}

func TestDeleteKeyringTokenForProfile(t *testing.T) {
	keyring.MockInit()

	_ = keyring.Set(keyringService, keyringUserForProfile("staging"), "to-delete")
	deleteKeyringTokenForProfile("staging")

	_, err := keyring.Get(keyringService, keyringUserForProfile("staging"))
	if err == nil {
		t.Error("expected keyring entry to be deleted")
	}
}

// --- helpers ---

// writeTestProfiles writes a ProfileStore to profiles.json in the given home dir.
func writeTestProfiles(t *testing.T, homeDir string, store *ProfileStore) {
	t.Helper()
	cfgDir := filepath.Join(homeDir, ".config", "jiru")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "profiles.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
