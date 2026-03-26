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
		if path == "" {
			break
		}
	}

	return all, nil
}

// ConfluencePage fetches a single Confluence page by ID, including its ADF body.
func (c *Client) ConfluencePage(pageID string) (*confluence.Page, error) {
	path := api.Wiki(fmt.Sprintf("/pages/%s?body-format=atlas_doc_format", url.PathEscape(pageID)))

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
	path := api.Wiki(fmt.Sprintf("/pages/%s/ancestors", url.PathEscape(pageID)))

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
	path := api.Wiki(fmt.Sprintf("/spaces/%s/pages?limit=%d&sort=title", url.PathEscape(spaceID), limit))

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

// UpdateConfluencePage updates a Confluence page's title and/or body.
// The version must be the current version number + 1 (optimistic locking).
// bodyADF is the ADF JSON string for the page body.
func (c *Client) UpdateConfluencePage(pageID, title, bodyADF string, version int) (*confluence.Page, error) {
	body := map[string]any{
		"id":     pageID,
		"status": "current",
		"title":  title,
		"version": map[string]any{
			"number": version,
		},
		"body": map[string]any{
			"representation": "atlas_doc_format",
			"value":          bodyADF,
		},
	}

	path := api.Wiki(fmt.Sprintf("/pages/%s", url.PathEscape(pageID)))
	resp, err := c.http.Put(context.Background(), path, body)
	if err != nil {
		return nil, fmt.Errorf("update confluence page %s: %w", pageID, err)
	}
	result, err := api.DecodeResponse[api.ConfluencePage](resp)
	if err != nil {
		return nil, fmt.Errorf("update confluence page %s: %w", pageID, err)
	}

	return convertConfluencePage(result), nil
}

// ConfluenceSearchCQL searches Confluence using CQL (Confluence Query Language).
// Uses the v1 /wiki/rest/api/search endpoint with offset-based pagination.
func (c *Client) ConfluenceSearchCQL(cql string, limit int) ([]confluence.PageSearchResult, error) {
	if limit <= 0 {
		limit = 25
	}
	path := "/wiki/rest/api/search?limit=" + fmt.Sprintf("%d", limit) +
		"&cql=" + url.QueryEscape(cql) +
		"&expand=content.space"

	resp, err := c.http.Get(context.Background(), path)
	if err != nil {
		return nil, fmt.Errorf("confluence search: %w", err)
	}
	result, err := api.DecodeResponse[api.ConfluenceSearchResult](resp)
	if err != nil {
		return nil, fmt.Errorf("confluence search: %w", err)
	}

	out := make([]confluence.PageSearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		out = append(out, confluence.PageSearchResult{
			ID:      r.Content.ID,
			Title:   r.Content.Title,
			Excerpt: r.Excerpt,
		})
	}
	return out, nil
}

// ConfluencePageComments returns footer and inline comments for a Confluence page.
// Footer comments are returned first, then inline comments, each sorted by creation date.
func (c *Client) ConfluencePageComments(pageID string) ([]confluence.Comment, error) {
	footer, err := c.fetchComments(pageID, "footer-comments", false)
	if err != nil {
		return nil, err
	}
	inline, err := c.fetchComments(pageID, "inline-comments", true)
	if err != nil {
		return nil, err
	}
	return append(footer, inline...), nil
}

// fetchComments paginates through a comment endpoint and converts to domain comments.
func (c *Client) fetchComments(pageID, endpoint string, inline bool) ([]confluence.Comment, error) {
	var all []confluence.Comment
	path := api.Wiki(fmt.Sprintf("/pages/%s/%s?body-format=atlas_doc_format&sort=-created-date&limit=25",
		url.PathEscape(pageID), endpoint))

	for {
		resp, err := c.http.Get(context.Background(), path)
		if err != nil {
			return nil, fmt.Errorf("confluence %s %s: %w", endpoint, pageID, err)
		}
		result, err := api.DecodeResponse[api.ConfluenceCommentsResult](resp)
		if err != nil {
			return nil, fmt.Errorf("confluence %s %s: %w", endpoint, pageID, err)
		}

		for _, r := range result.Results {
			all = append(all, convertConfluenceComment(&r, inline))
		}

		if result.Links.Next == "" {
			break
		}
		path = extractPath(result.Links.Next)
		if path == "" {
			break
		}
	}
	return all, nil
}

// convertConfluenceComment converts an API comment to a domain comment.
func convertConfluenceComment(c *api.ConfluenceComment, inline bool) confluence.Comment {
	comment := confluence.Comment{
		ID:     c.ID,
		Inline: inline,
	}
	if c.Version != nil {
		comment.Author = c.Version.AuthorID
		if t, err := time.Parse(time.RFC3339, c.Version.CreatedAt); err == nil {
			comment.Created = t
		}
	}
	if c.Body != nil && c.Body.AtlasDocFormat != nil {
		comment.BodyADF = c.Body.AtlasDocFormat.Value
	}
	if inline {
		comment.ResolutionStatus = c.ResolutionStatus
		if c.Properties != nil {
			comment.MarkerRef = c.Properties.InlineMarkerRef
			comment.HighlightedText = c.Properties.InlineOriginalSelection
		}
	}
	return comment
}

// ConfluencePageURL returns the browser URL for a Confluence page.
func (c *Client) ConfluencePageURL(pageID string) string {
	return c.config.ServerURL() + "/wiki/pages/" + url.PathEscape(pageID)
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
// Returns empty string on parse failure — callers treat this as a pagination terminator.
func extractPath(fullURL string) string {
	u, err := url.Parse(fullURL)
	if err != nil || u.Path == "" {
		return ""
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}
