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
	Issues []jira.Issue
	Query  string
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
