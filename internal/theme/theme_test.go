package theme

import "testing"

func TestStatusCategory(t *testing.T) {
	tests := []struct {
		status string
		want   int
	}{
		{"Done", 2},
		{"Closed", 2},
		{"Resolved", 2},
		{"In Progress", 1},
		{"In Review", 1},
		{"To Do", 0},
		{"Open", 0},
		{"Backlog", 0},
		{"Unknown Status", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := StatusCategory(tt.status)
			if got != tt.want {
				t.Errorf("StatusCategory(%q) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusStyle_ReturnsNonNil(t *testing.T) {
	statuses := []string{"Done", "In Progress", "To Do", "Unknown"}
	for _, s := range statuses {
		t.Run(s, func(t *testing.T) {
			style := StatusStyle(s)
			// Verify the style can render without panic.
			_ = style.Render("test")
		})
	}
}

func TestStatusStyle_Categories(t *testing.T) {
	// Done statuses should use StyleStatusDone.
	for _, s := range []string{"Done", "Closed", "Resolved"} {
		got := StatusStyle(s)
		if got.GetForeground() != StyleStatusDone.GetForeground() {
			t.Errorf("StatusStyle(%q) should match StyleStatusDone", s)
		}
	}

	// In Progress statuses should use StyleStatusInProgress.
	for _, s := range []string{"In Progress", "In Review"} {
		got := StatusStyle(s)
		if got.GetForeground() != StyleStatusInProgress.GetForeground() {
			t.Errorf("StatusStyle(%q) should match StyleStatusInProgress", s)
		}
	}
}
