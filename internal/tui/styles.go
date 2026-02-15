package tui

import "github.com/charmbracelet/lipgloss"

// Color palette.
var (
	colorGreen  = lipgloss.Color("2")
	colorYellow = lipgloss.Color("3")
	colorRed    = lipgloss.Color("1")
	colorGray   = lipgloss.Color("8")
	colorWhite  = lipgloss.Color("15")
	colorCyan   = lipgloss.Color("6")
)

// Layout styles.
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorCyan)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Underline(true).
			Foreground(colorWhite)

	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			PaddingTop(1)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	// Process owner color styles.
	userProcessStyle   = lipgloss.NewStyle().Foreground(colorGreen)
	systemProcessStyle = lipgloss.NewStyle().Foreground(colorGray)
	rootProcessStyle   = lipgloss.NewStyle().Foreground(colorRed)

	// Kill confirmation styles.
	dangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")).
			Background(lipgloss.Color("52")).
			Padding(0, 1)

	warnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGreen)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorRed)

	// Info view styles.
	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			Width(14)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorWhite)
)

// processStyle returns the appropriate style based on the process owner.
func processStyle(user string) lipgloss.Style {
	switch user {
	case "root":
		return rootProcessStyle
	case "_postgres", "_mysql", "_www", "daemon", "nobody", "_windowserver",
		"_spotlight", "_mdnsresponder", "_netbios", "_locationd":
		return systemProcessStyle
	default:
		return userProcessStyle
	}
}
