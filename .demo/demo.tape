# jiru demo — produces .demo/demo.gif and .demo/demo.mp4 for the README.
#
# Prereqs:
#   make build      # produces ./jiru with version info
#   brew install vhs ffmpeg
#
# Run from the repo root:
#   make demo       # equivalent to: vhs .demo/demo.tape
#
# The recording uses the --demo flag which boots the in-memory client; no
# real Jira credentials are read and saved filters / recents stay isolated
# under the __demo profile.

Output .demo/demo.gif
Output .demo/demo.mp4

Set Shell "zsh"
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 13
Set Width 1280
Set Height 760
Set Theme { "name": "Monokai", "background": "#272822", "foreground": "#f8f8f2", "cursor": "#f8f8f0", "selection": "#49483e", "black": "#272822", "red": "#f92672", "green": "#a6e22e", "yellow": "#f4bf75", "blue": "#66d9ef", "magenta": "#ae81ff", "cyan": "#a1efe4", "white": "#f8f8f2", "brightBlack": "#75715e", "brightRed": "#f92672", "brightGreen": "#a6e22e", "brightYellow": "#f4bf75", "brightBlue": "#66d9ef", "brightMagenta": "#ae81ff", "brightCyan": "#a1efe4", "brightWhite": "#f9f8f5" }
Set TypingSpeed 70ms
Set PlaybackSpeed 1.0

# ── Clean prompt (hidden) ────────────────────────────────────
Hide
Type "clear"
Enter
Show

# ── Launch — the boot-path methods sleep so the logo + spinner show ──
Type "./jiru --demo"
Enter
Sleep 2.8s

# ── Sprint list — let the viewer take in the 16 issues ───────
Sleep 1.5s

# ── Kanban toggle (b) ────────────────────────────────────────
Type "b"
Sleep 2.5s

# Navigate right into the In Progress column to land on a card.
Type "l"
Sleep 400ms
Type "j"
Sleep 600ms

# ── Move menu on kanban (m) — transition a card to Done ──────
Type "m"
Sleep 1.8s
# Default cursor at "Start work" (index 0); j j lands on "Done" (index 2).
Type "j"
Sleep 400ms
Type "j"
Sleep 500ms
Enter
Sleep 2.5s

# Back to the list view. The list cursor preserves its pre-toggle position,
# so a single `j` from the top lands on ACME-101 (the featured story).
Type "b"
Sleep 1.5s
Type "j"
Sleep 600ms
Enter
Sleep 3.5s

# Scroll the detail view so the wiki-markup comments come into frame.
Type "d"
Sleep 1.8s
Type "d"
Sleep 1.8s

# ── Link menu (L) from inside the issue ──────────────────────
Type "L"
Sleep 1.8s
# Touch the cursor so it visibly responds, then commit on "blocks".
Type "j"
Sleep 500ms
Type "k"
Sleep 600ms
Enter
Sleep 1.2s
# Step 2: type the target issue key.
Type "ACME-104"
Sleep 1.5s
Enter
Sleep 2.8s

# ── Cross-system: i picker — jump from this issue to the runbook page ─
Type "i"
Sleep 2s
# Picker entries for ACME-101: Parent (ACME-100), Description (ACME-104),
# Confluence Pages (Acme Onboarding Runbook). j j → the runbook page.
Type "j"
Sleep 450ms
Type "j"
Sleep 600ms
Enter
Sleep 3s

# Confluence page rendered — scroll to show the ADF content.
Type "d"
Sleep 1.8s
Type "d"
Sleep 1.8s

# ── Cross-system: i picker — jump from the page back to a Jira issue ─
Type "i"
Sleep 2s
# Picker entries for the runbook: Dashboard Architecture (page), ACME-101,
# ACME-104. j j → ACME-104, a different issue than the one we came from.
Type "j"
Sleep 450ms
Type "j"
Sleep 600ms
Enter
Sleep 3s

# Home (H) — jump straight back to the sprint list from anywhere.
Type "H"
Sleep 1.2s

# ── JQL search with autocomplete (s) ─────────────────────────
Type "s"
Sleep 1s
# Type the start of a clause; the autocomplete popup surfaces priority values.
Type "priority = "
Sleep 2.5s
Type "High"
Sleep 1.2s
Enter
Sleep 2.5s

# Three escapes to fully close: results → input → dismiss completions → hide.
Escape
Sleep 400ms
Escape
Sleep 300ms
Escape
Sleep 700ms

# ── Saved filters (f) ────────────────────────────────────────
Type "f"
Sleep 1.8s
# Cursor lands on "Recent bugs" — the top favourite. Apply directly.
Enter
Sleep 2.5s

# Open the first bug to set up the branch wizard.
Enter
Sleep 2.5s

# ── Branch wizard (n) — slug auto-populated from the issue ───
Type "n"
Sleep 2.8s
# Show the wizard but don't actually invoke git.
Escape
Sleep 800ms

# Back out and quit.
Escape
Sleep 500ms
Escape
Sleep 500ms
Type "q"
Sleep 600ms
Type "q"
Sleep 800ms
