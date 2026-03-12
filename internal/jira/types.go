package jira

import "time"

// Issue represents a Jira issue in our domain.
type Issue struct {
	Key         string
	Summary     string
	Description string
	Status      string
	StatusID    string
	Priority    string
	Assignee    string
	Reporter    string
	Labels      []string
	IssueType   string
	Created     time.Time
	Updated     time.Time
	Comments    []Comment
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
