package demo

import (
	"strings"
	"time"

	"github.com/undont/jiru/internal/confluence"
)

// spaceFixture pairs a Confluence space with its top-level pages so we can
// answer SpacePages without rebuilding the slice on every call.
type spaceFixture struct {
	space confluence.Space
	pages []string // Page IDs, in display order.
}

// pageFixture is the canonical page record. The on-the-wire Page returned
// from ConfluencePage is built lazily so the caller can mutate without
// touching the fixture.
type pageFixture struct {
	page     confluence.Page
	comments []confluence.Comment
}

func (s *state) seedConfluence() {
	engineering := confluence.Space{
		ID:          "spc-engineering",
		Key:         "ENG",
		Name:        "Engineering",
		Type:        "global",
		Description: "Platform, frontend and infra wiki.",
	}
	personal := confluence.Space{
		ID:          "spc-ada",
		Key:         "~ada",
		Name:        "Ada Lovelace",
		Type:        "personal",
		Description: "Drafts, scratch notes, weekly review.",
	}

	now := s.now

	// --- Featured page (runbook) — cross-references ACME-101 via plain text ---
	runbookADF := buildADF([]any{
		heading(2, "Cold-start rollout runbook"),
		paragraph(text("This runbook covers the rollout sequence for the cold-start work captured in "), code("ACME-101"), text(". Follow each step in order and keep the on-call channel informed at the gate points.")),
		panel("info", "When in doubt, page the platform on-call before flipping any flag."),
		heading(3, "1. Pre-flight"),
		bullets(
			"Confirm the staging dashboard P75 is under 1.6s for the last 30 minutes.",
			"Verify the dashboard.lazy-metrics flag exists in LaunchDarkly with default = false.",
			"Snapshot the current Grafana board — link in the team wiki.",
		),
		heading(3, "2. Canary"),
		bullets(
			"Set dashboard.lazy-metrics = true for the platform team segment (≈ 40 users).",
			"Watch the cold-start panel for 20 minutes. Roll back at any sign of regression.",
			"Capture before / after numbers in ACME-101.",
		),
		heading(3, "3. Wider rollout"),
		bullets(
			"Ramp to 25% of all sessions. Watch P75 and TTI panels.",
			"Ramp to 100% if P75 stays under 1.2s for 30 minutes.",
		),
		heading(3, "Rollback"),
		paragraph(text("Flip the flag to false in LaunchDarkly. The change is purely client-side, no deploy required. Follow up with a postmortem comment on ACME-101.")),
		heading(3, "Out of scope — follow-ups"),
		paragraph(text("The wider migration to the new metrics service is tracked separately in ACME-104. Do not pair the rollout with that work; we want a clean signal on the deferred-fetch numbers first.")),
		heading(3, "Related"),
		paragraph(text("Sister doc: "), link("Dashboard architecture overview", ConfluencePageURL("100201", "Dashboard+Architecture"))),
	})

	architectureADF := buildADF([]any{
		heading(2, "Dashboard architecture overview"),
		paragraph(text("This page captures the data flow for the Acme dashboard as of Sprint 17. For the active rollout sequence, see the "), link("Acme Onboarding Runbook", ConfluencePageURL("100200", "Acme+Onboarding+Runbook")), text(".")),
		heading(3, "Components"),
		bullets(
			"Edge cache — long-lived, serves the shell HTML.",
			"Metrics service — paged JSON, fronted by Cloudflare.",
			"Dashboard client — React 18 with Suspense boundaries per panel.",
		),
		heading(3, "Open issues"),
		paragraph(text("ACME-101 introduces deferred metrics fetching. ACME-104 covers the longer-term migration to the new metrics service.")),
	})

	weeklyReviewADF := buildADF([]any{
		heading(2, "Week 21 — review"),
		bullets(
			"Shipped the feature flag wiring (ACME-103).",
			"Drafted the cold-start runbook.",
			"Pairing with Bilal tomorrow on the skeleton state PR.",
		),
		heading(3, "Looking ahead"),
		paragraph(text("Push to land ACME-101 by Friday. The metrics-service migration in ACME-104 stays in the backlog until next sprint.")),
	})

	platformAnnouncementsADF := buildADF([]any{
		heading(2, "Platform announcements"),
		paragraph(text("Helm chart v1.18.0 lands this week. Track the staging rollout in ACME-120.")),
		paragraph(text("Quarterly key rotation: ACME-121 is in review, owner is Felix.")),
	})

	s.pages = map[string]*pageFixture{
		"100200": {
			page: confluence.Page{
				ID:       "100200",
				Title:    "Acme Onboarding Runbook",
				SpaceID:  engineering.ID,
				SpaceKey: engineering.Key,
				Status:   "current",
				Version:  4,
				Author:   "acct-carmen",
				Created:  now.Add(-9 * 24 * time.Hour),
				Updated:  now.Add(-6 * time.Hour),
				BodyADF:  runbookADF,
			},
			comments: []confluence.Comment{
				{
					ID:      "cm-1",
					Author:  "acct-bilal",
					Created: now.Add(-3 * 24 * time.Hour),
					BodyADF: buildADF([]any{paragraph(text("Added the canary segment numbers. Carmen can you sanity-check before we ramp?"))}),
				},
			},
		},
		"100201": {
			page: confluence.Page{
				ID:       "100201",
				Title:    "Dashboard Architecture",
				SpaceID:  engineering.ID,
				SpaceKey: engineering.Key,
				Status:   "current",
				Version:  2,
				Author:   UserAccount,
				Created:  now.Add(-30 * 24 * time.Hour),
				Updated:  now.Add(-2 * 24 * time.Hour),
				BodyADF:  architectureADF,
			},
		},
		"100210": {
			page: confluence.Page{
				ID:       "100210",
				Title:    "Platform announcements",
				SpaceID:  engineering.ID,
				SpaceKey: engineering.Key,
				Status:   "current",
				Version:  18,
				Author:   "acct-devon",
				Created:  now.Add(-180 * 24 * time.Hour),
				Updated:  now.Add(-1 * 24 * time.Hour),
				BodyADF:  platformAnnouncementsADF,
			},
		},
		"100300": {
			page: confluence.Page{
				ID:       "100300",
				Title:    "Week 21 — review",
				SpaceID:  personal.ID,
				SpaceKey: personal.Key,
				Status:   "current",
				Version:  1,
				Author:   UserAccount,
				Created:  now.Add(-1 * 24 * time.Hour),
				Updated:  now.Add(-1 * 24 * time.Hour),
				BodyADF:  weeklyReviewADF,
			},
		},
	}

	s.spaces = []spaceFixture{
		{space: engineering, pages: []string{"100200", "100201", "100210"}},
		{space: personal, pages: []string{"100300"}},
	}
}

// spacesSnapshot returns a copy of the space list for the wiki list view.
func (s *state) spacesSnapshot() []confluence.Space {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]confluence.Space, 0, len(s.spaces))
	for _, sf := range s.spaces {
		out = append(out, sf.space)
	}
	return out
}

// spacePages returns the pages belonging to the given space, in fixture order.
func (s *state) spacePages(spaceID string) []confluence.Page {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sf := range s.spaces {
		if sf.space.ID != spaceID {
			continue
		}
		out := make([]confluence.Page, 0, len(sf.pages))
		for _, id := range sf.pages {
			if pf, ok := s.pages[id]; ok {
				out = append(out, pf.page)
			}
		}
		return out
	}
	return nil
}

// page returns the canonical page record for the given ID, or nil.
func (s *state) page(id string) *confluence.Page {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pf, ok := s.pages[id]
	if !ok {
		return nil
	}
	cp := pf.page
	return &cp
}

// pageAncestors returns the ancestor chain for the given page (empty for now —
// the demo's pages are flat).
func (s *state) pageAncestors(_ string) []confluence.PageAncestor { return nil }

// pageComments returns a copy of the comments slice for the given page.
func (s *state) pageComments(id string) []confluence.Comment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pf, ok := s.pages[id]
	if !ok {
		return nil
	}
	out := make([]confluence.Comment, len(pf.comments))
	copy(out, pf.comments)
	return out
}

// --- Tiny ADF builders ----------------------------------------------------
//
// The renderer accepts ADF as a JSON string, so we build the document with
// fmt-style concatenation. Each helper returns a JSON fragment; buildADF
// wraps them in a top-level doc node. Keeping the helpers small and local
// avoids dragging in a full ADF builder dependency for a handful of fixtures.

// buildADF wraps a slice of top-level block nodes in an ADF document envelope.
func buildADF(blocks []any) string {
	var sb strings.Builder
	sb.WriteString(`{"type":"doc","version":1,"content":[`)
	for i, b := range blocks {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(b.(string))
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func heading(level int, txt string) string {
	return adfNode("heading", map[string]any{"level": level}, []string{text(txt)})
}

func paragraph(children ...string) string {
	return adfNode("paragraph", nil, children)
}

func bullets(items ...string) string {
	listItems := make([]string, 0, len(items))
	for _, it := range items {
		para := adfNode("paragraph", nil, []string{text(it)})
		listItems = append(listItems, adfNode("listItem", nil, []string{para}))
	}
	return adfNode("bulletList", nil, listItems)
}

func panel(kind, msg string) string {
	body := adfNode("paragraph", nil, []string{text(msg)})
	return adfNode("panel", map[string]any{"panelType": kind}, []string{body})
}

// text returns a text node with no marks. Embedded quotes and backslashes
// are escaped — the rest of ADF is JSON, so this is the only escape path.
func text(s string) string {
	return `{"type":"text","text":"` + escapeJSON(s) + `"}`
}

// code returns a text node with the "code" inline mark.
func code(s string) string {
	return `{"type":"text","text":"` + escapeJSON(s) + `","marks":[{"type":"code"}]}`
}

// link returns a text node with a link mark pointing at href.
func link(label, href string) string {
	return `{"type":"text","text":"` + escapeJSON(label) + `","marks":[{"type":"link","attrs":{"href":"` + escapeJSON(href) + `"}}]}`
}

func adfNode(kind string, attrs map[string]any, children []string) string {
	var sb strings.Builder
	sb.WriteString(`{"type":"`)
	sb.WriteString(kind)
	sb.WriteString(`"`)
	if len(attrs) > 0 {
		sb.WriteString(`,"attrs":{`)
		first := true
		for k, v := range attrs {
			if !first {
				sb.WriteByte(',')
			}
			first = false
			sb.WriteString(`"`)
			sb.WriteString(k)
			sb.WriteString(`":`)
			switch vv := v.(type) {
			case string:
				sb.WriteByte('"')
				sb.WriteString(escapeJSON(vv))
				sb.WriteByte('"')
			case int:
				sb.WriteString(itoa(vv))
			default:
				sb.WriteString(`null`)
			}
		}
		sb.WriteByte('}')
	}
	if len(children) > 0 {
		sb.WriteString(`,"content":[`)
		for i, c := range children {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(c)
		}
		sb.WriteByte(']')
	}
	sb.WriteByte('}')
	return sb.String()
}

// escapeJSON handles the subset of escapes the fixtures need — quotes,
// backslashes, and newlines. Tabs and control characters are not produced
// by the fixture authors so we keep the helper minimal.
func escapeJSON(s string) string {
	if !strings.ContainsAny(s, `"\`+"\n") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 4)
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
