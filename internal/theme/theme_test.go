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
		// Cancelled statuses.
		{"Cancelled", 3},
		{"Canceled", 3},
		{"Won't Do", 3},
		{"Rejected", 3},
		{"Declined", 3},
		{"Obsolete", 3},
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
	statuses := []string{"Done", "In Progress", "To Do", "Unknown", "Cancelled"}
	for _, s := range statuses {
		t.Run(s, func(t *testing.T) {
			style := StatusStyle(s)
			// Verify the style can render without panic.
			_ = style.Render("test")
		})
	}
}

func TestStatusStyle_AllKnownStatuses(t *testing.T) {
	statuses := []string{
		"To Do", "Open", "Backlog", "In Progress", "In Review",
		"Done", "Closed", "Resolved", "Cancelled", "Won't Do",
		"Unknown Status", "", // should return a default style
	}
	for _, s := range statuses {
		t.Run(s, func(t *testing.T) {
			style := StatusStyle(s)
			_ = style.Render(s) // must not panic
		})
	}
}

func TestRenderLogo_NarrowTerminal(t *testing.T) {
	result := RenderLogo(10)
	if result != "" {
		t.Errorf("expected empty logo for narrow terminal, got non-empty (%d bytes)", len(result))
	}
}

func TestRenderLogo_ExactWidth(t *testing.T) {
	result := RenderLogo(LogoWidth)
	if result == "" {
		t.Error("expected non-empty logo at exact LogoWidth")
	}
}

func TestRenderLogo_BelowWidth(t *testing.T) {
	result := RenderLogo(LogoWidth - 1)
	if result != "" {
		t.Error("expected empty logo just below LogoWidth")
	}
}

func TestRenderLogo_WideTerminal(t *testing.T) {
	result := RenderLogo(120)
	if result == "" {
		t.Error("expected non-empty logo for wide terminal")
	}
}

func TestStatusCategory_AllBranches(t *testing.T) {
	// Verify coverage of all four category branches.
	if StatusCategory("Done") != 2 {
		t.Error("Done should be category 2")
	}
	if StatusCategory("In Progress") != 1 {
		t.Error("In Progress should be category 1")
	}
	if StatusCategory("Custom") != 0 {
		t.Error("Custom should be category 0")
	}
	if StatusCategory("Cancelled") != 3 {
		t.Error("Cancelled should be category 3")
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

	// In Progress statuses should be bold and use a colour from the in-progress palette.
	for _, s := range []string{"In Progress", "In Review"} {
		got := StatusStyle(s)
		if !got.GetBold() {
			t.Errorf("StatusStyle(%q) should be bold", s)
		}
		fg := got.GetForeground()
		found := false
		for _, c := range statusInProgressColours {
			if fg == c {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("StatusStyle(%q) foreground not in in-progress palette", s)
		}
	}

	// Different in-progress statuses should get different colours.
	s1 := StatusStyle("In Progress").GetForeground()
	s2 := StatusStyle("In Review").GetForeground()
	if s1 == s2 {
		t.Error("In Progress and In Review should have different colours")
	}

	// Cancelled statuses should use StyleStatusCancelled.
	for _, s := range []string{"Cancelled", "Canceled", "Won't Do", "Rejected"} {
		got := StatusStyle(s)
		if got.GetForeground() != StyleStatusCancelled.GetForeground() {
			t.Errorf("StatusStyle(%q) should match StyleStatusCancelled", s)
		}
	}
}

func TestSetStatusCategoryMap(t *testing.T) {
	// Install a custom mapping with instance-specific status names.
	SetStatusCategoryMap(map[string]int{
		"Completed":   2,
		"In Dev":      1,
		"Awaiting QA": 1,
		"New":         0,
		"Cancelled":   3,
	})
	defer SetStatusCategoryMap(nil) // Clean up for other tests.

	tests := []struct {
		status     string
		wantCat    int
		wantDone   bool // should use StyleStatusDone?
		wantProg   bool // should use in-progress palette?
		wantCancel bool // should use StyleStatusCancelled?
	}{
		{"Completed", 2, true, false, false},
		{"In Dev", 1, false, true, false},
		{"Awaiting QA", 1, false, true, false},
		{"New", 0, false, false, false},
		{"Cancelled", 3, false, false, true},
		// Statuses not in the map fall back to hardcoded matching.
		{"Done", 2, true, false, false},
		{"In Progress", 1, false, true, false},
		// Unknown statuses default to "to do".
		{"Whatever", 0, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			cat := StatusCategory(tt.status)
			if cat != tt.wantCat {
				t.Errorf("StatusCategory(%q) = %d, want %d", tt.status, cat, tt.wantCat)
			}

			style := StatusStyle(tt.status)
			if tt.wantDone && style.GetForeground() != StyleStatusDone.GetForeground() {
				t.Errorf("StatusStyle(%q) should match StyleStatusDone", tt.status)
			}
			if tt.wantProg {
				fg := style.GetForeground()
				found := false
				for _, c := range statusInProgressColours {
					if fg == c {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("StatusStyle(%q) foreground not in in-progress palette", tt.status)
				}
			}
			if tt.wantCancel && style.GetForeground() != StyleStatusCancelled.GetForeground() {
				t.Errorf("StatusStyle(%q) should match StyleStatusCancelled", tt.status)
			}
		})
	}
}

func TestUserStyle_EmptyName(t *testing.T) {
	s := UserStyle("")
	// Should return subtle style, not panic.
	_ = s.Render("test")
}

func TestUserStyle_DifferentColoursForSimilarNames(t *testing.T) {
	// These two names collided with the old h*31+c hash (both → index 3).
	s1 := UserStyle("Eldan Halberthal")
	s2 := UserStyle("Oran Halberthal")

	// Extract foreground colours — they should differ.
	fg1 := s1.GetForeground()
	fg2 := s2.GetForeground()

	if fg1 == fg2 {
		t.Errorf("expected different colours for 'Eldan Halberthal' and 'Oran Halberthal', both got %v", fg1)
	}
}

func TestUserStyle_Deterministic(t *testing.T) {
	s1 := UserStyle("Alice")
	s2 := UserStyle("Alice")
	if s1.GetForeground() != s2.GetForeground() {
		t.Error("same name should always produce the same colour")
	}
}

func TestStatusSubPriority_DevBeforeReviewBeforeQA(t *testing.T) {
	dev := StatusSubPriority("In Development")
	review := StatusSubPriority("Code Review")
	qa := StatusSubPriority("In QA")

	if dev >= review {
		t.Errorf("In Development (%d) should be before Code Review (%d)", dev, review)
	}
	if review >= qa {
		t.Errorf("Code Review (%d) should be before In QA (%d)", review, qa)
	}
}

func TestStatusSubPriority_ReadyBeforeDevelopment(t *testing.T) {
	ready := StatusSubPriority("Ready for Development")
	dev := StatusSubPriority("In Development")

	if ready >= dev {
		t.Errorf("Ready for Development (%d) should be before In Development (%d)", ready, dev)
	}
}

func TestStatusSubPriority_UnknownIsNeutral(t *testing.T) {
	if p := StatusSubPriority("Something Custom"); p != 50 {
		t.Errorf("expected neutral 50, got %d", p)
	}
}

func TestSetStatusCategoryMap_NilClearsOverride(t *testing.T) {
	SetStatusCategoryMap(map[string]int{"Foo": 2})
	if StatusCategory("Foo") != 2 {
		t.Error("expected map override")
	}
	SetStatusCategoryMap(nil)
	if StatusCategory("Foo") != 0 {
		t.Error("expected fallback after clearing map")
	}
}
