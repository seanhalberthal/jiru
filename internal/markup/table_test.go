package markup

import (
	"strings"
	"testing"
)

func TestParseTableRow_DataRow(t *testing.T) {
	row := parseTableRow("|Alpha|Beta|Gamma|")
	if row.isHeader {
		t.Error("expected data row, got header")
	}
	if len(row.cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(row.cells))
	}
	want := []string{"Alpha", "Beta", "Gamma"}
	for i, w := range want {
		if row.cells[i] != w {
			t.Errorf("cell[%d] = %q, want %q", i, row.cells[i], w)
		}
	}
}

func TestParseTableRow_HeaderRow(t *testing.T) {
	row := parseTableRow("||Name||Age||City||")
	if !row.isHeader {
		t.Error("expected header row")
	}
	if len(row.cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(row.cells))
	}
	want := []string{"Name", "Age", "City"}
	for i, w := range want {
		if row.cells[i] != w {
			t.Errorf("cell[%d] = %q, want %q", i, row.cells[i], w)
		}
	}
}

func TestParseTableRow_CellsAreTrimmed(t *testing.T) {
	row := parseTableRow("| hello | world |")
	if len(row.cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(row.cells))
	}
	if row.cells[0] != "hello" || row.cells[1] != "world" {
		t.Errorf("cells = %v, want [hello, world]", row.cells)
	}
}

func TestRenderTable_SingleDataRow(t *testing.T) {
	rows := []tableRow{
		{cells: []string{"A", "B", "C"}, isHeader: false},
	}
	got := renderTable(rows, 80)
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") || !strings.Contains(got, "C") {
		t.Errorf("renderTable should contain all cells, got:\n%s", got)
	}
	// Should use box drawing characters.
	if !strings.Contains(got, "\u2502") { // │
		t.Error("renderTable should use box drawing separators")
	}
}

func TestRenderTable_HeaderWithSeparator(t *testing.T) {
	rows := []tableRow{
		{cells: []string{"Name", "Value"}, isHeader: true},
		{cells: []string{"foo", "bar"}, isHeader: false},
	}
	got := renderTable(rows, 80)

	// Header separator should be present.
	if !strings.Contains(got, "\u2500") { // ─
		t.Error("renderTable should have header separator line")
	}
	if !strings.Contains(got, "\u251c") { // ├
		t.Error("renderTable should have left separator junction")
	}
	if !strings.Contains(got, "\u2524") { // ┤
		t.Error("renderTable should have right separator junction")
	}
	if !strings.Contains(got, "\u253c") { // ┼
		t.Error("renderTable should have cross junction")
	}
}

func TestRenderTable_Empty(t *testing.T) {
	got := renderTable(nil, 80)
	if got != "" {
		t.Errorf("renderTable(nil) = %q, want empty", got)
	}
}

func TestRenderTable_UnevenCells(t *testing.T) {
	rows := []tableRow{
		{cells: []string{"A", "B", "C"}, isHeader: false},
		{cells: []string{"X"}, isHeader: false},
	}
	got := renderTable(rows, 80)
	// Should not panic and should contain all provided cells.
	if !strings.Contains(got, "A") || !strings.Contains(got, "X") {
		t.Errorf("renderTable should handle uneven rows, got:\n%s", got)
	}
}

func TestRenderTable_NarrowWidth(t *testing.T) {
	rows := []tableRow{
		{cells: []string{"A very long cell value", "Another long one"}, isHeader: false},
	}
	// Should not panic with a very narrow width.
	got := renderTable(rows, 20)
	if got == "" {
		t.Error("renderTable should produce output even with narrow width")
	}
}

func TestParseTable_ConsecutiveRows(t *testing.T) {
	lines := []string{
		"||Header1||Header2||",
		"|Value1|Value2|",
		"|Value3|Value4|",
		"Not a table line",
	}
	rendered, consumed := parseTable(lines, 0, 80)
	if consumed != 3 {
		t.Errorf("parseTable consumed %d lines, want 3", consumed)
	}
	if rendered == "" {
		t.Error("parseTable should produce non-empty output")
	}
	stripped := stripANSI(rendered)
	if !strings.Contains(stripped, "Header1") {
		t.Error("output should contain header text")
	}
	if !strings.Contains(stripped, "Value4") {
		t.Error("output should contain last data cell")
	}
}

func TestParseTable_NoTableLines(t *testing.T) {
	lines := []string{"Just plain text", "No tables here"}
	rendered, consumed := parseTable(lines, 0, 80)
	if consumed != 0 {
		t.Errorf("parseTable consumed %d lines, want 0", consumed)
	}
	if rendered != "" {
		t.Errorf("parseTable = %q, want empty", rendered)
	}
}

func TestParseTable_StartsFromOffset(t *testing.T) {
	lines := []string{
		"Not a table",
		"|A|B|",
		"|C|D|",
	}
	rendered, consumed := parseTable(lines, 1, 80)
	if consumed != 2 {
		t.Errorf("parseTable consumed %d lines, want 2", consumed)
	}
	if rendered == "" {
		t.Error("parseTable should produce output starting from offset")
	}
}

func TestPadOrTruncate_Pad(t *testing.T) {
	got := padOrTruncate("hi", 10)
	if len(got) != 10 {
		t.Errorf("padOrTruncate length = %d, want 10", len(got))
	}
	if !strings.HasPrefix(got, "hi") {
		t.Errorf("padOrTruncate = %q, want prefix 'hi'", got)
	}
}

func TestPadOrTruncate_ExactWidth(t *testing.T) {
	got := padOrTruncate("hello", 5)
	if got != "hello" {
		t.Errorf("padOrTruncate = %q, want %q", got, "hello")
	}
}

func TestPadOrTruncate_Truncate(t *testing.T) {
	got := padOrTruncate("a long string that needs truncating", 10)
	if len(got) > 10 {
		t.Errorf("padOrTruncate length = %d, want <= 10", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("padOrTruncate = %q, want ... suffix", got)
	}
}

func TestPadOrTruncate_VeryShortWidth(t *testing.T) {
	// Width <= 3 should not attempt truncation with ellipsis.
	got := padOrTruncate("hello", 3)
	// With width=3, the string is wider but the truncation guard (width > 3) is false,
	// so it falls through and returns the original string.
	if got != "hello" {
		t.Errorf("padOrTruncate = %q, want %q (width too small for truncation)", got, "hello")
	}
}

func TestRender_TableIntegration(t *testing.T) {
	input := "||Name||Score||\n|Alice|95|\n|Bob|87|"
	got := Render(input, 80)
	stripped := stripANSI(got)

	if !strings.Contains(stripped, "Name") {
		t.Error("rendered table should contain 'Name'")
	}
	if !strings.Contains(stripped, "Alice") {
		t.Error("rendered table should contain 'Alice'")
	}
	if !strings.Contains(stripped, "87") {
		t.Error("rendered table should contain '87'")
	}
}
