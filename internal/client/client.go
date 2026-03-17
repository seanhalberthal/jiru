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
	From      int    // Offset used for this page (for chaining the next fetch via Agile API).
	NextToken string // Token for JQL search pagination (v3 API).
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
	Transitions(key string) ([]jira.Transition, error)
	TransitionIssue(key, transitionID string) error
	AddComment(key, body string) error
	ChildIssues(key string) ([]jira.ChildIssue, error)
}

// CreateIssueRequest holds the fields needed to create a Jira issue.
type CreateIssueRequest struct {
	Project     string
	ProjectType string // "classic" or "next-gen" — controls parent field handling.
	IssueType   string
	Summary     string
	Description string
	Priority    string
	Assignee    string
	Labels      []string
	Components  []string
	ParentKey   string
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
func (c *Client) SprintIssuesPage(sprintID, from, pageSize int) (*PageResult, error) {
	result, err := c.inner.SprintIssues(sprintID, "ORDER BY updated DESC", uint(from), uint(pageSize))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sprint issues: %w", err)
	}

	issues := make([]jira.Issue, 0, len(result.Issues))
	for _, iss := range result.Issues {
		issues = append(issues, convertIssue(iss))
	}

	// Jira Cloud's IsLast flag is unreliable — it can report true prematurely.
	// Instead, continue fetching until we get an empty page. The MaxTotalIssues
	// safety cap prevents runaway pagination.
	return &PageResult{
		Issues:  issues,
		HasMore: len(result.Issues) > 0,
		From:    from,
	}, nil
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
// Uses the v3 /search/jql API. Note: token-based pagination on this endpoint
// is unreliable on many Jira Cloud instances — prefer BoardIssuesPage for
// board-level issue fetching.
func (c *Client) SearchJQLPage(jql string, pageSize int, from int, nextToken string) (*PageResult, error) {
	path := fmt.Sprintf("/search/jql?jql=%s&maxResults=%d&fields=*all",
		url.QueryEscape(jql), pageSize)
	if nextToken != "" {
		path += "&next_page=" + url.QueryEscape(nextToken)
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

	var searchRes jiracli.SearchResult
	if err := json.NewDecoder(res.Body).Decode(&searchRes); err != nil {
		return nil, err
	}

	issues := make([]jira.Issue, 0, len(searchRes.Issues))
	for _, iss := range searchRes.Issues {
		issues = append(issues, convertIssue(iss))
	}
	return &PageResult{
		Issues:    issues,
		HasMore:   searchRes.NextPageToken != "" && len(searchRes.Issues) > 0,
		NextToken: searchRes.NextPageToken,
	}, nil
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

	var searchRes jiracli.SearchResult
	if err := json.NewDecoder(res.Body).Decode(&searchRes); err != nil {
		return nil, err
	}

	issues := make([]jira.Issue, 0, len(searchRes.Issues))
	for _, iss := range searchRes.Issues {
		issues = append(issues, convertIssue(iss))
	}
	return &PageResult{
		Issues:  issues,
		HasMore: len(searchRes.Issues) > 0,
		From:    from,
	}, nil
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
	res, err := c.inner.EpicIssues(epicKey, "", uint(from), uint(pageSize))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epic issues for %s: %w", epicKey, err)
	}
	issues := make([]jira.Issue, 0, len(res.Issues))
	for _, iss := range res.Issues {
		issues = append(issues, convertIssue(iss))
	}
	return &PageResult{
		Issues:  issues,
		HasMore: len(res.Issues) > 0,
		From:    from,
	}, nil
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

	resp, err := c.inner.CreateV2(cr)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return &CreateIssueResponse{Key: resp.Key}, nil
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

// Transitions returns the available status transitions for an issue.
func (c *Client) Transitions(key string) ([]jira.Transition, error) {
	raw, err := c.inner.TransitionsV2(key)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transitions for %s: %w", key, err)
	}
	transitions := make([]jira.Transition, 0, len(raw))
	for _, t := range raw {
		transitions = append(transitions, jira.Transition{
			ID:   string(t.ID),
			Name: t.Name,
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

// AddComment posts a comment on an issue.
func (c *Client) AddComment(key, body string) error {
	err := c.inner.AddIssueComment(key, body, false)
	if err != nil {
		return fmt.Errorf("failed to add comment to %s: %w", key, err)
	}
	return nil
}
