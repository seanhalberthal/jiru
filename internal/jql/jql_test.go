package jql

import (
	"strings"
	"testing"
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
		{`status = "Ready for Deve`, 24, `"Ready for Deve`, 9},
		{`status = "`, 10, `"`, 9},
		{`status = "D`, 11, `"D`, 9},
		{`status = "Done" AND `, 20, "", 20},
	}
	for _, tt := range tests {
		word, start := CurrentWord(tt.input, tt.cursor)
		if word != tt.word || start != tt.start {
			t.Errorf("CurrentWord(%q, %d) = (%q, %d), want (%q, %d)",
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

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		cursor  int
		wantCtx Context
		wantFld string
		wantPfx string
	}{
		{"empty input", "", 0, CtxField, "", ""},
		{"typing field name", "sta", 3, CtxField, "", "sta"},
		{"after field, space before operator", "status ", 7, CtxOperator, "status", ""},
		{"typing operator", "status =", 8, CtxOperator, "status", "="},
		{"after operator, value position", "status = ", 9, CtxValue, "status", ""},
		{"typing value prefix", "status = In", 11, CtxValue, "status", "In"},
		{"after value, keyword position", "status = Done ", 14, CtxKeyword, "", ""},
		{"after AND, back to field", "status = Done AND ", 18, CtxField, "", ""},
		{"typing AND keyword", "status = Done AN", 16, CtxKeyword, "", "AN"},
		{"IN with parens, value position", "status IN (Do", 13, CtxValue, "status", "Do"},
		{"IN with comma, next value", "status IN (Done, ", 17, CtxValue, "status", ""},
		{"after IN close paren", "status IN (Done, Open) ", 23, CtxKeyword, "", ""},
		{"ORDER BY field position", "status = Done ORDER BY ", 23, CtxOrderBy, "", ""},
		{"issuetype value position", "issuetype = ", 12, CtxValue, "issuetype", ""},
		{"IS NOT operator", "resolution IS NOT ", 18, CtxValue, "resolution", ""},
		{"NOT IN operator", "status NOT IN (", 15, CtxValue, "status", ""},
		{"quoted value prefix", `status = "Ready for Deve`, 24, CtxValue, "status", "Ready for Deve"},
		{"open quote empty value", `status = "`, 10, CtxValue, "status", ""},
		{"quoted value after AND", `assignee = currentUser() AND status = "In Prog`, 47, CtxValue, "status", "In Prog"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.input, tt.cursor)
			if got.Context != tt.wantCtx {
				t.Errorf("context = %d, want %d", got.Context, tt.wantCtx)
			}
			if tt.wantFld != "" && got.Field != tt.wantFld {
				t.Errorf("field = %q, want %q", got.Field, tt.wantFld)
			}
			if got.Prefix != tt.wantPfx {
				t.Errorf("prefix = %q, want %q", got.Prefix, tt.wantPfx)
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
	for _, item := range items {
		if strings.Contains(item.Label, " ") && !strings.HasPrefix(item.InsertText, "\"") {
			t.Errorf("expected quoted InsertText for %q, got %q", item.Label, item.InsertText)
		}
	}
	for _, item := range items {
		if !strings.Contains(item.Label, " ") && strings.HasPrefix(item.InsertText, "\"") {
			t.Errorf("did not expect quoted InsertText for %q, got %q", item.Label, item.InsertText)
		}
	}

	items = vp.ValuesForField("issuetype")
	if len(items) != 5 {
		t.Fatalf("expected 5 issue type values, got %d", len(items))
	}
	items2 := vp.ValuesForField("type")
	if len(items2) != len(items) {
		t.Errorf("expected type alias to return same values as issuetype")
	}
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
		got := QuoteIfNeeded(tt.input)
		if got != tt.want {
			t.Errorf("QuoteIfNeeded(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatch_FieldContext(t *testing.T) {
	ctx := ParseResult{Context: CtxField, Prefix: "sta"}
	matches := Match(ctx, nil)

	found := false
	for _, m := range matches {
		if m.Label == "status" || m.Label == "statusCategory" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'status' or 'statusCategory' in field matches for 'sta'")
	}
}

func TestMatch_ValueContext_WithDynamicValues(t *testing.T) {
	vp := &ValueProvider{Statuses: []string{"To Do", "In Progress", "Done"}}

	ctx := ParseResult{Context: CtxValue, Field: "status", Prefix: "Do"}
	matches := Match(ctx, vp)

	found := false
	for _, m := range matches {
		if m.Label == "Done" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Done' in matches for status value prefix 'Do'")
	}
	for _, m := range matches {
		if m.Label == "To Do" {
			t.Error("did not expect 'To Do' to match prefix 'Do'")
		}
	}
}

func TestMatch_QuotedMultiWordValue(t *testing.T) {
	vp := &ValueProvider{Statuses: []string{"Ready For Development", "In Progress", "Done"}}

	ctx := Parse(`status = "Ready for Deve`, 24)
	if ctx.Context != CtxValue {
		t.Fatalf("expected CtxValue, got %d", ctx.Context)
	}
	if ctx.Prefix != "Ready for Deve" {
		t.Fatalf("expected prefix 'Ready for Deve', got %q", ctx.Prefix)
	}

	matches := Match(ctx, vp)
	found := false
	for _, m := range matches {
		if m.Label == "Ready For Development" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Ready For Development' to match quoted prefix 'Ready for Deve'")
	}
}

func TestMatch_ValueContext_ShowsAllOnEmptyPrefix(t *testing.T) {
	vp := &ValueProvider{Priorities: []string{"Highest", "High", "Medium", "Low", "Lowest"}}

	ctx := ParseResult{Context: CtxValue, Field: "priority", Prefix: ""}
	matches := Match(ctx, vp)
	if len(matches) == 0 {
		t.Error("expected completions for empty prefix in value context")
	}
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

func TestMatch_OperatorContext(t *testing.T) {
	ctx := ParseResult{Context: CtxOperator, Prefix: "!"}
	matches := Match(ctx, nil)

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

func TestMatch_KeywordContext(t *testing.T) {
	ctx := ParseResult{Context: CtxKeyword, Prefix: "A"}
	matches := Match(ctx, nil)

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

func TestMatch_EmptyPrefix_ShowsCompletions(t *testing.T) {
	tests := []struct {
		name    string
		ctx     Context
		wantMin int
	}{
		{"field context", CtxField, 10},
		{"operator context", CtxOperator, 8},
		{"keyword context", CtxKeyword, 4},
		{"orderby context", CtxOrderBy, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseResult{Context: tt.ctx, Prefix: ""}
			matches := Match(ctx, nil)
			if len(matches) < tt.wantMin {
				t.Errorf("expected at least %d completions, got %d", tt.wantMin, len(matches))
			}
			if len(matches) > MaxCompletions {
				t.Errorf("expected at most %d completions, got %d", MaxCompletions, len(matches))
			}
		})
	}
}

func TestMatch_MaxCompletionsCap(t *testing.T) {
	ctx := ParseResult{Context: CtxField, Prefix: "s"}
	matches := Match(ctx, nil)
	if len(matches) > MaxCompletions {
		t.Errorf("expected at most %d matches, got %d", MaxCompletions, len(matches))
	}
}

func TestMatch_NilValues_StillOffersFunctionsAndKeywords(t *testing.T) {
	ctx := ParseResult{Context: CtxValue, Field: "status", Prefix: "cur"}
	matches := Match(ctx, nil)

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

func TestAccept(t *testing.T) {
	newVal, newCur := Accept("status = D", 10, Item{Label: "Done"})
	if newVal != "status = Done" {
		t.Errorf("Accept value = %q, want %q", newVal, "status = Done")
	}
	if newCur != 13 {
		t.Errorf("Accept cursor = %d, want 13", newCur)
	}
}

func TestAccept_WithInsertText(t *testing.T) {
	newVal, newCur := Accept("assignee = curr", 15, Item{Label: "currentUser()", InsertText: "currentUser()"})
	if newVal != "assignee = currentUser()" {
		t.Errorf("Accept value = %q, want %q", newVal, "assignee = currentUser()")
	}
	if newCur != 24 {
		t.Errorf("Accept cursor = %d, want 24", newCur)
	}
}
