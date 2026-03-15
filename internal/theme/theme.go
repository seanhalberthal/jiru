package theme

import "github.com/charmbracelet/lipgloss"

// Colours — adaptive, so they respect terminal theme.
var (
	ColourSubtle  = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"}
	ColourPrimary = lipgloss.AdaptiveColor{Light: "#0055ff", Dark: "#7aa2f7"}
	ColourSuccess = lipgloss.AdaptiveColor{Light: "#008800", Dark: "#9ece6a"}
	ColourWarning = lipgloss.AdaptiveColor{Light: "#885500", Dark: "#e0af68"}
	ColourError   = lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#f7768e"}
	ColourLogo    = lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#818CF8"}
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

// StatusStyle returns the appropriate style for a given status category.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "Done", "Closed", "Resolved":
		return StyleStatusDone
	case "In Progress", "In Review":
		return StyleStatusInProgress
	default:
		return StyleStatusOpen
	}
}

// StatusCategory returns a sort-friendly category for a status name.
// Returns 0 for "to do", 1 for "in progress", 2 for "done".
func StatusCategory(status string) int {
	switch status {
	case "Done", "Closed", "Resolved":
		return 2
	case "In Progress", "In Review":
		return 1
	default:
		return 0
	}
}
