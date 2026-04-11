---
description: Commit staged changes and push; push to an existing PR if one exists, otherwise ask before creating one.
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

6. **CHANGELOG Update** (MANDATORY for user-facing changes):
    - **Only required for commits with these prefixes**: `fix:`, `feat:`, `add:`, `update:`, `breaking:`, `remove:`
    - **Skip for**: `refactor:`, `chore:`, `docs:`, `test:`, `ci:`, internal-only changes

    **Step A — RECONCILE `[Unreleased]` AGAINST GIT TAGS (CRITICAL, DO THIS FIRST)**:
    - **STOP**: You MUST complete this step before adding any new entries. Do not skip it even if the changelog looks clean.
    - **1. Enumerate tags**: run `git tag --sort=-v:refname | head -10`. These are the most recent release tags, newest first.
    - **2. Check each tag has a section**: for each tag, look for a matching `## [x.y.z]` heading in `CHANGELOG.md`. Any tag without a section is a drift candidate.
    - **3. For every drift candidate** (in reverse chronological order, so oldest-missing first):
      - Find the previous tag: `git tag --sort=-v:refname | grep -A1 '^<tag>$' | tail -1`
      - List the commits in that release: `git log --oneline <prev-tag>..<tag>`
      - Read the current `[Unreleased]` section and identify which entries describe commits that landed in `<tag>`
      - Look up the tag date: `git log <tag> -1 --format="%ci"` (use the `YYYY-MM-DD` portion)
      - Create a new `## [x.y.z] — YYYY-MM-DD` section directly below `[Unreleased]` and move the matching entries into it, preserving their `### Added` / `### Changed` / `### Fixed` / `### Removed` subheadings
      - If `[Unreleased]` ends up empty, leave the heading in place with no body (it's the landing zone for Step B)
    - **4. Sanity check**: after reconciliation, every tag from step 1 must either have a `## [x.y.z]` section or be older than the oldest section in the changelog. `[Unreleased]` must contain only entries for commits that are NOT yet tagged.
    - **Why this matters**: entries stranded under `[Unreleased]` after a release make the changelog lie about what shipped when. Catching drift now prevents it from compounding.

    **Step B — ADD NEW ENTRY** under `[Unreleased]`:
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

## Pull Request Handling

### Detect Existing PR

- After pushing, check whether a PR already exists for the current branch:
    ```sh
    gh pr view --json number,url,state 2>/dev/null
    ```
- If the command succeeds, a PR exists. The push has already updated it — print the PR URL and stop. Do NOT open a new PR.

### No Existing PR — Ask Before Creating One

- **Do NOT create a PR by default.** Ask the user first.
- Present a short confirmation prompt, e.g. "No PR exists for `<branch>`. Create one now? (y/N)"
- If the user declines (or doesn't confirm), stop after the push. The commit is safely on the remote; they can open a PR later themselves.
- **If the user confirms, always use `gh pr create` directly** — do NOT suggest the GitHub web URL (`https://github.com/.../pull/new/<branch>`) as an alternative. The CLI path is the expected flow; offering a web URL makes the agent look uncertain and forces the user to context-switch.
- Run `gh pr create` with this format:
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

## Post-Push Verification

- Run `git log --oneline -3` to confirm the commit looks correct
- If a PR was created, display the PR URL

## Notes

- **Project structure**: Go module at `github.com/seanhalberthal/jiru`
- **Key directories**: `internal/config/`, `internal/client/`, `internal/jira/`, `internal/ui/`
- **Quality gate**: `make check` is the single command that runs all checks (fmt, tidy, vet, lint, test)
- **Repo is currently private** — no remote configured yet
