package theme

import (
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// Colours — adaptive, so they respect terminal theme.
var (
	ColourSubtle    = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"}
	ColourPrimary   = lipgloss.AdaptiveColor{Light: "#0055ff", Dark: "#7aa2f7"}
	ColourSuccess   = lipgloss.AdaptiveColor{Light: "#008800", Dark: "#9ece6a"}
	ColourWarning   = lipgloss.AdaptiveColor{Light: "#885500", Dark: "#e0af68"}
	ColourError     = lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#f7768e"}
	ColourCancelled = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#636363"}
	ColourLogo      = lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#818CF8"}
)

// Logo is the ASCII art logo rendered in the terminal.
const Logo = `       █████ █████ ███████████   █████  █████
      ░░███ ░░███ ░░███░░░░░███ ░░███  ░░███
       ░███  ░███  ░███    ░███  ░███   ░███
       ░███  ░███  ░██████████   ░███   ░███
       ░███  ░███  ░███░░░░░███  ░███   ░███
 ███   ░███  ░███  ░███    ░███  ░███   ░███
░░████████   █████ █████   █████ ░░████████
 ░░░░░░░░   ░░░░░ ░░░░░   ░░░░░   ░░░░░░░░`

// LogoWidth is the minimum terminal width needed to display the logo.
const LogoWidth = 48

// RenderLogo returns the logo styled in muted blue, or empty if the terminal is too narrow.
func RenderLogo(width int) string {
	if width < LogoWidth {
		return ""
	}
	return lipgloss.NewStyle().Foreground(ColourLogo).Render(Logo)
}

// Styles used across the application.
var (
	StyleApp = lipgloss.NewStyle().
			Padding(1, 2)

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColourPrimary)

	StyleSubtle = lipgloss.NewStyle().
			Foreground(ColourSubtle)

	StyleStatusOpen = lipgloss.NewStyle().
			Foreground(ColourPrimary).
			Bold(true)

	StyleStatusInProgress = lipgloss.NewStyle().
				Foreground(ColourWarning).
				Bold(true)

	StyleStatusDone = lipgloss.NewStyle().
			Foreground(ColourSuccess).
			Bold(true)

	StyleStatusCancelled = lipgloss.NewStyle().
				Foreground(ColourCancelled).
				Strikethrough(true)

	StyleKey = lipgloss.NewStyle().
			Foreground(ColourPrimary).
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(ColourError).
			Bold(true)

	StyleHelpKey = lipgloss.NewStyle().
			Foreground(ColourSubtle).
			Bold(true)

	StyleHelpDesc = lipgloss.NewStyle().
			Foreground(ColourSubtle)

	// Board view styles.
	StyleColumnTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColourPrimary).
				Padding(0, 1).
				MarginBottom(1)

	StyleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColourSubtle).
			Padding(0, 1).
			MarginBottom(1)

	StyleCardSelected = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColourPrimary).
				Padding(0, 1).
				MarginBottom(1)

	StyleColumnBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, false, false).
				BorderForeground(ColourSubtle)

	StyleErrorDialog = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColourError).
				Padding(1, 3).
				Align(lipgloss.Center)
)

// statusCategoryMap holds the instance-specific status→category mapping
// populated from the Jira /status API. Guarded by statusMu.
var (
	statusCategoryMap map[string]int
	statusMu          sync.RWMutex
)

// SetStatusCategoryMap installs the instance-specific mapping from the Jira
// /status API. Values: 0 = to do, 1 = in progress, 2 = done.
// Once set, StatusStyle and StatusCategory use this instead of hardcoded names.
func SetStatusCategoryMap(m map[string]int) {
	statusMu.Lock()
	statusCategoryMap = m
	statusMu.Unlock()
}

// StatusStyle returns the appropriate style for a given status.
// Uses the instance-specific category map if available, falls back to
// hardcoded name matching.
func StatusStyle(status string) lipgloss.Style {
	switch StatusCategory(status) {
	case 3:
		return StyleStatusCancelled
	case 2:
		return StyleStatusDone
	case 1:
		return StyleStatusInProgress
	default:
		return StyleStatusOpen
	}
}

// userColours is a palette of distinct, readable colours for user names.
var userColours = []lipgloss.AdaptiveColor{
	{Light: "#1e6f5c", Dark: "#73daca"},
	{Light: "#8b5cf6", Dark: "#bb9af7"},
	{Light: "#d97706", Dark: "#e0af68"},
	{Light: "#0891b2", Dark: "#7dcfff"},
	{Light: "#be185d", Dark: "#f7768e"},
	{Light: "#4f46e5", Dark: "#7aa2f7"},
	{Light: "#059669", Dark: "#9ece6a"},
	{Light: "#9333ea", Dark: "#c49af7"},
	{Light: "#c2410c", Dark: "#ff9e64"},
	{Light: "#0d9488", Dark: "#2ac3de"},
}

// UserStyle returns a consistent colour style for a given name.
// The same name always produces the same colour via hashing.
func UserStyle(name string) lipgloss.Style {
	if name == "" {
		return StyleSubtle
	}
	var h uint32
	for _, c := range name {
		h = h*31 + uint32(c)
	}
	colour := userColours[h%uint32(len(userColours))]
	return lipgloss.NewStyle().Foreground(colour)
}

// StatusCategory returns a sort-friendly category for a status name.
// Returns 0 for "to do", 1 for "in progress", 2 for "done", 3 for "cancelled".
// Uses the instance-specific category map if available, falls back to
// hardcoded name matching.
func StatusCategory(status string) int {
	statusMu.RLock()
	m := statusCategoryMap
	statusMu.RUnlock()

	if m != nil {
		if cat, ok := m[status]; ok {
			return cat
		}
	}

	// Fallback for when metadata hasn't loaded yet.
	if IsCancelledName(status) {
		return 3
	}
	switch status {
	case "Done", "Closed", "Resolved":
		return 2
	case "In Progress", "In Review":
		return 1
	default:
		return 0
	}
}

// IsDone returns true if the status category represents terminal/completed work
// (both "done" and "cancelled"). Use this for progress counting.
func IsDone(status string) bool {
	cat := StatusCategory(status)
	return cat == 2 || cat == 3
}

// IsCancelledName checks if a status name represents a cancelled/rejected state.
func IsCancelledName(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "cancel"):
		return true
	case strings.Contains(lower, "won't do"):
		return true
	case strings.Contains(lower, "reject"):
		return true
	case strings.Contains(lower, "decline"):
		return true
	case strings.Contains(lower, "obsolete"):
		return true
	default:
		return false
	}
}
