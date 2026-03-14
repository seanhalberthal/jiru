# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
- `H` — home
- `S` — re-open setup wizard
- `?` — JQL search
- `/` — in-page list filter
- `h`/`l` — column navigation in board view
