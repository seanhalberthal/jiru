# Custom Keybindings Plan (`keybindings.json`)

Status: **Proposal — review before implementing**
Owner: TBD
Branch: `claude/custom-keybindings-plan-3HGXk`

## 1. Goal

Let users customise every keybinding in Jiru via a single config file
(`~/.config/jiru/keybindings.json`) without needing to recompile or touch
profile config. The same file also exposes a `space_as_enter` toggle so users
can decide whether the spacebar should behave like `enter` (open/select) or
fall through to its default behaviour (scroll/paginate in `bubbles/list`).

The design should:

1. **Ship with sensible defaults.** A user who never touches the file should
   see no behavioural change.
2. **Be discoverable.** First launch should either generate a commented
   example file or surface a message in `--help`/setup telling the user where
   to find it.
3. **Fail safe.** A broken `keybindings.json` should never brick the app —
   it should fall back to defaults and show an error in the status bar.
4. **Be profile-agnostic.** Keybindings are a user-level preference, not a
   per-workspace one. Unlike `profiles.json` they are not scoped by profile.

## 2. Why a `space_as_enter` flag?

The codebase is currently inconsistent about whether space acts like enter:

- Explicitly bound to `enter` in `boardview/model.go:415`,
  `boardpickview/model.go:83`, `issuelistview/model.go:48`,
  `searchview/model.go:80`, `wikilistview/model.go:67`,
  `transitionpickview/model.go:114`, `filterpickview/model.go:239`,
  `issuepickview/model.go:89`, `assignpickview/model.go:126`,
  `linkpickview/model.go:130`, `profilepickview/model.go:86`,
  `createview/model.go:465,495,531,605`, `setupview/model.go:611,642,670,689,710`.
- **Not** bound in the global `KeyMap.Open` (`internal/ui/keys.go:57`),
  which uses only `"enter"`.

That mismatch means:

- In some contexts space opens an item. In others (e.g. the issue detail
  view, help overlay) it does not. It also collides with the `bubbles/list`
  default pagination key (space = next page).
- Vim users tend to hate space-as-enter (they expect space to be a leader or
  noop). Non-vim users often love it because it's faster than reaching for
  enter.

So the user-facing toggle is: **"When `space_as_enter` is true, the spacebar
everywhere triggers whatever is bound to `open`/`select`. When false, space
falls back to the child view's default (usually next-page in lists, noop
elsewhere)."**

## 3. File location and format

Location: `~/.config/jiru/keybindings.json`
(respects `XDG_CONFIG_HOME`; reuses `configDir()` in
`internal/config/config.go:242`).

```json
{
  "space_as_enter": true,
  "bindings": {
    "up":          ["k", "up"],
    "down":        ["j", "down"],
    "top":         ["g"],
    "bottom":      ["G"],
    "open":        ["enter"],
    "back":        ["esc"],
    "open_url":    ["o"],
    "refresh":     ["r"],
    "quit":        ["q", "ctrl+c"],
    "search":      ["s"],
    "help":        ["?"],
    "board":       ["b"],
    "setup":       ["S"],
    "branch":      ["n"],
    "create":      ["c"],
    "transition":  ["m"],
    "comment":     ["c"],
    "filters":     ["f"],
    "assign":      ["a"],
    "edit":        ["e"],
    "link":        ["L"],
    "delete":      ["D"],
    "parent":      ["p"],
    "issue_pick":  ["i"],
    "profile":     ["P"],
    "home_tab":    ["tab"],
    "home":        ["H"],
    "watch":       ["w"],
    "board_pick":  ["B"]
  }
}
```

Notes:

- Action names are snake_case copies of the fields in
  `internal/ui/keys.go:6` (`KeyMap` struct). Using snake_case so the JSON
  doesn't leak Go naming conventions on users.
- Values are always arrays so users can bind multiple keys to one action
  (e.g. both `k` and `up`). An empty array means "unbind this action".
- Missing actions inherit the default — the user only needs to specify the
  ones they want to change.
- Key names mirror `tea.KeyMsg.String()` conventions (e.g. `"ctrl+s"`,
  `"shift+tab"`, `"alt+j"`, `"up"`, `"pgdown"`, `"space"` or `" "`, etc.).
  We normalise `"space"` ↔ `" "` during parsing so both work.

### Why JSON, not YAML or TOML?

- `profiles.json` is already JSON (`internal/config/profiles.go:30`). No new
  dependency.
- Keybinding values are primitive enough that YAML's advantages don't apply.
- JSON makes comment-support ugly, so we'll ship a **sibling**
  `keybindings.example.jsonc` file (with `//` comments) to document actions.
  Load path stays `.json`, parse path stays stdlib.

## 4. Schema and validation

Define a new package `internal/keybindings/`:

```go
// internal/keybindings/keybindings.go
package keybindings

type File struct {
    SpaceAsEnter bool                  `json:"space_as_enter"`
    Bindings     map[string][]string   `json:"bindings"`
}

// Load reads keybindings.json. Returns defaults if the file is missing.
// Returns (config, warnings, error). Warnings are non-fatal (unknown action
// names, invalid key strings) — errors are only for unreadable/unparsable files.
func Load() (Resolved, []Warning, error)

// Resolved is the merged, validated result.
type Resolved struct {
    SpaceAsEnter bool
    Actions      map[Action][]string // Canonical action → []key strings.
}

type Action string

const (
    ActionUp         Action = "up"
    ActionDown       Action = "down"
    // ... one per KeyMap field
)

// Default returns the built-in defaults (mirrors DefaultKeyMap in internal/ui/keys.go).
func Default() Resolved
```

Validation rules (all produce **warnings**, not errors):

1. Unknown action name → warning, ignored.
2. Invalid key string (empty, unrecognised modifier) → warning, key is
   dropped from the list.
3. Duplicate keys across actions → warning listing the conflict; **the
   later-defined action wins** at dispatch time. We log it so the user can
   fix it.
4. Action with zero keys after validation → warning, that action is
   effectively unbound (which is allowed — users might want to disable
   `delete`).
5. `ctrl+c` and `esc` have a hard floor: even if the user rebinds `quit` or
   `back`, `ctrl+c` still quits and `esc` still backs out. This is a safety
   rail — losing your only way to exit the app is a terrible experience.
   The floor is applied **after** user bindings merge, in `Load()`.

## 5. Integration points

### 5.1 `internal/ui/keys.go`

`DefaultKeyMap()` should be rewritten to read from
`keybindings.Resolved`:

```go
func KeyMapFromResolved(r keybindings.Resolved) KeyMap {
    return KeyMap{
        Up:   binding(r, keybindings.ActionUp,  "k/↑", "up"),
        Down: binding(r, keybindings.ActionDown,"j/↓", "down"),
        // ...
    }
}

func binding(r keybindings.Resolved, a keybindings.Action, help, desc string) key.Binding {
    keys := r.Actions[a]
    if r.SpaceAsEnter && isOpenLike(a) {
        keys = appendUnique(keys, " ")
    }
    return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
}

func isOpenLike(a keybindings.Action) bool {
    return a == keybindings.ActionOpen
}
```

`DefaultKeyMap()` becomes `KeyMapFromResolved(keybindings.Default())` so
existing tests and fallbacks still work.

### 5.2 `NewApp` wiring (`internal/ui/app.go:146`)

```go
// internal/ui/app.go
resolved, warnings, err := keybindings.Load()
if err != nil {
    // Non-fatal — log and use defaults.
    resolved = keybindings.Default()
    app.statusMsg = fmt.Sprintf("keybindings.json: %v (using defaults)", err)
    app.statusIsError = true
}
for _, w := range warnings {
    // Append to a startup-warning buffer that the UI displays once.
}
app.keys = KeyMapFromResolved(resolved)
app.spaceAsEnter = resolved.SpaceAsEnter
```

Pass the resolved config into `NewApp` so callers (and tests) can inject a
specific config:

```go
func NewApp(c client.JiraClient, directIssue string, partial *config.Config,
    missing []string, version string, kb keybindings.Resolved) App
```

Default callers use `keybindings.Default()`. The real load happens at the
call site in `main.go`, so tests don't touch disk.

### 5.3 Child views

Child views that hard-code `"enter"` / `" "` need to consult the resolved
config. The cleanest way is to **pass the resolved keybinding slice down**
from `App` at construction time rather than have every child re-read the
file.

Proposed signature change for list-like views:

```go
// issuelistview
func New(openKeys []string) Model
```

and similar for:

- `boardview`
- `boardpickview`
- `searchview`
- `wikilistview`
- `transitionpickview`
- `filterpickview`
- `issuepickview`
- `assignpickview`
- `linkpickview`
- `profilepickview`
- `createview`
- `setupview` (its `"enter", " "` checks at
  `internal/ui/setupview/model.go:611,642,670,689,710`)

Plus `boardview` uses a raw `switch msg.String()` at
`internal/ui/boardview/model.go:415` — this needs to check against the
resolved key slice instead of a literal.

The parent in `internal/ui/app.go` constructs each child with
`openKeys := a.keys.Open.Keys()` so it all flows from one source.

### 5.4 `space_as_enter` semantics

Two options considered; **going with option B**:

- **Option A (simple):** append `" "` to the global `Open` binding whenever
  `space_as_enter=true`. Problem: many child views bind space explicitly
  via `case "enter", " ":` style dispatch, so they won't respect the
  toggle unless we thread it through.
- **Option B (thorough, chosen):** the resolved config exposes
  `OpenKeys() []string` — when `space_as_enter=true`, `" "` is
  automatically appended to the open/select action. Every view that
  currently hard-codes `" "` reads from `OpenKeys()` instead. When the flag
  is off, space disappears from every open-like binding and reverts to the
  child widget's default (in `bubbles/list`, that's next-page).

Default value: **`true`** (matches current behaviour; no surprises on
upgrade).

### 5.5 Footer text

The footer currently hard-codes the "enter/space" hint strings in
`internal/ui/footer.go:33`. These should become dynamic:

```go
openHint := "enter"
if a.spaceAsEnter {
    openHint = "enter/space"
}
open := footerBinding{openHint, "open"}
```

Same for the `sel := footerBinding{"enter/space", "select"}` line. The
footer helper takes the app's `spaceAsEnter` flag as an extra parameter.

### 5.6 Help overlay

`helpview` should render the resolved, user-customised keymap — not a
hard-coded legend. It already receives the `KeyMap`; double-check it reads
`.Keys()` rather than static strings.

## 6. First-run experience

On first launch, if `keybindings.json` does not exist:

1. **Do not** write a file automatically (users may never customise and a
   mystery file is scary).
2. **Do** offer a new CLI subcommand: `jiru keybindings init` that writes
   a fully-commented `keybindings.example.jsonc` AND an active
   `keybindings.json` populated with the defaults. Docs in README and the
   help overlay point to it.
3. Also add `jiru keybindings path` (prints the resolved path) and
   `jiru keybindings check` (parses and reports warnings without launching
   the TUI).

These live in `main.go` alongside existing CLI flags.

## 7. Error handling and UX

| Scenario                               | Behaviour                                                              |
|----------------------------------------|------------------------------------------------------------------------|
| File missing                           | Use defaults. No message.                                              |
| File unreadable (permissions, etc.)    | Use defaults. Red status message: `keybindings.json: permission denied`. |
| File present but JSON-invalid          | Use defaults. Red status message: `keybindings.json: parse error at line N`. |
| File valid but some actions unknown    | Apply the rest. Yellow status message: `keybindings.json: unknown actions: foo, bar`. |
| File valid but produces duplicate keys | Apply last-write-wins. Yellow status message listing the conflict.     |
| User unbinds `quit` and `back`         | Still works via safety floor (`ctrl+c`, `esc`).                        |

Status messages display via the existing `statusMsg` channel in `App` and
auto-dismiss after 5 seconds. Warnings are batched into one message so the
user doesn't get spammed.

## 8. Hot reload (deferred)

Out of scope for v1. Reloading requires restarting the app. We could add
an `fsnotify` watcher later; the plumbing is already structured to support
it (all views consume `KeyMap` by value, so swapping it is cheap).

## 9. Tests

New package `internal/keybindings/` with:

- `keybindings_test.go`
  - `TestLoad_MissingFile_ReturnsDefaults`
  - `TestLoad_EmptyFile_ReturnsDefaults`
  - `TestLoad_InvalidJSON_ReturnsErrorAndDefaults`
  - `TestLoad_PartialOverride_MergesWithDefaults` — user defines only `up`
    and `down`, everything else stays default.
  - `TestLoad_UnknownActionName_ProducesWarning`
  - `TestLoad_EmptyKeyList_ProducesUnboundAction`
  - `TestLoad_DuplicateKey_ProducesWarning`
  - `TestLoad_SpaceAsEnter_True_AppendsSpaceToOpen`
  - `TestLoad_SpaceAsEnter_False_DoesNotAppendSpace`
  - `TestLoad_SpaceNormalisation` — `"space"` and `" "` both resolve to the
    same key.
  - `TestLoad_QuitBackSafetyFloor` — rebinding `quit` to `[]` still lets
    `ctrl+c` quit; rebinding `back` to `[]` still lets `esc` back out.
  - `TestDefault_MatchesDefaultKeyMap` — golden test that
    `keybindings.Default()` matches `ui.DefaultKeyMap()` field-by-field so
    we catch drift.

All file-system tests use `t.TempDir()` and override `XDG_CONFIG_HOME`.

UI-level tests:

- `internal/ui/keys_test.go`
  - `TestKeyMapFromResolved_RespectsOverrides`
  - `TestKeyMapFromResolved_SpaceAsEnter`
- `internal/ui/footer_test.go`
  - `TestFooter_OpenHint_SwitchesOnSpaceAsEnter`
- For every child view that previously hard-coded `" "`, add a test that
  space is consumed when `space_as_enter=true` and not when false.
  The `issuelistview`, `boardview`, `searchview`, `wikilistview`,
  `filterpickview` ones are the most valuable since those are the
  high-traffic views.

CLI-level tests:

- `main_test.go` (or wherever `main.go` is tested) for the three new
  `jiru keybindings ...` subcommands.

## 10. Migration and backwards compatibility

- No existing config file is touched.
- No profile schema change.
- Users who never create `keybindings.json` get identical behaviour to
  today (assuming we default `space_as_enter=true`, which matches current
  behaviour in the list-ish views that already bind space).
- Document in `CHANGELOG.md` under a "New" section with a link to the
  example file.

## 11. Open questions / risks

1. **Child view refactor is wide.** ~12 views need constructor signature
   changes. Mitigation: do it in one PR, keep the diff mechanical, add a
   helper `keybindings.OpenKeys(r)` to avoid repetition.
2. **`bubbles/list` filter key.** The `/` filter key is controlled by
   `bubbles/list`, not our `KeyMap`. Users can't rebind it via
   `keybindings.json`. Call this out in the docs; if someone complains we
   can expose `list.KeyMap` as a separate section later.
3. **Help text rendering.** `key.WithHelp("k/↑", "up")` hard-codes the
   rendered string. If a user rebinds `up` to `w`, the footer will still
   say `k/↑`. We need to regenerate the help string from the resolved
   keys:
   ```go
   help := strings.Join(prettyKeyNames(keys), "/")
   ```
   This is a small but easy-to-forget change in `KeyMapFromResolved`.
4. **Chord bindings (e.g. `gg`, `<leader>f`).** Not supported in v1.
   `bubbles/key` dispatches on single `tea.KeyMsg` events. We can fake
   two-char sequences with a small state machine later but it's out of
   scope. Document the limitation.
5. **Cross-platform key strings.** `alt` vs `option` on macOS —
   `bubbletea` normalises this but we should test on both.
6. **Footer space hint inconsistency.** Several views hard-code
   `"enter/space"` in their own `View()` functions (e.g.
   `transitionpickview/model.go:164`). These need to consult
   `space_as_enter` too. Easy to miss.

## 12. Phased rollout

Splitting into two PRs keeps the diff reviewable:

**PR 1 — Plumbing (no user-visible change).**
- Create `internal/keybindings/` package with `Load`, `Default`,
  `Resolved`, tests.
- Change `NewApp` / `DefaultKeyMap` to accept a `Resolved`.
- Thread `openKeys` / `spaceAsEnter` through every child view.
- CLI: `jiru keybindings path|check|init`.
- `space_as_enter` default = `true`, so behaviour is unchanged.

**PR 2 — Documentation and discoverability.**
- README section on customisation.
- `keybindings.example.jsonc` file with inline docs.
- CHANGELOG entry.
- Optional: tiny "keybindings loaded (3 overrides)" line on the loading
  screen for feedback.

## 13. Out of scope

- Per-profile keybindings.
- Chord sequences (`gg`, `<leader>x`).
- Rebinding bubbles/list internals (`/` filter, `pgdown`, etc.).
- GUI editor for keybindings.
- Live reload.
- Importing keybinding schemes from other TUIs.

---

**Summary of files touched:**

- New: `internal/keybindings/keybindings.go`,
  `internal/keybindings/keybindings_test.go`,
  `keybindings.example.jsonc` (docs asset).
- Modified: `internal/ui/keys.go`, `internal/ui/app.go`,
  `internal/ui/footer.go`, `internal/ui/navigate.go`, `main.go`,
  plus every child view that hard-codes `"enter", " "` (see §5.3).
- Tests: `internal/keybindings/keybindings_test.go`,
  `internal/ui/keys_test.go`, `internal/ui/footer_test.go`, plus
  per-view space-as-enter regression tests.
