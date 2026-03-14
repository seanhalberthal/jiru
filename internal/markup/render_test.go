package markup

import (
	"strings"
	"testing"
)

func TestRender_EmptyInput(t *testing.T) {
	if got := Render("", 80); got != "" {
		t.Errorf("Render(\"\") = %q, want \"\"", got)
	}
}

func TestRender_PlainText(t *testing.T) {
	input := "Hello world, no markup here."
	got := Render(input, 80)
	if !containsText(got, "Hello world, no markup here.") {
		t.Errorf("plain text should pass through unchanged, got %q", stripANSI(got))
	}
}

func TestRender_Bold(t *testing.T) {
	got := Render("This is *bold* text.", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "bold") {
		t.Errorf("bold text not found in output: %q", stripped)
	}
	// Should not contain the asterisks.
	if strings.Contains(stripped, "*bold*") {
		t.Errorf("raw bold markup should be removed, got %q", stripped)
	}
}

func TestRender_Italic(t *testing.T) {
	got := Render("This is _italic_ text.", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "italic") {
		t.Errorf("italic text not found in output: %q", stripped)
	}
	if strings.Contains(stripped, "_italic_") {
		t.Errorf("raw italic markup should be removed, got %q", stripped)
	}
}

func TestRender_Strikethrough(t *testing.T) {
	got := Render("This is -deleted- text.", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "deleted") {
		t.Errorf("strikethrough text not found in output: %q", stripped)
	}
}

func TestRender_Underline(t *testing.T) {
	got := Render("This is +underlined+ text.", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "underlined") {
		t.Errorf("underline text not found in output: %q", stripped)
	}
}

func TestRender_Monospace(t *testing.T) {
	got := Render("Use {{monospace}} here.", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "monospace") {
		t.Errorf("monospace text not found in output: %q", stripped)
	}
	if strings.Contains(stripped, "{{monospace}}") {
		t.Errorf("raw monospace markup should be removed, got %q", stripped)
	}
}

func TestRender_MixedInline(t *testing.T) {
	got := Render("This has *bold* and _italic_ and {{code}} mixed.", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "bold") || !strings.Contains(stripped, "italic") || !strings.Contains(stripped, "code") {
		t.Errorf("mixed inline formatting not rendered correctly: %q", stripped)
	}
}

func TestRender_Headings(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"h1. Main Title", "Main Title"},
		{"h2. Sub Title", "Sub Title"},
		{"h3. Section", "Section"},
		{"h4. Subsection", "Subsection"},
		{"h5. Minor", "Minor"},
		{"h6. Smallest", "Smallest"},
	}
	for _, tt := range tests {
		got := Render(tt.input, 80)
		if !containsText(got, tt.contains) {
			t.Errorf("Render(%q) should contain %q, got %q", tt.input, tt.contains, stripANSI(got))
		}
	}
}

func TestRender_BulletedLists(t *testing.T) {
	input := "* Item one\n** Nested item\n*** Deep nested"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Item one") {
		t.Errorf("bullet list item not found: %q", stripped)
	}
	if !strings.Contains(stripped, "Nested item") {
		t.Errorf("nested bullet item not found: %q", stripped)
	}
}

func TestRender_NumberedLists(t *testing.T) {
	input := "# First\n## Nested\n# Second"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "First") || !strings.Contains(stripped, "Second") {
		t.Errorf("numbered list items not found: %q", stripped)
	}
}

func TestRender_SquareBulletLists(t *testing.T) {
	input := "- Dash item\n-- Nested dash"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Dash item") || !strings.Contains(stripped, "Nested dash") {
		t.Errorf("dash list items not found: %q", stripped)
	}
}

func TestRender_CodeBlock(t *testing.T) {
	input := "{code:go}\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n{code}"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "func main()") {
		t.Errorf("code block content not found: %q", stripped)
	}
	if !strings.Contains(stripped, "go") {
		t.Errorf("code block language label not found: %q", stripped)
	}
}

func TestRender_CodeBlockNoLanguage(t *testing.T) {
	input := "{code}\nsome code\n{code}"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "some code") {
		t.Errorf("code block content not found: %q", stripped)
	}
}

func TestRender_NoformatBlock(t *testing.T) {
	input := "{noformat}\nraw text\n  preserved indentation\n{noformat}"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "raw text") {
		t.Errorf("noformat block content not found: %q", stripped)
	}
}

func TestRender_BlockQuoteBq(t *testing.T) {
	got := Render("bq. This is a quote.", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "This is a quote.") {
		t.Errorf("blockquote text not found: %q", stripped)
	}
}

func TestRender_QuoteBlock(t *testing.T) {
	input := "{quote}\nQuoted line one\nQuoted line two\n{quote}"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Quoted line one") || !strings.Contains(stripped, "Quoted line two") {
		t.Errorf("quote block content not found: %q", stripped)
	}
}

func TestRender_Panel(t *testing.T) {
	input := "{panel:title=My Panel}\nPanel content here.\n{panel}"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "My Panel") {
		t.Errorf("panel title not found: %q", stripped)
	}
	if !strings.Contains(stripped, "Panel content here.") {
		t.Errorf("panel content not found: %q", stripped)
	}
}

func TestRender_PanelNoTitle(t *testing.T) {
	input := "{panel}\nJust content.\n{panel}"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Just content.") {
		t.Errorf("panel content not found: %q", stripped)
	}
}

func TestRender_AdmonitionBlocks(t *testing.T) {
	macros := []string{"info", "warning", "tip", "note"}
	for _, macro := range macros {
		input := "{" + macro + "}\nSome " + macro + " text.\n{" + macro + "}"
		got := Render(input, 80)
		stripped := stripANSI(got)
		if !strings.Contains(stripped, "Some "+macro+" text.") {
			t.Errorf("{%s} block content not found: %q", macro, stripped)
		}
		// Should contain the capitalised macro name as title.
		expected := strings.ToUpper(macro[:1]) + macro[1:]
		if !strings.Contains(stripped, expected) {
			t.Errorf("{%s} block title %q not found: %q", macro, expected, stripped)
		}
	}
}

func TestRender_HorizontalRule(t *testing.T) {
	got := Render("----", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "─") {
		t.Errorf("horizontal rule not rendered: %q", stripped)
	}
}

func TestRender_LineBreaks(t *testing.T) {
	got := Render(`first\\second`, 80)
	if !strings.Contains(got, "\n") {
		t.Errorf("line break not rendered, got %q", got)
	}
}

func TestRender_LinkURL(t *testing.T) {
	got := Render("[https://example.com]", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "https://example.com") {
		t.Errorf("URL link not found: %q", stripped)
	}
}

func TestRender_LinkAliased(t *testing.T) {
	got := Render("[Example|https://example.com]", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Example") {
		t.Errorf("aliased link text not found: %q", stripped)
	}
	if !strings.Contains(stripped, "https://example.com") {
		t.Errorf("aliased link URL not found: %q", stripped)
	}
}

func TestRender_LinkUserMention(t *testing.T) {
	got := Render("[~jsmith]", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "@jsmith") {
		t.Errorf("user mention not rendered: %q", stripped)
	}
}

func TestRender_Image(t *testing.T) {
	got := Render("!screenshot.png!", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "[image: screenshot.png]") {
		t.Errorf("image placeholder not rendered: %q", stripped)
	}
}

func TestRender_Colour(t *testing.T) {
	got := Render("{color:red}error text{color}", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "error text") {
		t.Errorf("colour text not found: %q", stripped)
	}
	if strings.Contains(stripped, "{color") {
		t.Errorf("raw colour markup should be removed: %q", stripped)
	}
}

func TestRender_Table(t *testing.T) {
	input := "||Name||Age||\n|Alice|30|\n|Bob|25|"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Name") || !strings.Contains(stripped, "Age") {
		t.Errorf("table headers not found: %q", stripped)
	}
	if !strings.Contains(stripped, "Alice") || !strings.Contains(stripped, "Bob") {
		t.Errorf("table data not found: %q", stripped)
	}
}

func TestRender_Superscript(t *testing.T) {
	got := Render("E = mc^2^", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "^2") {
		t.Errorf("superscript not rendered: %q", stripped)
	}
}

func TestRender_Subscript(t *testing.T) {
	got := Render("H~2~O", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "_2") {
		t.Errorf("subscript not rendered: %q", stripped)
	}
}

func TestRender_Citation(t *testing.T) {
	got := Render("??Citation source??", 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "— Citation source") {
		t.Errorf("citation not rendered: %q", stripped)
	}
}

func TestRender_ComplexDocument(t *testing.T) {
	input := `h1. Project Overview

This is a *complex* document with _multiple_ elements.

* First item
* Second item with {{code}}
** Nested item

{code:java}
public class Foo {
    void bar() {}
}
{code}

----

|| Key || Value ||
| name | test |

bq. A wise quote.

{info}
Some useful information.
{info}`

	got := Render(input, 80)
	stripped := stripANSI(got)

	checks := []string{
		"Project Overview",
		"complex",
		"multiple",
		"First item",
		"Second item",
		"Nested item",
		"public class Foo",
		"Key",
		"Value",
		"A wise quote.",
		"Some useful information.",
		"Info",
	}
	for _, check := range checks {
		if !strings.Contains(stripped, check) {
			t.Errorf("complex document missing %q in output: %q", check, stripped)
		}
	}
}

func TestRender_UnclosedCodeBlock(t *testing.T) {
	input := "{code:python}\ndef hello():\n  pass"
	got := Render(input, 80)
	stripped := stripANSI(got)
	// Should gracefully render the content without panicking.
	if !strings.Contains(stripped, "def hello():") {
		t.Errorf("unclosed code block content not rendered: %q", stripped)
	}
}

func TestRender_UnclosedPanel(t *testing.T) {
	input := "{panel:title=Test}\nSome content without closing."
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Some content without closing.") {
		t.Errorf("unclosed panel content not rendered: %q", stripped)
	}
}

func TestRender_UnknownMacroPassthrough(t *testing.T) {
	input := "{somemacro}\nContent\n{somemacro}"
	got := Render(input, 80)
	stripped := stripANSI(got)
	// Unknown macros should pass through as-is.
	if !strings.Contains(stripped, "{somemacro}") {
		t.Errorf("unknown macro should pass through, got %q", stripped)
	}
}

func TestRender_WrappingRespectsWidth(t *testing.T) {
	input := "This is a sentence that should be wrapped at a narrow width to test wrapping behaviour."
	got := Render(input, 30)
	lines := strings.Split(got, "\n")
	if len(lines) <= 1 {
		t.Errorf("text should wrap at width 30, got single line: %q", got)
	}
}

func TestRender_ZeroWidthDisablesWrapping(t *testing.T) {
	input := "This is a long sentence that should not be wrapped."
	got := Render(input, 0)
	if strings.Count(got, "\n") > 0 {
		t.Errorf("zero width should disable wrapping, got %q", got)
	}
}

func TestRender_NegativeWidthDisablesWrapping(t *testing.T) {
	input := "This is a long sentence that should not be wrapped."
	got := Render(input, -1)
	if strings.Count(got, "\n") > 0 {
		t.Errorf("negative width should disable wrapping, got %q", got)
	}
}

func TestRender_StripsRawANSIEscapes(t *testing.T) {
	// Simulates text pasted from a terminal into a Jira ticket containing
	// raw ANSI colour codes (24-bit SGR sequences and resets).
	input := "the base \x1b[38;2;224;175;104mDataRow\x1b[0m div is missing \x1b[38;2;224;175;104mh-full\x1b[0m"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "the base DataRow div is missing h-full") {
		t.Errorf("ANSI codes not stripped from input, got %q", stripped)
	}
	// Must not contain raw escape fragments.
	if strings.Contains(stripped, "38;2;") || strings.Contains(stripped, "[0m") {
		t.Errorf("raw ANSI fragments still present in output: %q", stripped)
	}
}

func TestRender_StripsOrphanedANSIEscapes(t *testing.T) {
	// Bracket-prefixed orphans: [38;2;...m and [0m (ESC stripped, bracket kept).
	input := "the base [38;2;224;175;104mDataRow[0m div is missing [38;2;224;175;104mh-full[0m"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "the base DataRow div is missing h-full") {
		t.Errorf("orphaned ANSI codes not stripped, got %q", stripped)
	}
	if strings.Contains(stripped, "38;2;") || strings.Contains(stripped, "[0m") {
		t.Errorf("raw ANSI fragments still present: %q", stripped)
	}
}

func TestRender_StripsBareANSIWithoutBracket(t *testing.T) {
	// Bare 24-bit colour codes without bracket OR ESC prefix.
	input := "the base 38;2;224;175;104mDataRow[0m inner grid div is missing [38;2;224;175;104mh-full[0m"
	got := Render(input, 80)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "the base DataRow inner grid div is missing h-full") {
		t.Errorf("bare ANSI codes not stripped, got %q", stripped)
	}
	if strings.Contains(stripped, "38;2;") {
		t.Errorf("raw 24-bit colour code still present: %q", stripped)
	}
}

func TestRender_RealWorldJiraANSILeak(t *testing.T) {
	// Exact text from a real Jira ticket with mixed orphaned ANSI codes.
	input := `Additionally, the base 38;2;224;175;104mDataRow[0m inner grid div is missing [38;2;224;175;104mh-full[0m, so its [38;2;224;175;104mitems-center[0m class
has no effect — content is not vertically centred within the fixed [38;2;224;175;104mh-[55px row height.`
	got := Render(input, 120)
	stripped := stripANSI(got)
	// All ANSI fragments should be gone.
	if strings.Contains(stripped, "38;2;") {
		t.Errorf("ANSI colour codes still leaking: %q", stripped)
	}
	if strings.Contains(stripped, "[0m") {
		t.Errorf("ANSI reset still leaking: %q", stripped)
	}
	// The actual content words should remain.
	for _, word := range []string{"DataRow", "h-full", "items-center", "h-"} {
		if !strings.Contains(stripped, word) {
			t.Errorf("content word %q lost after stripping, got %q", word, stripped)
		}
	}
}

// --- End-to-end ticket rendering regression tests ---
// These use real wiki markup from Jira tickets to catch interactions
// between inline patterns (monospace producing ANSI codes that later
// patterns like links could consume).

func TestRender_MonospaceWithBracketsInContent(t *testing.T) {
	// {{h-[55px]}} contains brackets — the link regex must not match them.
	input := "the fixed {{h-[55px]}} row height"
	got := Render(input, 120)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "h-[55px]") {
		t.Errorf("monospace content with brackets mangled, got %q", stripped)
	}
	// Must not contain raw ANSI fragments.
	if strings.Contains(stripped, "38;2;") || strings.Contains(stripped, ";104m") {
		t.Errorf("ANSI fragments leaked into output: %q", stripped)
	}
}

func TestRender_MonospaceFollowedByLink(t *testing.T) {
	// Monospace then a real wiki link on the same line.
	input := "Use {{DataRow}} component — see [docs|https://example.com]"
	got := Render(input, 120)
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "DataRow") {
		t.Errorf("monospace text lost: %q", stripped)
	}
	if !strings.Contains(stripped, "docs") {
		t.Errorf("link alias lost: %q", stripped)
	}
	if !strings.Contains(stripped, "https://example.com") {
		t.Errorf("link URL lost: %q", stripped)
	}
}

func TestRender_MultipleMonospaceWithBrackets(t *testing.T) {
	// Multiple monospace spans, one containing brackets.
	input := "Add {{h-full}} to {{DataRow}} so {{items-center}} works within {{h-[55px]}} row"
	got := Render(input, 120)
	stripped := stripANSI(got)
	for _, word := range []string{"h-full", "DataRow", "items-center", "h-[55px]"} {
		if !strings.Contains(stripped, word) {
			t.Errorf("monospace content %q lost in output: %q", word, stripped)
		}
	}
	if strings.Contains(stripped, "38;2;") {
		t.Errorf("ANSI fragments leaked: %q", stripped)
	}
}

func TestRender_RealTicketContent(t *testing.T) {
	// Full ticket content from a real Jira issue — exercises headings,
	// bullet lists with monospace, and mixed inline patterns.
	input := `h2. Overview

Multiple DataTable consumers use custom row components that duplicate the grid layout logic already provided by the base {{DataRow}} component. These should be refactored to use {{DataRow}} directly, with column-specific rendering handled via {{render}} functions in column definitions.

Additionally, the base {{DataRow}} inner grid div is missing {{h-full}}, so its {{items-center}} class has no effect — content is not vertically centred within the fixed {{h-[55px]}} row height.

h2. Audit of Custom Row Components

h3. Already using DataRow (no changes needed)
* {{TimelineRow}} — {{web/src/features/timeline/components/timelineRow.tsx}}
* {{CaseTaskRowWrapper}} — {{web/src/features/tasks/pages/caseTaskTab.tsx}}

h3. Duplicating grid layout (must refactor)
* {{PracticeAreaRow}} — {{web/src/features/chambersSettings/components/practiceAreaRow.tsx}}
* {{CalendarSyncRow}} — {{web/src/features/chambersSettings/components/calendarSyncRow.tsx}}

h3. Frontend Changes
* Add {{h-full}} to the inner grid div in {{web/src/components/base/dataRow.tsx}} so {{items-center}} vertically centres content within the {{h-[55px]}} row
* Refactor the 4 grid-duplicating components to use {{DataRow}} with column {{render}} functions
* Remove duplicated {{gridTemplateColumns}} computation from refactored components`

	got := Render(input, 120)
	stripped := stripANSI(got)

	// No ANSI fragments should leak through.
	if strings.Contains(stripped, "38;2;") {
		t.Errorf("ANSI fragments leaked in ticket output: %q", stripped)
	}
	if strings.Contains(stripped, ";104m") {
		t.Errorf("ANSI code suffix leaked: %q", stripped)
	}

	// Key content must be preserved.
	mustContain := []string{
		"Overview",
		"DataRow",
		"h-full",
		"items-center",
		"h-[55px]",
		"TimelineRow",
		"PracticeAreaRow",
		"gridTemplateColumns",
		"web/src/components/base/dataRow.tsx",
	}
	for _, want := range mustContain {
		if !strings.Contains(stripped, want) {
			t.Errorf("ticket content %q missing from rendered output", want)
		}
	}
}

// containsText checks that the rendered output contains the expected plain text.
func containsText(rendered, expected string) bool {
	stripped := stripANSI(rendered)
	return strings.Contains(stripped, expected)
}
