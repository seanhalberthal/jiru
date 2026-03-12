# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```sh
make build       # Build binary for current platform → ./jiratui
make test        # Run tests with race detector (go test -race ./...)
make lint        # Run golangci-lint v2
make lint-fix    # Run golangci-lint with auto-fix
make check       # Run all checks: fmt, tidy, vet, lint, test
make build-all   # Cross-compile to dist/ (linux/darwin × amd64/arm64)
```

Version is injected at build time via `-X main.version=...` from `git describe`.

## Architecture

This is a terminal UI for Jira built with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework (Elm architecture: Model → Update → View).

### Data flow

`main.go` → loads `config.Config` (env vars, falls back to jira-cli's `~/.config/.jira/.config.yml`) → creates `client.Client` (wraps `jira-cli`'s API client) → passes to `ui.App` (root Bubble Tea model).

### UI layer (`internal/ui/`)

- **`app.go`** — Root model. Manages three view states: `viewLoading` → `viewSprint` → `viewIssue`. Orchestrates async commands (auth, sprint fetch, issue fetch) and routes messages to child models.
- **`messages.go`** — All custom `tea.Msg` types (`ClientReadyMsg`, `SprintLoadedMsg`, `IssuesLoadedMsg`, `IssueSelectedMsg`, `IssueDetailMsg`, `OpenURLMsg`, `ErrMsg`).
- **`keys.go`** — Global `KeyMap` with vim-style bindings.
- **`sprintview/`** — Issue list using `bubbles/list`. Custom `issueDelegate` renders two-line items (key + summary + status / type + assignee). Exposes `SelectedIssue()` for parent to detect selection.
- **`issueview/`** — Detail pane using `bubbles/viewport`. Renders metadata, description, and last 10 comments with text wrapping.

### Supporting packages

- **`internal/config/`** — Loads config from env vars (`JIRA_DOMAIN`, `JIRA_USER`, `JIRA_API_TOKEN`, `JIRA_AUTH_TYPE`, `JIRA_BOARD_ID`), falls back to jira-cli config file.
- **`internal/client/`** — Wraps `jira-cli`'s `Client` with typed methods (`Me`, `ActiveSprint`, `SprintIssues`, `GetIssue`). Converts jira-cli types to domain types.
- **`internal/jira/`** — Domain types (`Issue`, `Comment`, `Sprint`) decoupled from the API client.
- **`internal/theme/`** — Adaptive colours and lipgloss styles shared across views. `StatusStyle()` maps status names to colour styles.

### Key pattern

Child models (`sprintview.Model`, `issueview.Model`) are value types. They signal events to the parent via sentinel fields (e.g., `SelectedIssue()`, `OpenURL()`) rather than returning messages — the parent polls these after calling `Update`.
