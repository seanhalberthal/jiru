package ui

import (
	"time"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/confluence"
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
	Seq        int    // Pagination sequence — stale pages are discarded.
	NextToken  string // Cursor for v3 /search/jql pagination.
}

// IssuesPageMsg carries a subsequent page of issues during progressive loading.
type IssuesPageMsg struct {
	Issues  []jira.Issue
	HasMore bool
	// Fetch context — used to chain the next page fetch.
	Source     string // "sprint", "board", "epic", "search"
	From       int    // Next offset for pagination.
	SprintID   int
	SprintName string
	EpicKey    string
	JQL        string
	Project    string
	Seq        int    // Pagination sequence — stale pages are discarded.
	NextToken  string // Cursor for v3 /search/jql pagination.
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

// BoardSelectedMsg is sent when the user selects a board from the homepage.
type BoardSelectedMsg struct {
	Board jira.Board
}

// BoardPickLoadedMsg is sent when boards are fetched for the board picker.
type BoardPickLoadedMsg struct {
	Boards []jira.Board
}

// SearchResultsMsg is sent when JQL search results arrive.
type SearchResultsMsg struct {
	Issues    []jira.Issue
	Query     string
	HasMore   bool
	From      int
	Seq       int
	NextToken string // Cursor for v3 /search/jql pagination.
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
	Name       string
	Mode       string // "local", "remote", or "both".
	Copied     bool   // True when the git command was copied to clipboard instead of executed.
	NameCopied bool   // True when the branch name was copied to the clipboard after a successful create.
	Err        error
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

// IssueAssignedMsg is sent after an issue assignment completes.
type IssueAssignedMsg struct {
	Key      string
	Assignee string // display name for status msg
	Err      error
}

// AssignUserSearchMsg carries user search results for the assign view.
type AssignUserSearchMsg struct {
	Users []jira.UserInfo
}

// IssueEditedMsg is sent after an issue edit completes.
type IssueEditedMsg struct {
	Key string
	Err error
}

// LinkTypesLoadedMsg carries available issue link types.
type LinkTypesLoadedMsg struct {
	Types []jira.IssueLinkType
}

// IssueLinkCreatedMsg is sent after an issue link is created.
type IssueLinkCreatedMsg struct {
	SourceKey string
	TargetKey string
	Err       error
}

// IssueDeletedMsg is sent after an issue is deleted.
type IssueDeletedMsg struct {
	Key string
	Err error
}

// BranchInfoMsg carries git branch information for the issue detail view.
type BranchInfoMsg struct {
	IssueKey string
	Branches []jira.BranchInfo
}

// FilterSavedMsg is sent when a filter has been successfully saved or updated.
type FilterSavedMsg struct {
	Filter jira.SavedFilter
}

// FilterDeletedMsg is sent when a filter has been successfully deleted.
type FilterDeletedMsg struct {
	ID string
}

// FilterDuplicatedMsg is sent after a filter is successfully duplicated.
type FilterDuplicatedMsg struct {
	Filter jira.SavedFilter
}

// ProfileSwitchedMsg is sent after a profile switch completes.
type ProfileSwitchedMsg struct {
	Client client.JiraClient
	Config *config.Config
	Name   string
}

// --- Confluence messages ---

// SpacesLoadedMsg is sent when Confluence spaces have been fetched.
type SpacesLoadedMsg struct {
	Spaces []confluence.Space
}

// SpacePagesLoadedMsg carries pages for a space.
type SpacePagesLoadedMsg struct {
	Pages   []confluence.Page
	SpaceID string
}

// ConfluencePageLoadedMsg carries a fetched Confluence page.
type ConfluencePageLoadedMsg struct {
	Page      *confluence.Page
	Ancestors []confluence.PageAncestor
	SpaceKey  string
}

// ConfluenceCommentsLoadedMsg carries comments for a Confluence page.
type ConfluenceCommentsLoadedMsg struct {
	PageID   string
	Comments []confluence.Comment
}

// RemoteLinksLoadedMsg carries remote links for a Jira issue.
type RemoteLinksLoadedMsg struct {
	Links    []jira.RemoteLink
	IssueKey string
}

// IssueWatchToggledMsg is sent after a watch/unwatch toggle completes.
type IssueWatchToggledMsg struct {
	Key        string
	IsWatching bool // New state: true = now watching, false = unwatched.
	Err        error
}

// statusDismissMsg is a tick message to auto-dismiss the status message.
type statusDismissMsg struct {
	setAt time.Time // The statusMsgTime when the tick was scheduled — prevents stale dismissals.
}
