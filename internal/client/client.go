package client

import (
	"strings"
	"time"

	"github.com/seanhalberthal/jiru/internal/api"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/jira"
)

const DefaultPageSize = 100

const MaxTotalIssues = 2000 // Safety cap to prevent runaway pagination.

// PageResult holds a single page of issues with pagination metadata.
type PageResult struct {
	Issues    []jira.Issue
	HasMore   bool   // True if more pages are available.
	From      int    // Offset used for this page (for chaining the next fetch).
	Total     int    // Total issues reported by the server (0 if unknown).
	NextToken string // Token for v3 /search/jql cursor-based pagination.
}

// JiraClient defines the interface for Jira API operations.
// Used by the UI layer to allow testing with stubs.
type JiraClient interface {
	Me() (string, error)
	Config() *config.Config
	IssueURL(key string) string
	CreateIssue(req *CreateIssueRequest) (*CreateIssueResponse, error)
	GetIssue(key string) (*jira.Issue, error)
	IssueTypesWithID(project string) ([]jira.IssueTypeInfo, error)
	BoardIssues(project string, statuses ...string) ([]jira.Issue, error)
	BoardIssuesPage(boardID, from, pageSize int) (*PageResult, error)
	EpicIssues(epicKey string) ([]jira.Issue, error)
	EpicIssuesPage(epicKey string, from, pageSize int) (*PageResult, error)
	SprintIssues(sprintID int) ([]jira.Issue, error)
	SprintIssuesPage(sprintID, from, pageSize int) (*PageResult, error)
	SprintIssueStats(sprintID int, categoryFn func(string) int) (open, inProgress, done, total int, err error)
	Transitions(key string) ([]jira.Transition, error)
	TransitionIssue(key, transitionID string) error
	ChildIssues(key string) ([]jira.ChildIssue, error)
	AssignIssue(key, accountID string) error
	EditIssue(key string, req *EditIssueRequest) error
	LinkIssue(inwardKey, outwardKey, linkType string) error
	GetIssueLinkTypes() ([]jira.IssueLinkType, error)
	DeleteIssue(key string, cascade bool) error
	Boards(project string) ([]jira.Board, error)
	BoardSprints(boardID int, state string) ([]jira.Sprint, error)
	SearchJQL(jql string, limit uint) ([]jira.Issue, error)
	SearchJQLPage(jql string, pageSize int, from int, nextToken string) (*PageResult, error)
	BoardFilterJQL(boardID int) (string, error)
	JQLMetadata() (*jira.JQLMetadata, error)
	Projects() ([]jira.Project, error)
	ResolveParents(issues []jira.Issue) map[string]ParentInfo
	SearchUsers(project, prefix string) ([]jira.UserInfo, error)
	CreateMetaFields(project, issueTypeID string) ([]jira.CustomFieldDef, error)
	AddComment(key, body string) error
	WatchIssue(key string) error
	UnwatchIssue(key string) error

	// Confluence operations
	ConfluenceSpaces() ([]confluence.Space, error)
	ConfluencePage(pageID string) (*confluence.Page, error)
	ConfluencePageAncestors(pageID string) ([]confluence.PageAncestor, error)
	ConfluenceSpacePages(spaceID string, limit int) ([]confluence.Page, error)
	ConfluenceSearchCQL(cql string, limit int) ([]confluence.PageSearchResult, error)
	ConfluencePageComments(pageID string) ([]confluence.Comment, error)
	ConfluencePageURL(pageID string) string
	UpdateConfluencePage(pageID, title, bodyADF string, version int) (*confluence.Page, error)
	RemoteLinks(key string) ([]jira.RemoteLink, error)
	GetUserDisplayName(accountID string) string
}

// EditIssueRequest holds the fields for editing an existing issue.
// Empty fields are not sent to the API.
type EditIssueRequest struct {
	Summary     string   // empty = no change
	Description string   // empty = no change
	Priority    string   // empty = no change
	Labels      []string // "-label" removes, "label" adds; nil = no change
}

// CreateIssueRequest holds the fields needed to create a Jira issue.
type CreateIssueRequest struct {
	Project      string
	ProjectType  string // "classic" or "next-gen" — controls parent field handling.
	IssueType    string
	Summary      string
	Description  string
	Priority     string
	Assignee     string
	Labels       []string
	Components   []string
	ParentKey    string
	CustomFields map[string]any // field ID → value (string, float64, or option object)
}

// CreateIssueResponse holds the result of creating an issue.
type CreateIssueResponse struct {
	Key string
}

// ParentInfo holds resolved parent issue metadata.
type ParentInfo struct {
	Key       string
	Summary   string
	IssueType string // e.g., "Epic", "Feature", "Initiative" — whatever the Jira instance uses.
}

// Client wraps the API client and exposes typed service methods.
type Client struct {
	http      *api.Client
	config    *config.Config
	accountID string // Cached from Me() — used by WatchIssue/UnwatchIssue.
}

// New creates a new Jira API client from the given configuration.
func New(cfg *config.Config) *Client {
	auth := api.AuthBasic
	if cfg.AuthType == "bearer" {
		auth = api.AuthBearer
	}

	return &Client{
		http: api.New(api.Config{
			BaseURL:  cfg.ServerURL(),
			Username: cfg.User,
			Token:    cfg.APIToken,
			Auth:     auth,
		}),
		config: cfg,
	}
}

// Config returns the client configuration.
func (c *Client) Config() *config.Config {
	return c.config
}

// toPageResult converts a raw API search result into a PageResult, using the
// server-reported total for accurate HasMore determination rather than the
// unreliable len(issues) > 0 heuristic.
func (c *Client) toPageResult(resp *api.SearchResult, from int) *PageResult {
	issues := make([]jira.Issue, 0, len(resp.Issues))
	for _, iss := range resp.Issues {
		issues = append(issues, convertIssue(iss))
	}

	newFrom := from + len(issues)

	var hasMore bool
	if len(issues) == 0 {
		hasMore = false
	} else if resp.Total > 0 {
		hasMore = newFrom < resp.Total
	} else {
		hasMore = true
	}

	return &PageResult{
		Issues:  issues,
		HasMore: hasMore && newFrom < MaxTotalIssues,
		From:    from,
		Total:   resp.Total,
	}
}

// parseJiraTime attempts to parse a Jira timestamp using known formats.
// Jira Cloud may return either compact (-0700) or colon (-07:00) timezone offsets.
func parseJiraTime(s string) (time.Time, bool) {
	for _, layout := range []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05.000Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func convertIssue(iss *api.Issue) jira.Issue {
	i := jira.Issue{
		Key:        iss.Key,
		Summary:    iss.Fields.Summary,
		Status:     iss.Fields.Status.Name,
		Priority:   iss.Fields.Priority.Name,
		Assignee:   iss.Fields.Assignee.DisplayName,
		Reporter:   iss.Fields.Reporter.DisplayName,
		Labels:     iss.Fields.Labels,
		IssueType:  iss.Fields.IssueType.Name,
		IsWatching: iss.Fields.Watches.IsWatching,
	}

	if t, ok := parseJiraTime(iss.Fields.Created); ok {
		i.Created = t
	}
	if t, ok := parseJiraTime(iss.Fields.Updated); ok {
		i.Updated = t
	}

	if iss.Fields.Parent != nil {
		i.ParentKey = iss.Fields.Parent.Key
	}

	if desc, ok := iss.Fields.Description.(string); ok {
		i.Description = desc
	}

	for _, c := range iss.Fields.Comment.Comments {
		body := ""
		if s, ok := c.Body.(string); ok {
			body = s
		}
		comment := jira.Comment{
			Author: c.Author.DisplayName,
			Body:   body,
		}
		if t, ok := parseJiraTime(c.Created); ok {
			comment.Created = t
		}
		i.Comments = append(i.Comments, comment)
	}

	return i
}

// EnrichWithParents populates ParentType and ParentSummary on issues using resolved parent data.
func EnrichWithParents(issues []jira.Issue, parents map[string]ParentInfo) []jira.Issue {
	for i, iss := range issues {
		if info, ok := parents[iss.ParentKey]; ok {
			issues[i].ParentType = info.IssueType
			issues[i].ParentSummary = info.Summary
		}
	}
	return issues
}

// JQLEscape escapes a string for safe use in JQL string literals.
func JQLEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}
