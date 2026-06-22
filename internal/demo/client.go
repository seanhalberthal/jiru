package demo

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/undont/jiru/internal/client"
	"github.com/undont/jiru/internal/config"
	"github.com/undont/jiru/internal/confluence"
	"github.com/undont/jiru/internal/jira"
)

// Client is an in-memory implementation of client.JiraClient. It returns
// curated fixtures and applies mutating operations (transitions, comments,
// links, edits, assignments) to the underlying state so the demo feels live.
type Client struct {
	cfg   *config.Config
	state *state
}

// New constructs a demo client with the shared synthetic config.
func New() *Client {
	return &Client{cfg: Config(), state: newState()}
}

// bootLatency is the artificial delay injected into the first auth + sprint
// fetch so the loading screen and ASCII logo are on-screen long enough to
// see during the demo recording. The three boot-path methods each sleep
// once for this amount.
const bootLatency = 700 * time.Millisecond

// Compile-time assertion that *Client implements the JiraClient interface.
var _ client.JiraClient = (*Client)(nil)

// --- Authentication / identity ------------------------------------------------

func (c *Client) Me() (string, error) {
	time.Sleep(bootLatency)
	return UserName, nil
}
func (c *Client) Config() *config.Config     { return c.cfg }
func (c *Client) IssueURL(key string) string { return "https://" + Domain + "/browse/" + key }

// --- Lookups ----------------------------------------------------------------

func (c *Client) GetIssue(key string) (*jira.Issue, error) {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	iss := c.state.findIssue(key)
	if iss == nil {
		return nil, fmt.Errorf("issue %s not found", key)
	}
	clone := *iss
	clone.Comments = append([]jira.Comment(nil), iss.Comments...)
	return &clone, nil
}

func (c *Client) Boards(_ string) ([]jira.Board, error) {
	return []jira.Board{
		{ID: BoardID, Name: "Acme Sprint Board", Type: "scrum"},
		{ID: BoardID + 1, Name: "Acme Bugs Triage", Type: "kanban"},
	}, nil
}

func (c *Client) BoardSprints(_ int, state string) ([]jira.Sprint, error) {
	time.Sleep(bootLatency)
	if state != "" && state != "active" {
		return nil, nil
	}
	return []jira.Sprint{
		{ID: SprintID, Name: "Sprint 17 — Q2 Launch", State: "active", Goal: "Land the dashboard cold-start work and clear the security bug backlog."},
	}, nil
}

func (c *Client) Projects() ([]jira.Project, error) {
	return []jira.Project{
		{Key: ProjectKey, Name: ProjectName, Type: "software"},
	}, nil
}

func (c *Client) IssueTypesWithID(_ string) ([]jira.IssueTypeInfo, error) {
	return []jira.IssueTypeInfo{
		{ID: "10001", Name: "Story"},
		{ID: "10002", Name: "Bug"},
		{ID: "10003", Name: "Task"},
		{ID: "10004", Name: "Epic"},
		{ID: "10005", Name: "Sub-task"},
	}, nil
}

func (c *Client) GetIssueLinkTypes() ([]jira.IssueLinkType, error) {
	out := make([]jira.IssueLinkType, len(c.state.linkTypes))
	copy(out, c.state.linkTypes)
	return out, nil
}

func (c *Client) Transitions(_ string) ([]jira.Transition, error) {
	out := make([]jira.Transition, len(c.state.trans))
	copy(out, c.state.trans)
	return out, nil
}

func (c *Client) ChildIssues(key, _ string) ([]jira.ChildIssue, error) {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	var children []jira.ChildIssue
	for _, iss := range c.state.issues {
		if iss.ParentKey != key {
			continue
		}
		children = append(children, jira.ChildIssue{
			Key:             iss.Key,
			Summary:         iss.Summary,
			Status:          iss.Status,
			IssueType:       iss.IssueType,
			Assignee:        iss.Assignee,
			AssigneeAcronym: iss.AssigneeAcronym,
			Unassigned:      iss.Assignee == "",
			StoryPoints:     iss.StoryPoints,
			FixVersions:     iss.FixVersions,
		})
	}
	return children, nil
}

func (c *Client) RemoteLinks(key string) ([]jira.RemoteLink, error) {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	links, ok := c.state.links[key]
	if !ok {
		return nil, nil
	}
	out := make([]jira.RemoteLink, len(links))
	copy(out, links)
	return out, nil
}

func (c *Client) GetUserDisplayName(accountID string) string {
	if accountID == "" {
		return ""
	}
	for _, u := range c.state.users {
		if u.AccountID == accountID {
			return u.DisplayName
		}
	}
	return accountID
}

// --- Search & listing -------------------------------------------------------

func (c *Client) BoardIssues(_ string, statuses ...string) ([]jira.Issue, error) {
	issues := c.state.snapshotIssues()
	if len(statuses) == 0 {
		return issues, nil
	}
	allow := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		allow[strings.ToLower(s)] = true
	}
	out := issues[:0]
	for _, iss := range issues {
		if allow[strings.ToLower(iss.Status)] {
			out = append(out, iss)
		}
	}
	return out, nil
}

func (c *Client) BoardIssuesPage(_ int, from, pageSize int) (*client.PageResult, error) {
	return pageIssues(c.state.snapshotIssues(), from, pageSize), nil
}

func (c *Client) EpicIssues(epicKey string) ([]jira.Issue, error) {
	issues := c.state.snapshotIssues()
	out := issues[:0]
	for _, iss := range issues {
		if iss.ParentKey == epicKey {
			out = append(out, iss)
		}
	}
	return out, nil
}

func (c *Client) EpicIssuesPage(epicKey string, from, pageSize int) (*client.PageResult, error) {
	all, _ := c.EpicIssues(epicKey)
	return pageIssues(all, from, pageSize), nil
}

func (c *Client) SprintIssues(_ int) ([]jira.Issue, error) {
	return c.state.snapshotIssues(), nil
}

func (c *Client) SprintIssuesPage(_, from, pageSize int) (*client.PageResult, error) {
	return pageIssues(c.state.snapshotIssues(), from, pageSize), nil
}

func (c *Client) SprintIssueStats(_ int, categoryFn func(string) int) (open, inProgress, done, total int, err error) {
	issues := c.state.snapshotIssues()
	for _, iss := range issues {
		total++
		switch categoryFn(iss.Status) {
		case 1:
			inProgress++
		case 2, 3:
			done++
		default:
			open++
		}
	}
	return
}

func (c *Client) SearchJQL(jqlQuery string, limit uint) ([]jira.Issue, error) {
	page, _ := c.SearchJQLPage(jqlQuery, int(limit), 0, "")
	return page.Issues, nil
}

// SearchJQLPage applies a permissive interpretation of the JQL string,
// filtering the fixture set against the recognised clauses. The first call
// (the boot-time sprint query) gets an extra delay so the loading screen
// + ASCII logo stay on-screen long enough to register on the recording.
func (c *Client) SearchJQLPage(jqlQuery string, pageSize int, from int, _ string) (*client.PageResult, error) {
	if from == 0 {
		c.state.firstSprint.Do(func() { time.Sleep(bootLatency) })
	}
	all := c.state.snapshotIssues()
	filtered := filterByJQL(all, jqlQuery)
	return pageIssues(filtered, from, pageSize), nil
}

func (c *Client) BoardFilterJQL(_ int) (string, error) {
	return fmt.Sprintf("project = %s ORDER BY updated DESC", ProjectKey), nil
}

func (c *Client) JQLMetadata() (*jira.JQLMetadata, error) { return c.state.jqlMetadata(), nil }

func (c *Client) ResolveParents(issues []jira.Issue) map[string]client.ParentInfo {
	return c.state.resolveParents(issues)
}

func (c *Client) SearchUsers(_, prefix string) ([]jira.UserInfo, error) {
	if prefix == "" {
		out := make([]jira.UserInfo, len(c.state.users))
		copy(out, c.state.users)
		return out, nil
	}
	lower := strings.ToLower(prefix)
	var matches []jira.UserInfo
	for _, u := range c.state.users {
		if strings.Contains(strings.ToLower(u.DisplayName), lower) {
			matches = append(matches, u)
		}
	}
	return matches, nil
}

func (c *Client) CreateMetaFields(_, _ string) ([]jira.CustomFieldDef, error) {
	// Story-points field with a sensible name so create-issue surfaces it.
	return []jira.CustomFieldDef{
		{ID: "customfield_10016", Name: "Story Points", FieldType: "number", Required: false},
	}, nil
}

// --- Mutations --------------------------------------------------------------

func (c *Client) CreateIssue(req *client.CreateIssueRequest) (*client.CreateIssueResponse, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	key := fmt.Sprintf("%s-%d", ProjectKey, 200+len(c.state.issues))
	iss := jira.Issue{
		Key:         key,
		Summary:     req.Summary,
		Description: req.Description,
		Status:      "To Do",
		Priority:    firstNonEmpty(req.Priority, "Medium"),
		Assignee:    req.Assignee,
		Reporter:    UserName,
		Labels:      append([]string(nil), req.Labels...),
		IssueType:   firstNonEmpty(req.IssueType, "Task"),
		ParentKey:   req.ParentKey,
		Created:     time.Now(),
		Updated:     time.Now(),
	}
	c.state.issues = append(c.state.issues, iss)
	c.state.byKey[key] = len(c.state.issues) - 1
	return &client.CreateIssueResponse{Key: key}, nil
}

func (c *Client) TransitionIssue(key, transitionID string) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	iss := c.state.findIssue(key)
	if iss == nil {
		return fmt.Errorf("issue %s not found", key)
	}
	for _, t := range c.state.trans {
		if t.ID == transitionID {
			iss.Status = t.ToStatus
			iss.Updated = time.Now()
			return nil
		}
	}
	return fmt.Errorf("transition %s not found", transitionID)
}

func (c *Client) AddComment(key, body string) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	iss := c.state.findIssue(key)
	if iss == nil {
		return fmt.Errorf("issue %s not found", key)
	}
	iss.Comments = append(iss.Comments, jira.Comment{
		Author:  UserName,
		Created: time.Now(),
		Body:    body,
	})
	iss.Updated = time.Now()
	return nil
}

func (c *Client) WatchIssue(key string) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if iss := c.state.findIssue(key); iss != nil {
		iss.IsWatching = true
	}
	return nil
}

func (c *Client) UnwatchIssue(key string) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if iss := c.state.findIssue(key); iss != nil {
		iss.IsWatching = false
	}
	return nil
}

func (c *Client) AssignIssue(key, accountID string) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	iss := c.state.findIssue(key)
	if iss == nil {
		return fmt.Errorf("issue %s not found", key)
	}
	for _, u := range c.state.users {
		if u.AccountID == accountID {
			iss.Assignee = u.DisplayName
			iss.AssigneeAcronym = acronym(u.DisplayName)
			iss.Updated = time.Now()
			return nil
		}
	}
	// Unassign on unknown account ID, matching the real Jira behaviour for "".
	iss.Assignee = ""
	iss.AssigneeAcronym = ""
	return nil
}

func (c *Client) EditIssue(key string, req *client.EditIssueRequest) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	iss := c.state.findIssue(key)
	if iss == nil {
		return fmt.Errorf("issue %s not found", key)
	}
	if req.Summary != "" {
		iss.Summary = req.Summary
	}
	if req.Description != "" {
		iss.Description = req.Description
	}
	if req.Priority != "" {
		iss.Priority = req.Priority
	}
	if req.IssueType != "" {
		iss.IssueType = req.IssueType
	}
	if req.Labels != nil {
		// Apply +label / -label edits in place.
		set := make(map[string]bool, len(iss.Labels))
		for _, l := range iss.Labels {
			set[l] = true
		}
		for _, l := range req.Labels {
			if after, ok := strings.CutPrefix(l, "-"); ok {
				delete(set, after)
				continue
			}
			set[l] = true
		}
		iss.Labels = iss.Labels[:0]
		for l := range set {
			iss.Labels = append(iss.Labels, l)
		}
	}
	if req.FixVersions != nil {
		// Apply +version / -version edits in place.
		set := make(map[string]bool, len(iss.FixVersions))
		for _, v := range iss.FixVersions {
			set[v] = true
		}
		for _, v := range req.FixVersions {
			if after, ok := strings.CutPrefix(v, "-"); ok {
				delete(set, after)
				continue
			}
			set[v] = true
		}
		iss.FixVersions = iss.FixVersions[:0]
		for v := range set {
			iss.FixVersions = append(iss.FixVersions, v)
		}
	}
	if req.StoryPoints != nil {
		if *req.StoryPoints == nil {
			iss.StoryPoints = nil
		} else {
			val := **req.StoryPoints
			iss.StoryPoints = &val
		}
	}
	if req.Parent != nil {
		if *req.Parent == "" {
			iss.ParentKey = ""
			iss.ParentSummary = ""
			iss.ParentType = ""
		} else {
			iss.ParentKey = *req.Parent
			iss.ParentSummary = ""
			iss.ParentType = ""
			if parent := c.state.findIssue(*req.Parent); parent != nil {
				iss.ParentSummary = parent.Summary
				iss.ParentType = parent.IssueType
			}
		}
	}
	iss.Updated = time.Now()
	return nil
}

func (c *Client) LinkIssue(inwardKey, outwardKey, linkType string) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	// outwardKey is the source ("blocks"), inwardKey is the target ("is blocked by").
	source := c.state.findIssue(outwardKey)
	if source == nil {
		return fmt.Errorf("issue %s not found", outwardKey)
	}
	target := c.state.findIssue(inwardKey)
	if target == nil {
		return fmt.Errorf("issue %s not found", inwardKey)
	}

	// Resolve the directional labels for the link type so the relationship
	// reads naturally from each side.
	outward, inward := linkType, linkType
	for _, lt := range c.state.linkTypes {
		if lt.Name == linkType {
			outward, inward = lt.Outward, lt.Inward
			break
		}
	}

	// Synthesise a shared link ID so deleting from either side removes both
	// reciprocal entries, mirroring real Jira behaviour.
	c.state.linkSeq++
	linkID := fmt.Sprintf("%d", c.state.linkSeq)

	source.LinkedIssues = append(source.LinkedIssues, jira.LinkedIssue{
		LinkID:    linkID,
		Relation:  outward,
		Key:       target.Key,
		Summary:   target.Summary,
		Status:    target.Status,
		IssueType: target.IssueType,
	})
	target.LinkedIssues = append(target.LinkedIssues, jira.LinkedIssue{
		LinkID:    linkID,
		Relation:  inward,
		Key:       source.Key,
		Summary:   source.Summary,
		Status:    source.Status,
		IssueType: source.IssueType,
	})

	source.Comments = append(source.Comments, jira.Comment{
		Author:  UserName,
		Created: time.Now(),
		Body:    fmt.Sprintf("Linked: %s %s.", linkType, inwardKey),
	})
	now := time.Now()
	source.Updated = now
	target.Updated = now
	return nil
}

func (c *Client) DeleteIssueLink(linkID string) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	// Remove every LinkedIssue carrying this ID — a link surfaces on both the
	// source and target issues, so deleting it clears both reciprocal entries.
	found := false
	now := time.Now()
	for i := range c.state.issues {
		iss := &c.state.issues[i]
		kept := iss.LinkedIssues[:0]
		for _, link := range iss.LinkedIssues {
			if link.LinkID == linkID {
				found = true
				continue
			}
			kept = append(kept, link)
		}
		if len(kept) != len(iss.LinkedIssues) {
			iss.LinkedIssues = kept
			iss.Updated = now
		}
	}
	if !found {
		return fmt.Errorf("issue link %s not found", linkID)
	}
	return nil
}

func (c *Client) DeleteIssue(key string, _ bool) error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	idx, ok := c.state.byKey[key]
	if !ok {
		return fmt.Errorf("issue %s not found", key)
	}
	c.state.issues = append(c.state.issues[:idx], c.state.issues[idx+1:]...)
	delete(c.state.byKey, key)
	// Reindex.
	for i, iss := range c.state.issues {
		c.state.byKey[iss.Key] = i
	}
	return nil
}

// --- Confluence -------------------------------------------------------------

func (c *Client) ConfluenceSpaces() ([]confluence.Space, error) {
	return c.state.spacesSnapshot(), nil
}

func (c *Client) ConfluencePage(pageID string) (*confluence.Page, error) {
	p := c.state.page(pageID)
	if p == nil {
		return nil, fmt.Errorf("page %s not found", pageID)
	}
	return p, nil
}

func (c *Client) ConfluencePageAncestors(pageID string) ([]confluence.PageAncestor, error) {
	return c.state.pageAncestors(pageID), nil
}

func (c *Client) ConfluenceSpacePages(spaceID string, _ int) ([]confluence.Page, error) {
	return c.state.spacePages(spaceID), nil
}

func (c *Client) ConfluenceSearchCQL(_ string, _ int) ([]confluence.PageSearchResult, error) {
	return nil, nil
}

func (c *Client) ConfluencePageComments(pageID string) ([]confluence.Comment, error) {
	return c.state.pageComments(pageID), nil
}

func (c *Client) ConfluencePageURL(pageID string) string {
	return ConfluencePageURL(pageID, "Page")
}

func (c *Client) UpdateConfluencePage(pageID, title, bodyADF string, version int) (*confluence.Page, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	pf, ok := c.state.pages[pageID]
	if !ok {
		return nil, fmt.Errorf("page %s not found", pageID)
	}
	pf.page.Title = title
	pf.page.BodyADF = bodyADF
	pf.page.Version = version + 1
	pf.page.Updated = time.Now()
	out := pf.page
	return &out, nil
}

// --- Helpers ----------------------------------------------------------------

// pageIssues slices a fixture list to mimic the cursor-based search API,
// reporting HasMore when more results exist past the page boundary.
func pageIssues(all []jira.Issue, from, pageSize int) *client.PageResult {
	if from < 0 {
		from = 0
	}
	if from >= len(all) {
		return &client.PageResult{Issues: nil, HasMore: false, From: from, Total: len(all)}
	}
	end := min(from+pageSize, len(all))
	page := all[from:end]
	hasMore := end < len(all)
	token := ""
	if hasMore {
		token = fmt.Sprintf("demo-cursor-%d", end)
	}
	return &client.PageResult{
		Issues:    append([]jira.Issue(nil), page...),
		HasMore:   hasMore,
		From:      from,
		Total:     len(all),
		NextToken: token,
	}
}

// filterByJQL applies a loose interpretation of common JQL clauses to the
// issue list. Recognised clauses filter the result set; unrecognised ones
// pass through so the demo's search bar always returns *something*
// relevant. The grammar handled is just enough to support the queries the
// app itself produces on boot (sprint = N, project = X) plus the clauses
// users are likely to type in the search bar.
func filterByJQL(issues []jira.Issue, jqlQuery string) []jira.Issue {
	q := strings.TrimSpace(jqlQuery)
	if q == "" {
		return issues
	}

	// Strip ORDER BY ... — we don't apply ordering, the fixture order is fine
	// for demo purposes.
	if idx := strings.Index(strings.ToUpper(q), " ORDER BY "); idx >= 0 {
		q = q[:idx]
	}

	clauses := splitClauses(q)
	out := make([]jira.Issue, 0, len(issues))
	for _, iss := range issues {
		if clausesMatch(iss, clauses) {
			out = append(out, iss)
		}
	}
	return out
}

// splitClauses breaks a JQL query into top-level AND-joined clauses. OR is
// not supported — the demo's queries are conjunctive — but the alternative
// of returning unfiltered results is acceptable when an OR appears.
func splitClauses(q string) []string {
	upper := strings.ToUpper(q)
	if strings.Contains(upper, " OR ") {
		// Unsupported — return as a single clause so it falls into the
		// permissive free-text branch.
		return []string{q}
	}
	parts := splitOnAnd(q)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitOnAnd splits on the word "AND" while respecting double-quoted strings.
func splitOnAnd(q string) []string {
	var (
		parts   []string
		current strings.Builder
		inStr   bool
		i       = 0
	)
	for i < len(q) {
		ch := q[i]
		if ch == '"' {
			inStr = !inStr
			current.WriteByte(ch)
			i++
			continue
		}
		if !inStr && i+4 <= len(q) {
			// Check for " AND " with case-insensitive match and word boundaries.
			if i > 0 && q[i-1] == ' ' && strings.EqualFold(q[i:i+3], "AND") && (i+3 == len(q) || q[i+3] == ' ') {
				parts = append(parts, current.String())
				current.Reset()
				i += 3
				continue
			}
		}
		current.WriteByte(ch)
		i++
	}
	parts = append(parts, current.String())
	return parts
}

// clausesMatch checks every recognised clause; unrecognised clauses are
// treated as match-all so the demo stays responsive on novel queries.
func clausesMatch(iss jira.Issue, clauses []string) bool {
	for _, c := range clauses {
		if !clauseMatches(iss, c) {
			return false
		}
	}
	return true
}

// clauseMatches evaluates a single clause against an issue. The grammar is
// intentionally small: <field> <op> <value> where op is "=" or "in (...)".
func clauseMatches(iss jira.Issue, clause string) bool {
	c := strings.TrimSpace(clause)
	lower := strings.ToLower(c)

	// Bare key — pass through if it looks like an unrewritten issue reference.
	if upper := strings.ToUpper(c); strings.HasPrefix(upper, ProjectKey+"-") && !strings.Contains(c, "=") {
		return iss.Key == upper
	}

	field, op, value := parseClause(c)
	switch field {
	case "sprint", "project":
		// All demo issues belong to the demo sprint and project, so any
		// equality clause is satisfied.
		_ = value
		return true
	case "key":
		return iss.Key == strings.ToUpper(value)
	case "parent":
		return iss.ParentKey == strings.ToUpper(value)
	case "status":
		return matchValue(iss.Status, op, value)
	case "issuetype", "type":
		return matchValue(iss.IssueType, op, value)
	case "priority":
		return matchValue(iss.Priority, op, value)
	case "assignee":
		if strings.EqualFold(value, "currentUser()") || strings.EqualFold(value, "currentUser") {
			return strings.EqualFold(iss.Assignee, UserName)
		}
		return matchValue(iss.Assignee, op, value)
	case "reporter":
		return matchValue(iss.Reporter, op, value)
	case "labels":
		return matchLabel(iss.Labels, op, value)
	case "text", "summary", "description":
		hay := strings.ToLower(iss.Summary + " " + iss.Description)
		return strings.Contains(hay, strings.ToLower(strings.Trim(value, `"' `)))
	}

	// Unrecognised clause — let it pass so the demo never returns empty.
	_ = lower
	return true
}

// parseClause splits a clause into (field, op, value). op is "=", "!=", or "in".
func parseClause(c string) (field, op, value string) {
	// "in (...)" form first.
	if idx := strings.Index(strings.ToLower(c), " in "); idx >= 0 {
		field = strings.ToLower(strings.TrimSpace(c[:idx]))
		op = "in"
		value = strings.TrimSpace(c[idx+4:])
		value = strings.TrimPrefix(value, "(")
		value = strings.TrimSuffix(value, ")")
		return
	}
	for _, sep := range []string{"!=", "="} {
		if before, after, ok := strings.Cut(c, sep); ok {
			field = strings.ToLower(strings.TrimSpace(before))
			op = sep
			value = strings.TrimSpace(after)
			return
		}
	}
	return "", "", ""
}

// matchValue applies the op (=, !=, in) to a single-value field.
func matchValue(actual, op, value string) bool {
	actual = strings.ToLower(strings.TrimSpace(actual))
	value = strings.ToLower(strings.Trim(value, `"' `))
	switch op {
	case "=":
		return actual == value
	case "!=":
		return actual != value
	case "in":
		return slices.Contains(splitInList(value), actual)
	}
	return true
}

// matchLabel handles `labels = "x"` and `labels in ("x", "y")`.
func matchLabel(labels []string, op, value string) bool {
	if op == "in" {
		want := splitInList(value)
		for _, l := range labels {
			for _, w := range want {
				if strings.EqualFold(l, w) {
					return true
				}
			}
		}
		return false
	}
	target := strings.Trim(value, `"' `)
	for _, l := range labels {
		if strings.EqualFold(l, target) {
			return op != "!="
		}
	}
	return op == "!="
}

// splitInList parses the inside of an `in (...)` value list into lowercase
// trimmed entries.
func splitInList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.Trim(p, ` "'`))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func acronym(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return ""
	}
	out := make([]byte, 0, 2)
	for i, p := range parts {
		if i >= 2 {
			break
		}
		out = append(out, p[0])
	}
	return strings.ToUpper(string(out))
}

// --- Filters / recents seeding ----------------------------------------------

// SeedSavedFilters returns the demo's curated filter list. main.go writes these
// to disk under the __demo profile before the App is constructed so the
// filterpickview overlay has content on first launch.
func SeedSavedFilters() []jira.SavedFilter {
	t := time.Now()
	return []jira.SavedFilter{
		{
			Name:      "My open work",
			JQL:       `assignee = currentUser() AND status != "Done" ORDER BY updated DESC`,
			Favourite: true,
			CreatedAt: t.Add(-30 * 24 * time.Hour),
			UpdatedAt: t.Add(-1 * 24 * time.Hour),
		},
		{
			Name:      "Recent bugs",
			JQL:       `project = ACME AND issuetype = Bug AND status != "Done" ORDER BY priority DESC, updated DESC`,
			Favourite: true,
			CreatedAt: t.Add(-20 * 24 * time.Hour),
			UpdatedAt: t.Add(-2 * 24 * time.Hour),
		},
		{
			Name:      "Awaiting review",
			JQL:       `status = "In Review" ORDER BY updated DESC`,
			Favourite: false,
			CreatedAt: t.Add(-10 * 24 * time.Hour),
			UpdatedAt: t.Add(-3 * 24 * time.Hour),
		},
	}
}
