package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	jiracli "github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/validate"
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

// paginatedResponse is the JSON shape returned by Jira search and Agile issue
// list endpoints. Unlike the jira-cli SearchResult, it captures the total field
// which is essential for detecting API truncation (the Agile v1 endpoints on
// Jira Cloud can silently stop returning issues before the real total).
type paginatedResponse struct {
	Total         int              `json:"total"`
	StartAt       int              `json:"startAt"`
	MaxResults    int              `json:"maxResults"`
	IsLast        bool             `json:"isLast"`
	NextPageToken string           `json:"nextPageToken"`
	Issues        []*jiracli.Issue `json:"issues"`
}

// toPageResult converts a raw API response into a PageResult, using the server-
// reported total for accurate HasMore determination rather than the unreliable
// len(issues) > 0 heuristic.
func (c *Client) toPageResult(resp *paginatedResponse, from int) *PageResult {
	issues := make([]jira.Issue, 0, len(resp.Issues))
	for _, iss := range resp.Issues {
		issues = append(issues, convertIssue(iss))
	}

	newFrom := from + len(issues)

	var hasMore bool
	if len(issues) == 0 {
		// No issues returned — stop. Callers that can fall back to an
		// alternative endpoint should check Total separately.
		hasMore = false
	} else if resp.Total > 0 {
		hasMore = newFrom < resp.Total
	} else {
		// No total info — assume more until we get an empty page.
		hasMore = true
	}

	return &PageResult{
		Issues:  issues,
		HasMore: hasMore && newFrom < MaxTotalIssues,
		From:    from,
		Total:   resp.Total,
	}
}

// JiraClient defines the interface for Jira API operations.
// Used by the UI layer to allow testing with stubs.
type JiraClient interface {
	Me() (string, error)
	Config() *config.Config
	SprintIssues(sprintID int) ([]jira.Issue, error)
	GetIssue(key string) (*jira.Issue, error)
	IssueURL(key string) string
	Boards(project string) ([]jira.Board, error)
	BoardSprints(boardID int, state string) ([]jira.Sprint, error)
	SearchJQL(jql string, limit uint) ([]jira.Issue, error)
	SprintIssueStats(sprintID int) (open, inProgress, done, total int, err error)
	ResolveParents(issues []jira.Issue) map[string]ParentInfo
	BoardIssues(project string, statuses ...string) ([]jira.Issue, error)
	EpicIssues(epicKey string) ([]jira.Issue, error)
	SprintIssuesPage(sprintID, from, pageSize int) (*PageResult, error)
	SearchJQLPage(jql string, pageSize int, from int, nextToken string) (*PageResult, error)
	BoardIssuesPage(boardID, from, pageSize int) (*PageResult, error)
	EpicIssuesPage(epicKey string, from, pageSize int) (*PageResult, error)
	Projects() ([]jira.Project, error)
	JQLMetadata() (*jira.JQLMetadata, error)
	SearchUsers(project, prefix string) ([]UserInfo, error)
	CreateIssue(req *CreateIssueRequest) (*CreateIssueResponse, error)
	IssueTypes(project string) ([]string, error)
	IssueTypesWithID(project string) ([]jira.IssueTypeInfo, error)
	CreateMetaFields(project, issueTypeID string) ([]jira.CustomFieldDef, error)
	Transitions(key string) ([]jira.Transition, error)
	TransitionIssue(key, transitionID string) error
	AddComment(key, body string) error
	ChildIssues(key string) ([]jira.ChildIssue, error)
	AssignIssue(key, accountID string) error
	EditIssue(key string, req *EditIssueRequest) error
	LinkIssue(inwardKey, outwardKey, linkType string) error
	GetIssueLinkTypes() ([]jira.IssueLinkType, error)
	DeleteIssue(key string, cascade bool) error
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

// Client wraps jira-cli's Client and exposes typed service methods.
type Client struct {
	inner  *jiracli.Client
	config *config.Config
}

// New creates a new Jira API client from the given configuration.
func New(cfg *config.Config) *Client {
	authType := jiracli.AuthTypeBasic
	if cfg.AuthType == "bearer" {
		authType = jiracli.AuthTypeBearer
	}

	inner := jiracli.NewClient(
		jiracli.Config{
			Server:   cfg.ServerURL(),
			Login:    cfg.User,
			APIToken: cfg.APIToken,
			AuthType: &authType,
		},
		jiracli.WithTimeout(30*time.Second),
	)

	return &Client{inner: inner, config: cfg}
}

// Config returns the client configuration.
func (c *Client) Config() *config.Config {
	return c.config
}

// Me verifies authentication and returns the current user's display name.
func (c *Client) Me() (string, error) {
	me, err := c.inner.Me()
	if err != nil {
		return "", fmt.Errorf("auth check failed: %w", err)
	}
	return me.Name, nil
}

// SprintIssuesPage fetches a single page of sprint issues.
// Uses a raw API call (bypassing jira-cli's SearchResult which lacks the total
// field) so we can detect Agile API truncation and fall back to JQL search.
func (c *Client) SprintIssuesPage(sprintID, from, pageSize int) (*PageResult, error) {
	path := fmt.Sprintf("/sprint/%d/issue?startAt=%d&maxResults=%d&jql=%s",
		sprintID, from, pageSize, url.QueryEscape("ORDER BY updated DESC"))

	res, err := c.inner.GetV1(context.Background(), path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sprint issues: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("empty response from sprint issues")
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sprint issues failed with status %d", res.StatusCode)
	}

	var resp paginatedResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	result := c.toPageResult(&resp, from)

	// Agile API truncation: the server reports more issues exist but returned
	// an empty page. This is a known Jira Cloud limitation where the Agile v1
	// endpoints stop returning results beyond a certain offset. Fall back to
	// JQL search which handles deep pagination reliably.
	if len(result.Issues) == 0 && resp.Total > from {
		jql := fmt.Sprintf("sprint = %d ORDER BY updated DESC", sprintID)
		return c.SearchJQLPage(jql, pageSize, from, "")
	}

	return result, nil
}

// SprintIssues fetches all issues in the given sprint (all pages).
func (c *Client) SprintIssues(sprintID int) ([]jira.Issue, error) {
	var all []jira.Issue
	from := 0
	for {
		page, err := c.SprintIssuesPage(sprintID, from, DefaultPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Issues...)
		if !page.HasMore || len(all) >= MaxTotalIssues {
			break
		}
		from += len(page.Issues)
	}
	return all, nil
}

// GetIssue fetches full details for a single issue.
func (c *Client) GetIssue(key string) (*jira.Issue, error) {
	iss, err := c.inner.GetIssueV2(key)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue %s: %w", key, err)
	}
	result := convertIssue(iss)
	return &result, nil
}

// IssueURL returns the browser URL for a given issue key.
func (c *Client) IssueURL(key string) string {
	return fmt.Sprintf("%s/browse/%s", c.config.ServerURL(), key)
}

// Boards returns all boards visible to the authenticated user.
// If project is non-empty, filters to that project only.
func (c *Client) Boards(project string) ([]jira.Board, error) {
	res, err := c.inner.Boards(project, "")
	if err != nil {
		return nil, err
	}
	boards := make([]jira.Board, 0, len(res.Boards))
	for _, b := range res.Boards {
		boards = append(boards, jira.Board{
			ID:   b.ID,
			Name: b.Name,
			Type: b.Type,
		})
	}
	return boards, nil
}

// BoardSprints returns active sprints for a board.
func (c *Client) BoardSprints(boardID int, state string) ([]jira.Sprint, error) {
	qp := "state=" + state
	res, err := c.inner.Sprints(boardID, qp, 0, 50)
	if err != nil {
		return nil, err
	}
	sprints := make([]jira.Sprint, 0, len(res.Sprints))
	for _, s := range res.Sprints {
		sprints = append(sprints, jira.Sprint{
			ID:    s.ID,
			Name:  s.Name,
			State: s.Status,
		})
	}
	return sprints, nil
}

// SearchJQLPage executes a JQL query and returns a single page of results.
// Uses the v3 /search/jql API with cursor-based (nextPageToken) pagination.
// The from parameter tracks cumulative progress for the MaxTotalIssues cap.
func (c *Client) SearchJQLPage(jql string, pageSize int, from int, nextToken string) (*PageResult, error) {
	path := fmt.Sprintf("/search/jql?jql=%s&maxResults=%d&fields=*all",
		url.QueryEscape(jql), pageSize)
	if nextToken != "" {
		path += "&nextPageToken=" + url.QueryEscape(nextToken)
	}

	res, err := c.inner.Get(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("empty response from search")
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status %d", res.StatusCode)
	}

	var resp paginatedResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	result := c.toPageResult(&resp, from)
	result.NextToken = resp.NextPageToken
	// For cursor-based pagination, override HasMore using the token.
	result.HasMore = resp.NextPageToken != "" && len(result.Issues) > 0 && (from+len(result.Issues)) < MaxTotalIssues
	return result, nil
}

// BoardIssuesPage fetches a single page of issues for a board using the
// Agile v1 API with reliable offset-based pagination.
func (c *Client) BoardIssuesPage(boardID, from, pageSize int) (*PageResult, error) {
	path := fmt.Sprintf("/board/%d/issue?startAt=%d&maxResults=%d&jql=%s", boardID, from, pageSize, url.QueryEscape("ORDER BY updated DESC"))

	res, err := c.inner.GetV1(context.Background(), path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch board issues: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("empty response from board issues")
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("board issues failed with status %d", res.StatusCode)
	}

	var resp paginatedResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return c.toPageResult(&resp, from), nil
}

// SearchJQL executes a JQL query and returns all matching issues (all pages).
func (c *Client) SearchJQL(jql string, limit uint) ([]jira.Issue, error) {
	var all []jira.Issue
	pageSize := int(limit)
	if pageSize == 0 || pageSize > DefaultPageSize {
		pageSize = DefaultPageSize
	}
	cap := int(limit)
	if cap == 0 {
		cap = MaxTotalIssues
	}
	nextToken := ""
	for {
		page, err := c.SearchJQLPage(jql, pageSize, len(all), nextToken)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Issues...)
		if !page.HasMore || len(all) >= cap {
			break
		}
		nextToken = page.NextToken
	}
	if len(all) > cap {
		all = all[:cap]
	}
	return all, nil
}

// SprintIssueStats returns issue counts grouped by status category for a sprint.
// Uses theme.StatusCategory for categorisation, which respects the instance-specific
// status mapping when available.
func (c *Client) SprintIssueStats(sprintID int) (open, inProgress, done, total int, err error) {
	issues, err := c.SprintIssues(sprintID)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for _, iss := range issues {
		total++
		switch theme.StatusCategory(iss.Status) {
		case 2, 3:
			done++
		case 1:
			inProgress++
		default:
			open++
		}
	}
	return open, inProgress, done, total, nil
}

// ParentInfo holds resolved parent issue metadata.
type ParentInfo struct {
	Key       string
	Summary   string
	IssueType string // e.g., "Epic", "Feature", "Initiative" — whatever the Jira instance uses.
}

// ResolveParents fetches metadata for all unique parent keys found in the given issues.
// Uses a single JQL query (key in (...)) rather than N individual API calls.
// Returns a map of parent key → ParentInfo.
func (c *Client) ResolveParents(issues []jira.Issue) map[string]ParentInfo {
	seen := make(map[string]bool)
	var keys []string
	for _, iss := range issues {
		if iss.ParentKey != "" && !seen[iss.ParentKey] {
			if validate.IssueKey(iss.ParentKey) != nil {
				continue // skip malformed keys
			}
			seen[iss.ParentKey] = true
			keys = append(keys, "'"+JQLEscape(iss.ParentKey)+"'")
		}
	}

	if len(keys) == 0 {
		return nil
	}

	jql := "key in (" + strings.Join(keys, ", ") + ")"
	searchRes, err := c.inner.Search(jql, uint(len(keys)))

	results := make(map[string]ParentInfo, len(keys))
	if err != nil {
		for _, k := range keys {
			results[k] = ParentInfo{Key: k}
		}
		return results
	}

	for _, iss := range searchRes.Issues {
		results[iss.Key] = ParentInfo{
			Key:       iss.Key,
			Summary:   iss.Fields.Summary,
			IssueType: iss.Fields.IssueType.Name,
		}
	}

	for _, k := range keys {
		if _, ok := results[k]; !ok {
			results[k] = ParentInfo{Key: k}
		}
	}

	return results
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
// Jira JQL uses backslash-escaped backslashes, double quotes, and single quotes.
func JQLEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

// BoardIssues fetches all open issues for a board's project via JQL.
// Used for kanban boards that don't have sprints.
func (c *Client) BoardIssues(project string, statuses ...string) ([]jira.Issue, error) {
	if err := validate.ProjectKey(project); err != nil {
		return nil, fmt.Errorf("BoardIssues: %w", err)
	}
	escapedProject := JQLEscape(project)
	jql := fmt.Sprintf("project = '%s' AND statusCategory != Done ORDER BY status ASC, updated DESC", escapedProject)
	if len(statuses) > 0 {
		quoted := make([]string, len(statuses))
		for i, s := range statuses {
			quoted[i] = "'" + JQLEscape(s) + "'"
		}
		jql = fmt.Sprintf("project = '%s' AND status in (%s) ORDER BY status ASC, updated DESC",
			escapedProject, strings.Join(quoted, ", "))
	}
	return c.SearchJQL(jql, 200)
}

// EpicIssuesPage fetches a single page of epic child issues.
func (c *Client) EpicIssuesPage(epicKey string, from, pageSize int) (*PageResult, error) {
	path := fmt.Sprintf("/epic/%s/issue?startAt=%d&maxResults=%d", epicKey, from, pageSize)

	res, err := c.inner.GetV1(context.Background(), path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epic issues for %s: %w", epicKey, err)
	}
	if res == nil {
		return nil, fmt.Errorf("empty response from epic issues for %s", epicKey)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("epic issues failed with status %d for %s", res.StatusCode, epicKey)
	}

	var resp paginatedResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return c.toPageResult(&resp, from), nil
}

// EpicIssues fetches all issues belonging to the given epic (all pages).
func (c *Client) EpicIssues(epicKey string) ([]jira.Issue, error) {
	var all []jira.Issue
	from := 0
	for {
		page, err := c.EpicIssuesPage(epicKey, from, DefaultPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Issues...)
		if !page.HasMore || len(all) >= MaxTotalIssues {
			break
		}
		from += len(page.Issues)
	}
	return all, nil
}

// JQLMetadata fetches metadata for JQL autocompletion from the Jira instance.
// Makes parallel REST calls for statuses, issue types, priorities, resolutions,
// projects, labels, and optionally project-scoped components/versions.
// Individual endpoint failures are silently ignored — we return what we can.
func (c *Client) JQLMetadata() (*jira.JQLMetadata, error) {
	meta := &jira.JQLMetadata{}

	type result struct {
		field string
		vals  []string
		err   error
	}

	ch := make(chan result, 7)
	statusCh := make(chan *statusResult, 1)

	go func() {
		sr, err := c.fetchStatuses()
		if err != nil {
			ch <- result{"statuses", nil, err}
			statusCh <- nil
		} else {
			ch <- result{"statuses", sr.names, nil}
			statusCh <- sr
		}
	}()

	go func() {
		vals, err := c.fetchIssueTypes()
		ch <- result{"issuetypes", vals, err}
	}()

	go func() {
		vals, err := c.fetchPriorities()
		ch <- result{"priorities", vals, err}
	}()

	go func() {
		vals, err := c.fetchResolutions()
		ch <- result{"resolutions", vals, err}
	}()

	go func() {
		vals, err := c.fetchProjects()
		ch <- result{"projects", vals, err}
	}()

	go func() {
		vals, err := c.fetchLabels()
		ch <- result{"labels", vals, err}
	}()

	go func() {
		if c.config.Project == "" {
			ch <- result{"components", nil, nil}
			return
		}
		vals, err := c.fetchComponents(c.config.Project)
		ch <- result{"components", vals, err}
	}()

	go func() {
		if c.config.Project == "" {
			ch <- result{"versions", nil, nil}
			return
		}
		vals, err := c.fetchVersions(c.config.Project)
		ch <- result{"versions", vals, err}
	}()

	for range 8 {
		r := <-ch
		switch r.field {
		case "statuses":
			meta.Statuses = r.vals
		case "issuetypes":
			meta.IssueTypes = r.vals
		case "priorities":
			meta.Priorities = r.vals
		case "resolutions":
			meta.Resolutions = r.vals
		case "projects":
			meta.Projects = r.vals
		case "labels":
			meta.Labels = r.vals
		case "components":
			meta.Components = r.vals
		case "versions":
			meta.Versions = r.vals
		}
	}

	// Populate status category mapping from the enriched status fetch.
	if sr := <-statusCh; sr != nil {
		meta.StatusCategories = sr.categories
	}

	return meta, nil
}

// UserInfo holds user display name and account ID from search results.
type UserInfo struct {
	AccountID   string
	DisplayName string
}

// SearchUsers searches for assignable users matching the given prefix.
// Uses the v3 API which supports the `query` parameter for searching by
// display name and email. The v2 API's `username` parameter is deprecated
// and ignored on Jira Cloud.
func (c *Client) SearchUsers(project, prefix string) ([]UserInfo, error) {
	users, err := c.inner.UserSearch(&jiracli.UserSearchOptions{
		Project:    project,
		Query:      prefix,
		MaxResults: 10,
	})
	if err != nil {
		return nil, err
	}
	infos := make([]UserInfo, 0, len(users))
	for _, u := range users {
		if u.DisplayName != "" {
			infos = append(infos, UserInfo{
				AccountID:   u.AccountID,
				DisplayName: u.DisplayName,
			})
		}
	}
	return infos, nil
}

// fetchNameList is a helper that GETs a REST API v2 endpoint and decodes
// a JSON array of objects with a "name" field, returning deduplicated names.
func (c *Client) fetchNameList(path string) ([]string, error) {
	res, err := c.inner.GetV2(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", res.StatusCode, path)
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&items); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		if !seen[item.Name] {
			names = append(names, item.Name)
			seen[item.Name] = true
		}
	}
	return names, nil
}

// statusResult holds both names and category mappings from the /status endpoint.
type statusResult struct {
	names      []string
	categories map[string]int // status name → 0 (todo), 1 (in progress), 2 (done)
}

func (c *Client) fetchStatuses() (*statusResult, error) {
	res, err := c.inner.GetV2(context.Background(), "/status", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for /status", res.StatusCode)
	}
	var items []struct {
		Name           string `json:"name"`
		StatusCategory struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	}
	if err := json.NewDecoder(res.Body).Decode(&items); err != nil {
		return nil, err
	}
	sr := &statusResult{
		categories: make(map[string]int, len(items)),
	}
	seen := make(map[string]bool)
	for _, item := range items {
		if !seen[item.Name] {
			sr.names = append(sr.names, item.Name)
			seen[item.Name] = true
		}
		switch item.StatusCategory.Key {
		case "done":
			if theme.IsCancelledName(item.Name) {
				sr.categories[item.Name] = 3
			} else {
				sr.categories[item.Name] = 2
			}
		case "indeterminate":
			sr.categories[item.Name] = 1
		default: // "new" or anything else
			sr.categories[item.Name] = 0
		}
	}
	return sr, nil
}

func (c *Client) fetchIssueTypes() ([]string, error) {
	return c.fetchNameList("/issuetype")
}

func (c *Client) fetchPriorities() ([]string, error) {
	return c.fetchNameList("/priority")
}

func (c *Client) fetchResolutions() ([]string, error) {
	return c.fetchNameList("/resolution")
}

func (c *Client) fetchLabels() ([]string, error) {
	res, err := c.inner.GetV2(context.Background(), "/label", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for /label", res.StatusCode)
	}
	var resp struct {
		Values []string `json:"values"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}
	return resp.Values, nil
}

// Projects returns all projects visible to the authenticated user.
func (c *Client) Projects() ([]jira.Project, error) {
	projects, err := c.inner.Project()
	if err != nil {
		return nil, err
	}
	result := make([]jira.Project, 0, len(projects))
	for _, p := range projects {
		result = append(result, jira.Project{Key: p.Key, Name: p.Name, Type: p.Type})
	}
	return result, nil
}

func (c *Client) fetchProjects() ([]string, error) {
	projects, err := c.Projects()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(projects))
	for _, p := range projects {
		keys = append(keys, p.Key)
	}
	return keys, nil
}

func (c *Client) fetchComponents(project string) ([]string, error) {
	path := fmt.Sprintf("/project/%s/components", project)
	return c.fetchNameList(path)
}

func (c *Client) fetchVersions(project string) ([]string, error) {
	versions, err := c.inner.Release(project)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(versions))
	for _, v := range versions {
		if !v.Released && !v.Archived {
			names = append(names, v.Name)
		}
	}
	return names, nil
}

// CreateIssue creates a new issue in Jira.
func (c *Client) CreateIssue(req *CreateIssueRequest) (*CreateIssueResponse, error) {
	cr := &jiracli.CreateRequest{
		Project:   req.Project,
		IssueType: req.IssueType,
		Summary:   req.Summary,
		Body:      req.Description,
		Priority:  req.Priority,
		Assignee:  req.Assignee,
		Labels:    req.Labels,
	}

	if req.ProjectType != "" {
		cr.ForProjectType(req.ProjectType)
	}

	// Set installation type so assignee/reporter use the correct field
	// (accountId for Cloud, name for Server).
	if c.config.AuthType == "bearer" {
		cr.ForInstallationType(jiracli.InstallationTypeLocal)
	} else {
		cr.ForInstallationType(jiracli.InstallationTypeCloud)
	}

	if req.ParentKey != "" {
		cr.ParentIssueKey = req.ParentKey
	}

	if len(req.Components) > 0 {
		cr.Components = req.Components
	}

	if len(req.CustomFields) > 0 {
		cr.CustomFields = make(map[string]string)
		var cfMeta []jiracli.IssueTypeField
		for id, val := range req.CustomFields {
			switch v := val.(type) {
			case string:
				cr.CustomFields[id] = v
				cfMeta = append(cfMeta, jiracli.IssueTypeField{
					Name: id,
					Key:  id,
					Schema: struct {
						DataType string `json:"type"`
						Items    string `json:"items,omitempty"`
					}{DataType: "string"},
				})
			case float64:
				cr.CustomFields[id] = fmt.Sprintf("%g", v)
				cfMeta = append(cfMeta, jiracli.IssueTypeField{
					Name: id,
					Key:  id,
					Schema: struct {
						DataType string `json:"type"`
						Items    string `json:"items,omitempty"`
					}{DataType: "number"},
				})
			case map[string]string:
				cr.CustomFields[id] = v["value"]
				cfMeta = append(cfMeta, jiracli.IssueTypeField{
					Name: id,
					Key:  id,
					Schema: struct {
						DataType string `json:"type"`
						Items    string `json:"items,omitempty"`
					}{DataType: "option"},
				})
			}
		}
		cr.WithCustomFields(cfMeta)
	}

	resp, err := c.inner.CreateV2(cr)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return &CreateIssueResponse{Key: resp.Key}, nil
}

// IssueTypesWithID returns available issue types with their IDs for a project.
func (c *Client) IssueTypesWithID(project string) ([]jira.IssueTypeInfo, error) {
	if project != "" {
		meta, err := c.inner.GetCreateMeta(&jiracli.CreateMetaRequest{
			Projects: project,
			Expand:   "projects.issuetypes",
		})
		if err == nil && len(meta.Projects) > 0 {
			var types []jira.IssueTypeInfo
			for _, it := range meta.Projects[0].IssueTypes {
				types = append(types, jira.IssueTypeInfo{ID: it.ID, Name: it.Name})
			}
			if len(types) > 0 {
				return types, nil
			}
		}
	}
	// Fallback: get types without IDs.
	names, err := c.fetchIssueTypes()
	if err != nil {
		return nil, err
	}
	var types []jira.IssueTypeInfo
	for _, n := range names {
		types = append(types, jira.IssueTypeInfo{Name: n})
	}
	return types, nil
}

// CreateMetaFields fetches custom field definitions for a project + issue type.
func (c *Client) CreateMetaFields(project, issueTypeID string) ([]jira.CustomFieldDef, error) {
	path := fmt.Sprintf("/issue/createmeta/%s/issuetypes/%s", project, issueTypeID)
	res, err := c.inner.Get(context.Background(), path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch create metadata fields: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var resp struct {
		Values []struct {
			FieldID  string `json:"fieldId"`
			Name     string `json:"name"`
			Required bool   `json:"required"`
			Schema   struct {
				Type  string `json:"type"`
				Items string `json:"items,omitempty"`
			} `json:"schema"`
			AllowedValues []struct {
				Value string `json:"value"`
				Name  string `json:"name"`
			} `json:"allowedValues"`
		} `json:"values"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode create metadata fields: %w", err)
	}

	// Standard fields to skip (already handled by the wizard).
	skip := map[string]bool{
		"summary": true, "issuetype": true, "project": true,
		"description": true, "priority": true, "assignee": true,
		"labels": true, "reporter": true, "components": true,
		"parent": true, "attachment": true, "issuelinks": true,
	}

	var fields []jira.CustomFieldDef
	for _, f := range resp.Values {
		if skip[f.FieldID] || !strings.HasPrefix(f.FieldID, "customfield_") {
			continue
		}
		fieldType := "unsupported"
		switch f.Schema.Type {
		case "string":
			fieldType = "string"
		case "number":
			fieldType = "number"
		case "option":
			fieldType = "option"
		}

		var allowed []string
		for _, v := range f.AllowedValues {
			val := v.Value
			if val == "" {
				val = v.Name
			}
			if val != "" {
				allowed = append(allowed, val)
			}
		}

		fields = append(fields, jira.CustomFieldDef{
			ID:            f.FieldID,
			Name:          f.Name,
			FieldType:     fieldType,
			Required:      f.Required,
			AllowedValues: allowed,
		})
	}
	return fields, nil
}

// IssueTypes returns available issue types for a project.
// Falls back to the global issue type list if the project-specific fetch fails.
func (c *Client) IssueTypes(project string) ([]string, error) {
	if project != "" {
		// Try to get project-specific issue types via create metadata.
		meta, err := c.inner.GetCreateMeta(&jiracli.CreateMetaRequest{
			Projects: project,
			Expand:   "projects.issuetypes",
		})
		if err == nil && len(meta.Projects) > 0 {
			var types []string
			for _, it := range meta.Projects[0].IssueTypes {
				types = append(types, it.Name)
			}
			if len(types) > 0 {
				return types, nil
			}
		}
	}
	// Fallback to global issue type list.
	return c.fetchIssueTypes()
}

func convertIssue(iss *jiracli.Issue) jira.Issue {
	i := jira.Issue{
		Key:       iss.Key,
		Summary:   iss.Fields.Summary,
		Status:    iss.Fields.Status.Name,
		Priority:  iss.Fields.Priority.Name,
		Assignee:  iss.Fields.Assignee.Name,
		Reporter:  iss.Fields.Reporter.Name,
		Labels:    iss.Fields.Labels,
		IssueType: iss.Fields.IssueType.Name,
	}

	// Parse timestamps.
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", iss.Fields.Created); err == nil {
		i.Created = t
	}
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", iss.Fields.Updated); err == nil {
		i.Updated = t
	}

	// Extract parent key (epic for stories, story for subtasks).
	if iss.Fields.Parent != nil {
		i.ParentKey = iss.Fields.Parent.Key
	}

	// Description from V2 is a plain string.
	if desc, ok := iss.Fields.Description.(string); ok {
		i.Description = desc
	}

	// Convert comments.
	for _, c := range iss.Fields.Comment.Comments {
		body := ""
		if s, ok := c.Body.(string); ok {
			body = s
		}
		i.Comments = append(i.Comments, jira.Comment{
			Author: c.Author.DisplayName,
			Body:   body,
		})
	}

	return i
}

// transitionsResponse is the JSON shape returned by GET /issue/{key}/transitions.
type transitionsResponse struct {
	Transitions []struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
		To   struct {
			Name string `json:"name"`
		} `json:"to"`
	} `json:"transitions"`
}

// Transitions returns the available status transitions for an issue.
// Uses a direct API call instead of jira-cli's TransitionsV2 to capture the
// target status name (to.name), which the library's Transition struct omits.
func (c *Client) Transitions(key string) ([]jira.Transition, error) {
	if err := validate.IssueKey(key); err != nil {
		return nil, fmt.Errorf("Transitions: %w", err)
	}
	path := fmt.Sprintf("/issue/%s/transitions", key)
	res, err := c.inner.GetV2(context.Background(), path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transitions for %s: %w", key, err)
	}
	if res == nil {
		return nil, fmt.Errorf("empty response fetching transitions for %s", key)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching transitions for %s", res.StatusCode, key)
	}

	var out transitionsResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode transitions for %s: %w", key, err)
	}

	transitions := make([]jira.Transition, 0, len(out.Transitions))
	for _, t := range out.Transitions {
		transitions = append(transitions, jira.Transition{
			ID:       t.ID.String(),
			Name:     t.Name,
			ToStatus: t.To.Name,
		})
	}
	return transitions, nil
}

// TransitionIssue performs a status transition on an issue.
func (c *Client) TransitionIssue(key, transitionID string) error {
	req := &jiracli.TransitionRequest{
		Transition: &jiracli.TransitionRequestData{ID: transitionID},
	}
	_, err := c.inner.Transition(key, req)
	if err != nil {
		return fmt.Errorf("failed to transition %s: %w", key, err)
	}
	return nil
}

// ChildIssues fetches child/subtask issues for the given parent key via JQL.
func (c *Client) ChildIssues(key string) ([]jira.ChildIssue, error) {
	if err := validate.IssueKey(key); err != nil {
		return nil, fmt.Errorf("ChildIssues: %w", err)
	}
	jql := fmt.Sprintf("parent = '%s' ORDER BY status ASC, key ASC", JQLEscape(key))
	issues, err := c.SearchJQL(jql, 50)
	if err != nil {
		return nil, err
	}
	children := make([]jira.ChildIssue, 0, len(issues))
	for _, iss := range issues {
		children = append(children, jira.ChildIssue{
			Key:       iss.Key,
			Summary:   iss.Summary,
			Status:    iss.Status,
			IssueType: iss.IssueType,
		})
	}
	return children, nil
}

// AssignIssue assigns an issue to a user by account ID.
// Pass "none" to unassign, "default" for the default assignee.
func (c *Client) AssignIssue(key, accountID string) error {
	err := c.inner.AssignIssueV2(key, accountID)
	if err != nil {
		return fmt.Errorf("failed to assign %s: %w", key, err)
	}
	return nil
}

// EditIssue updates fields on an existing issue.
// Only non-empty fields in the request are sent.
func (c *Client) EditIssue(key string, req *EditIssueRequest) error {
	cr := &jiracli.EditRequest{
		Summary:  req.Summary,
		Body:     req.Description,
		Priority: req.Priority,
		Labels:   req.Labels,
	}
	err := c.inner.Edit(key, cr)
	if err != nil {
		return fmt.Errorf("failed to edit %s: %w", key, err)
	}
	return nil
}

// LinkIssue creates a link between two issues.
func (c *Client) LinkIssue(inwardKey, outwardKey, linkType string) error {
	err := c.inner.LinkIssue(inwardKey, outwardKey, linkType)
	if err != nil {
		return fmt.Errorf("failed to link %s → %s: %w", inwardKey, outwardKey, err)
	}
	return nil
}

// GetIssueLinkTypes returns all available issue link types.
func (c *Client) GetIssueLinkTypes() ([]jira.IssueLinkType, error) {
	raw, err := c.inner.GetIssueLinkTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch link types: %w", err)
	}
	types := make([]jira.IssueLinkType, 0, len(raw))
	for _, t := range raw {
		types = append(types, jira.IssueLinkType{
			ID:      t.ID,
			Name:    t.Name,
			Inward:  t.Inward,
			Outward: t.Outward,
		})
	}
	return types, nil
}

// DeleteIssue deletes an issue. If cascade is true, subtasks are also deleted.
func (c *Client) DeleteIssue(key string, cascade bool) error {
	err := c.inner.DeleteIssue(key, cascade)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", key, err)
	}
	return nil
}

// AddComment posts a comment on an issue.
func (c *Client) AddComment(key, body string) error {
	err := c.inner.AddIssueComment(key, body, false)
	if err != nil {
		return fmt.Errorf("failed to add comment to %s: %w", key, err)
	}
	return nil
}
