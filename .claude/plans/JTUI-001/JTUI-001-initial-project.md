# jiratui — Project Plan (v0.0.1)

**📍 CURRENT PROGRESS: All milestones complete**

## Overview

A terminal UI for Jira built in Go, using `ankitpokhrel/jira-cli`'s `pkg/jira` as the API client
and the Charm stack (Bubbletea + Bubbles + Lipgloss) for the TUI layer. Designed for read-heavy
daily use: browse boards, sprints, and issues without touching a browser.

---

## Goals for v0.0.1

A working, installable binary that lets you:

- Authenticate via existing jira-cli config or environment variables
- Browse the active sprint on a configured board
- Select an issue and read its details (description, status, assignee, priority, labels, comments)
- Navigate with vim-style keybindings
- Quit cleanly

No write operations in scope for v0.0.1. Read-only.

---

## Tech Stack

| Concern         | Choice                                      |
|-----------------|---------------------------------------------|
| Language        | Go                                          |
| API client      | `github.com/ankitpokhrel/jira-cli/pkg/jira` |
| TUI framework   | `github.com/charmbracelet/bubbletea`        |
| Components      | `github.com/charmbracelet/bubbles`          |
| Styling/layout  | `github.com/charmbracelet/lipgloss`         |
| Config          | Reuse jira-cli's `~/.config/.jira/.config.yml` + `JIRA_API_TOKEN` env var |
| Distribution    | `go install` (same as supplyscan-mcp)       |

---

## Project Structure

```
jiratui/
├── main.go
├── go.mod
├── go.sum
├── internal/
│   ├── client/
│   │   └── client.go        # wraps pkg/jira, exposes typed service methods
│   ├── config/
│   │   └── config.go        # reads jira-cli config + env vars
│   ├── ui/
│   │   ├── app.go           # root bubbletea model, top-level Update/View
│   │   ├── keys.go          # global keymap definitions
│   │   ├── styles.go        # lipgloss styles and colour palette
│   │   ├── sprintview/
│   │   │   ├── model.go     # sprint issue list model
│   │   │   └── delegate.go  # bubbles list item delegate
│   │   └── issueview/
│   │       └── model.go     # issue detail viewport model
│   └── jira/
│       └── types.go         # local domain types (avoids leaking pkg/jira structs into UI)
└── README.md
```

---

## Milestones

### Milestone 1 — Repo & Scaffold

- `go mod init github.com/seanhalberthal/jiratui`
- Add dependencies: bubbletea, bubbles, lipgloss, jira-cli
- Wire up a minimal bubbletea app that opens and closes cleanly
- Confirm `go install` works end-to-end

**Done when:** `jiratui` opens a blank terminal window and `q` quits.

---

### Milestone 2 — Config & Auth

- Implement `internal/config` to load from:
  1. `JIRA_DOMAIN`, `JIRA_API_TOKEN`, `JIRA_USER` env vars (priority)
  2. jira-cli's existing `~/.config/.jira/.config.yml` as fallback
- Implement `internal/client` wrapping `pkg/jira.NewClient`
- Add a startup auth check (`client.Me()`) with a clear error message if it fails
- Handle the `JIRA_AUTH_TYPE=bearer` case (PAT vs basic)

**Done when:** Binary starts, verifies credentials, and prints your display name or a descriptive auth error.

---

### Milestone 3 — Sprint Issue List

- Fetch the configured board's active sprint via `client.Sprints(boardID, "state=active", ...)`
- Fetch issues in that sprint via `client.SprintIssues(sprintID, jql, from, limit)`
- Render as a `bubbles/list` with a custom delegate showing:
  - Issue key (e.g. `DANA-123`)
  - Summary (truncated to available width)
  - Status badge (coloured with Lipgloss)
  - Assignee
- Vim keybindings: `j`/`k` to navigate, `gg`/`G` for top/bottom
- Loading spinner while fetching (bubbles/spinner)
- Display total issue count and sprint name in the header

**Done when:** The active sprint's issues render in a scrollable list.

---

### Milestone 4 — Issue Detail View

- On `Enter`/`l`, push the issue detail view onto the navigation stack
- Fetch full issue via `client.GetIssue(key)` (or use already-fetched data where sufficient)
- Render in a `bubbles/viewport` (scrollable):
  - Header: key, summary, status, priority, assignee, reporter, labels
  - Description: convert ADF → CommonMark via `pkg/md`, render as plain text
  - Comments section: author, timestamp, body (most recent N)
- `Esc`/`h` to go back to the sprint list
- `o` to open the issue in the browser (`open`/`xdg-open`)

**Done when:** Selecting an issue shows its full details, scrollable, with back navigation.

---

### Milestone 5 — Layout & Polish

- Two-pane layout with Lipgloss: issue list on the left, detail preview on the right (if terminal is wide enough), single-pane on narrow terminals
- Consistent colour palette (inherit terminal colours where possible, don't hardcode)
- Keybinding help bar at the bottom (`?` to toggle full help)
- Graceful handling of empty sprints, API errors, and slow connections (timeouts + error states in the model)
- Respect `NO_COLOR` env var

**Done when:** It looks intentional and holds up at various terminal widths.

---

### Milestone 6 — Release

- Write `README.md`: what it is, install instructions, config, keybindings
- Tag `v0.0.1`
- Confirm `go install github.com/seanhalberthal/jiratui@latest` works from a clean environment

**Done when:** Installable by anyone with Go and a Jira Cloud account.

---

## Configuration (v0.0.1)

Minimal config via environment variables. No separate config file for now — reuse jira-cli's.

| Variable          | Purpose                          | Required |
|-------------------|----------------------------------|----------|
| `JIRA_DOMAIN`     | e.g. `yourorg.atlassian.net`     | Yes      |
| `JIRA_USER`       | Your Atlassian email             | Yes      |
| `JIRA_API_TOKEN`  | API token or PAT                 | Yes      |
| `JIRA_AUTH_TYPE`  | `basic` (default) or `bearer`    | No       |
| `JIRA_BOARD_ID`   | Board ID to load on startup      | Yes (for now) |

---

## Keybindings (v0.0.1)

| Key       | Action                        |
|-----------|-------------------------------|
| `j` / `↓` | Move down                    |
| `k` / `↑` | Move up                      |
| `gg`      | Jump to top                   |
| `G`       | Jump to bottom                |
| `Enter` / `l` | Open issue detail         |
| `Esc` / `h`   | Back to list              |
| `o`       | Open issue in browser         |
| `r`       | Refresh current view          |
| `?`       | Toggle help                   |
| `q`       | Quit                          |

---

## Out of Scope for v0.0.1

- Any write operations (create, edit, transition, comment)
- Epic/backlog views
- JQL search input
- Multiple board support
- Config file (beyond env vars)
- Auth via OS keychain

These are natural candidates for v0.1.0 once the read path is solid.
