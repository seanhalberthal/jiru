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

var jqlKeywords = []CompletionItem{
	{Label: "AND", Detail: "Logical AND", Kind: KindKeyword},
	{Label: "OR", Detail: "Logical OR", Kind: KindKeyword},
	{Label: "NOT", Detail: "Logical NOT", Kind: KindKeyword},
	{Label: "IN", Detail: "Value in list", Kind: KindKeyword},
	{Label: "NOT IN", Detail: "Value not in list", Kind: KindKeyword},
	{Label: "IS", Detail: "Field is value", Kind: KindKeyword},
	{Label: "IS NOT", Detail: "Field is not value", Kind: KindKeyword},
	{Label: "WAS", Detail: "Field was value (history)", Kind: KindKeyword},
	{Label: "WAS NOT", Detail: "Field was not value (history)", Kind: KindKeyword},
	{Label: "CHANGED", Detail: "Field changed", Kind: KindKeyword},
	{Label: "ORDER BY", Detail: "Sort results", Kind: KindKeyword},
	{Label: "ASC", Detail: "Ascending sort", Kind: KindKeyword},
	{Label: "DESC", Detail: "Descending sort", Kind: KindKeyword},
	{Label: "EMPTY", Detail: "Field is empty", Kind: KindKeyword},
	{Label: "NULL", Detail: "Field is null", Kind: KindKeyword},
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

// allCompletions is the merged catalogue.
var allCompletions []CompletionItem

func init() {
	allCompletions = make([]CompletionItem, 0,
		len(jqlFields)+len(jqlKeywords)+len(jqlFunctions)+len(jqlOperators))
	allCompletions = append(allCompletions, jqlFields...)
	allCompletions = append(allCompletions, jqlKeywords...)
	allCompletions = append(allCompletions, jqlFunctions...)
	allCompletions = append(allCompletions, jqlOperators...)
}

const maxCompletions = 10

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

// matchCompletions returns completions matching the given prefix.
func matchCompletions(prefix string) []CompletionItem {
	if prefix == "" {
		return nil
	}
	lower := strings.ToLower(prefix)
	var matches []CompletionItem
	for _, item := range allCompletions {
		if strings.HasPrefix(strings.ToLower(item.Label), lower) {
			matches = append(matches, item)
			if len(matches) >= maxCompletions {
				break
			}
		}
	}
	return matches
}
