// Package jql provides a JQL completion engine: context-aware parsing,
// dynamic value matching, and popup rendering for terminal UIs.
package jql

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/theme"
)

// Kind categorises a completion item for display.
type Kind int

const (
	KindField Kind = iota
	KindKeyword
	KindFunction
	KindOperator
	KindValue
)

// Item is a single autocomplete suggestion.
type Item struct {
	Label      string // What gets inserted (e.g., "assignee")
	Detail     string // Short description shown beside the label
	Kind       Kind   // Category for icon/colour
	InsertText string // If non-empty, inserted instead of Label (e.g., "currentUser()")
}

// String returns the text to insert.
func (c Item) String() string {
	if c.InsertText != "" {
		return c.InsertText
	}
	return c.Label
}

// KindLabel returns a short prefix for display (like LSP kind icons).
func (k Kind) KindLabel() string {
	switch k {
	case KindField:
		return "field"
	case KindKeyword:
		return "kw"
	case KindFunction:
		return "fn"
	case KindOperator:
		return "op"
	case KindValue:
		return "val"
	default:
		return ""
	}
}

// Context determines what kind of completion to offer.
type Context int

const (
	CtxField    Context = iota // Expecting a field name
	CtxOperator                // Expecting an operator
	CtxValue                   // Expecting a value (field is known)
	CtxKeyword                 // Expecting AND/OR/ORDER BY after a complete clause
	CtxOrderBy                 // Expecting a field after ORDER BY
)

// ParseResult holds the parsed JQL context at a cursor position.
type ParseResult struct {
	Context Context
	Field   string // The field name when context is CtxValue
	Prefix  string // What the user has typed so far for the current token
}

// MaxCompletions is the maximum number of completion items shown.
const MaxCompletions = 10

// --- Static catalogues ---

var jqlFields = []Item{
	{Label: "assignee", Detail: "Issue assignee", Kind: KindField},
	{Label: "reporter", Detail: "Issue reporter", Kind: KindField},
	{Label: "status", Detail: "Issue status", Kind: KindField},
	{Label: "statusCategory", Detail: "Status category (To Do, In Progress, Done)", Kind: KindField},
	{Label: "project", Detail: "Project key", Kind: KindField},
	{Label: "issuetype", Detail: "Issue type (Bug, Story, Task, ...)", Kind: KindField},
	{Label: "priority", Detail: "Issue priority", Kind: KindField},
	{Label: "labels", Detail: "Issue labels", Kind: KindField},
	{Label: "fixVersion", Detail: "Fix version", Kind: KindField},
	{Label: "affectedVersion", Detail: "Affected version", Kind: KindField},
	{Label: "component", Detail: "Project component", Kind: KindField},
	{Label: "resolution", Detail: "Issue resolution", Kind: KindField},
	{Label: "sprint", Detail: "Sprint name or ID", Kind: KindField},
	{Label: "epic", Detail: "Epic link", Kind: KindField},
	{Label: "parent", Detail: "Parent issue", Kind: KindField},
	{Label: "created", Detail: "Date created", Kind: KindField},
	{Label: "updated", Detail: "Date last updated", Kind: KindField},
	{Label: "resolved", Detail: "Date resolved", Kind: KindField},
	{Label: "due", Detail: "Due date", Kind: KindField},
	{Label: "summary", Detail: "Issue summary text", Kind: KindField},
	{Label: "description", Detail: "Issue description text", Kind: KindField},
	{Label: "text", Detail: "Full-text search across fields", Kind: KindField},
	{Label: "key", Detail: "Issue key (e.g. PROJ-123)", Kind: KindField},
	{Label: "type", Detail: "Alias for issuetype", Kind: KindField},
	{Label: "watcher", Detail: "Issue watchers", Kind: KindField},
	{Label: "voter", Detail: "Issue voters", Kind: KindField},
}

var jqlLogicalKeywords = []Item{
	{Label: "AND", Detail: "Logical AND", Kind: KindKeyword},
	{Label: "OR", Detail: "Logical OR", Kind: KindKeyword},
	{Label: "NOT", Detail: "Logical NOT", Kind: KindKeyword},
	{Label: "ORDER BY", Detail: "Sort results", Kind: KindKeyword},
}

var operatorKeywords = []Item{
	{Label: "IN", Detail: "Value in list", Kind: KindOperator},
	{Label: "NOT IN", Detail: "Value not in list", Kind: KindOperator},
	{Label: "IS", Detail: "Field is value", Kind: KindOperator},
	{Label: "IS NOT", Detail: "Field is not value", Kind: KindOperator},
	{Label: "WAS", Detail: "Field was value (history)", Kind: KindOperator},
	{Label: "WAS NOT", Detail: "Field was not value (history)", Kind: KindOperator},
	{Label: "CHANGED", Detail: "Field changed", Kind: KindOperator},
}

var valueKeywords = []Item{
	{Label: "EMPTY", Detail: "Field is empty", Kind: KindKeyword},
	{Label: "NULL", Detail: "Field is null", Kind: KindKeyword},
}

var sortKeywords = []Item{
	{Label: "ASC", Detail: "Ascending sort", Kind: KindKeyword},
	{Label: "DESC", Detail: "Descending sort", Kind: KindKeyword},
}

var jqlFunctions = []Item{
	{Label: "currentUser()", Detail: "Logged-in user", Kind: KindFunction, InsertText: "currentUser()"},
	{Label: "membersOf()", Detail: "Members of a group", Kind: KindFunction, InsertText: "membersOf()"},
	{Label: "now()", Detail: "Current date/time", Kind: KindFunction, InsertText: "now()"},
	{Label: "startOfDay()", Detail: "Start of today", Kind: KindFunction, InsertText: "startOfDay()"},
	{Label: "endOfDay()", Detail: "End of today", Kind: KindFunction, InsertText: "endOfDay()"},
	{Label: "startOfWeek()", Detail: "Start of this week", Kind: KindFunction, InsertText: "startOfWeek()"},
	{Label: "endOfWeek()", Detail: "End of this week", Kind: KindFunction, InsertText: "endOfWeek()"},
	{Label: "startOfMonth()", Detail: "Start of this month", Kind: KindFunction, InsertText: "startOfMonth()"},
	{Label: "endOfMonth()", Detail: "End of this month", Kind: KindFunction, InsertText: "endOfMonth()"},
	{Label: "startOfYear()", Detail: "Start of this year", Kind: KindFunction, InsertText: "startOfYear()"},
	{Label: "endOfYear()", Detail: "End of this year", Kind: KindFunction, InsertText: "endOfYear()"},
}

var jqlOperators = []Item{
	{Label: "=", Detail: "Equals", Kind: KindOperator},
	{Label: "!=", Detail: "Not equals", Kind: KindOperator},
	{Label: ">", Detail: "Greater than", Kind: KindOperator},
	{Label: ">=", Detail: "Greater than or equal", Kind: KindOperator},
	{Label: "<", Detail: "Less than", Kind: KindOperator},
	{Label: "<=", Detail: "Less than or equal", Kind: KindOperator},
	{Label: "~", Detail: "Contains text", Kind: KindOperator},
	{Label: "!~", Detail: "Does not contain text", Kind: KindOperator},
}

// --- Field/operator sets ---

var fieldSet map[string]bool

func init() {
	fieldSet = make(map[string]bool, len(jqlFields))
	for _, f := range jqlFields {
		fieldSet[strings.ToLower(f.Label)] = true
	}
}

func isField(tok string) bool {
	return fieldSet[strings.ToLower(tok)]
}

var operatorSet = map[string]bool{
	"=": true, "!=": true, ">": true, ">=": true,
	"<": true, "<=": true, "~": true, "!~": true,
}

func isOperator(tok string) bool {
	return operatorSet[tok]
}

// --- Tokeniser ---

// tokenise splits input into tokens, respecting quoted strings.
func tokenise(input string) []string {
	var tokens []string
	i := 0
	for i < len(input) {
		ch := input[i]
		if ch == ' ' || ch == '\t' {
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			quote := ch
			j := i + 1
			for j < len(input) && input[j] != quote {
				j++
			}
			if j < len(input) {
				j++
			}
			tokens = append(tokens, input[i:j])
			i = j
			continue
		}
		if ch == '(' || ch == ')' || ch == ',' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		if i+1 < len(input) {
			two := input[i : i+2]
			if two == "!=" || two == ">=" || two == "<=" || two == "!~" {
				tokens = append(tokens, two)
				i += 2
				continue
			}
		}
		if ch == '=' || ch == '>' || ch == '<' || ch == '~' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		j := i
		for j < len(input) {
			c := input[j]
			if c == ' ' || c == '\t' || c == '(' || c == ')' || c == ',' ||
				c == '=' || c == '>' || c == '<' || c == '~' || c == '!' ||
				c == '"' || c == '\'' {
				break
			}
			j++
		}
		if j > i {
			tokens = append(tokens, input[i:j])
			i = j
		} else {
			i++
		}
	}
	return tokens
}

// --- Context parsing ---

// Parse determines the completion context at the cursor position.
func Parse(input string, cursor int) ParseResult {
	if cursor > len(input) {
		cursor = len(input)
	}
	before := input[:cursor]
	tokens := tokenise(before)
	prefix, _ := CurrentWord(input, cursor)
	prefix = strings.TrimLeft(prefix, "\"'")

	if len(tokens) == 0 {
		return ParseResult{Context: CtxField, Prefix: prefix}
	}

	state := CtxField
	var lastField string
	parenDepth := 0
	prevState := state

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		upper := strings.ToUpper(tok)
		prevState = state

		switch state {
		case CtxField, CtxOrderBy:
			if upper == "ORDER" {
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "BY" {
					i++
					state = CtxOrderBy
					continue
				}
			}
			if upper == "NOT" {
				continue
			}
			if isField(tok) {
				lastField = strings.ToLower(tok)
				state = CtxOperator
			}

		case CtxOperator:
			if isOperator(tok) {
				state = CtxValue
			} else if upper == "IN" || upper == "NOT" || upper == "IS" || upper == "WAS" || upper == "CHANGED" {
				if upper == "NOT" && i+1 < len(tokens) {
					next := strings.ToUpper(tokens[i+1])
					if next == "IN" {
						i++
						state = CtxValue
						continue
					}
				}
				if upper == "IS" && i+1 < len(tokens) {
					next := strings.ToUpper(tokens[i+1])
					if next == "NOT" {
						i++
					}
				}
				if upper == "WAS" && i+1 < len(tokens) {
					next := strings.ToUpper(tokens[i+1])
					if next == "NOT" {
						i++
					}
				}
				if upper == "CHANGED" {
					state = CtxKeyword
					continue
				}
				state = CtxValue
			}

		case CtxValue:
			if tok == "(" {
				parenDepth++
				continue
			}
			if tok == ")" {
				parenDepth--
				if parenDepth <= 0 {
					parenDepth = 0
					state = CtxKeyword
				}
				continue
			}
			if tok == "," && parenDepth > 0 {
				continue
			}
			if parenDepth > 0 {
				continue
			}
			if upper == "AND" || upper == "OR" || upper == "ORDER" {
				state = CtxField
				lastField = ""
				if upper == "ORDER" && i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "BY" {
					i++
					state = CtxOrderBy
				}
			} else {
				state = CtxKeyword
			}

		case CtxKeyword:
			if upper == "AND" || upper == "OR" || upper == "NOT" {
				state = CtxField
				lastField = ""
			} else if upper == "ORDER" {
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "BY" {
					i++
					state = CtxOrderBy
				} else {
					state = CtxField
				}
				lastField = ""
			} else if upper == "ASC" || upper == "DESC" {
				continue
			}
		}
	}

	endsWithSpace := len(before) > 0 && (before[len(before)-1] == ' ' || before[len(before)-1] == '\t')

	if !endsWithSpace && len(tokens) > 0 {
		lastTok := tokens[len(tokens)-1]
		lastUpper := strings.ToUpper(lastTok)

		switch state {
		case CtxOperator:
			if isField(lastTok) {
				state = CtxField
				lastField = ""
			}
		case CtxValue:
			if isOperator(lastTok) || lastUpper == "IN" || lastUpper == "IS" || lastUpper == "WAS" {
				state = CtxOperator
			}
		case CtxKeyword:
			if prevState != CtxKeyword {
				if parenDepth > 0 {
					state = CtxValue
				} else if !isLogicalKeyword(lastUpper) {
					state = CtxValue
				}
			}
		case CtxField:
			if isLogicalKeyword(lastUpper) {
				state = CtxKeyword
			}
		}
	}

	return ParseResult{Context: state, Field: lastField, Prefix: prefix}
}

func isLogicalKeyword(upper string) bool {
	return upper == "AND" || upper == "OR" || upper == "NOT" || upper == "ORDER"
}

// --- Value provider ---

// ValueProvider supplies dynamic completion values from the Jira instance.
type ValueProvider struct {
	Statuses    []string
	IssueTypes  []string
	Priorities  []string
	Resolutions []string
	Projects    []string
	Labels      []string
	Components  []string
	Versions    []string
	Sprints     []string
	Users       []string // Populated by user search results.
}

// ValuesForField returns the known values for a JQL field.
func (vp *ValueProvider) ValuesForField(field string) []Item {
	var vals []string
	switch field {
	case "status":
		vals = vp.Statuses
	case "issuetype", "type":
		vals = vp.IssueTypes
	case "priority":
		vals = vp.Priorities
	case "resolution":
		vals = vp.Resolutions
	case "project":
		vals = vp.Projects
	case "labels":
		vals = vp.Labels
	case "component":
		vals = vp.Components
	case "fixversion", "affectedversion":
		vals = vp.Versions
	case "sprint":
		vals = vp.Sprints
	case "assignee", "reporter":
		vals = vp.Users
	default:
		return nil
	}

	items := make([]Item, 0, len(vals))
	for _, v := range vals {
		items = append(items, Item{
			Label:      v,
			Detail:     field + " value",
			Kind:       KindValue,
			InsertText: QuoteIfNeeded(v),
		})
	}
	return items
}

// QuoteIfNeeded wraps values containing spaces in double quotes for JQL.
func QuoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t") {
		return "\"" + s + "\""
	}
	return s
}

// --- Completion matching ---

// CurrentWord extracts the word being typed at the cursor position.
// Returns the word and its start index in the input string.
func CurrentWord(input string, cursor int) (word string, start int) {
	if cursor > len(input) {
		cursor = len(input)
	}

	before := input[:cursor]
	inQuote := false
	quotePos := 0
	for i := 0; i < len(before); i++ {
		ch := before[i]
		if ch == '"' || ch == '\'' {
			if inQuote && before[quotePos] == ch {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quotePos = i
			}
		}
	}

	if inQuote {
		return input[quotePos:cursor], quotePos
	}

	start = cursor
	for start > 0 {
		ch := input[start-1]
		if ch == ' ' || ch == '(' || ch == ')' || ch == ',' {
			break
		}
		start--
	}
	return input[start:cursor], start
}

// Match returns completions based on cursor context and dynamic values.
func Match(ctx ParseResult, values *ValueProvider) []Item {
	var candidates []Item

	switch ctx.Context {
	case CtxField:
		candidates = jqlFields
	case CtxOrderBy:
		candidates = jqlFields
		candidates = append(candidates, sortKeywords...)
	case CtxOperator:
		candidates = make([]Item, 0, len(jqlOperators)+len(operatorKeywords))
		candidates = append(candidates, jqlOperators...)
		candidates = append(candidates, operatorKeywords...)
	case CtxValue:
		if values != nil {
			candidates = values.ValuesForField(ctx.Field)
		}
		candidates = append(candidates, jqlFunctions...)
		candidates = append(candidates, valueKeywords...)
	case CtxKeyword:
		candidates = jqlLogicalKeywords
	}

	if ctx.Prefix == "" {
		if len(candidates) > MaxCompletions {
			return candidates[:MaxCompletions]
		}
		return candidates
	}

	return filterByPrefix(candidates, ctx.Prefix)
}

func filterByPrefix(items []Item, prefix string) []Item {
	lower := strings.ToLower(prefix)
	var matches []Item
	for _, item := range items {
		if strings.HasPrefix(strings.ToLower(item.Label), lower) {
			matches = append(matches, item)
			if len(matches) >= MaxCompletions {
				break
			}
		}
	}
	return matches
}

// --- Accept helper ---

// Accept inserts a completion item into the input value at the cursor position.
// Returns the new input value and cursor position.
func Accept(inputValue string, cursor int, item Item) (newValue string, newCursor int) {
	_, start := CurrentWord(inputValue, cursor)
	insertText := item.String()
	newValue = inputValue[:start] + insertText + inputValue[cursor:]
	newCursor = start + len(insertText)
	return newValue, newCursor
}

// --- Popup rendering ---

// RenderPopup renders the completion popup as a styled string.
func RenderPopup(items []Item, selected int) string {
	normalStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1).
		Background(theme.ColourPrimary).
		Foreground(lipgloss.Color("#000000"))

	kindStyle := lipgloss.NewStyle().
		Foreground(theme.ColourSubtle).
		PaddingRight(1)

	detailStyle := lipgloss.NewStyle().
		Foreground(theme.ColourSubtle)

	var rows []string
	for i, item := range items {
		kind := kindStyle.Render(item.Kind.KindLabel())
		detail := detailStyle.Render(item.Detail)
		line := fmt.Sprintf("%s %-20s %s", kind, item.Label, detail)

		if i == selected {
			rows = append(rows, selectedStyle.Render(line))
		} else {
			rows = append(rows, normalStyle.Render(line))
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourSubtle).
		Render(strings.Join(rows, "\n"))
}
