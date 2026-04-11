package jira

import (
	"strings"
	"time"
)

// Issue represents a Jira issue in our domain.
type Issue struct {
	Key           string    `json:"key"`
	Summary       string    `json:"summary"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	Priority      string    `json:"priority"`
	Assignee      string    `json:"assignee"`
	Reporter      string    `json:"reporter"`
	Labels        []string  `json:"labels"`
	IssueType     string    `json:"issue_type"`
	ParentKey     string    `json:"parent_key,omitempty"`
	ParentType    string    `json:"parent_type,omitempty"`
	ParentSummary string    `json:"parent_summary,omitempty"`
	Created       time.Time `json:"created"`
	Updated       time.Time `json:"updated"`
	Comments      []Comment `json:"comments"`
	IsWatching    bool      `json:"is_watching"`
}

// ChildIssue is a lightweight representation of a child/subtask issue.
type ChildIssue struct {
	Key       string `json:"key"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	IssueType string `json:"issue_type"`
}

// Comment represents a comment on a Jira issue.
type Comment struct {
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
	Body    string    `json:"body"`
}

// Sprint represents an active sprint.
type Sprint struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Goal  string `json:"goal"`
}

// Project represents a Jira project.
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Board represents a Jira board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// SavedFilter represents a named, persisted JQL search filter.
type SavedFilter struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	JQL       string    `json:"jql"`
	Favourite bool      `json:"favourite"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IssueLinkType represents a type of link between two issues.
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// BranchInfo holds git branch information related to a Jira issue.
type BranchInfo struct {
	Name         string `json:"name"`
	RemoteCommit int    `json:"remote_commit"`
}

// Transition represents an available status transition for an issue.
type Transition struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToStatus string `json:"to_status"`
}

// JQLMetadata holds cached metadata for JQL autocompletion.
type JQLMetadata struct {
	Statuses         []string       `json:"statuses"`
	StatusCategories map[string]int `json:"status_categories"`
	IssueTypes       []string       `json:"issue_types"`
	Priorities       []string       `json:"priorities"`
	Resolutions      []string       `json:"resolutions"`
	Projects         []string       `json:"projects"`
	Labels           []string       `json:"labels"`
	Components       []string       `json:"components"`
	Versions         []string       `json:"versions"`
	Sprints          []string       `json:"sprints"`
}

// RemoteLink represents an external link on a Jira issue.
type RemoteLink struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Icon  string `json:"icon"` // e.g. "confluence"
}

// IssueTypeInfo holds an issue type's ID and display name.
type IssueTypeInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CustomFieldDef describes a custom field available on an issue type.
type CustomFieldDef struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	FieldType     string   `json:"field_type"`
	Required      bool     `json:"required"`
	AllowedValues []string `json:"allowed_values"`
}

// UserInfo holds user display name and account ID from search results.
type UserInfo struct {
	AccountID   string
	DisplayName string
}

// IsCancelledName returns true if the status name suggests a cancelled/rejected state.
func IsCancelledName(name string) bool {
	lower := strings.ToLower(name)
	for _, kw := range []string{"cancel", "won't do", "reject", "decline", "obsolete"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
