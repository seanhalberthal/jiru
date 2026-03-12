# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Home screen with board list, active sprint names, and issue stats when `JIRA_BOARD_ID` is not set
- JQL search view (`/` key) for searching issues across projects
- Direct issue opening via CLI argument (e.g. `jiratui PROJ-123`)
- `JIRA_PROJECT` environment variable to filter the board list by project
- `H` key to navigate back to the home screen
- New client methods: `Boards`, `BoardSprints`, `SearchJQL`, `SprintIssueStats`

### Changed

- `JIRA_BOARD_ID` is now optional — when set, the app skips the home screen and loads the sprint directly
- Load Jira credentials from zsh config files (`~/.zshenv`, `~/.zprofile`, `~/.zshrc`, `~/.secrets.zsh`, `~/.config/secrets.zsh`, `~/.config/zsh/secrets.zsh`) as a fallback between environment variables and jira-cli config
- Support `JIRA_URL` alias for `JIRA_DOMAIN` (protocol stripped automatically) and `JIRA_USERNAME` alias for `JIRA_USER`, for compatibility with tools like mcp-atlassian
