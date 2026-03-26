package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/seanhalberthal/jiru/internal/api"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/validate"
)

// SprintIssuesPage fetches a single page of sprint issues.
func (c *Client) SprintIssuesPage(sprintID, from, pageSize int) (*PageResult, error) {
	path := fmt.Sprintf("/sprint/%d/issue?startAt=%d&maxResults=%d&jql=%s",
		sprintID, from, pageSize, url.QueryEscape("ORDER BY updated DESC"))

	resp, err := c.http.Get(context.Background(), api.V1(path))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sprint issues: %w", err)
	}
	sr, err := api.DecodeResponse[api.SearchResult](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sprint issues: %w", err)
	}

	return c.toPageResult(sr, from), nil
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

// searchFields lists the issue fields requested from the v3 /search/jql API.
// Requesting specific fields instead of *all avoids known Jira Cloud bugs where
// large responses cause nextPageToken to be omitted, breaking cursor pagination.
// It also allows higher maxResults caps (up to 5000 vs ~100 with *all).
//
// description and comment are intentionally excluded: the v3 API returns them as
// ADF (Atlassian Document Format) JSON objects, not strings, so convertIssue
// discards them anyway. The issue detail view fetches via GetIssue (v2) which
// returns the string representations. Excluding these large fields dramatically
// reduces response size and improves pagination reliability.
const searchFields = "summary,status,priority,assignee,reporter,labels,issuetype,created,updated,parent"

// SearchJQLPage executes a JQL query and returns a single page of results.
// Uses the v3 /search/jql API with cursor-based (nextPageToken) pagination.
// The from parameter tracks cumulative progress for the MaxTotalIssues cap.
func (c *Client) SearchJQLPage(jql string, pageSize int, from int, nextToken string) (*PageResult, error) {
	path := fmt.Sprintf("/search/jql?jql=%s&maxResults=%d&fields=%s",
		url.QueryEscape(jql), pageSize, searchFields)
	if nextToken != "" {
		path += "&nextPageToken=" + url.QueryEscape(nextToken)
	}

	resp, err := c.http.Get(context.Background(), api.V3(path))
	if err != nil {
		return nil, err
	}
	sr, err := api.DecodeResponse[api.SearchResult](resp)
	if err != nil {
		return nil, err
	}

	result := c.toPageResult(sr, from)
	result.NextToken = sr.NextPageToken
	// For cursor-based pagination, override HasMore using the token.
	result.HasMore = sr.NextPageToken != "" && len(result.Issues) > 0 && (from+len(result.Issues)) < MaxTotalIssues
	return result, nil
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
		// Detect cursor loop — Jira Cloud has a known bug where nextPageToken
		// can repeat, returning the same page forever.
		if page.NextToken == nextToken {
			break
		}
		nextToken = page.NextToken
	}
	if len(all) > cap {
		all = all[:cap]
	}
	return all, nil
}

// BoardIssuesPage fetches a single page of issues for a board using the
// Agile v1 API with reliable offset-based pagination.
func (c *Client) BoardIssuesPage(boardID, from, pageSize int) (*PageResult, error) {
	path := fmt.Sprintf("/board/%d/issue?startAt=%d&maxResults=%d&jql=%s",
		boardID, from, pageSize, url.QueryEscape("ORDER BY updated DESC"))

	resp, err := c.http.Get(context.Background(), api.V1(path))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch board issues: %w", err)
	}
	sr, err := api.DecodeResponse[api.SearchResult](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch board issues: %w", err)
	}

	return c.toPageResult(sr, from), nil
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

	resp, err := c.http.Get(context.Background(), api.V1(path))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epic issues for %s: %w", epicKey, err)
	}
	sr, err := api.DecodeResponse[api.SearchResult](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epic issues for %s: %w", epicKey, err)
	}

	return c.toPageResult(sr, from), nil
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

// SprintIssueStats returns issue counts grouped by status category for a sprint.
// categoryFn maps a status name to its category (0=todo, 1=in progress, 2=done, 3=cancelled).
func (c *Client) SprintIssueStats(sprintID int, categoryFn func(string) int) (open, inProgress, done, total int, err error) {
	issues, err := c.SprintIssues(sprintID)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for _, iss := range issues {
		total++
		switch categoryFn(iss.Status) {
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

// ResolveParents fetches metadata for all unique parent keys found in the given issues.
// Uses a single JQL query (key in (...)) rather than N individual API calls.
func (c *Client) ResolveParents(issues []jira.Issue) map[string]ParentInfo {
	seen := make(map[string]bool)
	var keys []string
	for _, iss := range issues {
		if iss.ParentKey != "" && !seen[iss.ParentKey] {
			if validate.IssueKey(iss.ParentKey) != nil {
				continue
			}
			seen[iss.ParentKey] = true
			keys = append(keys, "'"+JQLEscape(iss.ParentKey)+"'")
		}
	}

	if len(keys) == 0 {
		return nil
	}

	jql := "key in (" + strings.Join(keys, ", ") + ")"
	searchIssues, err := c.SearchJQL(jql, uint(len(keys)))

	results := make(map[string]ParentInfo, len(keys))
	if err != nil {
		for _, k := range keys {
			results[k] = ParentInfo{Key: k}
		}
		return results
	}

	for _, iss := range searchIssues {
		results[iss.Key] = ParentInfo{
			Key:       iss.Key,
			Summary:   iss.Summary,
			IssueType: iss.IssueType,
		}
	}

	for _, k := range keys {
		if _, ok := results[k]; !ok {
			results[k] = ParentInfo{Key: k}
		}
	}

	return results
}

// Boards returns all boards visible to the authenticated user.
// If project is non-empty, filters to that project only.
func (c *Client) Boards(project string) ([]jira.Board, error) {
	path := "/board?maxResults=100"
	if project != "" {
		path += "&projectKeyOrId=" + url.QueryEscape(project)
	}

	resp, err := c.http.Get(context.Background(), api.V1(path))
	if err != nil {
		return nil, err
	}
	res, err := api.DecodeResponse[api.BoardResult](resp)
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

// BoardSprints returns sprints for a board filtered by state.
func (c *Client) BoardSprints(boardID int, state string) ([]jira.Sprint, error) {
	path := fmt.Sprintf("/board/%d/sprint?state=%s&startAt=0&maxResults=50", boardID, url.QueryEscape(state))

	resp, err := c.http.Get(context.Background(), api.V1(path))
	if err != nil {
		return nil, err
	}
	res, err := api.DecodeResponse[api.SprintResult](resp)
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

// BoardFilterJQL returns the JQL query associated with a board's filter.
// Uses the Agile v1 board configuration endpoint to get the filter ID, then
// fetches the filter's JQL from the v2 REST API.
func (c *Client) BoardFilterJQL(boardID int) (string, error) {
	cfgPath := fmt.Sprintf("/board/%d/configuration", boardID)
	cfgResp, err := c.http.Get(context.Background(), api.V1(cfgPath))
	if err != nil {
		return "", fmt.Errorf("board configuration: %w", err)
	}
	cfg, err := api.DecodeResponse[api.BoardConfigResponse](cfgResp)
	if err != nil {
		return "", fmt.Errorf("board configuration: %w", err)
	}
	if cfg.Filter.ID == "" {
		return "", fmt.Errorf("board %d has no filter", boardID)
	}

	filterPath := fmt.Sprintf("/filter/%s", cfg.Filter.ID)
	filterResp, err := c.http.Get(context.Background(), api.V2(filterPath))
	if err != nil {
		return "", fmt.Errorf("filter %s: %w", cfg.Filter.ID, err)
	}
	f, err := api.DecodeResponse[api.FilterResponse](filterResp)
	if err != nil {
		return "", fmt.Errorf("filter %s: %w", cfg.Filter.ID, err)
	}
	return f.JQL, nil
}
