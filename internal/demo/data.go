// Package demo provides a self-contained, in-memory implementation of
// client.JiraClient backed by curated fixtures. It powers the --demo flag,
// allowing the app to be launched without any real Jira credentials so
// screenshots, gifs and UI development don't depend on a live Jira tenant.
package demo

import (
	"fmt"
	"sync"
	"time"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

// Demo identity. These are referenced from Config and synthesised URLs so
// every link rendered in the UI looks like a real Atlassian tenant.
const (
	Domain      = "acme.atlassian.net"
	ProjectKey  = "ACME"
	ProjectName = "Acme Platform"
	BoardID     = 42
	SprintID    = 1701
	UserName    = "Ada Lovelace"
	UserEmail   = "ada@acme.dev"
	UserAccount = "acct-ada"
	// ProfileName is the profile slug used so the demo's saved filters and
	// recents don't collide with the user's real ~/.config/jiru data.
	ProfileName = "__demo"
)

// ConfluencePageURL returns a URL pointing at a demo Confluence page in the
// shape the cross-ref extractor expects.
func ConfluencePageURL(pageID, slug string) string {
	return fmt.Sprintf("https://%s/wiki/spaces/ENG/pages/%s/%s", Domain, pageID, slug)
}

// Config returns a fully populated config so the UI bypasses the setup wizard
// and the JQL/board fetches all see the demo project.
func Config() *config.Config {
	return &config.Config{
		Domain:          Domain,
		User:            UserEmail,
		APIToken:        "demo-token",
		AuthType:        config.AuthBasic,
		BoardID:         BoardID,
		Project:         ProjectKey,
		BranchMode:      "local",
		BranchCopyName:  true,
		BranchUppercase: true,
	}
}

// state holds the mutable demo data — issues, comments, links, etc. — so the
// fake client can reflect transitions and edits made during the recording.
// Guarded by a mutex because tea.Cmd goroutines run concurrently with Update.
type state struct {
	mu          sync.RWMutex
	now         time.Time
	issues      []jira.Issue   // Sprint board ordering preserved.
	byKey       map[string]int // Issue key → index into issues.
	links       map[string][]jira.RemoteLink
	users       []jira.UserInfo
	spaces      []spaceFixture
	pages       map[string]*pageFixture
	linkTypes   []jira.IssueLinkType
	trans       []jira.Transition
	linkSeq     int       // Monotonic counter for synthesised issue-link IDs.
	firstSprint sync.Once // Tracks whether the boot sprint search has run.
}

func newState() *state {
	s := &state{
		now:       time.Now(),
		links:     make(map[string][]jira.RemoteLink),
		pages:     make(map[string]*pageFixture),
		linkTypes: seedLinkTypes(),
		trans:     seedTransitions(),
		users:     seedUsers(),
		linkSeq:   20100, // Above the seeded link IDs (20001, 20002).
	}
	s.seedIssues()
	s.seedConfluence()
	s.seedRemoteLinks()
	return s
}

// minutesAgo / hoursAgo / daysAgo produce fixed offsets from s.now so the
// demo's timestamps look natural without depending on wall-clock drift between
// runs.
func (s *state) minutesAgo(n int) time.Time { return s.now.Add(-time.Duration(n) * time.Minute) }
func (s *state) hoursAgo(n int) time.Time   { return s.now.Add(-time.Duration(n) * time.Hour) }
func (s *state) daysAgo(n int) time.Time    { return s.now.Add(-time.Duration(n) * 24 * time.Hour) }

func seedLinkTypes() []jira.IssueLinkType {
	return []jira.IssueLinkType{
		{ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
		{ID: "10001", Name: "Relates", Inward: "relates to", Outward: "relates to"},
		{ID: "10002", Name: "Duplicates", Inward: "is duplicated by", Outward: "duplicates"},
		{ID: "10003", Name: "Causes", Inward: "is caused by", Outward: "causes"},
	}
}

func seedTransitions() []jira.Transition {
	return []jira.Transition{
		{ID: "11", Name: "To Do", ToStatus: "To Do"},
		{ID: "21", Name: "Start work", ToStatus: "In Progress"},
		{ID: "31", Name: "Ready for review", ToStatus: "In Review"},
		{ID: "41", Name: "Done", ToStatus: "Done"},
		{ID: "51", Name: "Cancelled", ToStatus: "Cancelled"},
	}
}

func seedUsers() []jira.UserInfo {
	return []jira.UserInfo{
		{AccountID: UserAccount, DisplayName: UserName},
		{AccountID: "acct-bilal", DisplayName: "Bilal Khan"},
		{AccountID: "acct-carmen", DisplayName: "Carmen Ortiz"},
		{AccountID: "acct-devon", DisplayName: "Devon Park"},
		{AccountID: "acct-emily", DisplayName: "Emily Tan"},
		{AccountID: "acct-felix", DisplayName: "Felix Schroeder"},
	}
}

// seedIssues populates the demo sprint with 16 hand-curated issues covering
// every status category, with one Epic that owns three children and one
// Story with rich wiki-markup comments + cross-refs to a Confluence page.
func (s *state) seedIssues() {
	sp := func(n float64) *float64 { return &n }

	// The "featured" story description showcases the wiki-markup renderer and
	// embeds a cross-reference to another Jira issue and a Confluence page so
	// the `i` picker has something to surface.
	featuredDescription := `h2. Goal

Reduce the *dashboard cold-start* time from ~2.8s to under 1s on the median
session. The current bottleneck is the synchronous metrics fetch on first
paint. See [Acme Onboarding Runbook|` + ConfluencePageURL("100200", "Acme+Onboarding+Runbook") + `]
for the rollout checklist.

h3. Scope

* Defer the metrics fetch until after first paint
* Add a skeleton state for the chart panel
* Wire a feature flag (` + "`dashboard.lazy-metrics`" + `) so we can roll back without a deploy

h3. Out of scope

* Migrating to the new metrics service — tracked in ACME-104.

{panel:title=Notes from kickoff}
Bilal owns the FE work, Carmen owns the flag plumbing.
We agreed the SLA target is the P75 cold-start, not P50.
{panel}

{code:typescript}
// Before — synchronous, blocks first paint.
const metrics = await fetchMetrics();
render(<Dashboard metrics={metrics} />);

// After — defer until after hydration.
render(<Dashboard metricsPromise={fetchMetrics()} />);
{code}
`

	issues := []jira.Issue{
		// --- Epic owning the cold-start initiative --------------------------------
		{
			Key:             ProjectKey + "-100",
			Summary:         "Dashboard cold-start under 1 second",
			Description:     "Umbrella epic for the Q2 cold-start initiative. Three child stories deliver the metrics defer, the skeleton state, and the feature-flag wiring.",
			Status:          "In Progress",
			Priority:        "High",
			Assignee:        UserName,
			AssigneeAcronym: "AL",
			Reporter:        "Carmen Ortiz",
			Labels:          []string{"perf", "frontend"},
			IssueType:       "Epic",
			StoryPoints:     sp(13),
			Created:         s.daysAgo(14),
			Updated:         s.hoursAgo(3),
		},
		// --- Featured Story (with comments + cross-refs) --------------------------
		{
			Key:             ProjectKey + "-101",
			Summary:         "Defer metrics fetch until after first paint",
			Description:     featuredDescription,
			Status:          "In Progress",
			Priority:        "High",
			Assignee:        "Bilal Khan",
			AssigneeAcronym: "BK",
			Reporter:        UserName,
			Labels:          []string{"perf", "frontend", "feature-flag"},
			IssueType:       "Story",
			ParentKey:       ProjectKey + "-100",
			ParentType:      "Epic",
			ParentSummary:   "Dashboard cold-start under 1 second",
			LinkedIssues: []jira.LinkedIssue{
				{LinkID: "20001", Relation: "is blocked by", Key: ProjectKey + "-104", Summary: "Move dashboard reads to metrics service", Status: "To Do", IssueType: "Task"},
				{LinkID: "20002", Relation: "blocks", Key: ProjectKey + "-102", Summary: "Add dashboard skeleton state", Status: "To Do", IssueType: "Story"},
			},
			StoryPoints: sp(5),
			Created:     s.daysAgo(6),
			Updated:     s.hoursAgo(2),
			Comments: []jira.Comment{
				{
					Author:  "Carmen Ortiz",
					Created: s.daysAgo(3),
					Body: `Pulled the latest profile from staging — the synchronous fetch is
*definitely* the long pole. Numbers:

|| Stage || Before || After ||
| First paint | 2,820ms | 1,140ms |
| TTI | 3,410ms | 1,290ms |

We should land this behind ` + "{{dashboard.lazy-metrics}}" + ` so we can roll back
cleanly. ACME-104 will handle the actual metrics service migration once
this ships.`,
				},
				{
					Author:  "Bilal Khan",
					Created: s.hoursAgo(20),
					Body: `Pushed a draft. Two open questions:

# Should the skeleton state animate, or just hold static? Carmen prefers static.
# Are we OK losing the ` + "{{lastUpdated}}" + ` timestamp during the skeleton window?

I left the rollout details in [Acme Onboarding Runbook|` + ConfluencePageURL("100200", "Acme+Onboarding+Runbook") + `]
so the on-call has them next to the dashboard playbook.`,
				},
				{
					Author:  UserName,
					Created: s.hoursAgo(2),
					Body: `+1 to static skeleton — animating during cold-start feels noisy.
Let's keep the timestamp slot reserved and just blank it; that way the
layout doesn't shift when the data lands.`,
				},
			},
		},
		// --- Two other Epic children ---------------------------------------------
		{
			Key:             ProjectKey + "-102",
			Summary:         "Skeleton state for dashboard chart panel",
			Description:     "Render a lightweight placeholder while the deferred metrics resolve. Reserve space for the legend and the lastUpdated badge so no layout shift occurs.",
			Status:          "In Review",
			Priority:        "Medium",
			Assignee:        "Bilal Khan",
			AssigneeAcronym: "BK",
			Reporter:        UserName,
			Labels:          []string{"frontend"},
			IssueType:       "Story",
			ParentKey:       ProjectKey + "-100",
			ParentType:      "Epic",
			ParentSummary:   "Dashboard cold-start under 1 second",
			StoryPoints:     sp(3),
			Created:         s.daysAgo(5),
			Updated:         s.hoursAgo(8),
		},
		{
			Key:             ProjectKey + "-103",
			Summary:         "Feature flag: dashboard.lazy-metrics",
			Description:     "Wire the flag in LaunchDarkly and gate the new metric-loading code-path so we can roll back without a deploy.",
			Status:          "Done",
			Priority:        "Medium",
			Assignee:        "Carmen Ortiz",
			AssigneeAcronym: "CO",
			Reporter:        UserName,
			Labels:          []string{"feature-flag", "infra"},
			IssueType:       "Task",
			ParentKey:       ProjectKey + "-100",
			ParentType:      "Epic",
			ParentSummary:   "Dashboard cold-start under 1 second",
			StoryPoints:     sp(2),
			Created:         s.daysAgo(7),
			Updated:         s.daysAgo(1),
		},
		// --- Tracked-but-out-of-scope link target --------------------------------
		{
			Key:             ProjectKey + "-104",
			Summary:         "Migrate dashboard to new metrics service",
			Description:     "Out of scope for the cold-start epic. Tracked separately so the rollout sequence stays explicit.",
			Status:          "To Do",
			Priority:        "Low",
			Assignee:        "Devon Park",
			AssigneeAcronym: "DP",
			Reporter:        "Carmen Ortiz",
			Labels:          []string{"metrics"},
			IssueType:       "Story",
			StoryPoints:     sp(8),
			Created:         s.daysAgo(2),
			Updated:         s.hoursAgo(28),
		},
		// --- Bugs ----------------------------------------------------------------
		{
			Key:             ProjectKey + "-110",
			Summary:         "Filter dropdown loses focus when scrolled",
			Description:     "Tabbing through the filter list inside a scrollable container drops focus back to the body. Reproduces on Firefox 128+ only.",
			Status:          "In Progress",
			Priority:        "Medium",
			Assignee:        "Emily Tan",
			AssigneeAcronym: "ET",
			Reporter:        "Felix Schroeder",
			Labels:          []string{"a11y", "bug", "firefox"},
			IssueType:       "Bug",
			StoryPoints:     sp(2),
			Created:         s.daysAgo(4),
			Updated:         s.hoursAgo(5),
		},
		{
			Key:             ProjectKey + "-111",
			Summary:         "Date range picker accepts invalid ISO strings",
			Description:     "Pasting a partial ISO date silently coerces to epoch zero. Validation only runs on the calendar widget, not on the textual input.",
			Status:          "To Do",
			Priority:        "High",
			Assignee:        "Bilal Khan",
			AssigneeAcronym: "BK",
			Reporter:        "Emily Tan",
			Labels:          []string{"bug", "validation"},
			IssueType:       "Bug",
			StoryPoints:     sp(1),
			Created:         s.daysAgo(1),
			Updated:         s.minutesAgo(45),
		},
		{
			Key:             ProjectKey + "-112",
			Summary:         "Stale CSRF token on session resume",
			Description:     "Suspending the laptop overnight and resuming triggers a 403 on the next mutation. The token isn't refreshed eagerly.",
			Status:          "Done",
			Priority:        "High",
			Assignee:        "Carmen Ortiz",
			AssigneeAcronym: "CO",
			Reporter:        UserName,
			Labels:          []string{"auth", "bug", "security"},
			IssueType:       "Bug",
			StoryPoints:     sp(3),
			Created:         s.daysAgo(9),
			Updated:         s.daysAgo(2),
		},
		// --- Tasks ---------------------------------------------------------------
		{
			Key:             ProjectKey + "-120",
			Summary:         "Promote staging Helm chart to v1.18.0",
			Description:     "Cut a release of the platform Helm chart and roll it through staging behind the canary gate.",
			Status:          "To Do",
			Priority:        "Low",
			Assignee:        "Devon Park",
			AssigneeAcronym: "DP",
			Reporter:        UserName,
			Labels:          []string{"infra", "release"},
			IssueType:       "Task",
			StoryPoints:     sp(1),
			Created:         s.daysAgo(1),
			Updated:         s.hoursAgo(6),
		},
		{
			Key:             ProjectKey + "-121",
			Summary:         "Rotate signing keys for the analytics pipeline",
			Description:     "Quarterly rotation. Pair with infra on the keychain handover and update the runbook timestamps.",
			Status:          "In Review",
			Priority:        "Medium",
			Assignee:        "Felix Schroeder",
			AssigneeAcronym: "FS",
			Reporter:        "Carmen Ortiz",
			Labels:          []string{"infra", "security"},
			IssueType:       "Task",
			StoryPoints:     sp(2),
			Created:         s.daysAgo(3),
			Updated:         s.hoursAgo(11),
		},
		// --- Sub-task ------------------------------------------------------------
		{
			Key:             ProjectKey + "-130",
			Summary:         "Add Playwright coverage for skeleton state",
			Description:     "Add a regression test that asserts the chart panel renders the skeleton when the metrics promise is still pending.",
			Status:          "To Do",
			Priority:        "Medium",
			Assignee:        "Emily Tan",
			AssigneeAcronym: "ET",
			Reporter:        "Bilal Khan",
			Labels:          []string{"frontend", "testing"},
			IssueType:       "Sub-task",
			ParentKey:       ProjectKey + "-102",
			ParentType:      "Story",
			ParentSummary:   "Skeleton state for dashboard chart panel",
			StoryPoints:     sp(1),
			Created:         s.daysAgo(2),
			Updated:         s.hoursAgo(4),
		},
		// --- Backlog / planning placeholders --------------------------------------
		{
			Key:             ProjectKey + "-140",
			Summary:         "Audit dependency tree for unused packages",
			Description:     "Run depcheck against the monorepo and remove anything that hasn't been imported in 12 months.",
			Status:          "To Do",
			Priority:        "Low",
			Assignee:        "Devon Park",
			AssigneeAcronym: "DP",
			Reporter:        UserName,
			Labels:          []string{"chore", "dependencies"},
			IssueType:       "Task",
			Created:         s.daysAgo(11),
			Updated:         s.daysAgo(6),
		},
		{
			Key:             ProjectKey + "-141",
			Summary:         "Spike: server-driven feature flags",
			Description:     "Investigate whether the platform team's flag service can replace the LaunchDarkly client SDK on the frontend.",
			Status:          "In Progress",
			Priority:        "Low",
			Assignee:        "Felix Schroeder",
			AssigneeAcronym: "FS",
			Reporter:        "Carmen Ortiz",
			Labels:          []string{"spike", "feature-flag"},
			IssueType:       "Task",
			StoryPoints:     sp(3),
			Created:         s.daysAgo(8),
			Updated:         s.hoursAgo(36),
		},
		{
			Key:             ProjectKey + "-150",
			Summary:         "Document the dashboard cold-start playbook",
			Description:     "Capture the rollout sequence, dashboards to watch, and rollback steps in the runbook. Owner is on-call this rotation.",
			Status:          "Done",
			Priority:        "Medium",
			Assignee:        UserName,
			AssigneeAcronym: "AL",
			Reporter:        "Carmen Ortiz",
			Labels:          []string{"docs"},
			IssueType:       "Task",
			StoryPoints:     sp(1),
			Created:         s.daysAgo(10),
			Updated:         s.daysAgo(1),
		},
		// --- Cancelled (to show the category) ------------------------------------
		{
			Key:             ProjectKey + "-160",
			Summary:         "Trial reading from edge cache for cold-start",
			Description:     "Superseded by the metrics-defer approach in ACME-101.",
			Status:          "Cancelled",
			Priority:        "Low",
			Assignee:        "Devon Park",
			AssigneeAcronym: "DP",
			Reporter:        UserName,
			Labels:          []string{"perf"},
			IssueType:       "Story",
			Created:         s.daysAgo(13),
			Updated:         s.daysAgo(4),
		},
	}

	s.issues = issues
	s.byKey = make(map[string]int, len(issues))
	for i, iss := range issues {
		s.byKey[iss.Key] = i
	}
}

// seedRemoteLinks attaches the runbook Confluence page as a remote link on the
// featured story so the `i` picker shows a Linked Pages group as well.
func (s *state) seedRemoteLinks() {
	s.links[ProjectKey+"-101"] = []jira.RemoteLink{
		{ID: 1, Title: "Acme Onboarding Runbook", URL: ConfluencePageURL("100200", "Acme+Onboarding+Runbook"), Icon: "confluence"},
	}
}

// snapshotIssues returns a shallow copy of the issues slice so callers can
// mutate without disturbing internal state. Comments are not deep-copied —
// they are appended in-place via AddComment, which always takes the lock.
func (s *state) snapshotIssues() []jira.Issue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]jira.Issue, len(s.issues))
	copy(out, s.issues)
	return out
}

// findIssue returns a pointer to the canonical issue by key, or nil. Callers
// must hold s.mu (read or write) for the duration they touch the result.
func (s *state) findIssue(key string) *jira.Issue {
	idx, ok := s.byKey[key]
	if !ok {
		return nil
	}
	return &s.issues[idx]
}

// resolveParents stubs the parent-resolution step so issue rows in the list
// view show their parent's type/summary inline.
func (s *state) resolveParents(issues []jira.Issue) map[string]client.ParentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]client.ParentInfo)
	for _, iss := range issues {
		if iss.ParentKey == "" {
			continue
		}
		if _, seen := out[iss.ParentKey]; seen {
			continue
		}
		if p := s.findIssue(iss.ParentKey); p != nil {
			out[iss.ParentKey] = client.ParentInfo{
				Key:       p.Key,
				Summary:   p.Summary,
				IssueType: p.IssueType,
			}
		}
	}
	return out
}

// jqlMetadata exposes a curated metadata payload so the JQL autocomplete
// dropdown has rich content to surface in the recording.
func (s *state) jqlMetadata() *jira.JQLMetadata {
	return &jira.JQLMetadata{
		Statuses: []string{"To Do", "In Progress", "In Review", "Done", "Cancelled"},
		StatusCategories: map[string]int{
			"To Do":       0,
			"In Progress": 1,
			"In Review":   1,
			"Done":        2,
			"Cancelled":   3,
		},
		IssueTypes:  []string{"Bug", "Epic", "Story", "Sub-task", "Task"},
		Priorities:  []string{"Highest", "High", "Medium", "Low", "Lowest"},
		Resolutions: []string{"Done", "Won't Do", "Duplicate"},
		Projects:    []string{ProjectKey},
		Labels:      []string{"a11y", "auth", "bug", "chore", "dependencies", "docs", "feature-flag", "firefox", "frontend", "infra", "metrics", "perf", "release", "security", "spike", "testing", "validation"},
		Components:  []string{"dashboard", "frontend-web", "metrics-service", "platform-infra"},
		Versions:    []string{"v1.17.0", "v1.18.0", "v1.19.0"},
		Sprints:     []string{"Sprint 17 — Q2 Launch"},
	}
}
