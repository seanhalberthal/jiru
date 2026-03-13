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

// containsText checks that the rendered output contains the expected plain text.
func containsText(rendered, expected string) bool {
	stripped := stripANSI(rendered)
	return strings.Contains(stripped, expected)
}
