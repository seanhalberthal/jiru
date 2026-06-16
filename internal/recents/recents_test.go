package recents

import (
	"fmt"
	"testing"
)

func TestAdd_NewEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	SetProfile("test-new")

	if err := Add("101", "Page A", "ENG"); err != nil {
		t.Fatal(err)
	}
	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].PageID != "101" || entries[0].Title != "Page A" {
		t.Errorf("entry = %+v", entries[0])
	}
}

func TestAdd_DeduplicatesAndBumpsToTop(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	SetProfile("test-dedup")

	_ = Add("101", "Page A", "ENG")
	_ = Add("102", "Page B", "ENG")
	_ = Add("101", "Page A Updated", "ENG")

	entries, _ := Load()
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].PageID != "101" {
		t.Errorf("first entry = %q, want 101", entries[0].PageID)
	}
	if entries[0].Title != "Page A Updated" {
		t.Errorf("title = %q, want %q", entries[0].Title, "Page A Updated")
	}
}

func TestAdd_TrimsToMaxEntries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	SetProfile("test-trim")

	for i := 0; i < MaxEntries+5; i++ {
		if err := Add(fmt.Sprintf("%d", i), fmt.Sprintf("Page %d", i), "ENG"); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := Load()
	if len(entries) != MaxEntries {
		t.Errorf("got %d entries, want %d", len(entries), MaxEntries)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	SetProfile("test-missing")

	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestSorted_OrdersByViewedAtDesc(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	SetProfile("test-sorted")

	_ = Add("101", "Page A", "ENG")
	_ = Add("102", "Page B", "ENG")
	_ = Add("103", "Page C", "ENG")

	entries, _ := Load()
	sorted := Sorted(entries)
	if sorted[0].PageID != "103" {
		t.Errorf("first sorted = %q, want 103", sorted[0].PageID)
	}
}

func TestSetProfile_AffectsFilePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	SetProfile("alpha")
	_ = Add("1", "A", "X")

	SetProfile("beta")
	entries, _ := Load()
	if len(entries) != 0 {
		t.Error("beta profile should have no entries")
	}
}

func TestSanitiseProfile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"../evil", "_evil"},
		{"../../etc/cron.d", "_.._etc_cron.d"},
		{".hidden", "hidden"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitiseProfile(tt.input)
			if got != tt.want {
				t.Errorf("sanitiseProfile(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenameProfileFiles_MovesRecents(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { SetProfile("") })

	SetProfile("staging")
	if err := Add("101", "Page A", "ENG"); err != nil {
		t.Fatal(err)
	}

	if err := RenameProfileFiles("staging", "prod"); err != nil {
		t.Fatalf("RenameProfileFiles failed: %v", err)
	}

	SetProfile("staging")
	if es, _ := Load(); len(es) != 0 {
		t.Errorf("old profile should have no recents after rename, got %d", len(es))
	}
	SetProfile("prod")
	es, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 1 || es[0].PageID != "101" {
		t.Errorf("expected migrated recent under new profile, got %v", es)
	}
}

func TestRenameProfileFiles_MissingSourceNoOp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { SetProfile("") })

	if err := RenameProfileFiles("ghost", "prod"); err != nil {
		t.Fatalf("expected no-op for missing source, got %v", err)
	}
}

func TestDeleteProfileFiles_RemovesRecents(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { SetProfile("") })

	SetProfile("staging")
	if err := Add("101", "Page A", "ENG"); err != nil {
		t.Fatal(err)
	}

	if err := DeleteProfileFiles("staging"); err != nil {
		t.Fatalf("DeleteProfileFiles failed: %v", err)
	}

	SetProfile("staging")
	if es, _ := Load(); len(es) != 0 {
		t.Errorf("expected no recents after delete, got %d", len(es))
	}
}

func TestDeleteProfileFiles_MissingNoOp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { SetProfile("") })

	if err := DeleteProfileFiles("ghost"); err != nil {
		t.Fatalf("expected no-op for missing file, got %v", err)
	}
}
