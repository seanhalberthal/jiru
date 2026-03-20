package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/seanhalberthal/jiru/internal/api"
	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/jira"
)

// ConfluenceSpaces returns all Confluence spaces visible to the authenticated user.
func (c *Client) ConfluenceSpaces() ([]confluence.Space, error) {
	var all []confluence.Space
	path := api.Wiki("/spaces?limit=250&sort=name")

	for {
		resp, err := c.http.Get(context.Background(), path)
		if err != nil {
			return nil, fmt.Errorf("confluence spaces: %w", err)
		}
		result, err := api.DecodeResponse[api.ConfluenceSpacesResult](resp)
		if err != nil {
			return nil, fmt.Errorf("confluence spaces: %w", err)
		}

		for _, s := range result.Results {
			desc := ""
			if s.Description != nil && s.Description.Plain != nil {
				desc = s.Description.Plain.Value
			}
			all = append(all, confluence.Space{
				ID:          s.ID,
				Key:         s.Key,
				Name:        s.Name,
				Type:        s.Type,
				Description: desc,
			})
		}

		if result.Links.Next == "" {
			break
		}
		// The next URL is a full path — extract path+query portion.
		path = extractPath(result.Links.Next)
	}

	return all, nil
}

// ConfluencePage fetches a single Confluence page by ID, including its ADF body.
func (c *Client) ConfluencePage(pageID string) (*confluence.Page, error) {
	path := api.Wiki(fmt.Sprintf("/pages/%s?body-format=atlas_doc_format", pageID))

	resp, err := c.http.Get(context.Background(), path)
	if err != nil {
		return nil, fmt.Errorf("confluence page %s: %w", pageID, err)
	}
	result, err := api.DecodeResponse[api.ConfluencePage](resp)
	if err != nil {
		return nil, fmt.Errorf("confluence page %s: %w", pageID, err)
	}

	return convertConfluencePage(result), nil
}

// ConfluencePageAncestors returns the ancestor chain for a page (root → immediate parent).
func (c *Client) ConfluencePageAncestors(pageID string) ([]confluence.PageAncestor, error) {
	path := api.Wiki(fmt.Sprintf("/pages/%s/ancestors", pageID))

	resp, err := c.http.Get(context.Background(), path)
	if err != nil {
		return nil, fmt.Errorf("confluence page ancestors %s: %w", pageID, err)
	}
	result, err := api.DecodeResponse[api.ConfluenceAncestorsResult](resp)
	if err != nil {
		return nil, fmt.Errorf("confluence page ancestors %s: %w", pageID, err)
	}

	ancestors := make([]confluence.PageAncestor, 0, len(result.Results))
	for _, a := range result.Results {
		ancestors = append(ancestors, confluence.PageAncestor{
			ID:    a.ID,
			Title: a.Title,
		})
	}
	return ancestors, nil
}

// ConfluenceSpacePages returns pages in a space, sorted by title.
func (c *Client) ConfluenceSpacePages(spaceID string, limit int) ([]confluence.Page, error) {
	if limit <= 0 {
		limit = 50
	}
	path := api.Wiki(fmt.Sprintf("/spaces/%s/pages?limit=%d&sort=title", spaceID, limit))

	resp, err := c.http.Get(context.Background(), path)
	if err != nil {
		return nil, fmt.Errorf("confluence space pages %s: %w", spaceID, err)
	}
	result, err := api.DecodeResponse[api.ConfluencePagesResult](resp)
	if err != nil {
		return nil, fmt.Errorf("confluence space pages %s: %w", spaceID, err)
	}

	pages := make([]confluence.Page, 0, len(result.Results))
	for _, p := range result.Results {
		pages = append(pages, *convertConfluencePage(&p))
	}
	return pages, nil
}

// ConfluencePageURL returns the browser URL for a Confluence page.
func (c *Client) ConfluencePageURL(pageID string) string {
	return c.config.ServerURL() + "/wiki/pages/" + pageID
}

// RemoteLinks returns the remote links for a Jira issue, filtered to those
// on the same Atlassian instance.
func (c *Client) RemoteLinks(key string) ([]jira.RemoteLink, error) {
	path := api.V2(fmt.Sprintf("/issue/%s/remotelink", key))

	resp, err := c.http.Get(context.Background(), path)
	if err != nil {
		return nil, fmt.Errorf("remote links for %s: %w", key, err)
	}
	results, err := api.DecodeResponse[[]api.RemoteLinkResponse](resp)
	if err != nil {
		return nil, fmt.Errorf("remote links for %s: %w", key, err)
	}

	baseURL := c.config.ServerURL()
	var links []jira.RemoteLink
	for _, r := range *results {
		// Only include links from the same instance.
		if !strings.HasPrefix(r.Object.URL, baseURL) {
			continue
		}
		icon := ""
		if r.Object.Icon != nil {
			icon = r.Object.Icon.Title
		}
		links = append(links, jira.RemoteLink{
			ID:    r.ID,
			Title: r.Object.Title,
			URL:   r.Object.URL,
			Icon:  icon,
		})
	}
	return links, nil
}

// GetUserDisplayName resolves an Atlassian account ID to a display name.
// Returns the raw accountID if the lookup fails.
func (c *Client) GetUserDisplayName(accountID string) string {
	if accountID == "" {
		return ""
	}
	path := api.V2(fmt.Sprintf("/user?accountId=%s", url.QueryEscape(accountID)))
	resp, err := c.http.Get(context.Background(), path)
	if err != nil {
		return accountID
	}
	user, err := api.DecodeResponse[api.User](resp)
	if err != nil || user.DisplayName == "" {
		return accountID
	}
	return user.DisplayName
}

// convertConfluencePage converts an API page to a domain page.
func convertConfluencePage(p *api.ConfluencePage) *confluence.Page {
	page := &confluence.Page{
		ID:       p.ID,
		Title:    p.Title,
		SpaceID:  p.SpaceID,
		ParentID: p.ParentID,
		Status:   p.Status,
	}

	if p.Version != nil {
		page.Version = p.Version.Number
		page.Author = p.Version.AuthorID
		if t, err := time.Parse(time.RFC3339, p.Version.CreatedAt); err == nil {
			page.Updated = t
		}
	}

	if t, err := time.Parse(time.RFC3339, p.CreatedAt); err == nil {
		page.Created = t
	}

	if p.Body != nil {
		if p.Body.AtlasDocFormat != nil {
			page.BodyADF = p.Body.AtlasDocFormat.Value
		}
		if p.Body.Storage != nil {
			page.BodyStore = p.Body.Storage.Value
		}
	}

	return page
}

// extractPath strips the scheme+host from a full URL, returning just the path+query.
func extractPath(fullURL string) string {
	u, err := url.Parse(fullURL)
	if err != nil {
		return fullURL
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}
