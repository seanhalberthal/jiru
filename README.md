# jiratui

A terminal UI for Jira built in Go. Browse your active sprint and read issue details without leaving the terminal.

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

jiratui reads configuration from environment variables. It falls back to jira-cli's config (`~/.config/.jira/.config.yml`) for domain, user, and board ID.

| Variable | Purpose | Required |
|---|---|---|
| `JIRA_DOMAIN` | e.g. `yourorg.atlassian.net` | Yes |
| `JIRA_USER` | Your Atlassian email | Yes |
| `JIRA_API_TOKEN` | API token or PAT | Yes |
| `JIRA_AUTH_TYPE` | `basic` (default) or `bearer` | No |
| `JIRA_BOARD_ID` | Board ID to load on startup | Yes |

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
| `Enter` / `l` | Open issue detail |
| `Esc` / `h` | Back to list |
| `o` | Open issue in browser |
| `r` | Refresh current view |
| `/` | Filter issues |
| `?` | Toggle help |
| `q` | Quit |

## Usage

```sh
jiratui              # Launch the TUI
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
