package searchview

import "strings"

// CompletionKind categorises a completion item for display.
type CompletionKind int

const (
	KindField CompletionKind = iota
	KindKeyword
	KindFunction
	KindOperator
	KindValue
)

// CompletionItem is a single autocomplete suggestion.
type CompletionItem struct {
	Label      string         // What gets inserted (e.g., "assignee")
	Detail     string         // Short description shown beside the label
	Kind       CompletionKind // Category for icon/colour
	InsertText string         // If non-empty, inserted instead of Label (e.g., "currentUser()")
}

// String returns the text to insert.
func (c CompletionItem) String() string {
	if c.InsertText != "" {
		return c.InsertText
	}
	return c.Label
}

// KindLabel returns a short prefix for display (like LSP kind icons).
func (k CompletionKind) KindLabel() string {
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

// --- Static catalogues ---

var jqlFields = []CompletionItem{
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

// jqlLogicalKeywords are offered after a complete clause (field op value).
var jqlLogicalKeywords = []CompletionItem{
	{Label: "AND", Detail: "Logical AND", Kind: KindKeyword},
	{Label: "OR", Detail: "Logical OR", Kind: KindKeyword},
	{Label: "NOT", Detail: "Logical NOT", Kind: KindKeyword},
	{Label: "ORDER BY", Detail: "Sort results", Kind: KindKeyword},
}

// operatorKeywords are keyword-style operators offered in operator position.
var operatorKeywords = []CompletionItem{
	{Label: "IN", Detail: "Value in list", Kind: KindOperator},
	{Label: "NOT IN", Detail: "Value not in list", Kind: KindOperator},
	{Label: "IS", Detail: "Field is value", Kind: KindOperator},
	{Label: "IS NOT", Detail: "Field is not value", Kind: KindOperator},
	{Label: "WAS", Detail: "Field was value (history)", Kind: KindOperator},
	{Label: "WAS NOT", Detail: "Field was not value (history)", Kind: KindOperator},
	{Label: "CHANGED", Detail: "Field changed", Kind: KindOperator},
}

// valueKeywords are offered in value position alongside dynamic values.
var valueKeywords = []CompletionItem{
	{Label: "EMPTY", Detail: "Field is empty", Kind: KindKeyword},
	{Label: "NULL", Detail: "Field is null", Kind: KindKeyword},
}

// sortKeywords are offered after ORDER BY field.
var sortKeywords = []CompletionItem{
	{Label: "ASC", Detail: "Ascending sort", Kind: KindKeyword},
	{Label: "DESC", Detail: "Descending sort", Kind: KindKeyword},
}

var jqlFunctions = []CompletionItem{
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

var jqlOperators = []CompletionItem{
	{Label: "=", Detail: "Equals", Kind: KindOperator},
	{Label: "!=", Detail: "Not equals", Kind: KindOperator},
	{Label: ">", Detail: "Greater than", Kind: KindOperator},
	{Label: ">=", Detail: "Greater than or equal", Kind: KindOperator},
	{Label: "<", Detail: "Less than", Kind: KindOperator},
	{Label: "<=", Detail: "Less than or equal", Kind: KindOperator},
	{Label: "~", Detail: "Contains text", Kind: KindOperator},
	{Label: "!~", Detail: "Does not contain text", Kind: KindOperator},
}

const maxCompletions = 10

// --- JQL field set (for context detection) ---

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

// --- Cursor context detection ---

// cursorContext determines what kind of completion to offer.
type cursorContext int

const (
	ctxField    cursorContext = iota // Expecting a field name
	ctxOperator                      // Expecting an operator
	ctxValue                         // Expecting a value (field is known)
	ctxKeyword                       // Expecting AND/OR/ORDER BY after a complete clause
	ctxOrderBy                       // Expecting a field after ORDER BY
)

type parseResult struct {
	context cursorContext
	field   string // The field name when context is ctxValue
	prefix  string // What the user has typed so far for the current token
}

// tokenise splits input into tokens, respecting quoted strings.
// Parentheses and commas are separate tokens. Multi-character operators
// (!=, >=, <=, !~) are kept together.
func tokenise(input string) []string {
	var tokens []string
	i := 0
	for i < len(input) {
		ch := input[i]
		// Skip whitespace.
		if ch == ' ' || ch == '\t' {
			i++
			continue
		}
		// Quoted string.
		if ch == '"' || ch == '\'' {
			quote := ch
			j := i + 1
			for j < len(input) && input[j] != quote {
				j++
			}
			if j < len(input) {
				j++ // include closing quote
			}
			tokens = append(tokens, input[i:j])
			i = j
			continue
		}
		// Parentheses and commas.
		if ch == '(' || ch == ')' || ch == ',' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		// Two-character operators.
		if i+1 < len(input) {
			two := input[i : i+2]
			if two == "!=" || two == ">=" || two == "<=" || two == "!~" {
				tokens = append(tokens, two)
				i += 2
				continue
			}
		}
		// Single-character operators.
		if ch == '=' || ch == '>' || ch == '<' || ch == '~' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		// Word token.
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

// parseJQLContext determines the completion context at the cursor position.
func parseJQLContext(input string, cursor int) parseResult {
	if cursor > len(input) {
		cursor = len(input)
	}
	before := input[:cursor]
	tokens := tokenise(before)
	prefix, _ := currentWord(input, cursor)

	if len(tokens) == 0 {
		return parseResult{context: ctxField, prefix: prefix}
	}

	// Walk tokens to determine state.
	state := ctxField
	var lastField string
	parenDepth := 0
	prevState := state // Track state before processing each token.

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		upper := strings.ToUpper(tok)
		prevState = state

		switch state {
		case ctxField, ctxOrderBy:
			if upper == "ORDER" {
				// Look ahead for "BY".
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "BY" {
					i++ // skip "BY"
					state = ctxOrderBy
					continue
				}
			}
			if upper == "NOT" {
				// NOT can be a prefix in field position (NOT field ...) — stay in ctxField.
				continue
			}
			if isField(tok) {
				lastField = strings.ToLower(tok)
				state = ctxOperator
			}
			// If we don't recognise it as a field, it might be a value token
			// (e.g. partial typing) — stay in current state.

		case ctxOperator:
			if isOperator(tok) {
				state = ctxValue
			} else if upper == "IN" || upper == "NOT" || upper == "IS" || upper == "WAS" || upper == "CHANGED" {
				// Keyword operators: IN, NOT IN, IS, IS NOT, WAS, WAS NOT, CHANGED.
				if upper == "NOT" && i+1 < len(tokens) {
					next := strings.ToUpper(tokens[i+1])
					if next == "IN" {
						i++ // skip "IN"
						state = ctxValue
						continue
					}
				}
				if upper == "IS" && i+1 < len(tokens) {
					next := strings.ToUpper(tokens[i+1])
					if next == "NOT" {
						i++ // skip "NOT"
					}
				}
				if upper == "WAS" && i+1 < len(tokens) {
					next := strings.ToUpper(tokens[i+1])
					if next == "NOT" {
						i++ // skip "NOT"
					}
				}
				if upper == "CHANGED" {
					state = ctxKeyword
					continue
				}
				state = ctxValue
			}

		case ctxValue:
			if tok == "(" {
				parenDepth++
				continue
			}
			if tok == ")" {
				parenDepth--
				if parenDepth <= 0 {
					parenDepth = 0
					state = ctxKeyword
				}
				continue
			}
			if tok == "," && parenDepth > 0 {
				continue // Stay in value context for next IN element.
			}
			if parenDepth > 0 {
				continue // Value inside IN (...).
			}
			// We've consumed a value — move to keyword context.
			if upper == "AND" || upper == "OR" || upper == "ORDER" {
				state = ctxField
				lastField = ""
				if upper == "ORDER" && i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "BY" {
					i++
					state = ctxOrderBy
				}
			} else {
				state = ctxKeyword
			}

		case ctxKeyword:
			if upper == "AND" || upper == "OR" || upper == "NOT" {
				state = ctxField
				lastField = ""
			} else if upper == "ORDER" {
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "BY" {
					i++
					state = ctxOrderBy
				} else {
					state = ctxField
				}
				lastField = ""
			} else if upper == "ASC" || upper == "DESC" {
				// After sort direction, expect comma or end.
				continue
			}
		}
	}

	// If the prefix matches the last token, determine if it's a complete token
	// or partial — only matters for context accuracy.
	// If before ends with whitespace, the prefix is empty and we're past the last token.
	endsWithSpace := len(before) > 0 && (before[len(before)-1] == ' ' || before[len(before)-1] == '\t')

	if !endsWithSpace && len(tokens) > 0 {
		// The last token IS what the user is currently typing (the prefix).
		// The state machine advanced past it, so we need to "rewind" one step.
		lastTok := tokens[len(tokens)-1]
		lastUpper := strings.ToUpper(lastTok)

		switch state {
		case ctxOperator:
			// State advanced because last token was recognised as a field.
			// But the user is still typing it — stay in field context.
			if isField(lastTok) {
				state = ctxField
				lastField = ""
			}
		case ctxValue:
			// State advanced because last token was an operator.
			if isOperator(lastTok) || lastUpper == "IN" || lastUpper == "IS" || lastUpper == "WAS" {
				state = ctxOperator
			}
		case ctxKeyword:
			// Only rewind if the state was actually advanced TO ctxKeyword
			// by processing this token (i.e. the token was a value).
			// If state was already ctxKeyword before this token, the user
			// is typing a partial keyword — stay in ctxKeyword.
			if prevState != ctxKeyword {
				if parenDepth > 0 {
					state = ctxValue
				} else if !isLogicalKeyword(lastUpper) {
					state = ctxValue
				}
			}
		case ctxField:
			// If we got here via AND/OR, the user is typing the keyword itself.
			if isLogicalKeyword(lastUpper) {
				state = ctxKeyword
			}
		}
	}

	return parseResult{context: state, field: lastField, prefix: prefix}
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
func (vp *ValueProvider) ValuesForField(field string) []CompletionItem {
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

	items := make([]CompletionItem, 0, len(vals))
	for _, v := range vals {
		items = append(items, CompletionItem{
			Label:      v,
			Detail:     field + " value",
			Kind:       KindValue,
			InsertText: quoteIfNeeded(v),
		})
	}
	return items
}

// quoteIfNeeded wraps values containing spaces in double quotes for JQL.
func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t") {
		return "\"" + s + "\""
	}
	return s
}

// --- Completion matching ---

// currentWord extracts the word being typed at the cursor position.
// Returns the word and its start index in the input string.
func currentWord(input string, cursor int) (word string, start int) {
	if cursor > len(input) {
		cursor = len(input)
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

// matchCompletions returns completions based on cursor context and dynamic values.
func matchCompletions(ctx parseResult, values *ValueProvider) []CompletionItem {
	var candidates []CompletionItem

	switch ctx.context {
	case ctxField:
		candidates = jqlFields
	case ctxOrderBy:
		candidates = jqlFields
		candidates = append(candidates, sortKeywords...)
	case ctxOperator:
		candidates = make([]CompletionItem, 0, len(jqlOperators)+len(operatorKeywords))
		candidates = append(candidates, jqlOperators...)
		candidates = append(candidates, operatorKeywords...)
	case ctxValue:
		if values != nil {
			candidates = values.ValuesForField(ctx.field)
		}
		candidates = append(candidates, jqlFunctions...)
		candidates = append(candidates, valueKeywords...)
	case ctxKeyword:
		candidates = jqlLogicalKeywords
	}

	if ctx.prefix == "" {
		// For value context, show all candidates (up to max).
		if ctx.context == ctxValue {
			if len(candidates) > maxCompletions {
				return candidates[:maxCompletions]
			}
			return candidates
		}
		return nil
	}

	return filterByPrefix(candidates, ctx.prefix)
}

func filterByPrefix(items []CompletionItem, prefix string) []CompletionItem {
	lower := strings.ToLower(prefix)
	var matches []CompletionItem
	for _, item := range items {
		if strings.HasPrefix(strings.ToLower(item.Label), lower) {
			matches = append(matches, item)
			if len(matches) >= maxCompletions {
				break
			}
		}
	}
	return matches
}
