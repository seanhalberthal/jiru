package markup

import (
	"strings"
	"testing"
)

// --- renderInline tests ---

func TestRenderInline_Bold(t *testing.T) {
	got := renderInline("Hello *world* today")
	stripped := stripANSI(got)
	if strings.Contains(stripped, "*world*") {
		t.Errorf("bold markup not processed: %q", stripped)
	}
	if !strings.Contains(stripped, "world") {
		t.Errorf("bold text not found: %q", stripped)
	}
}

func TestRenderInline_Italic(t *testing.T) {
	got := renderInline("Hello _world_ today")
	stripped := stripANSI(got)
	if strings.Contains(stripped, "_world_") {
		t.Errorf("italic markup not processed: %q", stripped)
	}
}

func TestRenderInline_Monospace(t *testing.T) {
	got := renderInline("Use {{code}} here")
	stripped := stripANSI(got)
	if strings.Contains(stripped, "{{code}}") {
		t.Errorf("monospace markup not processed: %q", stripped)
	}
	if !strings.Contains(stripped, "code") {
		t.Errorf("monospace text not found: %q", stripped)
	}
}

func TestRenderInline_Link(t *testing.T) {
	got := renderInline("[https://example.com]")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "https://example.com") {
		t.Errorf("link URL not found: %q", stripped)
	}
}

func TestRenderInline_AliasedLink(t *testing.T) {
	got := renderInline("[Click here|https://example.com]")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Click here") {
		t.Errorf("aliased link text not found: %q", stripped)
	}
}

func TestRenderInline_UserMention(t *testing.T) {
	got := renderInline("[~john.doe]")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "@john.doe") {
		t.Errorf("user mention not rendered: %q", stripped)
	}
}

func TestRenderInline_Image(t *testing.T) {
	got := renderInline("See !diagram.png!")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "[image: diagram.png]") {
		t.Errorf("image placeholder not rendered: %q", stripped)
	}
}

func TestRenderInline_Superscript(t *testing.T) {
	got := renderInline("x^2^")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "^2") {
		t.Errorf("superscript not rendered: %q", stripped)
	}
}

func TestRenderInline_Subscript(t *testing.T) {
	got := renderInline("H~2~O")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "_2") {
		t.Errorf("subscript not rendered: %q", stripped)
	}
}

func TestRenderInline_Citation(t *testing.T) {
	got := renderInline("??Some source??")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "— Some source") {
		t.Errorf("citation not rendered: %q", stripped)
	}
}

func TestRenderInline_Colour(t *testing.T) {
	got := renderInline("{color:red}warning{color}")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "warning") {
		t.Errorf("colour text not found: %q", stripped)
	}
	if strings.Contains(stripped, "{color") {
		t.Errorf("raw colour markup not removed: %q", stripped)
	}
}

func TestRenderInline_LineBreak(t *testing.T) {
	got := renderInline(`line one\\line two`)
	if !strings.Contains(got, "\n") {
		t.Errorf("line break not rendered: %q", got)
	}
}

// --- renderBlockLine tests ---

func TestRenderBlockLine_Heading(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"h1. Title", "Title"},
		{"h2. Subtitle", "Subtitle"},
		{"h3. Section", "Section"},
	}
	for _, tt := range tests {
		got, ok := renderBlockLine(tt.input, 80)
		if !ok {
			t.Errorf("renderBlockLine(%q) returned false", tt.input)
			continue
		}
		if !containsText(got, tt.contains) {
			t.Errorf("renderBlockLine(%q) = %q, want to contain %q", tt.input, stripANSI(got), tt.contains)
		}
	}
}

func TestRenderBlockLine_HorizontalRule(t *testing.T) {
	got, ok := renderBlockLine("----", 40)
	if !ok {
		t.Error("renderBlockLine(\"----\") returned false")
	}
	if !strings.Contains(stripANSI(got), "─") {
		t.Errorf("horizontal rule not rendered: %q", got)
	}
}

func TestRenderBlockLine_Blockquote(t *testing.T) {
	got, ok := renderBlockLine("bq. Quoted text", 80)
	if !ok {
		t.Error("renderBlockLine(\"bq. ...\") returned false")
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Quoted text") {
		t.Errorf("blockquote text not found: %q", stripped)
	}
}

func TestRenderBlockLine_BulletList(t *testing.T) {
	got, ok := renderBlockLine("* First item", 80)
	if !ok {
		t.Error("renderBlockLine(\"* ...\") returned false")
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "First item") {
		t.Errorf("bullet list text not found: %q", stripped)
	}
}

func TestRenderBlockLine_NestedBulletList(t *testing.T) {
	got, ok := renderBlockLine("** Nested item", 80)
	if !ok {
		t.Error("renderBlockLine(\"** ...\") returned false")
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Nested item") {
		t.Errorf("nested bullet text not found: %q", stripped)
	}
	// Should have indentation.
	if !strings.HasPrefix(stripped, "  ") {
		t.Errorf("nested bullet should be indented: %q", stripped)
	}
}

func TestRenderBlockLine_NumberedList(t *testing.T) {
	got, ok := renderBlockLine("# Numbered item", 80)
	if !ok {
		t.Error("renderBlockLine(\"# ...\") returned false")
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Numbered item") {
		t.Errorf("numbered list text not found: %q", stripped)
	}
}

func TestRenderBlockLine_BareHeading(t *testing.T) {
	// A bare "h2." with no text should be recognised as a block element and render empty.
	got, ok := renderBlockLine("h2.", 80)
	if !ok {
		t.Error("bare heading should be recognised as block element")
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("bare heading should render empty, got %q", got)
	}
}

func TestRenderBlockLine_HeadingNoSpace(t *testing.T) {
	// "h2.Title" with no space should still render.
	got, ok := renderBlockLine("h2.Title", 80)
	if !ok {
		t.Error("heading without space should be recognised as block element")
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Title") {
		t.Errorf("heading text not found: %q", stripped)
	}
}

func TestRenderBlockLine_NonBlock(t *testing.T) {
	got, ok := renderBlockLine("Just plain text", 80)
	if ok {
		t.Errorf("plain text should not be a block element, got %q", got)
	}
}

// --- parseBlock tests ---

func TestParseBlock_CodeBlock(t *testing.T) {
	lines := []string{"{code:python}", "def hello():", "    pass", "{code}"}
	got, consumed := parseBlock(lines, 0, 80)
	if consumed != 4 {
		t.Errorf("parseBlock consumed %d lines, want 4", consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "def hello():") {
		t.Errorf("code block content not found: %q", stripped)
	}
}

func TestParseBlock_PanelWithContent(t *testing.T) {
	lines := []string{"{panel:title=Info}", "Important content", "{panel}"}
	got, consumed := parseBlock(lines, 0, 80)
	if consumed != 3 {
		t.Errorf("parseBlock consumed %d lines, want 3", consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Important content") {
		t.Errorf("panel content not found: %q", stripped)
	}
}

func TestParseBlock_QuoteBlock(t *testing.T) {
	lines := []string{"{quote}", "Quoted text here", "{quote}"}
	got, consumed := parseBlock(lines, 0, 80)
	if consumed != 3 {
		t.Errorf("parseBlock consumed %d lines, want 3", consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Quoted text here") {
		t.Errorf("quote block content not found: %q", stripped)
	}
}

// --- Edge case: content on same line as opening tag ---

func TestParseBlock_CodeBlockContentOnSameLine(t *testing.T) {
	lines := []string{"{code:csharp}public class Foo {", "    int x;", "}", "{code}"}
	got, consumed := parseBlock(lines, 0, 80)
	if consumed != 4 {
		t.Errorf("parseBlock consumed %d lines, want 4", consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "public class Foo") {
		t.Errorf("code block inline content not found: %q", stripped)
	}
	if !strings.Contains(stripped, "csharp") {
		t.Errorf("code block language label not found: %q", stripped)
	}
}

func TestParseBlock_NoformatContentOnSameLine(t *testing.T) {
	lines := []string{"{noformat}some preformatted text", "more text", "{noformat}"}
	got, consumed := parseBlock(lines, 0, 80)
	if consumed != 3 {
		t.Errorf("parseBlock consumed %d lines, want 3", consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "some preformatted text") {
		t.Errorf("noformat inline content not found: %q", stripped)
	}
}

// --- Edge case: lenient closing tag detection ---

func TestParseBlock_CodeBlockClosingWithTrailingText(t *testing.T) {
	lines := []string{"{code}", "some code", "{code}trailing"}
	got, consumed := parseBlock(lines, 0, 80)
	if consumed != 3 {
		t.Errorf("parseBlock consumed %d lines, want 3, got %d", 3, consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "some code") {
		t.Errorf("code block content not found: %q", stripped)
	}
}

func TestParseBlock_NoformatClosingAtEndOfLine(t *testing.T) {
	lines := []string{"{noformat}", "preformatted", "end here{noformat}"}
	got, consumed := parseBlock(lines, 0, 80)
	if consumed != 3 {
		t.Errorf("parseBlock consumed %d lines, want 3, got %d", 3, consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "preformatted") {
		t.Errorf("noformat content not found: %q", stripped)
	}
}

func TestIsClosingTag(t *testing.T) {
	tests := []struct {
		line string
		tag  string
		want bool
	}{
		{"{code}", "{code}", true},
		{"  {code}  ", "{code}", true},
		{"{code}trailing", "{code}", true},
		{"text{code}", "{code}", true},
		{"{code:java}", "{code}", false},
		{"random text", "{code}", false},
		{"{noformat}", "{noformat}", true},
		{"text{noformat}", "{noformat}", true},
	}
	for _, tt := range tests {
		got := isClosingTag(tt.line, tt.tag)
		if got != tt.want {
			t.Errorf("isClosingTag(%q, %q) = %v, want %v", tt.line, tt.tag, got, tt.want)
		}
	}
}

// --- renderHeading tests ---

func TestRenderHeading_Levels(t *testing.T) {
	tests := []struct {
		level    int
		contains string
	}{
		{1, "═"},
		{2, "─"},
		{3, "▸"},
		{4, "Test"},
	}
	for _, tt := range tests {
		got := renderHeading("Test", tt.level)
		stripped := stripANSI(got)
		if !strings.Contains(stripped, tt.contains) {
			t.Errorf("renderHeading(\"Test\", %d) = %q, want to contain %q", tt.level, stripped, tt.contains)
		}
	}
}

// --- renderLink tests ---

func TestRenderLink_PlainURL(t *testing.T) {
	got := renderLink("https://example.com", "")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "https://example.com") {
		t.Errorf("plain URL not rendered: %q", stripped)
	}
}

func TestRenderLink_AliasedURL(t *testing.T) {
	got := renderLink("Click", "https://example.com")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Click") {
		t.Errorf("alias not rendered: %q", stripped)
	}
	if !strings.Contains(stripped, "https://example.com") {
		t.Errorf("URL not rendered: %q", stripped)
	}
}

func TestRenderLink_UserMention(t *testing.T) {
	got := renderLink("~admin", "")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "@admin") {
		t.Errorf("user mention not rendered: %q", stripped)
	}
}

func TestRenderLink_InternalPage(t *testing.T) {
	got := renderLink("Some Page", "")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Some Page") {
		t.Errorf("internal page link not rendered: %q", stripped)
	}
}

func TestRenderLink_AliasedPage(t *testing.T) {
	got := renderLink("Display", "Some Page")
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "Display") {
		t.Errorf("aliased page link not rendered: %q", stripped)
	}
}

// --- mapColour tests ---

func TestMapColour_KnownColours(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"red", "#f7768e"},
		{"green", "#9ece6a"},
		{"blue", "#7aa2f7"},
		{"grey", "#565f89"},
	}
	for _, tt := range tests {
		got := mapColour(tt.name)
		if string(got) != tt.want {
			t.Errorf("mapColour(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestMapColour_Unknown(t *testing.T) {
	got := mapColour("fuschia")
	if string(got) != "" {
		t.Errorf("mapColour(\"fuschia\") = %q, want empty", got)
	}
}

func TestMapColour_HexPassthrough(t *testing.T) {
	got := mapColour("#ff00ff")
	if string(got) != "#ff00ff" {
		t.Errorf("mapColour(\"#ff00ff\") = %q, want #ff00ff", got)
	}
}

// --- isWordBoundary tests ---

func TestIsWordBoundary(t *testing.T) {
	tests := []struct {
		s    string
		pos  int
		want bool
	}{
		{"hello world", 5, true},  // space
		{"hello world", 0, false}, // 'h'
		{"hello.", 5, true},       // '.'
		{"hello", -1, true},       // before start
		{"hello", 5, true},        // after end
		{"hello", 2, false},       // 'l'
	}
	for _, tt := range tests {
		got := isWordBoundary(tt.s, tt.pos)
		if got != tt.want {
			t.Errorf("isWordBoundary(%q, %d) = %v, want %v", tt.s, tt.pos, got, tt.want)
		}
	}
}

// --- Table-related tests ---

func TestParseTableRow_Header(t *testing.T) {
	row := parseTableRow("||Name||Age||")
	if !row.isHeader {
		t.Error("expected header row")
	}
	if len(row.cells) != 2 || row.cells[0] != "Name" || row.cells[1] != "Age" {
		t.Errorf("parseTableRow header = %v, want [Name, Age]", row.cells)
	}
}

func TestParseTableRow_Data(t *testing.T) {
	row := parseTableRow("|Alice|30|")
	if row.isHeader {
		t.Error("expected data row")
	}
	if len(row.cells) != 2 || row.cells[0] != "Alice" || row.cells[1] != "30" {
		t.Errorf("parseTableRow data = %v, want [Alice, 30]", row.cells)
	}
}

func TestParseTable_Simple(t *testing.T) {
	lines := []string{"||A||B||", "|1|2|", "|3|4|"}
	got, consumed := parseTable(lines, 0, 80)
	if consumed != 3 {
		t.Errorf("parseTable consumed %d, want 3", consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "A") || !strings.Contains(stripped, "B") {
		t.Errorf("table headers not found: %q", stripped)
	}
	if !strings.Contains(stripped, "1") || !strings.Contains(stripped, "4") {
		t.Errorf("table data not found: %q", stripped)
	}
}

func TestParseTable_UnevenColumns(t *testing.T) {
	lines := []string{"||A||B||C||", "|1|2|"}
	got, consumed := parseTable(lines, 0, 80)
	if consumed != 2 {
		t.Errorf("parseTable consumed %d, want 2", consumed)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "A") {
		t.Errorf("table header not found: %q", stripped)
	}
}

func TestPadOrTruncate(t *testing.T) {
	tests := []struct {
		input string
		width int
		check func(string) bool
		desc  string
	}{
		{"hi", 10, func(s string) bool { return len(s) == 10 }, "should pad to 10"},
		{"hello world", 5, func(s string) bool { return strings.HasSuffix(s, "...") }, "should truncate with ellipsis"},
		{"exact", 5, func(s string) bool { return s == "exact" }, "exact width unchanged"},
	}
	for _, tt := range tests {
		got := padOrTruncate(tt.input, tt.width)
		if !tt.check(got) {
			t.Errorf("padOrTruncate(%q, %d): %s, got %q", tt.input, tt.width, tt.desc, got)
		}
	}
}
