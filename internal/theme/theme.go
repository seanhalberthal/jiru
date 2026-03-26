package theme

import (
	"hash/fnv"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
)

// Colours — adaptive, so they respect terminal theme.
var (
	ColourSubtle    = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"}
	ColourPrimary   = lipgloss.AdaptiveColor{Light: "#0055ff", Dark: "#7aa2f7"}
	ColourSuccess   = lipgloss.AdaptiveColor{Light: "#008800", Dark: "#9ece6a"}
	ColourWarning   = lipgloss.AdaptiveColor{Light: "#885500", Dark: "#e0af68"}
	ColourError     = lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#f7768e"}
	ColourCancelled = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#636363"}
	ColourKey       = lipgloss.AdaptiveColor{Light: "#0e7490", Dark: "#2ac3de"}
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
			Foreground(ColourKey).
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
// hardcoded name matching. Todo and in-progress statuses get deterministic
// hash-based colours so different statuses within the same category are
// visually distinguishable.
func StatusStyle(status string) lipgloss.Style {
	switch StatusCategory(status) {
	case 3:
		return StyleStatusCancelled
	case 2:
		return StyleStatusDone
	case 1:
		colour := hashColour(status, statusInProgressColours)
		return lipgloss.NewStyle().Foreground(colour).Bold(true)
	default:
		colour := hashColour(status, statusTodoColours)
		return lipgloss.NewStyle().Foreground(colour).Bold(true)
	}
}

// hashColour returns a deterministic colour from a palette based on the name.
func hashColour(name string, palette []lipgloss.AdaptiveColor) lipgloss.AdaptiveColor {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return palette[h.Sum32()%uint32(len(palette))]
}

// statusTodoColours — blue hue family: variations from sky to indigo.
var statusTodoColours = []lipgloss.AdaptiveColor{
	{Light: "#1d4ed8", Dark: "#7aa2f7"}, // base blue
	{Light: "#2563eb", Dark: "#89b4fa"}, // lighter blue
	{Light: "#4338ca", Dark: "#818cf8"}, // indigo
	{Light: "#4f46e5", Dark: "#a5b4fc"}, // soft indigo
	{Light: "#3730a3", Dark: "#6d73de"}, // deep indigo
	{Light: "#1e40af", Dark: "#93c5fd"}, // sky blue
}

// statusInProgressColours — warm hue family: amber through orange.
var statusInProgressColours = []lipgloss.AdaptiveColor{
	{Light: "#b45309", Dark: "#e0af68"}, // base amber
	{Light: "#c2410c", Dark: "#ff9e64"}, // orange
	{Light: "#d97706", Dark: "#ffc777"}, // gold
	{Light: "#ea580c", Dark: "#ffb86c"}, // tangerine
	{Light: "#a16207", Dark: "#d4a054"}, // dark gold
	{Light: "#9a3412", Dark: "#f0a070"}, // burnt orange
}

// typeColours — purple-to-blue hue family.
var typeColours = []lipgloss.AdaptiveColor{
	{Light: "#7c3aed", Dark: "#bb9af7"}, // base purple
	{Light: "#8b5cf6", Dark: "#c4b5fd"}, // soft violet
	{Light: "#6d28d9", Dark: "#a78bfa"}, // deep purple
	{Light: "#6366f1", Dark: "#9b9ef7"}, // periwinkle
	{Light: "#4f46e5", Dark: "#818cf8"}, // indigo
	{Light: "#4338ca", Dark: "#a5b4fc"}, // blue-indigo
}

// TypeStyle returns a consistent colour style for an issue type via hashing.
func TypeStyle(issueType string) lipgloss.Style {
	if issueType == "" {
		return StyleSubtle
	}
	return lipgloss.NewStyle().Foreground(hashColour(issueType, typeColours))
}

// priorityColours — red-to-blue heat scale: urgent feels hot, low feels cool.
var priorityColours = map[string]lipgloss.AdaptiveColor{
	"highest":  {Light: "#b91c1c", Dark: "#f7768e"}, // red
	"critical": {Light: "#b91c1c", Dark: "#f7768e"},
	"blocker":  {Light: "#b91c1c", Dark: "#f7768e"},
	"high":     {Light: "#c2410c", Dark: "#ff9e64"}, // orange-red
	"major":    {Light: "#c2410c", Dark: "#ff9e64"},
	"medium":   {Light: "#b45309", Dark: "#e0af68"}, // amber
	"low":      {Light: "#1d4ed8", Dark: "#7aa2f7"}, // blue
	"minor":    {Light: "#1d4ed8", Dark: "#7aa2f7"},
	"lowest":   {Light: "#4338ca", Dark: "#818cf8"}, // indigo
	"trivial":  {Light: "#4338ca", Dark: "#818cf8"},
}

// PriorityStyle returns a colour style for a priority level.
// Known priorities get semantic heat-scale colours; unknown ones are hashed.
func PriorityStyle(priority string) lipgloss.Style {
	if priority == "" {
		return StyleSubtle
	}
	if c, ok := priorityColours[strings.ToLower(priority)]; ok {
		return lipgloss.NewStyle().Foreground(c)
	}
	return lipgloss.NewStyle().Foreground(hashColour(priority, typeColours))
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
// The same name always produces the same colour via FNV-1a hashing.
func UserStyle(name string) lipgloss.Style {
	if name == "" {
		return StyleSubtle
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	colour := userColours[h.Sum32()%uint32(len(userColours))]
	return lipgloss.NewStyle().Foreground(colour)
}

// StatusSubPriority returns a workflow sub-priority for ordering statuses
// within the same category. Focuses on the common dev workflow stages —
// development before review before testing. Unrecognised statuses get a
// neutral mid-range value (50) so they slot between recognised stages
// without disrupting relative order via SliceStable.
func StatusSubPriority(status string) int {
	lower := strings.ToLower(status)
	for _, kw := range workflowKeywords {
		if strings.Contains(lower, kw.keyword) {
			return kw.priority
		}
	}
	return 50
}

type keywordPriority struct {
	keyword  string
	priority int
}

// workflowKeywords — ordered by specificity (longer keywords first) to
// prevent partial matches. Only covers the broad workflow stages that
// are consistent across most Jira instances.
var workflowKeywords = []keywordPriority{
	// Early — ready/development
	{"ready for development", 10},
	{"selected for development", 10},
	{"ready for dev", 10},
	{"in development", 20},
	{"development", 20},
	{"in progress", 20},
	{"implementing", 20},

	// Mid — review
	{"code review", 40},
	{"peer review", 40},
	{"in review", 40},
	{"review", 45},

	// Late — testing/QA/release
	{"in test", 60},
	{"in qa", 60},
	{"testing", 60},
	{"qa", 65},
	{"uat", 70},
	{"user acceptance", 70},
	{"staging", 80},
	{"ready for release", 85},
	{"ready to deploy", 85},
	{"deploy", 90},
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
// Delegates to jira.IsCancelledName — the source of truth lives in the domain package.
func IsCancelledName(name string) bool {
	return jira.IsCancelledName(name)
}
