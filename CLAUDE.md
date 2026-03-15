# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```sh
make build       # Build binary for current platform → ./jiru
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

`main.go` → `config.PartialLoad()` (env vars → `~/.config/jiru/config.env` → zsh config files → jira-cli config) → if config complete, creates `client.Client` → passes to `ui.App` with partial config and missing fields. If fields are missing, the setup wizard is shown first.

### UI layer (`internal/ui/`)

- **`app.go`** — Root model. Manages view states: `viewSetup` → `viewLoading` → `viewHome` → `viewSprint` / `viewBoard` → `viewIssue` → `viewBranch`, plus `viewSearch` (overlay) and `viewCreate` (issue creation wizard). Orchestrates async commands (auth, board list, sprint fetch, issue fetch, JQL search, JQL metadata, user search, branch creation, issue creation) and routes messages to child models. Supports direct issue opening via CLI arg. `b` toggles between list (`sprintview`) and board (`boardview`) views. `S` opens setup wizard from home, sprint, or board views. `n` opens the branch creation wizard from issue view. `c` opens the create issue wizard from home, sprint, or board views. Status messages display above the footer in green (success) or red (error) and clear on the next keypress. Loading screen and setup wizard welcome step display the ASCII logo.
- **`messages.go`** — All custom `tea.Msg` types (`ClientReadyMsg`, `SprintLoadedMsg`, `IssuesLoadedMsg`, `IssueSelectedMsg`, `IssueDetailMsg`, `OpenURLMsg`, `ErrMsg`, `BoardsLoadedMsg`, `BoardSelectedMsg`, `SearchResultsMsg`, `SetupCompleteMsg`, `JQLMetadataMsg`, `UserSearchMsg`, `BranchCreatedMsg`, `IssueCreatedMsg`).
- **`keys.go`** — Global `KeyMap` with bindings (`esc` for back, `enter` for open, `?` for JQL search, `/` for list filtering, `H` for home, `S` for setup, `n` for branch creation, `c` for create issue). `q` navigates back from sub-views and quits at the top level. Global keys are suppressed when text input is active (`inputActive()` guard in `app.go`).
- **`footer.go`** — Persistent keybind footer renderer. Context-sensitive bar showing relevant keybinds per view (replaces the old `?`-toggled help overlay).
- **`homeview/`** — Board list using `bubbles/list`. Custom `boardDelegate` renders three-line items (name + type / sprint name / issue stats). Exposes `SelectedBoard()` for parent to detect selection.
- **`searchview/`** — JQL search with text input, results list, and context-aware autocomplete popup. Two states: `stateInput` (query entry with completion) and `stateResults` (browsable list). `completions.go` provides a JQL parser that determines cursor context (field, operator, value, keyword, ORDER BY) and offers appropriate completions — including dynamic values (statuses, issue types, priorities, etc.) from `ValueProvider` and live user search for assignee/reporter fields. Exposes `SubmittedQuery()`, `SelectedIssue()`, `Dismissed()`, `SetMetadata()`, `SetUserResults()`, and `NeedsUserSearch()`.
- **`setupview/`** — Interactive setup wizard for first-run configuration. Walks through domain, user, API token, auth type, then presents picker steps for project and board (fetched from Jira API with ↑/↓ navigation), followed by git repo path, branch case, and branch mode toggles. Async API validation at credential checkpoint. Picker steps cache results and re-fetch when the project changes. Signals completion via `Done()` / `Quit()` sentinel fields. Displays the ASCII logo on the welcome step.
- **`branchview/`** — Branch creation wizard. Two text inputs (branch name + base branch) with autocomplete for local/remote branches. `Slugify()` converts issue key + summary to a branch slug — lowercase or title case with ALL-CAPS project key. `BranchRequest` carries name, base, repo path, and mode (`local`/`remote`/`both`). Exposes `SubmittedBranch()` and `Dismissed()` sentinels.
- **`createview/`** — Issue creation wizard. Multi-step form: project (picker) → issue type (picker, project-scoped) → summary (text input, required) → priority (picker) → assignee (text input with live user search) → labels (text input with autocomplete hints) → parent issue (text input) → description (text input) → confirm (summary view). Signals completion via `Done()` / `Quit()` / `CreatedKey()` sentinels. Triggered by `c` key from home, sprint, or board views. On success, navigates to the created issue detail view.
- **`issuedelegate/`** — Shared issue list delegate and `Item` type used by `sprintview` and `searchview`. Renders two-line items (key + summary + status badge / type + assignee). `ToItems()` converts `[]jira.Issue` to `[]list.Item`.
- **`sprintview/`** — Issue list using `bubbles/list` with the shared `issuedelegate.Delegate`. Exposes `SelectedIssue()` for parent to detect selection.
- **`boardview/`** — Kanban board view. Groups issues by status into columns, with card rendering and scrolling. Supports parent-based filtering (e.g., by Epic or Feature). `b` toggles back to list view.
- **`issueview/`** — Detail pane using `bubbles/viewport`. Renders metadata, description (via `markup.Render`), and last 10 comments with wiki markup rendering and text wrapping.

### Supporting packages

- **`internal/config/`** — Loads config from env vars (`JIRA_DOMAIN`, `JIRA_USER`, `JIRA_API_TOKEN`, `JIRA_AUTH_TYPE`, `JIRA_BOARD_ID`, `JIRA_PROJECT`, `JIRA_REPO_PATH`, `JIRA_BRANCH_UPPERCASE`, `JIRA_BRANCH_MODE`), then `~/.config/jiru/config.env` (written by setup wizard), then zsh config files (`zshparse.go`), then jira-cli config file. `PartialLoad()` returns whatever values are available plus a list of missing required fields (used by setup wizard). `WriteConfig()` persists settings to config.env and stores the API token in the OS keychain via `keyring.go` (falls back to file). `JIRA_BOARD_ID` is now optional — when unset, the app shows the home screen with a board list. Supports aliases `JIRA_URL` and `JIRA_USERNAME`.
- **`internal/client/`** — Wraps `jira-cli`'s `Client` with typed methods (`Me`, `ActiveSprint`, `SprintIssues`, `GetIssue`, `Boards`, `BoardSprints`, `SearchJQL`, `SprintIssueStats`, `JQLMetadata`, `SearchUsers`, `Projects`). Exports a `JiraClient` interface implemented by `*Client`, used by the UI layer for testability. `JQLMetadata()` makes parallel REST calls to fetch statuses, issue types, priorities, resolutions, projects, labels, components, and versions. Converts jira-cli types to domain types.
- **`internal/validate/`** — Input validation helpers (`IssueKey`, `ProjectKey`, `Domain`, `Email`, `AuthType`, `BoardID`) using regex. Used by `main.go` (CLI arg validation), `client` (JQL injection prevention), and `setupview` (wizard field validation).
- **`internal/jira/`** — Domain types (`Issue`, `Comment`, `Sprint`, `Project`, `Board`, `BoardStats`, `JQLMetadata`) decoupled from the API client.
- **`internal/markup/`** — Atlassian wiki markup renderer. `Render(input, width)` converts wiki markup to styled terminal text using lipgloss. Handles inline formatting (bold, italic, underline, strikethrough, monospace, links, images, colour), block elements (headings, lists, code blocks, noformat, panels, quotes, admonitions, tables, horizontal rules), and styled text wrapping. Opening tags with inline content and lenient closing tag detection are supported.
- **`internal/theme/`** — Adaptive colours and lipgloss styles shared across views. `StatusStyle()` maps status names to colour styles. `RenderLogo()` returns the ASCII art logo styled in muted blue (or empty if the terminal is too narrow).

### Key pattern

Child models (`homeview.Model`, `sprintview.Model`, `boardview.Model`, `issueview.Model`, `searchview.Model`, `setupview.Model`) are value types. They signal events to the parent via sentinel fields (e.g., `SelectedBoard()`, `SelectedIssue()`, `SubmittedQuery()`, `OpenURL()`) rather than returning messages — the parent polls these after calling `Update`.
