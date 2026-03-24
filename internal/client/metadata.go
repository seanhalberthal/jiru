package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/seanhalberthal/jiru/internal/api"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Me verifies authentication and returns the current user's display name.
func (c *Client) Me() (string, error) {
	resp, err := c.http.Get(context.Background(), api.V2("/myself"))
	if err != nil {
		return "", fmt.Errorf("auth check failed: %w", err)
	}
	me, err := api.DecodeResponse[api.MeResponse](resp)
	if err != nil {
		return "", fmt.Errorf("auth check failed: %w", err)
	}
	c.accountID = me.AccountID
	if me.DisplayName != "" {
		return me.DisplayName, nil
	}
	return me.Name, nil
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
// display name and email.
func (c *Client) SearchUsers(project, prefix string) ([]UserInfo, error) {
	path := fmt.Sprintf("/user/assignable/search?project=%s&query=%s&maxResults=10",
		url.QueryEscape(project), url.QueryEscape(prefix))

	resp, err := c.http.Get(context.Background(), api.V3(path))
	if err != nil {
		return nil, err
	}
	users, err := api.DecodeResponse[[]api.User](resp)
	if err != nil {
		return nil, err
	}

	infos := make([]UserInfo, 0, len(*users))
	for _, u := range *users {
		if u.DisplayName != "" {
			infos = append(infos, UserInfo{
				AccountID:   u.AccountID,
				DisplayName: u.DisplayName,
			})
		}
	}
	return infos, nil
}

// Projects returns all projects visible to the authenticated user.
func (c *Client) Projects() ([]jira.Project, error) {
	resp, err := c.http.Get(context.Background(), api.V2("/project"))
	if err != nil {
		return nil, err
	}
	projects, err := api.DecodeResponse[[]api.Project](resp)
	if err != nil {
		return nil, err
	}
	result := make([]jira.Project, 0, len(*projects))
	for _, p := range *projects {
		result = append(result, jira.Project{Key: p.Key, Name: p.Name, Type: p.Type})
	}
	return result, nil
}

// IssueTypes returns available issue types for a project.
// Falls back to the global issue type list if the project-specific fetch fails.
func (c *Client) IssueTypes(project string) ([]string, error) {
	if project != "" {
		path := fmt.Sprintf("/issue/createmeta?projectKeys=%s&expand=projects.issuetypes",
			url.QueryEscape(project))
		resp, err := c.http.Get(context.Background(), api.V2(path))
		if err == nil {
			meta, decErr := api.DecodeResponse[api.CreateMetaResponse](resp)
			if decErr == nil && len(meta.Projects) > 0 {
				var types []string
				for _, it := range meta.Projects[0].IssueTypes {
					types = append(types, it.Name)
				}
				if len(types) > 0 {
					return types, nil
				}
			}
		}
	}
	return c.fetchIssueTypes()
}

// IssueTypesWithID returns available issue types with their IDs for a project.
func (c *Client) IssueTypesWithID(project string) ([]jira.IssueTypeInfo, error) {
	if project != "" {
		path := fmt.Sprintf("/issue/createmeta?projectKeys=%s&expand=projects.issuetypes",
			url.QueryEscape(project))
		resp, err := c.http.Get(context.Background(), api.V2(path))
		if err == nil {
			meta, decErr := api.DecodeResponse[api.CreateMetaResponse](resp)
			if decErr == nil && len(meta.Projects) > 0 {
				var types []jira.IssueTypeInfo
				for _, it := range meta.Projects[0].IssueTypes {
					types = append(types, jira.IssueTypeInfo{ID: it.ID, Name: it.Name})
				}
				if len(types) > 0 {
					return types, nil
				}
			}
		}
	}
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
	resp, err := c.http.Get(context.Background(), api.V3(path))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch create metadata fields: %w", err)
	}
	result, err := api.DecodeResponse[api.CreateMetaFieldsResponse](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch create metadata fields: %w", err)
	}

	skip := map[string]bool{
		"summary": true, "issuetype": true, "project": true,
		"description": true, "priority": true, "assignee": true,
		"labels": true, "reporter": true, "components": true,
		"parent": true, "attachment": true, "issuelinks": true,
	}

	var fields []jira.CustomFieldDef
	for _, f := range result.Values {
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

// --- internal fetch helpers ---

func (c *Client) fetchNameList(path string) ([]string, error) {
	resp, err := c.http.Get(context.Background(), api.V2(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path)
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
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

type statusResult struct {
	names      []string
	categories map[string]int
}

func (c *Client) fetchStatuses() (*statusResult, error) {
	resp, err := c.http.Get(context.Background(), api.V2("/status"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for /status", resp.StatusCode)
	}
	var items []api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
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
		default:
			sr.categories[item.Name] = 0
		}
	}
	return sr, nil
}

func (c *Client) fetchIssueTypes() ([]string, error)  { return c.fetchNameList("/issuetype") }
func (c *Client) fetchPriorities() ([]string, error)  { return c.fetchNameList("/priority") }
func (c *Client) fetchResolutions() ([]string, error) { return c.fetchNameList("/resolution") }
func (c *Client) fetchComponents(p string) ([]string, error) {
	return c.fetchNameList(fmt.Sprintf("/project/%s/components", p))
}

func (c *Client) fetchLabels() ([]string, error) {
	resp, err := c.http.Get(context.Background(), api.V2("/label"))
	if err != nil {
		return nil, err
	}
	result, err := api.DecodeResponse[api.LabelResponse](resp)
	if err != nil {
		return nil, err
	}
	return result.Values, nil
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

func (c *Client) fetchVersions(project string) ([]string, error) {
	path := fmt.Sprintf("/project/%s/version?released=false&maxResults=100", project)
	resp, err := c.http.Get(context.Background(), api.V2(path))
	if err != nil {
		return nil, err
	}
	versions, err := api.DecodeResponse[[]api.ProjectVersion](resp)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(*versions))
	for _, v := range *versions {
		if !v.Released && !v.Archived {
			names = append(names, v.Name)
		}
	}
	return names, nil
}
