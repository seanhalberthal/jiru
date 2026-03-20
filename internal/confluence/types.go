package confluence

import "time"

// Space represents a Confluence space.
type Space struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "global" or "personal"
	Description string `json:"description"`
}

// Page represents a Confluence page.
type Page struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	SpaceID   string    `json:"space_id"`
	SpaceKey  string    `json:"space_key"`
	ParentID  string    `json:"parent_id"`
	Status    string    `json:"status"`
	Version   int       `json:"version"`
	Author    string    `json:"author"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	BodyADF   string    `json:"body_adf"`     // Raw ADF JSON string
	BodyStore string    `json:"body_storage"` // Storage format (XHTML)
}

// PageSearchResult represents a page from CQL search results.
type PageSearchResult struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Excerpt string `json:"excerpt"`
}

// PageAncestor is a lightweight parent in the breadcrumb chain.
type PageAncestor struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Comment represents a comment on a Confluence page.
type Comment struct {
	ID      string    `json:"id"`
	Author  string    `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
}
