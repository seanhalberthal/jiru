package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	jiracli "github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/validate"
)

// JiraClient defines the interface for Jira API operations.
// Used by the UI layer to allow testing with stubs.
type JiraClient interface {
	Me() (string, error)
	Config() *config.Config
	ActiveSprint() (*jira.Sprint, error)
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
	Projects() ([]jira.Project, error)
	JQLMetadata() (*jira.JQLMetadata, error)
	SearchUsers(project, prefix string) ([]string, error)
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

// ActiveSprint returns the active sprint for the configured board.
func (c *Client) ActiveSprint() (*jira.Sprint, error) {
	result, err := c.inner.Sprints(c.config.BoardID, "state=active", 0, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active sprint: %w", err)
	}
	if len(result.Sprints) == 0 {
		return nil, fmt.Errorf("no active sprint found for board %d", c.config.BoardID)
	}

	s := result.Sprints[0]
	return &jira.Sprint{
		ID:    s.ID,
		Name:  s.Name,
		State: s.Status,
	}, nil
}

// SprintIssues fetches all issues in the given sprint.
func (c *Client) SprintIssues(sprintID int) ([]jira.Issue, error) {
	result, err := c.inner.SprintIssues(sprintID, "", 0, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sprint issues: %w", err)
	}

	issues := make([]jira.Issue, 0, len(result.Issues))
	for _, iss := range result.Issues {
		issues = append(issues, convertIssue(iss))
	}
	return issues, nil
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

// SearchJQL executes a JQL query and returns matching issues.
func (c *Client) SearchJQL(jql string, limit uint) ([]jira.Issue, error) {
	res, err := c.inner.Search(jql, limit)
	if err != nil {
		return nil, err
	}
	issues := make([]jira.Issue, 0, len(res.Issues))
	for _, iss := range res.Issues {
		issues = append(issues, convertIssue(iss))
	}
	return issues, nil
}

// SprintIssueStats returns issue counts grouped by status category for a sprint.
func (c *Client) SprintIssueStats(sprintID int) (open, inProgress, done, total int, err error) {
	issues, err := c.SprintIssues(sprintID)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for _, iss := range issues {
		total++
		switch iss.Status {
		case "Done", "Closed", "Resolved":
			done++
		case "In Progress", "In Review":
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
			keys = append(keys, "'"+iss.ParentKey+"'")
		}
	}

	if len(keys) == 0 {
		return nil
	}

	jql := "key in (" + strings.Join(keys, ", ") + ")"
	searchRes, err := c.inner.SearchV2(jql, 0, uint(len(keys)))

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

// BoardIssues fetches all open issues for a board's project via JQL.
// Used for kanban boards that don't have sprints.
func (c *Client) BoardIssues(project string, statuses ...string) ([]jira.Issue, error) {
	if err := validate.ProjectKey(project); err != nil {
		return nil, fmt.Errorf("BoardIssues: %w", err)
	}
	jql := fmt.Sprintf("project = '%s' AND statusCategory != Done ORDER BY status ASC, updated DESC", project)
	if len(statuses) > 0 {
		quoted := make([]string, len(statuses))
		for i, s := range statuses {
			quoted[i] = "'" + s + "'"
		}
		jql = fmt.Sprintf("project = '%s' AND status in (%s) ORDER BY status ASC, updated DESC",
			project, strings.Join(quoted, ", "))
	}
	return c.SearchJQL(jql, 200)
}

// EpicIssues fetches all issues belonging to the given epic.
func (c *Client) EpicIssues(epicKey string) ([]jira.Issue, error) {
	res, err := c.inner.EpicIssues(epicKey, "", 0, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epic issues for %s: %w", epicKey, err)
	}
	issues := make([]jira.Issue, 0, len(res.Issues))
	for _, iss := range res.Issues {
		issues = append(issues, convertIssue(iss))
	}
	return issues, nil
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

	ch := make(chan result, 8)

	go func() {
		vals, err := c.fetchStatuses()
		ch <- result{"statuses", vals, err}
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

	return meta, nil
}

// SearchUsers searches for assignable users matching the given prefix.
func (c *Client) SearchUsers(project, prefix string) ([]string, error) {
	users, err := c.inner.UserSearchV2(&jiracli.UserSearchOptions{
		Project:    project,
		Query:      prefix,
		MaxResults: 10,
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(users))
	for _, u := range users {
		if u.DisplayName != "" {
			names = append(names, u.DisplayName)
		}
	}
	return names, nil
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

func (c *Client) fetchStatuses() ([]string, error) {
	return c.fetchNameList("/status")
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
		result = append(result, jira.Project{Key: p.Key, Name: p.Name})
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
