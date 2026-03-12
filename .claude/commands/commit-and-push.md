---
description: Commit staged changes and create a PR, push on top if PR already exists.
---

# Commit and Push Changes

## Pre-Commit Validation

1. **Branch Check**:
    - **CRITICAL**: NEVER commit or push directly to `main`. This is a hard rule — no exceptions
    - If on `main`, STOP and create a feature branch first: `git checkout -b type/short-description`
    - Branch naming convention: `type/short-description` (e.g. `feat/zsh-credentials`, `fix/auth-timeout`)
    - If unsure about the branch name, ask the user

2. **Git Status Check**:
    - Run `git status` to see staged, unstaged, and untracked files
    - If there are no changes at all, stop and inform the user
    - Review which files are staged vs unstaged — ask the user if staging looks correct before proceeding

3. **Quality Checks**:
    - Run `make check` (runs fmt, tidy, vet, lint, test in sequence)
    - **CRITICAL**: If any check fails, STOP. Fix the issue, then re-run `make check`. Do NOT proceed with a failing check
    - If `make lint-fix` can resolve lint issues, run it and re-stage affected files

4. **README Update Check** (MANDATORY):
    - **STOP**: You MUST complete this step before proceeding to commit
    - **Read `README.md`** to understand current documentation
    - **Compare changes against README content** — for each changed file, check if:
      - New commands, features, or functionality were added
      - Installation steps or prerequisites changed
      - Configuration options changed (new env vars, new flags)
      - Keybindings were added or changed
      - Directory structure or file locations changed
    - **If ANY documentation updates are needed**:
      - Update the README BEFORE creating the commit
      - Stage the README changes along with the other changes
    - **If unsure**: Ask the user whether README updates are needed
    - **Do NOT skip this step** — documentation drift causes confusion

5. **CLAUDE.md Update Check**:
    - **Read `CLAUDE.md`** and check if changes affect documented conventions:
      - New packages or architectural patterns
      - Build command changes
      - New key patterns or data flow changes
      - New directories or file structure changes
    - **If conventions have changed**: Update CLAUDE.md and stage it
    - This is especially important when adding new packages under `internal/`

6. **CHANGELOG Update** (for user-facing changes):
    - **Only required for commits with these prefixes**: `fix:`, `feat:`, `add:`, `update:`, `breaking:`, `remove:`
    - **Skip for**: `refactor:`, `chore:`, `docs:`, `test:`, `ci:`, internal-only changes
    - **First: tidy existing entries.** Before adding anything new, check `CHANGELOG.md` for entries under `[Unreleased]` that belong to an already-released version. Cross-reference with `git tag --sort=-v:refname` and `git log --oneline` between tags. Move any misplaced entries into their correct `[x.y.z]` section (creating the section if needed)
    - **Then: add new entry** under `[Unreleased]` using this format:
      ```markdown
      ## [Unreleased]

      ### Added
      - New feature or capability

      ### Changed
      - Modified behaviour

      ### Fixed
      - Bug fix

      ### Removed
      - Removed feature
      ```
    - If `CHANGELOG.md` doesn't exist yet, create it with the `[Unreleased]` section
    - Stage `CHANGELOG.md` with the other changes

## Commit Process

### Analyse Changes

- Run `git diff --cached --stat` and `git diff --cached` to understand what's being committed
- Identify the nature of the change: new feature, bug fix, refactor, etc.

### Generate Commit Message

- Use conventional commit format: `type: brief description`
- Common prefixes: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`, `add:`, `update:`, `remove:`
- Keep the first line under 72 characters
- Use lowercase after the prefix
- Add a blank line and body paragraph for non-trivial changes
- **NEVER include `Co-Authored-By:` lines** — no AI attribution
- **NEVER mention Claude, Claude Code, or any AI assistant**

### Execute Commit and Push

1. Stage any additional files identified during checks (README, CLAUDE.md, CHANGELOG.md)
2. Create the commit:
    ```sh
    git commit -m "type: description"
    ```
3. Push to remote:
    ```sh
    git push -u origin HEAD
    ```

## Pull Request Creation

### Create Pull Request

- Use `gh pr create` with this format:
    ```sh
    gh pr create --title "type: short description" --body "$(cat <<'EOF'
    ## Summary
    - <1-3 bullet points describing the changes>

    ## Test plan
    - [ ] Testing steps...
    EOF
    )"
    ```
- **NEVER include AI attribution** in PR descriptions
- Keep descriptions short and focused on the changes

### Push to Existing PR

- If a PR already exists for the current branch, just push:
    ```sh
    git push
    ```
- The PR will update automatically

## Post-Push Verification

- Run `git log --oneline -3` to confirm the commit looks correct
- If a PR was created, display the PR URL

## Notes

- **Project structure**: Go module at `github.com/seanhalberthal/jiratui`
- **Key directories**: `internal/config/`, `internal/client/`, `internal/jira/`, `internal/ui/`
- **Quality gate**: `make check` is the single command that runs all checks (fmt, tidy, vet, lint, test)
- **Repo is currently private** — no remote configured yet
