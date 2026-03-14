# jiratui

A terminal UI for Jira built in Go. Browse boards, search issues with JQL, and read issue details without leaving the terminal.

## Install

```sh
go install github.com/seanhalberthal/jiratui@latest
```

Or build from source:

```sh
git clone https://github.com/seanhalberthal/jiratui.git
cd jiratui
make install
```

## Configuration

On first launch, if required credentials are missing, jiratui shows an interactive setup wizard that walks you through configuring your Jira connection. The wizard validates credentials against the Jira API and stores the API token in the OS keychain (macOS Keychain, GNOME Keyring, or Windows Credential Manager) with a fallback to the config file. Other settings are saved to `~/.config/jiratui/config.env`. You can re-open the wizard at any time by pressing `S` from the home screen.

jiratui resolves configuration from four sources, in priority order:

1. **Environment variables** — always take precedence
2. **jiratui config file** — `~/.config/jiratui/config.env` (written by the setup wizard)
3. **Zsh config files** — scans `~/.zshenv`, `~/.zprofile`, `~/.zshrc`, `~/.secrets.zsh`, `~/.config/secrets.zsh`, and `~/.config/zsh/secrets.zsh` for `export` statements
4. **jira-cli config** — falls back to `~/.config/.jira/.config.yml` for domain, user, and board ID

| Variable | Alias | Purpose | Required |
|---|---|---|---|
| `JIRA_DOMAIN` | `JIRA_URL` | e.g. `yourorg.atlassian.net` (protocol stripped automatically) | Yes |
| `JIRA_USER` | `JIRA_USERNAME` | Your Atlassian email | Yes |
| `JIRA_API_TOKEN` | | API token or PAT | Yes |
| `JIRA_AUTH_TYPE` | | `basic` (default) or `bearer` | No |
| `JIRA_BOARD_ID` | | Board ID to load on startup (skips home screen) | No |
| `JIRA_PROJECT` | | Project key to filter the board list | No |

The aliases (`JIRA_URL`, `JIRA_USERNAME`) provide compatibility with tools like mcp-atlassian that use different variable names.

### Getting an API token

1. Go to https://id.atlassian.com/manage-profile/security/api-tokens
2. Create a new token
3. Set `JIRA_API_TOKEN` to the generated value

### Finding your board ID

The board ID is in the URL when viewing a board in Jira: `https://yourorg.atlassian.net/jira/software/projects/PROJ/boards/123` — the board ID is `123`.

## Keybindings

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `h` / `l` | Move left / right (board columns) |
| `Enter` | Open / select |
| `Esc` | Back one level |
| `q` | Back one level (quit at top-level view) |
| `Ctrl+C` | Quit |
| `o` | Open issue in browser |
| `b` | Toggle board / list view |
| `e` | Filter by parent (Epic, Feature, etc.) |
| `r` | Refresh current view |
| `?` | Search issues (JQL) with context-aware autocomplete |
| `S` | Open setup wizard (from home screen) |
| `/` | Filter current list |
| `H` | Go to home screen |

## Usage

```sh
jiratui              # Launch the TUI (home screen or sprint view if JIRA_BOARD_ID is set)
jiratui PROJ-123     # Open a specific issue directly
jiratui --version    # Print version
```

## Development

```sh
make build     # Build for current platform
make test      # Run tests
make lint      # Run linter
make check     # Run all checks (fmt, tidy, vet, lint, test)
make help      # Show all targets
```

## Licence

MIT
