package filters_test

import (
	"testing"

	"github.com/undont/jiru/internal/filters"
	"github.com/undont/jiru/internal/jira"
)

func TestAddAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	f, err := filters.Add("My filter", "assignee = currentUser()")
	if err != nil {
		t.Fatal(err)
	}
	if f.ID == "" {
		t.Error("expected non-empty ID")
	}
	if f.Name != "My filter" {
		t.Errorf("expected name 'My filter', got %q", f.Name)
	}

	loaded, err := filters.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].ID != f.ID {
		t.Errorf("unexpected loaded filters: %v", loaded)
	}
}

func TestLoadEmptyWhenMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())
	fs, err := filters.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Errorf("expected empty slice, got %d", len(fs))
	}
}

func TestUpdate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	f, err := filters.Add("Original", "status = Open")
	if err != nil {
		t.Fatal(err)
	}

	err = filters.Update(f.ID, "Updated", "status = Done")
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := filters.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(loaded))
	}
	if loaded[0].Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", loaded[0].Name)
	}
	if loaded[0].JQL != "status = Done" {
		t.Errorf("expected JQL 'status = Done', got %q", loaded[0].JQL)
	}
	if !loaded[0].UpdatedAt.After(loaded[0].CreatedAt) {
		t.Error("expected UpdatedAt to be after CreatedAt")
	}
}

func TestDelete(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	f1, _ := filters.Add("First", "a = b")
	f2, _ := filters.Add("Second", "c = d")

	err := filters.Delete(f1.ID)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := filters.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 filter after delete, got %d", len(loaded))
	}
	if loaded[0].ID != f2.ID {
		t.Errorf("expected remaining filter to be %q, got %q", f2.ID, loaded[0].ID)
	}
}

func TestToggleFavourite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	f, _ := filters.Add("Test", "x = y")
	if f.Favourite {
		t.Error("expected Favourite to be false initially")
	}

	err := filters.ToggleFavourite(f.ID)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := filters.Load()
	if !loaded[0].Favourite {
		t.Error("expected Favourite to be true after toggle")
	}

	err = filters.ToggleFavourite(f.ID)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ = filters.Load()
	if loaded[0].Favourite {
		t.Error("expected Favourite to be false after second toggle")
	}
}

func TestSorted(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	f1, _ := filters.Add("Older", "a = b")
	f2, _ := filters.Add("Newer", "c = d")
	_ = filters.ToggleFavourite(f1.ID)

	loaded, _ := filters.Load()
	sorted := filters.Sorted(loaded)

	if len(sorted) != 2 {
		t.Fatalf("expected 2, got %d", len(sorted))
	}
	// Favourite first.
	if sorted[0].ID != f1.ID {
		t.Error("expected favourite filter first")
	}
	// Non-favourite: newer first (but only one non-fav here).
	if sorted[1].ID != f2.ID {
		t.Error("expected non-favourite filter second")
	}
}

func TestSortedNewerFirst(t *testing.T) {
	// Both non-favourite — newer should come first.
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	_, _ = filters.Add("Older", "a = b")
	f2, _ := filters.Add("Newer", "c = d")

	loaded, _ := filters.Load()
	sorted := filters.Sorted(loaded)

	if sorted[0].ID != f2.ID {
		t.Error("expected newer filter first when no favourites")
	}
}

func TestUpdateNonExistent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	// Should be a no-op, not an error.
	err := filters.Update("nonexistent", "Name", "JQL")
	if err != nil {
		t.Errorf("expected no error for updating non-existent filter, got %v", err)
	}
}

func TestDuplicate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	// Add a filter.
	f, err := filters.Add("My Filter", "status = Open")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Duplicate it.
	dup, err := filters.Duplicate(f.ID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if dup.Name != "My Filter (copy)" {
		t.Errorf("expected name 'My Filter (copy)', got %q", dup.Name)
	}
	if dup.JQL != "status = Open" {
		t.Errorf("expected JQL 'status = Open', got %q", dup.JQL)
	}
	if dup.ID == f.ID {
		t.Error("duplicated filter should have a different ID")
	}
	if dup.Favourite {
		t.Error("duplicated filter should not be a favourite")
	}

	// Verify both exist on disk.
	all, err := filters.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(all))
	}
}

func TestDuplicate_NotFound(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	dup, err := filters.Duplicate("nonexistent")
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if dup.ID != "" {
		t.Error("expected empty filter for nonexistent ID")
	}
}

func TestDuplicate_DoesNotInheritFavourite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	f, err := filters.Add("Fav Filter", "status = Open")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := filters.ToggleFavourite(f.ID); err != nil {
		t.Fatalf("ToggleFavourite: %v", err)
	}

	dup, err := filters.Duplicate(f.ID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if dup.Favourite {
		t.Error("duplicated filter must not inherit Favourite flag")
	}
}

func TestSortedDoesNotMutateInput(t *testing.T) {
	input := []jira.SavedFilter{
		{ID: "a", Name: "A", Favourite: false},
		{ID: "b", Name: "B", Favourite: true},
	}
	sorted := filters.Sorted(input)

	// Original should be unchanged.
	if input[0].ID != "a" {
		t.Error("Sorted mutated input slice")
	}
	if sorted[0].ID != "b" {
		t.Error("expected favourite first in sorted output")
	}
}

func TestRenameProfileFiles_MovesFilters(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { filters.SetProfile("") })

	filters.SetProfile("staging")
	if _, err := filters.Add("Mine", "assignee = currentUser()"); err != nil {
		t.Fatal(err)
	}

	if err := filters.RenameProfileFiles("staging", "prod"); err != nil {
		t.Fatalf("RenameProfileFiles failed: %v", err)
	}

	filters.SetProfile("staging")
	if fs, _ := filters.Load(); len(fs) != 0 {
		t.Errorf("old profile should have no filters after rename, got %d", len(fs))
	}
	filters.SetProfile("prod")
	fs, err := filters.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].Name != "Mine" {
		t.Errorf("expected migrated filter under new profile, got %v", fs)
	}
}

func TestRenameProfileFiles_MissingSourceNoOp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { filters.SetProfile("") })

	if err := filters.RenameProfileFiles("ghost", "prod"); err != nil {
		t.Fatalf("expected no-op for missing source, got %v", err)
	}
}

func TestDeleteProfileFiles_RemovesFilters(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { filters.SetProfile("") })

	filters.SetProfile("staging")
	if _, err := filters.Add("Mine", "x"); err != nil {
		t.Fatal(err)
	}

	if err := filters.DeleteProfileFiles("staging"); err != nil {
		t.Fatalf("DeleteProfileFiles failed: %v", err)
	}

	filters.SetProfile("staging")
	if fs, _ := filters.Load(); len(fs) != 0 {
		t.Errorf("expected no filters after delete, got %d", len(fs))
	}
}

func TestDeleteProfileFiles_MissingNoOp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { filters.SetProfile("") })

	if err := filters.DeleteProfileFiles("ghost"); err != nil {
		t.Fatalf("expected no-op for missing file, got %v", err)
	}
}
