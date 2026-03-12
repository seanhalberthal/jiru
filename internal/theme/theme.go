package theme

import "github.com/charmbracelet/lipgloss"

// Colours — adaptive, so they respect terminal theme.
var (
	ColourSubtle  = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"}
	ColourPrimary = lipgloss.AdaptiveColor{Light: "#0055ff", Dark: "#7aa2f7"}
	ColourSuccess = lipgloss.AdaptiveColor{Light: "#008800", Dark: "#9ece6a"}
	ColourWarning = lipgloss.AdaptiveColor{Light: "#885500", Dark: "#e0af68"}
	ColourError   = lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#f7768e"}
)

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
