package branchview

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/validate"
)

// BranchRequest holds the details for creating a git branch.
type BranchRequest struct {
	Name     string
	Base     string
	RepoPath string // Empty means clipboard-only mode.
	Mode     string // "local", "remote", or "both".
	CopyName bool   // Copy the branch name to the clipboard after a successful create.
}

// Model is the branch creation wizard view.
type Model struct {
	issue          jira.Issue
	repoPath       string   // Path to git repo (empty = clipboard mode).
	branchMode     string   // "local", "remote", or "both".
	branchCopyName bool     // Copy branch name to clipboard after create.
	branches       []string // Available branches for autocomplete.
	suggestions    []string // Filtered suggestions for base branch.
	suggIdx        int      // Selected suggestion index (-1 = none).
	showSugg       bool     // Whether to show suggestion popup.

	branchName  textinput.Model
	baseBranch  textinput.Model
	activeInput int // 0 = branchName, 1 = baseBranch

	width  int
	height int

	errMsg string // Validation error message.

	submitted *BranchRequest // Sentinel: branch to create.
	dismissed bool           // Sentinel: user cancelled.

	submitKeys key.Binding
	closeKeys  key.Binding
}

// New creates a branch creation wizard for the given issue.
func New(issue jira.Issue, repoPath string, branchUppercase bool, branchMode string, branchCopyName bool) Model {
	bn := textinput.New()
	bn.Placeholder = "branch-name"
	bn.CharLimit = 200
	bn.Width = 60
	bn.SetValue(Slugify(issue.Key+"-"+issue.Summary, branchUppercase))
	bn.CursorEnd()
	bn.Focus()

	bb := textinput.New()
	bb.Placeholder = "main"
	bb.CharLimit = 100
	bb.Width = 60
	bb.SetValue("main")

	if branchMode == "" {
		branchMode = "local"
	}

	m := Model{
		issue:          issue,
		repoPath:       repoPath,
		branchMode:     branchMode,
		branchCopyName: branchCopyName,
		branchName:     bn,
		baseBranch:     bb,
		suggIdx:        -1,
		submitKeys:     key.NewBinding(key.WithKeys("enter")),
		closeKeys:      key.NewBinding(key.WithKeys("esc")),
	}

	if repoPath != "" {
		m.branches = listBranches(repoPath)
		// Default to current branch if available.
		if cur := currentBranch(repoPath); cur != "" {
			bb.SetValue(cur)
			bb.CursorEnd()
			m.baseBranch = bb
		}
	}

	return m
}

// SetSize updates the dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Leave room for border (2) + padding (4) + prompt "> " (2) inside the box.
	inputWidth := max(m.dialogWidth()-8, 20)
	m.branchName.Width = inputWidth
	m.baseBranch.Width = inputWidth
	// bubbles/textinput caches its visible offset/offsetRight from the previous
	// Width whenever SetValue/SetCursor ran. Without nudging the cursor here,
	// raising Width would leave the visible window stuck at the old (smaller)
	// size, so a long pre-populated value renders truncated.
	m.branchName.CursorEnd()
	m.baseBranch.CursorEnd()
}

// dialogWidth is the outer width of the branch creation dialog. Wide enough to
// fit the 80-char branch name slug on roomy terminals, narrower otherwise.
func (m Model) dialogWidth() int {
	return min(max(m.width-4, 40), 100)
}

// SubmittedBranch returns the branch request (if set) and resets the sentinel.
func (m *Model) SubmittedBranch() *BranchRequest {
	r := m.submitted
	m.submitted = nil
	return r
}

// Dismissed returns true (once) if the user cancelled.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// InputActive returns true when a text input is focused.
func (m Model) InputActive() bool {
	return true
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		var cmd tea.Cmd
		if m.activeInput == 0 {
			m.branchName, cmd = m.branchName.Update(msg)
		} else {
			m.baseBranch, cmd = m.baseBranch.Update(msg)
		}
		return m, cmd
	}

	if key.Matches(keyMsg, m.closeKeys) {
		if m.showSugg {
			m.showSugg = false
			return m, nil
		}
		m.dismissed = true
		return m, nil
	}

	// Handle suggestion navigation when popup is visible.
	if m.showSugg && len(m.suggestions) > 0 {
		switch keyMsg.String() {
		case "down", "ctrl+n":
			if m.suggIdx < len(m.suggestions)-1 {
				m.suggIdx++
			}
			return m, nil
		case "up", "ctrl+p":
			if m.suggIdx > 0 {
				m.suggIdx--
			}
			return m, nil
		case "enter", "tab":
			m.baseBranch.SetValue(m.suggestions[m.suggIdx])
			m.baseBranch.CursorEnd()
			m.showSugg = false
			m.suggIdx = -1
			return m, nil
		}
	}

	if keyMsg.String() == "tab" || keyMsg.String() == "shift+tab" {
		if m.activeInput == 0 {
			m.activeInput = 1
			m.branchName.Blur()
			m.baseBranch.Focus()
			m.baseBranch.CursorEnd()
			m.updateSuggestions()
		} else {
			m.activeInput = 0
			m.baseBranch.Blur()
			m.branchName.Focus()
			m.branchName.CursorEnd()
			m.showSugg = false
		}
		return m, nil
	}

	if key.Matches(keyMsg, m.submitKeys) && !m.showSugg {
		name := m.branchName.Value()
		base := m.baseBranch.Value()

		if err := validate.BranchName(name); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		if err := validate.BranchName(base); err != nil {
			m.errMsg = "base branch: " + err.Error()
			return m, nil
		}

		m.errMsg = ""
		m.submitted = &BranchRequest{
			Name:     name,
			Base:     base,
			RepoPath: m.repoPath,
			Mode:     m.branchMode,
			CopyName: m.branchCopyName,
		}
		return m, nil
	}

	// Update the active input.
	var cmd tea.Cmd
	if m.activeInput == 0 {
		m.branchName, cmd = m.branchName.Update(msg)
	} else {
		m.baseBranch, cmd = m.baseBranch.Update(msg)
		m.updateSuggestions()
	}
	return m, cmd
}

// updateSuggestions filters branches based on current base branch input.
func (m *Model) updateSuggestions() {
	if len(m.branches) == 0 || m.activeInput != 1 {
		m.showSugg = false
		return
	}

	query := strings.ToLower(m.baseBranch.Value())
	var filtered []string
	for _, b := range m.branches {
		if strings.Contains(strings.ToLower(b), query) {
			filtered = append(filtered, b)
		}
	}

	m.suggestions = filtered
	m.showSugg = len(filtered) > 0 && query != ""
	if m.showSugg {
		m.suggIdx = 0
		// If there's exactly one match and it equals the input, hide suggestions.
		if len(filtered) == 1 && strings.EqualFold(filtered[0], m.baseBranch.Value()) {
			m.showSugg = false
		}
	}
}

// View renders the branch creation wizard.
func (m Model) View() string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2)

	labelStyle := theme.StyleSubtle

	var modeText string
	if m.repoPath == "" {
		modeText = "copies git command to clipboard"
	} else {
		switch m.branchMode {
		case "remote":
			modeText = "pushes branch to origin"
		case "both":
			modeText = "creates branch locally + pushes to origin"
		default:
			modeText = "creates branch locally"
		}
	}
	mode := theme.StyleSubtle.Render(modeText)

	var sections []string
	sections = append(sections, theme.StyleTitle.Render("Create Branch"))
	sections = append(sections, mode)
	sections = append(sections, "")
	sections = append(sections, labelStyle.Render("Branch name:"))
	sections = append(sections, m.branchName.View())
	sections = append(sections, "")
	sections = append(sections, labelStyle.Render("Base branch:"))
	sections = append(sections, m.baseBranch.View())

	// Show branch suggestions.
	if m.showSugg && len(m.suggestions) > 0 {
		maxShow := min(8, len(m.suggestions))
		var rows []string
		for i := range maxShow {
			label := "  " + m.suggestions[i]
			if i == m.suggIdx {
				label = theme.StyleKey.Render("▸ ") + m.suggestions[i]
			}
			rows = append(rows, label)
		}
		if len(m.suggestions) > maxShow {
			rows = append(rows, theme.StyleSubtle.Render(fmt.Sprintf("  … %d more", len(m.suggestions)-maxShow)))
		}
		sections = append(sections, strings.Join(rows, "\n"))
	}

	if m.errMsg != "" {
		sections = append(sections, theme.StyleError.Render(m.errMsg))
	}

	sections = append(sections, "")
	hint := "tab switch fields · enter create · esc cancel"
	if m.repoPath == "" {
		hint = "tab switch fields · enter copy · esc cancel"
	}
	sections = append(sections, theme.StyleSubtle.Render(hint))

	content := strings.Join(sections, "\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		border.Width(m.dialogWidth()).Render(content))
}

// listBranches returns local and remote branch names from the given repo path.
func listBranches(repoPath string) []string {
	out, err := exec.Command("git", "-C", repoPath, "branch", "-a", "--format=%(refname:short)").Output()
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var branches []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		name := strings.TrimSpace(line)
		// Strip "origin/" prefix for remote branches.
		name = strings.TrimPrefix(name, "origin/")
		if name == "" || name == "HEAD" || seen[name] {
			continue
		}
		seen[name] = true
		branches = append(branches, name)
	}
	sort.Strings(branches)
	return branches
}

// currentBranch returns the current branch name in the given repo.
func currentBranch(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// slugRe matches characters that should be replaced with hyphens.
var slugRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// Slugify converts an issue key and summary into a branch-friendly slug.
// The issue key prefix (e.g. "PROJ-123") is always ALL CAPS — Jira does not
// recognise lowercase issue-key prefixes in branch names. Summary words are
// Title-Cased when uppercase is true, lowercased otherwise.
//
//	uppercase=true:  "PROJ-123 Fix login bug!" → "PROJ-123-Fix-Login-Bug"
//	uppercase=false: "PROJ-123 Fix login bug!" → "PROJ-123-fix-login-bug"
func Slugify(s string, uppercase bool) string {
	s = slugRe.ReplaceAllString(s, "-")
	parts := strings.Split(s, "-")
	// Issue key is always LETTERS-DIGITS at the start (2 parts).
	keyParts := 0
	if len(parts) >= 2 && isAllLetters(parts[0]) && isAllDigits(parts[1]) {
		keyParts = 2
	}
	for i, p := range parts {
		if p == "" {
			continue
		}
		switch {
		case i < keyParts:
			parts[i] = strings.ToUpper(p)
		case uppercase:
			parts[i] = titleCase(p)
		default:
			parts[i] = strings.ToLower(p)
		}
	}
	s = strings.Join(parts, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
		// Prefer to end on a word boundary, but only if doing so doesn't drop
		// a large amount of content — a long unhyphenated word near the end
		// would otherwise trim back too aggressively (e.g. losing 25+ chars).
		if idx := strings.LastIndex(s, "-"); idx >= 70 {
			s = s[:idx]
		}
		s = strings.TrimRight(s, "-")
	}
	return s
}

func isAllLetters(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return len(s) > 0
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// titleCase converts a word to Title Case (first letter upper, rest lower).
func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
