# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Mermaid diagram rendering in issue descriptions — flowcharts, sequence diagrams, and other mermaid diagram types are rendered as Unicode box-drawing art instead of raw code text (via `pgavlin/mermaid-ascii`)
- Auto-detection of mermaid content inside `{code}`, `{noformat}`, and `{mermaid}` blocks
- Quit confirmation dialog — pressing `q`/`esc` at the top-level view shows a centred dialog instead of quitting immediately (`ctrl+c` still quits without confirmation)

### Fixed

- `f` key occasionally triggered page-forward instead of opening filters in the home view — caused by stale search visibility state and conflicting `bubbles/list` default keybindings
- Headings, lists, and other block-level elements inside `{note}`, `{info}`, `{warning}`, `{tip}`, `{panel}`, and `{quote}` blocks now render correctly instead of showing raw `h2.` / `h3.` markup

### Changed

- Board view columns now sort by status category (todo → in progress → done → cancelled) with workflow sub-priority within each category (development before review before QA)
- Board view column navigation now remembers cursor positions — returning to a previously-visited column restores your place instead of overwriting it
- Search board view title shows the saved filter name when results came from a saved filter

### Removed

- `H` (home) keybinding — use `esc`/`q` to navigate back instead

## [0.2.0] — 2026-03-20

### Added

- Board view for search results — press `b` from JQL search or saved filter results to view issues in a kanban board grouped by status

### Changed

- Replaced jira-cli dependency with a custom HTTP API client (`internal/api/`)
- Board issue loading (no active sprint) now uses v3 JQL search with the board's filter instead of the Agile v1 API, which had an undocumented truncation limit on Jira Cloud (~1000 issues)
- v3 JQL search requests specific fields instead of `*all` for better pagination reliability and performance
- Configuration now resolves from environment variables and profiles.yml only — removed zsh config file scanning and jira-cli config fallback
- Config and filter paths now respect `XDG_CONFIG_HOME` (defaults to `~/.config/jiru/`)
- Renamed `profiles.yaml` → `profiles.yml` and `filters.yaml` → `filters.yml`

### Fixed

- Board issue loading stuck at ~1000–1100 issues on Jira Cloud due to Agile v1 API truncation
- Cursor loop detection for v3 JQL pagination (known Jira Cloud bug where `nextPageToken` repeats)
- Loading indicator now shows consistently during progressive pagination

## [0.1.9] — 2026-03-20

### Added

- Named profiles — multiple Jira instances via `--profile <name>` flag or `P` in the TUI, with per-profile keychain storage
- CLI subcommands — `jiru get`, `jiru search`, `jiru list`, `jiru boards` for JSON output
- Profile management view (`P`) — create, switch, and delete profiles from the TUI
- Edit issue view (`e` from issue view) — edit summary and priority inline
- Auto-migration from legacy `config.env` to `profiles.yml` on first run

### Changed

- Config storage migrated from `~/.config/jiru/config.env` to `~/.config/jiru/profiles.yml`
- `ResetConfig` now cleans up profiles.yml, all profile keyring entries, and legacy config.env

### Fixed

- Status transition now shows the target status name (e.g., "Code Review") instead of the transition action name (e.g., "Review code")

## [0.1.8] — 2026-03-19

### Added

- Copy JQL to clipboard (`x`) from filter manager
- Refresh search results (`r`) to re-run the current JQL query

### Fixed

- UTF-8 label truncation in issue picker — multi-byte characters no longer produce garbled output
- Filter duplicate name capped at 100 characters to prevent overflow

### Changed

- Status and type colour hashing unified to FNV-1a for better distribution

## [0.1.7] — 2026-03-19

### Added

- Inline issue actions — assign (`a`), edit summary/priority (`e`), link issues (`l`), delete (`D`) from the issue view
- Issue key navigation — jump between referenced issues via the issue picker (`i`)
- Hash-based colour palettes — deterministic colours for user names, status badges, and issue types
- Copy issue URL to clipboard (`x`) from issue view
- Navigate to parent issue (`p`) from issue view

## [0.1.6] — 2026-03-18

### Added

- Saved JQL filters — save, edit, duplicate, favourite, delete, and apply JQL queries from a filter manager (`f` from home/sprint/board views)
- JQL autocompletion in filter editor (same engine as search view)
- Default issue sort changed to `updated DESC` (most recently active first) for sprint and board views

### Changed

- JQL parser and autocomplete engine extracted to `internal/jql` package (shared by search and filter views)
- Filters keybind changed from `F` to `f`
- "Save filter" option hidden in search results when results originated from a saved filter

## [0.1.5] — 2026-03-17

### Added

- Issue detail view shows parent issue link (key, summary, and type) when available
- Issue detail view shows child issues grouped by status category (To Do / In Progress / Done) with a progress bar
- Status field added to issue detail metadata section
- Dynamic status category mapping from the Jira `/status` API — custom status names (e.g. "Completed", "In Dev", "Awaiting QA") are now categorised correctly for styling, grouping, and statistics

### Changed

- `StatusStyle()` and `StatusCategory()` now use instance-specific status categories from the API, falling back to hardcoded names only before metadata loads
- `SprintIssueStats` uses the dynamic status mapping instead of hardcoded status names

## [0.1.4] — 2026-03-16

### Added

- Progressive pagination — issues load in batches of ~200, showing the first page immediately while remaining pages load in the background
- Half-page scrolling (`d`/`u`) in sprint list, board, and issue detail views
- Board view column windowing — shows up to 4 columns at a time, with `[3/10]` position indicator and scrolling via `h`/`l`
- Colour-coded user names in issue detail view (assignee, reporter, comment authors) — consistent colours per name via hashing
- Created and Updated timestamps shown in issue detail metadata
- Board view supports `r` to refresh (previously sprint-only)
- Board columns ordered by instance-wide status metadata when available (via JQL metadata fetch)
- JQL metadata fetched eagerly on authentication for faster search and board rendering

### Changed

- Migrated JQL search from deprecated v2 API to v3 `/search/jql` with token-based pagination
- Sprint, board, epic, and search views now fetch all matching issues (up to 2000) instead of being capped at 200
- Board fallback (no active sprint) now uses Agile v1 API (`/board/{id}/issue`) instead of JQL search for more reliable pagination

### Fixed

- Duplicate issues no longer appear during progressive pagination across sprint, board, and search views
- Cursor position preserved during progressive pagination — new pages no longer jump the user back to the top
- Sprint list filtering no longer disrupted by background pagination pages arriving
- Search results cursor position preserved when new pages append
- Board view selected card sometimes partially clipped at bottom edge
- Kanban board fallback no longer crashes when project key is missing

## [0.1.3] — 2026-03-15

### Added

- Status transitions (`m` from issue/board view) — pick from available transitions to move issues between statuses
- Add comment (`c` from issue view) — multi-line editor with `ctrl+s` to submit

### Fixed

- Actions (comments, transitions) could fire twice if a background message arrived before the server responded
- API token was unnecessarily exposed in the process environment
- Branch names are now validated against git naming rules before creation
- Clipboard-copied git commands are now properly shell-quoted

### Security

- JQL queries are escaped to prevent injection via status names or other user-controlled values
- Error messages no longer display internal URLs or API endpoints
- Git branch creation uses `--` separator to prevent argument injection

## [0.1.2] — 2026-03-15

### Added

- Issue creation wizard (`c` from home/sprint/board) — multi-step form with project/type pickers, priority, assignee search, labels, parent issue, and description
- Version display in footer bar

## [0.1.1] — 2026-03-15

### Added

- Branch creation from issue view (`n`) — create branches locally, push to remote, or both
- Configurable branch naming: lowercase (`proj-123-fix-login-bug`) or title case (`PROJ-123-Fix-Login-Bug`)
- New config options: `JIRA_REPO_PATH`, `JIRA_BRANCH_UPPERCASE`, `JIRA_BRANCH_MODE`
- Setup wizard steps for git repo path, branch case, and branch mode
- ASCII logo on setup wizard welcome screen and loading screen
- Coloured status messages (green for success, red for errors) above the footer
- `--reset` flag to clear all config and credentials

### Fixed

- Repo path and branch case settings not persisted (missing from config file parser)
- Dismissing setup wizard always navigated to empty home view instead of returning to previous view

## [0.1.0] — 2026-03-14

Initial release.

### Core

- Home screen with board list, active sprint names, and issue stats
- Sprint issue list with fuzzy filtering (`/`)
- Kanban board view (`b`) with status columns, card rendering, scrolling, and parent-based filtering (`e`)
- Issue detail view with metadata, description, and last 10 comments
- JQL search (`?`) with context-aware autocompletion (fields, operators, values, keywords, ORDER BY)
- Dynamic completion values fetched from the Jira instance (statuses, issue types, priorities, etc.)
- Live user search for assignee/reporter completions
- Direct issue opening via CLI argument (e.g., `jiru PROJ-123`)
- Open issue in browser with `enter` from detail view

### Setup & Configuration

- Interactive setup wizard — auto-launches when credentials are missing, walks through domain, user, API token, and auth type with async API verification
- Interactive project and board pickers in setup wizard (fetched from Jira API)
- OS keychain integration for API token storage (macOS Keychain, GNOME Keyring, Windows Credential Manager) with fallback to config file
- Config persistence to `~/.config/jiru/config.env`
- Loads credentials from env vars → config file → zsh config files → jira-cli config
- Supports `JIRA_URL`/`JIRA_USERNAME` aliases for compatibility with mcp-atlassian
- `JIRA_BOARD_ID` is optional — when unset, the app shows the home screen

### Rendering

- Atlassian wiki markup rendering for descriptions and comments — headings, lists, code blocks, tables, panels, admonitions, inline formatting, links, and images styled for the terminal
- Adaptive colour theme with status-specific styling
- Persistent keybind footer with context-sensitive bindings per view

### Security

- Input validation for issue keys, project keys, domains, emails, auth types, and board IDs
- JQL injection prevention — project/parent keys are validated and single-quoted
- URL scheme guard — only `https://` URLs are opened in the browser
- Auth type allowlist (`basic` / `bearer`)

### Navigation

- `esc` — back
- `q` — back one level, quits at top level
- `enter` — open
- `b` — toggle list/board view
- `S` — re-open setup wizard
- `?` — JQL search
- `/` — in-page list filter
- `h`/`l` — column navigation in board view
