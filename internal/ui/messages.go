package ui

import (
	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

// ClientReadyMsg is sent when the API client is initialised and auth verified.
type ClientReadyMsg struct {
	Client      client.JiraClient
	DisplayName string
}

// SprintLoadedMsg is sent when the active sprint is fetched.
type SprintLoadedMsg struct {
	Sprint *jira.Sprint
}

// IssuesLoadedMsg is sent when sprint issues are fetched.
type IssuesLoadedMsg struct {
	Issues []jira.Issue
	Title  string // Context title: sprint name, board name, or project key.
	// Pagination fields (zero values mean no more pages).
	HasMore    bool
	Source     string // "sprint", "board", "epic", "search"
	From       int
	SprintID   int
	SprintName string
	EpicKey    string
	JQL        string
	Project    string
	NextToken  string // Token for JQL search pagination (v3 API).
	Seq        int    // Pagination sequence — stale pages are discarded.
}

// IssuesPageMsg carries a subsequent page of issues during progressive loading.
type IssuesPageMsg struct {
	Issues  []jira.Issue
	HasMore bool
	// Fetch context — used to chain the next page fetch.
	Source     string // "sprint", "board", "epic", "search"
	From       int    // Next offset for Agile API pagination.
	SprintID   int
	SprintName string
	EpicKey    string
	JQL        string
	Project    string
	NextToken  string // Token for JQL search pagination (v3 API).
	Seq        int    // Pagination sequence — stale pages are discarded.
}

// IssueSelectedMsg is sent when the user selects an issue from the list.
type IssueSelectedMsg struct {
	Issue jira.Issue
}

// IssueDetailMsg is sent when full issue details are fetched.
type IssueDetailMsg struct {
	Issue *jira.Issue
}

// OpenURLMsg is sent when the user wants to open a URL in the browser.
type OpenURLMsg struct {
	URL string
}

// ErrMsg wraps an error for display.
type ErrMsg struct {
	Err error
}

// BoardsLoadedMsg is sent when the board list has been fetched.
type BoardsLoadedMsg struct {
	Boards []jira.BoardStats
}

// BoardSelectedMsg is sent when the user selects a board from the homepage.
type BoardSelectedMsg struct {
	Board jira.Board
}

// SearchResultsMsg is sent when JQL search results arrive.
type SearchResultsMsg struct {
	Issues    []jira.Issue
	Query     string
	HasMore   bool
	From      int
	NextToken string
	Seq       int
}

// SetupCompleteMsg is sent when the setup wizard finishes successfully.
type SetupCompleteMsg struct {
	Config *config.Config
}

// JQLMetadataMsg carries fetched JQL autocomplete metadata.
type JQLMetadataMsg struct {
	Meta *jira.JQLMetadata
}

// UserSearchMsg carries user search results for assignee/reporter completions.
type UserSearchMsg struct {
	Prefix string
	Names  []string
}

// BranchCreatedMsg is sent when a branch has been created or copied to clipboard.
type BranchCreatedMsg struct {
	Name   string
	Mode   string // "local", "remote", or "both".
	Copied bool   // True when the command was copied to clipboard instead of executed.
	Err    error
}

// TransitionsLoadedMsg carries available transitions for an issue.
type TransitionsLoadedMsg struct {
	Key         string
	Transitions []jira.Transition
}

// IssueTransitionedMsg is sent after a status transition completes.
type IssueTransitionedMsg struct {
	Key       string
	NewStatus string
	Err       error
}

// ChildIssuesMsg carries child/subtask issues for the issue detail view.
type ChildIssuesMsg struct {
	ParentKey string
	Children  []jira.ChildIssue
}

// CommentAddedMsg is sent after a comment is posted.
type CommentAddedMsg struct {
	Key string
	Err error
}

// FilterSavedMsg is sent when a filter has been successfully saved or updated.
type FilterSavedMsg struct {
	Filter jira.SavedFilter
}

// FilterDeletedMsg is sent when a filter has been successfully deleted.
type FilterDeletedMsg struct {
	ID string
}
