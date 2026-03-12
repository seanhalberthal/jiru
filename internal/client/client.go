package client

import (
	"fmt"
	"time"

	jiracli "github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/seanhalberthal/jiratui/internal/config"
	"github.com/seanhalberthal/jiratui/internal/jira"
)

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
