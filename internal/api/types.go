package api

import (
	"encoding/json"
	"strings"
)

// Issue is the raw JSON shape returned by the Jira REST API.
// Only fields used by jiru are included.
type Issue struct {
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// StoryPointFieldIDs lists the most common custom-field IDs that hold
// story points across Jira instances. Used as a fallback when runtime
// discovery hasn't resolved the tenant-specific field ID.
var StoryPointFieldIDs = []string{
	"customfield_10016", // Cloud "Story point estimate" on next-gen projects
	"customfield_10026", // Some Cloud variants
	"customfield_10002", // Classic Server/DC
	"customfield_10004", // Older next-gen tenants
}

// IssueFields contains the fields nested under an issue.
type IssueFields struct {
	Summary     string      `json:"summary"`
	Description any         `json:"description"` // string (v2) or ADF object (v3)
	Status      NameField   `json:"status"`
	Priority    NameField   `json:"priority"`
	Assignee    UserField   `json:"assignee"`
	Reporter    UserField   `json:"reporter"`
	IssueType   IssueType   `json:"issuetype"`
	Parent      *ParentRef  `json:"parent,omitempty"`
	Labels      []string    `json:"labels"`
	FixVersions []NameField `json:"fixVersions"`
	Created     string      `json:"created"`
	Updated     string      `json:"updated"`
	Comment     CommentWrap `json:"comment"`
	Watches     WatchField  `json:"watches"`
	IssueLinks  []IssueLink `json:"issuelinks"`

	// Custom captures all customfield_* values from the response so the
	// consumer can look up tenant-specific fields (e.g. story points) by ID
	// after the field is discovered at runtime. Populated by UnmarshalJSON.
	Custom map[string]json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the named struct fields normally and additionally
// captures every customfield_* key into Custom so callers can resolve
// tenant-specific fields (story points etc.) by ID.
func (f *IssueFields) UnmarshalJSON(data []byte) error {
	type alias IssueFields
	aux := (*alias)(f)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	custom := make(map[string]json.RawMessage)
	for k, v := range raw {
		if strings.HasPrefix(k, "customfield_") {
			custom[k] = v
		}
	}
	f.Custom = custom
	return nil
}

// StoryPoints returns the first numeric value found across preferredIDs (in
// order), then falls back to KnownStoryPointFieldIDs. Returns nil when no
// candidate decodes to a JSON number.
func (f IssueFields) StoryPoints(preferredIDs ...string) *float64 {
	seen := make(map[string]bool, len(preferredIDs)+len(StoryPointFieldIDs))
	candidates := make([]string, 0, len(preferredIDs)+len(StoryPointFieldIDs))
	for _, id := range preferredIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		candidates = append(candidates, id)
	}
	for _, id := range StoryPointFieldIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		candidates = append(candidates, id)
	}

	for _, id := range candidates {
		raw, ok := f.Custom[id]
		if !ok || len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var n float64
		if err := json.Unmarshal(raw, &n); err == nil {
			return &n
		}
	}
	return nil
}

// WatchField holds watcher information from the API.
type WatchField struct {
	IsWatching bool `json:"isWatching"`
	WatchCount int  `json:"watchCount"`
}

// NameField is a JSON object with a "name" field.
type NameField struct {
	Name string `json:"name"`
}

// UserField holds user information from the API.
type UserField struct {
	Name        string `json:"name"`        // Username (v2 / Server)
	DisplayName string `json:"displayName"` // Full name
	AccountID   string `json:"accountId"`   // Cloud account ID
	Acronym     string `json:"acronym"`     // Optional compact identifier when present.
}

// IssueType holds issue type information.
type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// ParentRef is a reference to a parent issue.
type ParentRef struct {
	Key string `json:"key"`
}

// CommentWrap holds the comment container from the API.
type CommentWrap struct {
	Comments []Comment `json:"comments"`
	Total    int       `json:"total"`
}

// Comment is a single issue comment.
type Comment struct {
	Author  UserField `json:"author"`
	Body    any       `json:"body"` // string (v2) or ADF object (v3)
	Created string    `json:"created"`
}

// IssueLink is a directional relationship attached to an issue.
type IssueLink struct {
	Type         IssueLinkTypeRef `json:"type"`
	InwardIssue  *LinkedIssueRef  `json:"inwardIssue"`
	OutwardIssue *LinkedIssueRef  `json:"outwardIssue"`
}

// IssueLinkTypeRef describes how a relationship reads from each side.
type IssueLinkTypeRef struct {
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// LinkedIssueRef is the linked issue payload embedded under an issue link.
type LinkedIssueRef struct {
	Key    string               `json:"key"`
	Fields LinkedIssueRefFields `json:"fields"`
}

// LinkedIssueRefFields contains the subset of linked issue fields the UI uses.
type LinkedIssueRefFields struct {
	Summary   string    `json:"summary"`
	Status    NameField `json:"status"`
	IssueType IssueType `json:"issuetype"`
}

// SearchResult is the response from search endpoints.
type SearchResult struct {
	Issues        []*Issue `json:"issues"`
	Total         int      `json:"total"`
	MaxResults    int      `json:"maxResults"`
	StartAt       int      `json:"startAt"`
	IsLast        bool     `json:"isLast"`        // Agile v1 — unreliable
	NextPageToken string   `json:"nextPageToken"` // v3 JQL search
}

// BoardResult is the response from the boards endpoint.
type BoardResult struct {
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
	Boards     []Board `json:"values"`
}

// Board represents a Jira board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// SprintResult is the response from the sprints endpoint.
type SprintResult struct {
	MaxResults int      `json:"maxResults"`
	IsLast     bool     `json:"isLast"`
	Sprints    []Sprint `json:"values"`
}

// Sprint represents a Jira sprint.
type Sprint struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"state"` // "active", "closed", "future"
}

// Project represents a Jira project.
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"projectTypeKey"`
}

// ProjectVersion represents a project release/version.
type ProjectVersion struct {
	Name     string `json:"name"`
	Released bool   `json:"released"`
	Archived bool   `json:"archived"`
}

// User represents a user from the user search endpoint.
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

// MeResponse is the response from the /myself endpoint.
type MeResponse struct {
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
	AccountID   string `json:"accountId"`
}

// CreateResponse is the response from creating an issue.
type CreateResponse struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// TransitionResponse is the response from the transitions endpoint.
type TransitionResponse struct {
	Transitions []Transition `json:"transitions"`
}

// Transition represents an available status transition.
type Transition struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	To   NameField `json:"to"`
}

// CreateMetaResponse is the response from the create metadata endpoint.
type CreateMetaResponse struct {
	Projects []CreateMetaProject `json:"projects"`
}

// CreateMetaProject holds project-level create metadata.
type CreateMetaProject struct {
	Key        string           `json:"key"`
	IssueTypes []CreateMetaType `json:"issuetypes"`
}

// CreateMetaType holds issue type info from create metadata.
type CreateMetaType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateMetaFieldsResponse is the response from the create metadata fields endpoint.
type CreateMetaFieldsResponse struct {
	Values []CreateMetaField `json:"values"`
}

// CreateMetaField is a single field definition from create metadata.
type CreateMetaField struct {
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
}

// IssueLinkType represents a type of link between issues.
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// IssueLinkTypesResponse wraps the link types endpoint response.
type IssueLinkTypesResponse struct {
	IssueLinkTypes []IssueLinkType `json:"issueLinkTypes"`
}

// StatusResponse is a single status from the /status endpoint.
type StatusResponse struct {
	Name           string `json:"name"`
	StatusCategory struct {
		Key string `json:"key"`
	} `json:"statusCategory"`
}

// LabelResponse is the response from the /label endpoint.
type LabelResponse struct {
	Values []string `json:"values"`
}

// BoardConfigResponse is the response from the board configuration endpoint.
type BoardConfigResponse struct {
	Filter struct {
		ID string `json:"id"`
	} `json:"filter"`
}

// FilterResponse is the response from the filter endpoint.
type FilterResponse struct {
	JQL string `json:"jql"`
}

// --- Confluence API response types ---

// ConfluenceSpace is the API response shape for a Confluence space.
type ConfluenceSpace struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Type        string `json:"type"`   // "global" or "personal"
	Status      string `json:"status"` // "current"
	Description *struct {
		Plain *struct {
			Value string `json:"value"`
		} `json:"plain"`
	} `json:"description"`
}

// ConfluenceSpacesResult is the paginated response for listing spaces.
type ConfluenceSpacesResult struct {
	Results []ConfluenceSpace `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// ConfluencePage is the API response shape for a Confluence page.
type ConfluencePage struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	SpaceID   string `json:"spaceId"`
	ParentID  string `json:"parentId"`
	AuthorID  string `json:"authorId"`
	CreatedAt string `json:"createdAt"`
	Version   *struct {
		Number    int    `json:"number"`
		Message   string `json:"message"`
		CreatedAt string `json:"createdAt"`
		AuthorID  string `json:"authorId"`
	} `json:"version"`
	Body *struct {
		Storage *struct {
			Value string `json:"value"`
		} `json:"storage"`
		AtlasDocFormat *struct {
			Value string `json:"value"` // JSON string (double-encoded ADF)
		} `json:"atlas_doc_format"`
	} `json:"body"`
}

// ConfluencePagesResult is the paginated response for listing/searching pages.
type ConfluencePagesResult struct {
	Results []ConfluencePage `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// ConfluenceAncestor is a page ancestor from the v2 ancestors endpoint.
type ConfluenceAncestor struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ConfluenceAncestorsResult is the response for page ancestors.
type ConfluenceAncestorsResult struct {
	Results []ConfluenceAncestor `json:"results"`
}

// ConfluenceSearchResult is the v1 CQL search response.
type ConfluenceSearchResult struct {
	Results []struct {
		Content struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
		} `json:"content"`
		Excerpt string `json:"excerpt"`
	} `json:"results"`
	Start     int `json:"start"`
	Limit     int `json:"limit"`
	Size      int `json:"size"`
	TotalSize int `json:"totalSize"`
}

// ConfluenceComment is the API response shape for a Confluence comment (footer or inline).
type ConfluenceComment struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version *struct {
		CreatedAt string `json:"createdAt"`
		AuthorID  string `json:"authorId"`
	} `json:"version"`
	Body *struct {
		AtlasDocFormat *struct {
			Value string `json:"value"`
		} `json:"atlas_doc_format"`
	} `json:"body"`
	// Inline-only fields.
	ResolutionStatus string `json:"resolutionStatus,omitempty"`
	Properties       *struct {
		InlineMarkerRef         string `json:"inlineMarkerRef"`
		InlineOriginalSelection string `json:"inlineOriginalSelection"`
	} `json:"properties,omitempty"`
}

// ConfluenceCommentsResult is the paginated response for listing comments.
type ConfluenceCommentsResult struct {
	Results []ConfluenceComment `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// RemoteLinkResponse is the response from the remote links endpoint.
type RemoteLinkResponse struct {
	ID     int `json:"id"`
	Object struct {
		URL   string `json:"url"`
		Title string `json:"title"`
		Icon  *struct {
			URL16x16 string `json:"url16x16"`
			Title    string `json:"title"`
		} `json:"icon"`
	} `json:"object"`
}
