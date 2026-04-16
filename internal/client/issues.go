package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/seanhalberthal/jiru/internal/api"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/validate"
)

// GetIssue fetches full details for a single issue.
func (c *Client) GetIssue(key string) (*jira.Issue, error) {
	resp, err := c.http.Get(context.Background(), api.V2(fmt.Sprintf("/issue/%s", key)))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue %s: %w", key, err)
	}
	iss, err := api.DecodeResponse[api.Issue](resp)
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

// CreateIssue creates a new issue in Jira.
func (c *Client) CreateIssue(req *CreateIssueRequest) (*CreateIssueResponse, error) {
	payload := c.buildCreatePayload(req)

	resp, err := c.http.Post(context.Background(), api.V2("/issue"), payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}
	result, err := api.DecodeResponse[api.CreateResponse](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return &CreateIssueResponse{Key: result.Key}, nil
}

func (c *Client) buildCreatePayload(req *CreateIssueRequest) map[string]any {
	fields := map[string]any{
		"project":   map[string]string{"key": req.Project},
		"issuetype": map[string]string{"name": req.IssueType},
		"summary":   req.Summary,
	}

	if req.Description != "" {
		fields["description"] = req.Description
	}
	if req.Priority != "" {
		fields["priority"] = map[string]string{"name": req.Priority}
	}
	if req.Assignee != "" {
		if c.config.AuthType == "bearer" {
			fields["assignee"] = map[string]string{"name": req.Assignee}
		} else {
			fields["assignee"] = map[string]string{"accountId": req.Assignee}
		}
	}
	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}
	if len(req.Components) > 0 {
		comps := make([]map[string]string, len(req.Components))
		for i, name := range req.Components {
			comps[i] = map[string]string{"name": name}
		}
		fields["components"] = comps
	}
	if req.ParentKey != "" {
		fields["parent"] = map[string]string{"key": req.ParentKey}
	}

	for id, val := range req.CustomFields {
		switch v := val.(type) {
		case string:
			fields[id] = v
		case float64:
			fields[id] = v
		case map[string]string:
			fields[id] = map[string]string{"value": v["value"]}
		}
	}

	return map[string]any{"fields": fields}
}

// EditIssue updates fields on an existing issue.
// Only non-empty fields in the request are sent.
func (c *Client) EditIssue(key string, req *EditIssueRequest) error {
	fields := make(map[string]any)
	if req.Summary != "" {
		fields["summary"] = req.Summary
	}
	if req.Description != "" {
		fields["description"] = req.Description
	}
	if req.Priority != "" {
		fields["priority"] = map[string]string{"name": req.Priority}
	}
	if req.Labels != nil {
		fields["labels"] = req.Labels
	}

	body := map[string]any{"fields": fields}
	resp, err := c.http.Put(context.Background(), api.V2(fmt.Sprintf("/issue/%s", key)), body)
	if err != nil {
		return fmt.Errorf("failed to edit %s: %w", key, err)
	}
	return api.CheckResponse(resp)
}

// DeleteIssue deletes an issue. If cascade is true, subtasks are also deleted.
func (c *Client) DeleteIssue(key string, cascade bool) error {
	path := fmt.Sprintf("/issue/%s?deleteSubtasks=%t", key, cascade)
	resp, err := c.http.Delete(context.Background(), api.V2(path))
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", key, err)
	}
	return api.CheckResponse(resp)
}

// AssignIssue assigns an issue to a user by account ID.
// Pass "none" to unassign, "default" to assign to the current authenticated user.
func (c *Client) AssignIssue(key, accountID string) error {
	field := "accountId"
	if c.config.AuthType == "bearer" {
		field = "name"
	}

	var value any
	switch accountID {
	case "none":
		value = nil
	case "default":
		if c.config.AuthType == "bearer" {
			value = c.userName
		} else {
			value = c.accountID
		}
	default:
		value = accountID
	}

	body := map[string]any{field: value}
	resp, err := c.http.Put(context.Background(), api.V2(fmt.Sprintf("/issue/%s/assignee", key)), body)
	if err != nil {
		return fmt.Errorf("failed to assign %s: %w", key, err)
	}
	return api.CheckResponse(resp)
}

// Transitions returns the available status transitions for an issue.
func (c *Client) Transitions(key string) ([]jira.Transition, error) {
	if err := validate.IssueKey(key); err != nil {
		return nil, fmt.Errorf("Transitions: %w", err)
	}
	path := fmt.Sprintf("/issue/%s/transitions", key)
	resp, err := c.http.Get(context.Background(), api.V2(path))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transitions for %s: %w", key, err)
	}
	result, err := api.DecodeResponse[api.TransitionResponse](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transitions for %s: %w", key, err)
	}

	transitions := make([]jira.Transition, 0, len(result.Transitions))
	for _, t := range result.Transitions {
		transitions = append(transitions, jira.Transition{
			ID:       t.ID,
			Name:     t.Name,
			ToStatus: t.To.Name,
		})
	}
	return transitions, nil
}

// TransitionIssue performs a status transition on an issue.
func (c *Client) TransitionIssue(key, transitionID string) error {
	body := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	resp, err := c.http.Post(context.Background(), api.V2(fmt.Sprintf("/issue/%s/transitions", key)), body)
	if err != nil {
		return fmt.Errorf("failed to transition %s: %w", key, err)
	}
	return api.CheckResponse(resp)
}

// AddComment posts a comment on an issue.
func (c *Client) AddComment(key, body string) error {
	payload := map[string]string{"body": body}
	resp, err := c.http.Post(context.Background(), api.V2(fmt.Sprintf("/issue/%s/comment", key)), payload)
	if err != nil {
		return fmt.Errorf("failed to add comment to %s: %w", key, err)
	}
	return api.CheckResponse(resp)
}

// WatchIssue adds the current user as a watcher for the given issue.
// Jira Cloud requires the account ID; Server/DC requires the username.
func (c *Client) WatchIssue(key string) error {
	ident := c.accountID
	if c.config.AuthType == "bearer" {
		ident = c.userName
	}
	resp, err := c.http.Post(context.Background(), api.V2(fmt.Sprintf("/issue/%s/watchers", key)), ident)
	if err != nil {
		return fmt.Errorf("failed to watch %s: %w", key, err)
	}
	return api.CheckResponse(resp)
}

// UnwatchIssue removes the current user as a watcher for the given issue.
func (c *Client) UnwatchIssue(key string) error {
	var path string
	if c.config.AuthType == "bearer" {
		path = fmt.Sprintf("/issue/%s/watchers?username=%s", key, url.QueryEscape(c.userName))
	} else {
		path = fmt.Sprintf("/issue/%s/watchers?accountId=%s", key, url.QueryEscape(c.accountID))
	}
	resp, err := c.http.Delete(context.Background(), api.V2(path))
	if err != nil {
		return fmt.Errorf("failed to unwatch %s: %w", key, err)
	}
	return api.CheckResponse(resp)
}

// ChildIssues fetches child issues for the given parent key.
// Epics use the agile epic endpoint; other issues use a parent-key JQL query.
func (c *Client) ChildIssues(key, issueType string) ([]jira.ChildIssue, error) {
	if err := validate.IssueKey(key); err != nil {
		return nil, fmt.Errorf("ChildIssues: %w", err)
	}

	var issues []jira.Issue
	var err error
	if strings.EqualFold(issueType, "epic") {
		issues, err = c.EpicIssues(key)
	} else {
		jql := fmt.Sprintf("parent = '%s' ORDER BY status ASC, key ASC", JQLEscape(key))
		issues, err = c.SearchJQL(jql, 50)
	}
	if err != nil {
		return nil, err
	}
	children := make([]jira.ChildIssue, 0, len(issues))
	for _, iss := range issues {
		acronym, unassigned := c.childAssigneeBadge(iss.Assignee, iss.AssigneeAcronym)
		children = append(children, jira.ChildIssue{
			Key:             iss.Key,
			Summary:         iss.Summary,
			Status:          iss.Status,
			IssueType:       iss.IssueType,
			Assignee:        iss.Assignee,
			AssigneeAcronym: acronym,
			Unassigned:      unassigned,
		})
	}
	return children, nil
}

func (c *Client) childAssigneeBadge(displayName, apiAcronym string) (string, bool) {
	if strings.TrimSpace(displayName) == "" {
		return "??", true
	}

	if apiAcronym = strings.TrimSpace(apiAcronym); apiAcronym != "" {
		return strings.ToUpper(apiAcronym), false
	}

	cacheKey := strings.ToLower(strings.TrimSpace(displayName))
	c.acronymMu.RLock()
	if acronym, ok := c.acronymCache[cacheKey]; ok {
		c.acronymMu.RUnlock()
		return acronym, false
	}
	c.acronymMu.RUnlock()

	acronym := deriveAcronym(displayName)
	c.acronymMu.Lock()
	c.acronymCache[cacheKey] = acronym
	c.acronymMu.Unlock()
	return acronym, false
}

func deriveAcronym(displayName string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "??"
	}

	parts := strings.FieldsFunc(displayName, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_' || r == '.'
	})
	var letters []rune
	for _, part := range parts {
		for _, r := range part {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters = append(letters, unicode.ToUpper(r))
				break
			}
		}
		if len(letters) == 3 {
			break
		}
	}
	if len(letters) == 0 {
		for _, r := range displayName {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters = append(letters, unicode.ToUpper(r))
			}
			if len(letters) == 2 {
				break
			}
		}
	}
	if len(letters) == 0 {
		return "??"
	}
	return string(letters)
}

// LinkIssue creates a link between two issues.
func (c *Client) LinkIssue(inwardKey, outwardKey, linkType string) error {
	body := map[string]any{
		"type":         map[string]string{"name": linkType},
		"inwardIssue":  map[string]string{"key": inwardKey},
		"outwardIssue": map[string]string{"key": outwardKey},
	}
	resp, err := c.http.Post(context.Background(), api.V2("/issueLink"), body)
	if err != nil {
		return fmt.Errorf("failed to link %s → %s: %w", inwardKey, outwardKey, err)
	}
	return api.CheckResponse(resp)
}

// GetIssueLinkTypes returns all available issue link types.
func (c *Client) GetIssueLinkTypes() ([]jira.IssueLinkType, error) {
	resp, err := c.http.Get(context.Background(), api.V2("/issueLinkType"))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch link types: %w", err)
	}
	result, err := api.DecodeResponse[api.IssueLinkTypesResponse](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch link types: %w", err)
	}
	types := make([]jira.IssueLinkType, 0, len(result.IssueLinkTypes))
	for _, t := range result.IssueLinkTypes {
		types = append(types, jira.IssueLinkType{
			ID:      t.ID,
			Name:    t.Name,
			Inward:  t.Inward,
			Outward: t.Outward,
		})
	}
	return types, nil
}
