# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Interactive setup wizard — auto-launches when Jira credentials are missing, walks through domain, user, API token, auth type, project, and board ID with per-step validation and async API verification
- OS keychain integration for API token storage (macOS Keychain, GNOME Keyring, Windows Credential Manager) with fallback to config file
- Config persistence to `~/.config/jiratui/config.env` via the setup wizard
- `S` keybind to re-open the setup wizard from the home screen
- Context-aware JQL autocompletion — parses cursor position to offer fields, operators, values, or keywords contextually instead of matching against a flat list
- Dynamic JQL completion values from the Jira instance (statuses, issue types, priorities, resolutions, projects, labels, components, versions) fetched via parallel REST calls
- Live user search for assignee/reporter completions (triggers after 2+ characters)
- New validation helpers: `Domain`, `Email`, `AuthType`, `BoardID`
- `JQLMetadata` domain type and `JQLMetadata()` / `SearchUsers()` client methods

- Atlassian wiki markup rendering in issue descriptions and comments — headings, lists, code blocks, tables, panels, admonitions, inline formatting, links, images, and more are styled for the terminal
- Input validation package (`internal/validate`) with `IssueKey` and `ProjectKey` validators
- CLI argument validation — rejects malformed issue keys before reaching the API
- JQL injection prevention — project keys and parent keys are validated and single-quoted in JQL queries
- URL scheme guard on `openBrowser` — only `https://` URLs are passed to the OS
- `AuthType` allowlist — `config.Load()` now rejects values other than `basic` or `bearer`
- `JiraClient` interface on the client package for UI-layer testability
- Comprehensive test suite: validate, config, client, theme, app, sprintview, homeview, issueview, searchview
- CI workflow (`.github/workflows/ci.yml`) — runs fmt, tidy, vet, lint, test, build on PRs and pushes to main

- JQL autocomplete popup in search view — suggests fields, keywords, functions, and operators as you type (Tab to accept, Down/Up to browse)
- Persistent keybind footer showing context-sensitive bindings per view (replaces `?` help toggle)
- `/` now triggers in-page list filtering in home and sprint views (via bubbles/list built-in filter)
- Esc on empty JQL input returns to the previous view instead of showing a blank screen
- Kanban board view (`b` key) with status columns, card rendering, scrolling, and parent-based filtering (`e` key)
- Centred error dialog for clearer error display
- Home screen with board list, active sprint names, and issue stats when `JIRA_BOARD_ID` is not set
- JQL search view (`?` key) for searching issues across projects
- Direct issue opening via CLI argument (e.g. `jiratui PROJ-123`)
- `JIRA_PROJECT` environment variable to filter the board list by project
- `H` key to navigate back to the home screen
- New client methods: `Boards`, `BoardSprints`, `SearchJQL`, `SprintIssueStats`

### Changed

- Reworked keybindings: `esc` is the universal back key, `q` goes back one level (quits only at the top-level view), `enter` is the only open key, `h`/`l` reserved for left/right column navigation in board view
- Removed `l` as an alias for open and `h` as an alias for back — simplifies key scheme and avoids conflicts in board view
- Removed uppercase `H`/`L` keybinds from board view
- Global keys (`q`, `?`, `H`, `b`, `r`) are now suppressed when text input is active (list filter or JQL search), preventing accidental quits or navigation while typing
- `App.client` field changed from `*client.Client` to `client.JiraClient` interface
- JQL search remapped from `/` to `?` — frees `/` for in-page list filtering (vim convention)
- Board view columns now fit within terminal width and respect window resizing
- Removed "Sprint" labels from UI — home screen shows iteration name directly, list view uses generic "Issues" title
- `JIRA_BOARD_ID` is now optional — when set, the app skips the home screen and loads the sprint directly
- Load Jira credentials from zsh config files (`~/.zshenv`, `~/.zprofile`, `~/.zshrc`, `~/.secrets.zsh`, `~/.config/secrets.zsh`, `~/.config/zsh/secrets.zsh`) as a fallback between environment variables and jira-cli config
- Support `JIRA_URL` alias for `JIRA_DOMAIN` (protocol stripped automatically) and `JIRA_USERNAME` alias for `JIRA_USER`, for compatibility with tools like mcp-atlassian
