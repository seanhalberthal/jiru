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
