# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Load Jira credentials from zsh config files (`~/.zshenv`, `~/.zprofile`, `~/.zshrc`, `~/.secrets.zsh`, `~/.config/secrets.zsh`, `~/.config/zsh/secrets.zsh`) as a fallback between environment variables and jira-cli config
- Support `JIRA_URL` alias for `JIRA_DOMAIN` (protocol stripped automatically) and `JIRA_USERNAME` alias for `JIRA_USER`, for compatibility with tools like mcp-atlassian
