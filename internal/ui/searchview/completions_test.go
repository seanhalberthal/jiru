package searchview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCurrentWord(t *testing.T) {
	tests := []struct {
		input  string
		cursor int
		word   string
		start  int
	}{
		{"assignee", 3, "ass", 0},
		{"assignee = ", 11, "", 11},
		{"status = Done AND pri", 21, "pri", 18},
		{"project = PROJ AND assignee = currentUser()", 18, "AND", 15},
		{"", 0, "", 0},
		{"membersOf(admin", 15, "admin", 10},
	}
	for _, tt := range tests {
		word, start := currentWord(tt.input, tt.cursor)
		if word != tt.word || start != tt.start {
			t.Errorf("currentWord(%q, %d) = (%q, %d), want (%q, %d)",
				tt.input, tt.cursor, word, start, tt.word, tt.start)
		}
	}
}

func TestMatchCompletions(t *testing.T) {
	matches := matchCompletions("ass")
	if len(matches) == 0 {
		t.Fatal("expected matches for 'ass'")
	}
	if matches[0].Label != "assignee" {
		t.Errorf("expected first match to be 'assignee', got %q", matches[0].Label)
	}

	// Case-insensitive.
	matches = matchCompletions("an")
	found := false
	for _, m := range matches {
		if m.Label == "AND" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'AND' in matches for 'an'")
	}

	// Empty prefix returns nothing.
	matches = matchCompletions("")
	if len(matches) != 0 {
		t.Errorf("expected no matches for empty prefix, got %d", len(matches))
	}

	// Max completions cap.
	matches = matchCompletions("s")
	if len(matches) > maxCompletions {
		t.Errorf("expected at most %d matches, got %d", maxCompletions, len(matches))
	}
}

func TestTabAcceptsCompletion(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)

	// Type "ass" one character at a time.
	for _, ch := range "ass" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	if m.input.Value() != "ass" {
		t.Fatalf("after typing: got %q, want %q", m.input.Value(), "ass")
	}
	if len(m.completions) == 0 {
		t.Fatal("expected completions after typing 'ass'")
	}

	// Press Tab — should accept first completion ("assignee").
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	got := m.input.Value()
	if got != "assignee" {
		t.Errorf("after Tab: got %q, want %q", got, "assignee")
	}

	// Completions should be cleared after acceptance.
	if len(m.completions) != 0 {
		t.Errorf("expected completions cleared after accept, got %d", len(m.completions))
	}
}

func TestTabAcceptsThroughAppPattern(t *testing.T) {
	// Mirrors how app.go calls searchview: value-type assignment.
	var search Model
	search = New()
	search.Show()
	search.SetSize(80, 24)

	// Type "sta"
	for _, ch := range "sta" {
		var cmd tea.Cmd
		search, cmd = search.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		_ = cmd
	}

	t.Logf("after typing 'sta': value=%q completions=%d", search.input.Value(), len(search.completions))

	// Press Tab
	search, _ = search.Update(tea.KeyMsg{Type: tea.KeyTab})

	got := search.input.Value()
	if got != "status" {
		t.Errorf("after Tab via app pattern: got %q, want %q", got, "status")
	}
}
