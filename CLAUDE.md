# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

**Always use `make` targets instead of running Go commands directly.** The Makefile handles version injection, flags, and consistent tooling.

```sh
make build       # Build binary for current platform → ./jiru
make test        # Run tests with race detector (go test -race ./...)
make test VERBOSE=1  # Run tests with detailed output
make lint        # Run golangci-lint v2
make lint-fix    # Run golangci-lint with auto-fix
make check       # Run all checks: fmt, tidy, vet, lint, test
make build-all   # Cross-compile to dist/ (linux/darwin × amd64/arm64)
```

Version is injected at build time via `-X main.version=...` from `git describe`.

After completing a feature or fix, always run `make build` so the user can test the changes immediately via `./jiru`.

## Architecture

Terminal UI for Jira built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm architecture: Model → Update → View).

### Data flow

`main.go` → `config.MigrateToProfiles()` (migrates legacy `config.env` → `profiles.yml` on first run) → `config.PartialLoad()` (env vars → `~/.config/jiru/profiles.yml`) → if config complete, creates `client.Client` and passes it to `ui.App` with partial config and missing fields. If fields are missing, the setup wizard is shown first.

### Layout

- **`internal/ui/`** — Bubble Tea root (`app.go`, `navigate.go`, `commands.go`, `messages.go`, `keys.go`, `footer.go`) plus per-view sub-packages under `internal/ui/<name>view/`. Each view is a value-type child model that signals events to the parent via sentinel fields (e.g. `SelectedIssue()`, `SubmittedQuery()`) rather than returning messages.
- **`internal/api/`** — thin HTTP client for the Jira REST API (auth, JSON, `V1`/`V2`/`V3` path helpers, generic `DecodeResponse[T]`). No external Jira library dependency.
- **`internal/client/`** — typed service methods (`Me`, `SprintIssues`, `SearchJQL`, `Boards`, `Transitions`, `AddComment`, `ChildIssues`, etc.) plus Confluence methods in `confluence.go`. Exports a `JiraClient` interface used by the UI layer for testability. `*Page` methods return one page with `PageResult` pagination state; non-paged variants loop internally.
- **`internal/config/`** — env vars + `~/.config/jiru/profiles.yml` + OS keychain for the API token. `PartialLoad()` returns whatever is available plus a list of missing fields (used by setup wizard). All env var names are centralised as unexported `env*` constants in `config.go`.
- **`internal/jira/`**, **`internal/confluence/`** — domain types decoupled from the API client.
- **`internal/jql/`** — JQL parser + autocomplete engine shared by search and filter views. `Parse(query, cursor)` → context; `Match(ctx, provider)` → candidates; `NormaliseQuery()` rewrites bare issue keys/numbers.
- **`internal/filters/`**, **`internal/recents/`** — JSON persistence for saved filters and recently-viewed Confluence pages (profile-aware).
- **`internal/markup/`** — Atlassian wiki markup → terminal text (lipgloss). Handles inline marks, block elements, panels/admonitions, tables, and mermaid diagrams (via `pgavlin/mermaid-ascii`).
- **`internal/adf/`** — Atlassian Document Format (ADF) → terminal text. `Render(json, width)` for Confluence pages; `ExtractPageRefs()` for cross-page navigation.
- **`internal/theme/`** — adaptive colours and lipgloss styles. `StatusStyle()`/`StatusCategory()` use instance-specific status-category mapping (set via `SetStatusCategoryMap()`).
- **`internal/validate/`** — regex-based input validators (`IssueKey`, `ProjectKey`, `Domain`, `Email`, `AuthType`, `BoardID`, `BranchName`). Used for CLI args, JQL injection prevention, and form validation.
- **`internal/demo/`** — in-memory `JiraClient` backed by hand-curated fixtures. Powers the `--demo` flag (also `JIRU_DEMO=1`).

### Jira API constraints

- **v2 `/rest/api/2/search` is gone** (410 Gone since May 2025). JQL search must use v3 `/rest/api/3/search/jql` with cursor-based `nextPageToken` pagination. Do not introduce new v2 search calls.
- **Agile v1** (`/rest/agile/1.0/`) endpoints (sprints, boards, epics) still use offset-based `startAt` pagination. The `IsLast` flag is unreliable on Jira Cloud — keep paging until an empty page (with `MaxTotalIssues=2000` as a safety cap).
- **v2 REST** (`/rest/api/2/`) is still used for non-search operations: issue CRUD, transitions, statuses, metadata, comments.
- Cursor loop detection: v3 JQL pagination can repeat `nextPageToken` on Jira Cloud — break the loop when the token repeats.

### Key patterns

- **Child models signal via sentinels**, not messages. Parent calls `view.Update(...)`, then polls `view.Selected*()` / `view.Submitted*()` / `view.Dismissed()` to drive navigation.
- **`bubbles/list` default keybindings conflict with app-level bindings** (`f`/`d` are NextPage, `b`/`u` are PrevPage). Override them in any list-using view that wants those keys for something else (filters, board toggle, scroll).
- **Status messages** display above the footer in green (success) or red (error) and auto-dismiss after 5 seconds. JQL search errors render as inline status, not as the error overlay.
- **Pagination sequence**: `App.paginationSeq` prevents stale `IssuesPageMsg` from corrupting a view after navigation — each new fetch bumps the seq and pages with a stale seq are dropped.
- **`inputActive()` guard** in `navigate.go` suppresses global keys when a textinput/textarea is focused, so typing into a form doesn't trigger app-level shortcuts.
