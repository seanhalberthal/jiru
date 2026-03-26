<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset=".github/assets/logo-light.svg">
  <img alt="jiru" src=".github/assets/logo-dark.svg" width="320">
</picture>

**A terminal UI for Jira and Confluence — browse sprints, view wiki pages, transition issues, and search with JQL without leaving the terminal.**

[![Release](https://img.shields.io/github/v/release/seanhalberthal/jiru?style=flat&logo=github&logoColor=white&label=Release)](https://github.com/seanhalberthal/jiru/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![CI](https://img.shields.io/github/actions/workflow/status/seanhalberthal/jiru/ci.yml?branch=main&style=flat&logo=githubactions&logoColor=white&label=CI)](https://github.com/seanhalberthal/jiru/actions)
[![Licence](https://img.shields.io/github/license/seanhalberthal/jiru?style=flat&label=licence)](LICENCE)
[![macOS](https://img.shields.io/badge/macOS-supported-000000?style=flat&logo=apple&logoColor=white)]()
[![Linux](https://img.shields.io/badge/Linux-supported-FCC624?style=flat&logo=linux&logoColor=black)]()

[Quick Start](#quick-start) · [Configuration](#configuration) · [Usage](#usage) · [Keybindings](#keybindings) · [Development](#development)

</div>

---

## Features

- **Sprint list view** — browse issues in the active sprint with filtering
- **Kanban board view** — status columns with card rendering, scrolling, and parent-based filtering
- **Issue detail view** — metadata, parent/child issue navigation, progress bar, description, and comments with full Atlassian wiki markup rendering
- **Inline issue actions** — assign (`a`), edit summary/priority (`e`), link issues (`L`), delete (`D`), and transition status (`m`) without leaving the terminal; transition, link, and copy URL also work from list, board, and search views
- **Comments** — post comments from the issue detail view (`c`) with a multi-line editor
- **JQL search** — context-aware autocomplete for fields, operators, values, and keywords, with live user search for assignee/reporter
- **Saved filters** — save, edit, duplicate, favourite, and apply JQL queries from a filter manager (`f`), with copy-to-clipboard for JQL
- **Issue creation** — multi-step wizard to create issues with project/type pickers, priority, assignee search, labels, and parent issue
- **Branch creation** — create branches from issues with configurable mode (local, remote, or both) and title-case or lowercase naming
- **Issue key navigation** — jump between referenced issues (parent, children, description/comment links) via the issue picker (`i`)
- **Confluence integration** — browse spaces, view pages with full ADF rendering, inline and footer comments, navigate page hierarchies, and track recently viewed pages (`Tab`)
- **Profiles** — multiple named profiles for different Jira instances, switchable with `--profile` or `P` in the TUI
- **CLI subcommands** — `get`, `search`, `list`, `boards`, `wiki` — JSON output for scripting and integration
- **Setup wizard** — interactive first-run configuration with API validation and OS keychain storage
- **Direct issue opening** — pass an issue key as a CLI argument to jump straight to it

---

## Quick Start

```sh
brew install seanhalberthal/tap/jiru
```

---

## Configuration

On first launch, if required credentials are missing, jiru shows an interactive setup wizard that validates credentials against the Jira API and stores the API token in the OS keychain (macOS Keychain or SecretService on Linux). Other settings are saved to `~/.config/jiru/profiles.yml`. Re-open the wizard at any time with `S`.

### Profiles

jiru supports multiple named profiles for different Jira instances (e.g. work, staging). Use `--profile <name>` or `P` from the TUI to switch between profiles. Each profile stores its own credentials, project, board, and branch settings.

Settings are stored in `$XDG_CONFIG_HOME/jiru/profiles.yml` (defaults to `~/.config/jiru/`) and the API token is kept in the OS keychain. The setup wizard handles all of this automatically.

Environment variables can override profile settings when needed (e.g. for CI or scripting):

| Variable | Alias | Purpose |
|---|---|---|
| `JIRA_DOMAIN` | `JIRA_URL` | Jira instance domain, e.g. `yourorg.atlassian.net` |
| `JIRA_USER` | `JIRA_USERNAME` | Atlassian email address |
| `JIRA_API_TOKEN` | | [API token](https://id.atlassian.com/manage-profile/security/api-tokens) or PAT |
| `JIRA_AUTH_TYPE` | | `basic` (default) or `bearer` |
| `JIRA_BOARD_ID` | | Board ID — when unset, the setup wizard prompts for one |
| `JIRA_PROJECT` | | Project key to filter the board list |
| `JIRA_REPO_PATH` | | Path to local git repo for branch creation |
| `JIRA_BRANCH_UPPERCASE` | | `true` for Title-Case branch names (e.g. `PROJ-123-Fix-Login-Bug`) |
| `JIRA_BRANCH_MODE` | | Branch creation mode: `local`, `remote`, or `both` (default: `local`) |

The aliases (`JIRA_URL`, `JIRA_USERNAME`) provide compatibility with tools like mcp-atlassian that use different variable names. `JIRA_DOMAIN` strips the protocol automatically if provided.

<details>
<summary>Finding your board ID</summary>

The board ID is in the URL when viewing a board in Jira:

```
https://yourorg.atlassian.net/jira/software/projects/PROJ/boards/123
```

The board ID is `123`.

</details>

---

## Usage

```sh
jiru                    # Launch the TUI
jiru PROJ-123           # Open a specific issue directly
jiru --profile staging  # Use a named profile
jiru --version          # Print version
jiru --reset            # Reset all config and credentials
```

### CLI subcommands

```sh
jiru get PROJ-123       # Fetch issue details as JSON
jiru search "JQL query" # Search issues via JQL
jiru list               # List issues in active sprint
jiru boards             # List available boards
jiru wiki               # Confluence wiki commands
```

All CLI subcommands support `--profile` and output JSON to stdout.

When `JIRA_BOARD_ID` is set, the TUI loads the sprint view directly. Otherwise, the setup wizard prompts for a board. You can switch boards at any time with `B`.

---

## Keybindings

### Navigation

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `d` / `u` | Half-page down / up |
| `g` / `G` | Jump to top / bottom |
| `h` / `l` | Move left / right (board columns) |
| `Enter` / `Space` | Open / select |
| `Esc` | Back one level |
| `q` | Back one level (quit at top level) |
| `Ctrl+C` | Quit |

### Global actions

| Key | Context | Action |
|---|---|---|
| `s` | Most views | Search issues (JQL) with autocomplete |
| `?` | Most views | Help overlay |
| `f` | Sprint / board / search board | Saved filters |
| `r` | Sprint / board / issue / search results / search board | Refresh current view |
| `m` | Sprint / board / search results / search board | Transition issue status |
| `L` | Sprint / board / search results / search board | Link issue |
| `x` | Sprint / board / search results / search board | Copy issue URL |
| `b` | Sprint / board / search results / search board | Toggle board / list view |
| `B` | Sprint / board | Switch board |
| `c` | Sprint / board | Create new issue |
| `H` | Most views | Go home (issue list) |
| `S` | Sprint / board | Open setup wizard |
| `P` | Sprint / board | Switch profile |
| `Tab` | Sprint / board | Switch to Confluence wiki view |
| `/` | Sprint / board / search results | Filter current list |

### Issue view

| Key | Action |
|---|---|
| `o` | Open issue in browser |
| `x` | Copy issue URL to clipboard |
| `m` | Transition issue status |
| `c` | Add comment |
| `a` | Assign issue |
| `e` | Edit summary / priority |
| `n` | Create branch from issue |
| `L` | Link to another issue |
| `D` | Delete issue |
| `w` | Toggle watch / unwatch |
| `p` | Navigate to parent issue |
| `i` | Issue picker (parent, child, mentioned) |

Global keys are suppressed when text input is active (search, create, branch, comment, etc.).

---

## Development

```sh
make build       # Build binary → ./jiru
make test        # Run tests with race detector
make lint        # Run golangci-lint v2
make check       # Run all checks: fmt, tidy, vet, lint, test
make build-all   # Cross-compile for linux/darwin × amd64/arm64
make help        # Show all targets
```

---

## Licence

[MIT](LICENCE)
