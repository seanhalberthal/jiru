package jira

import "time"

// Issue represents a Jira issue in our domain.
type Issue struct {
	Key           string
	Summary       string
	Description   string
	Status        string
	StatusID      string
	Priority      string
	Assignee      string
	Reporter      string
	Labels        []string
	IssueType     string
	ParentKey     string // Parent issue key (e.g., "PROJ-42")
	ParentType    string // Parent's issue type name (e.g., "Epic", "Feature", "Initiative")
	ParentSummary string // Parent's summary (e.g., "User Authentication")
	Created       time.Time
	Updated       time.Time
	Comments      []Comment
}

// BoardColumn is unused but reserved for future board configuration support.
type BoardColumn struct {
	Name     string
	Statuses []string
}

// ChildIssue is a lightweight representation of a child/subtask issue.
type ChildIssue struct {
	Key       string
	Summary   string
	Status    string
	IssueType string
}

// Comment represents a comment on a Jira issue.
type Comment struct {
	Author  string
	Created time.Time
	Body    string
}

// Sprint represents an active sprint.
type Sprint struct {
	ID    int
	Name  string
	State string
	Goal  string
}

// Project represents a Jira project.
type Project struct {
	Key  string
	Name string
	Type string // "classic" or "next-gen" (team-managed).
}

// Board represents a Jira board.
type Board struct {
	ID   int
	Name string
	Type string // "scrum", "kanban", etc.
}

// BoardStats holds summary counts for a board's active sprint.
type BoardStats struct {
	Board        Board
	ActiveSprint string // Name of active sprint, empty if none
	OpenIssues   int
	InProgress   int
	DoneIssues   int
	TotalIssues  int
}

// SavedFilter represents a named, persisted JQL search filter.
type SavedFilter struct {
	ID        string    `yaml:"id"`
	Name      string    `yaml:"name"`
	JQL       string    `yaml:"jql"`
	Favourite bool      `yaml:"favourite"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
}

// Transition represents an available status transition for an issue.
type Transition struct {
	ID   string
	Name string
}

// JQLMetadata holds cached metadata for JQL autocompletion.
type JQLMetadata struct {
	Statuses         []string
	StatusCategories map[string]int // status name → category (0=todo, 1=in progress, 2=done)
	IssueTypes       []string
	Priorities       []string
	Resolutions      []string
	Projects         []string // project keys
	Labels           []string
	Components       []string // from configured project
	Versions         []string // from configured project
	Sprints          []string // sprint names from configured board
}
