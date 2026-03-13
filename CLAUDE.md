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

`main.go` → loads `config.Config` (env vars → zsh config files → jira-cli's `~/.config/.jira/.config.yml`) → creates `client.Client` (wraps `jira-cli`'s API client) → passes to `ui.App` (root Bubble Tea model).

### UI layer (`internal/ui/`)

- **`app.go`** — Root model. Manages six view states: `viewLoading` → `viewHome` → `viewSprint` / `viewBoard` → `viewIssue`, plus `viewSearch` (overlay). Orchestrates async commands (auth, board list, sprint fetch, issue fetch, JQL search) and routes messages to child models. Supports direct issue opening via CLI arg. `b` toggles between list (`sprintview`) and board (`boardview`) views.
- **`messages.go`** — All custom `tea.Msg` types (`ClientReadyMsg`, `SprintLoadedMsg`, `IssuesLoadedMsg`, `IssueSelectedMsg`, `IssueDetailMsg`, `OpenURLMsg`, `ErrMsg`, `BoardsLoadedMsg`, `BoardSelectedMsg`, `SearchResultsMsg`).
- **`keys.go`** — Global `KeyMap` with bindings (`esc` for back, `enter` for open, `?` for JQL search, `/` for list filtering, `H` for home). `q` navigates back from sub-views and quits at the top level. Global keys are suppressed when text input is active (`inputActive()` guard in `app.go`).
- **`footer.go`** — Persistent keybind footer renderer. Context-sensitive bar showing relevant keybinds per view (replaces the old `?`-toggled help overlay).
- **`homeview/`** — Board list using `bubbles/list`. Custom `boardDelegate` renders three-line items (name + type / sprint name / issue stats). Exposes `SelectedBoard()` for parent to detect selection.
- **`searchview/`** — JQL search with text input, results list, and autocomplete popup. Two states: `stateInput` (query entry with completion) and `stateResults` (browsable list). `completions.go` provides JQL field/keyword/function/operator catalogue and prefix matching. Exposes `SubmittedQuery()`, `SelectedIssue()`, and `Dismissed()`.
- **`sprintview/`** — Issue list using `bubbles/list`. Custom `issueDelegate` renders two-line items (key + summary + status / type + assignee). Exposes `SelectedIssue()` for parent to detect selection.
- **`boardview/`** — Kanban board view. Groups issues by status into columns, with card rendering and scrolling. Supports parent-based filtering (e.g., by Epic or Feature). `b` toggles back to list view.
- **`issueview/`** — Detail pane using `bubbles/viewport`. Renders metadata, description (via `markup.Render`), and last 10 comments with wiki markup rendering and text wrapping.

### Supporting packages

- **`internal/config/`** — Loads config from env vars (`JIRA_DOMAIN`, `JIRA_USER`, `JIRA_API_TOKEN`, `JIRA_AUTH_TYPE`, `JIRA_BOARD_ID`, `JIRA_PROJECT`), then zsh config files (`zshparse.go`), then jira-cli config file. `JIRA_BOARD_ID` is now optional — when unset, the app shows the home screen with a board list. Supports aliases `JIRA_URL` and `JIRA_USERNAME`.
- **`internal/client/`** — Wraps `jira-cli`'s `Client` with typed methods (`Me`, `ActiveSprint`, `SprintIssues`, `GetIssue`, `Boards`, `BoardSprints`, `SearchJQL`, `SprintIssueStats`). Exports a `JiraClient` interface implemented by `*Client`, used by the UI layer for testability. Converts jira-cli types to domain types.
- **`internal/validate/`** — Input validation helpers (`IssueKey`, `ProjectKey`) using regex. Used by `main.go` (CLI arg validation), `client` (JQL injection prevention), and available for UI-layer validation.
- **`internal/jira/`** — Domain types (`Issue`, `Comment`, `Sprint`, `Board`, `BoardStats`) decoupled from the API client.
- **`internal/markup/`** — Atlassian wiki markup renderer. `Render(input, width)` converts wiki markup to styled terminal text using lipgloss. Handles inline formatting (bold, italic, underline, strikethrough, monospace, links, images, colour), block elements (headings, lists, code blocks, noformat, panels, quotes, admonitions, tables, horizontal rules), and styled text wrapping. Opening tags with inline content and lenient closing tag detection are supported.
- **`internal/theme/`** — Adaptive colours and lipgloss styles shared across views. `StatusStyle()` maps status names to colour styles.

### Key pattern

Child models (`homeview.Model`, `sprintview.Model`, `boardview.Model`, `issueview.Model`, `searchview.Model`) are value types. They signal events to the parent via sentinel fields (e.g., `SelectedBoard()`, `SelectedIssue()`, `SubmittedQuery()`, `OpenURL()`) rather than returning messages — the parent polls these after calling `Update`.
