package searchview

import (
	"strings"
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

func TestTokenise(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"status = Done", []string{"status", "=", "Done"}},
		{`status = "In Progress"`, []string{"status", "=", `"In Progress"`}},
		{"status IN (Done, Open)", []string{"status", "IN", "(", "Done", ",", "Open", ")"}},
		{"priority != High", []string{"priority", "!=", "High"}},
		{"created >= 2024-01-01", []string{"created", ">=", "2024-01-01"}},
		{"summary ~ test", []string{"summary", "~", "test"}},
		{"text !~ spam", []string{"text", "!~", "spam"}},
		{"", nil},
		{"  ", nil},
	}
	for _, tt := range tests {
		got := tokenise(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("tokenise(%q) = %v (len %d), want %v (len %d)",
				tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("tokenise(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseJQLContext(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		cursor  int
		wantCtx cursorContext
		wantFld string
		wantPfx string
	}{
		{
			name:    "empty input",
			input:   "",
			cursor:  0,
			wantCtx: ctxField,
			wantPfx: "",
		},
		{
			name:    "typing field name",
			input:   "sta",
			cursor:  3,
			wantCtx: ctxField,
			wantPfx: "sta",
		},
		{
			name:    "after field, space before operator",
			input:   "status ",
			cursor:  7,
			wantCtx: ctxOperator,
			wantFld: "status",
			wantPfx: "",
		},
		{
			name:    "typing operator",
			input:   "status =",
			cursor:  8,
			wantCtx: ctxOperator,
			wantFld: "status",
			wantPfx: "=",
		},
		{
			name:    "after operator, value position",
			input:   "status = ",
			cursor:  9,
			wantCtx: ctxValue,
			wantFld: "status",
			wantPfx: "",
		},
		{
			name:    "typing value prefix",
			input:   "status = In",
			cursor:  11,
			wantCtx: ctxValue,
			wantFld: "status",
			wantPfx: "In",
		},
		{
			name:    "after value, keyword position",
			input:   "status = Done ",
			cursor:  14,
			wantCtx: ctxKeyword,
			wantPfx: "",
		},
		{
			name:    "after AND, back to field",
			input:   "status = Done AND ",
			cursor:  18,
			wantCtx: ctxField,
			wantPfx: "",
		},
		{
			name:    "typing AND keyword",
			input:   "status = Done AN",
			cursor:  16,
			wantCtx: ctxKeyword,
			wantPfx: "AN",
		},
		{
			name:    "IN with parens, value position",
			input:   "status IN (Do",
			cursor:  13,
			wantCtx: ctxValue,
			wantFld: "status",
			wantPfx: "Do",
		},
		{
			name:    "IN with comma, next value",
			input:   "status IN (Done, ",
			cursor:  17,
			wantCtx: ctxValue,
			wantFld: "status",
			wantPfx: "",
		},
		{
			name:    "after IN close paren",
			input:   "status IN (Done, Open) ",
			cursor:  23,
			wantCtx: ctxKeyword,
			wantPfx: "",
		},
		{
			name:    "ORDER BY field position",
			input:   "status = Done ORDER BY ",
			cursor:  23,
			wantCtx: ctxOrderBy,
			wantPfx: "",
		},
		{
			name:    "issuetype value position",
			input:   "issuetype = ",
			cursor:  12,
			wantCtx: ctxValue,
			wantFld: "issuetype",
			wantPfx: "",
		},
		{
			name:    "IS NOT operator",
			input:   "resolution IS NOT ",
			cursor:  18,
			wantCtx: ctxValue,
			wantFld: "resolution",
			wantPfx: "",
		},
		{
			name:    "NOT IN operator",
			input:   "status NOT IN (",
			cursor:  15,
			wantCtx: ctxValue,
			wantFld: "status",
			wantPfx: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJQLContext(tt.input, tt.cursor)
			if got.context != tt.wantCtx {
				t.Errorf("context = %d, want %d", got.context, tt.wantCtx)
			}
			if tt.wantFld != "" && got.field != tt.wantFld {
				t.Errorf("field = %q, want %q", got.field, tt.wantFld)
			}
			if got.prefix != tt.wantPfx {
				t.Errorf("prefix = %q, want %q", got.prefix, tt.wantPfx)
			}
		})
	}
}

func TestValueProviderForField(t *testing.T) {
	vp := &ValueProvider{
		Statuses:   []string{"To Do", "In Progress", "Done", "In Review"},
		IssueTypes: []string{"Bug", "Story", "Task", "Epic", "Sub-task"},
		Projects:   []string{"PROJ", "TEST"},
	}

	items := vp.ValuesForField("status")
	if len(items) != 4 {
		t.Fatalf("expected 4 status values, got %d", len(items))
	}

	// Values with spaces should have quoted InsertText.
	for _, item := range items {
		if strings.Contains(item.Label, " ") && !strings.HasPrefix(item.InsertText, "\"") {
			t.Errorf("expected quoted InsertText for %q, got %q", item.Label, item.InsertText)
		}
	}

	// Values without spaces should not be quoted.
	for _, item := range items {
		if !strings.Contains(item.Label, " ") && strings.HasPrefix(item.InsertText, "\"") {
			t.Errorf("did not expect quoted InsertText for %q, got %q", item.Label, item.InsertText)
		}
	}

	items = vp.ValuesForField("issuetype")
	if len(items) != 5 {
		t.Fatalf("expected 5 issue type values, got %d", len(items))
	}

	// "type" is an alias for issuetype.
	items2 := vp.ValuesForField("type")
	if len(items2) != len(items) {
		t.Errorf("expected type alias to return same values as issuetype")
	}

	// Unknown field returns nil.
	items = vp.ValuesForField("nonexistent")
	if items != nil {
		t.Errorf("expected nil for unknown field, got %d items", len(items))
	}
}

func TestQuoteIfNeeded(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Done", "Done"},
		{"In Progress", "\"In Progress\""},
		{"Bug", "Bug"},
		{"To Do", "\"To Do\""},
	}
	for _, tt := range tests {
		got := quoteIfNeeded(tt.input)
		if got != tt.want {
			t.Errorf("quoteIfNeeded(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchCompletions_FieldContext(t *testing.T) {
	ctx := parseResult{context: ctxField, prefix: "sta"}
	matches := matchCompletions(ctx, nil)

	found := false
	for _, m := range matches {
		if m.Label == "status" {
			found = true
		}
		if m.Label == "statusCategory" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'status' or 'statusCategory' in field matches for 'sta'")
	}
}

func TestMatchCompletions_ValueContext_WithDynamicValues(t *testing.T) {
	vp := &ValueProvider{
		Statuses: []string{"To Do", "In Progress", "Done"},
	}

	ctx := parseResult{context: ctxValue, field: "status", prefix: "Do"}
	matches := matchCompletions(ctx, vp)

	found := false
	for _, m := range matches {
		if m.Label == "Done" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Done' in matches for status value prefix 'Do'")
	}

	// "To Do" should not match prefix "Do".
	for _, m := range matches {
		if m.Label == "To Do" {
			t.Error("did not expect 'To Do' to match prefix 'Do'")
		}
	}
}

func TestMatchCompletions_ValueContext_ShowsAllOnEmptyPrefix(t *testing.T) {
	vp := &ValueProvider{
		Priorities: []string{"Highest", "High", "Medium", "Low", "Lowest"},
	}

	ctx := parseResult{context: ctxValue, field: "priority", prefix: ""}
	matches := matchCompletions(ctx, vp)

	// Should include the dynamic values plus functions and keywords.
	if len(matches) == 0 {
		t.Error("expected completions for empty prefix in value context")
	}
	// Check dynamic values are present.
	found := false
	for _, m := range matches {
		if m.Label == "Highest" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Highest' in value completions")
	}
}

func TestMatchCompletions_OperatorContext(t *testing.T) {
	ctx := parseResult{context: ctxOperator, prefix: "!"}
	matches := matchCompletions(ctx, nil)

	found := false
	for _, m := range matches {
		if m.Label == "!=" {
			found = true
		}
	}
	if !found {
		t.Error("expected '!=' in operator matches for '!'")
	}
}

func TestMatchCompletions_KeywordContext(t *testing.T) {
	ctx := parseResult{context: ctxKeyword, prefix: "A"}
	matches := matchCompletions(ctx, nil)

	found := false
	for _, m := range matches {
		if m.Label == "AND" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'AND' in keyword matches for 'A'")
	}
}

func TestMatchCompletions_EmptyPrefix_ShowsCompletions(t *testing.T) {
	tests := []struct {
		name    string
		ctx     cursorContext
		wantMin int // minimum number of completions expected
	}{
		{"field context", ctxField, 10},
		{"operator context", ctxOperator, 8},
		{"keyword context", ctxKeyword, 4},
		{"orderby context", ctxOrderBy, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := parseResult{context: tt.ctx, prefix: ""}
			matches := matchCompletions(ctx, nil)
			if len(matches) < tt.wantMin {
				t.Errorf("expected at least %d completions, got %d", tt.wantMin, len(matches))
			}
			if len(matches) > maxCompletions {
				t.Errorf("expected at most %d completions, got %d", maxCompletions, len(matches))
			}
		})
	}
}

func TestMatchCompletions_MaxCompletionsCap(t *testing.T) {
	ctx := parseResult{context: ctxField, prefix: "s"}
	matches := matchCompletions(ctx, nil)
	if len(matches) > maxCompletions {
		t.Errorf("expected at most %d matches, got %d", maxCompletions, len(matches))
	}
}

func TestMatchCompletions_NilValues_StillOffersFunctionsAndKeywords(t *testing.T) {
	ctx := parseResult{context: ctxValue, field: "status", prefix: "cur"}
	matches := matchCompletions(ctx, nil)

	found := false
	for _, m := range matches {
		if m.Label == "currentUser()" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'currentUser()' in value completions even without dynamic values")
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
