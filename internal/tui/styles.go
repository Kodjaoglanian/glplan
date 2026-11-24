package tui

import "github.com/charmbracelet/lipgloss"

var (
	fg       = lipgloss.Color("#ededed")
	muted    = lipgloss.Color("#8b8b8b")
	faint    = lipgloss.Color("#5c5c5c")
	borderC  = lipgloss.Color("#303030")
	panelC   = lipgloss.Color("#111111")
	accentC  = lipgloss.Color("#ffffff")
	greenC   = lipgloss.Color("#50c878")
	redC     = lipgloss.Color("#f2555a")
	amberC   = lipgloss.Color("#f5a623")
	blueC    = lipgloss.Color("#4d9dff")

	headerStyle = lipgloss.NewStyle().
		Foreground(fg).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(borderC)

	tabStyle    = lipgloss.NewStyle().Foreground(muted).Padding(0, 2)
	activeTab   = lipgloss.NewStyle().Foreground(accentC).Bold(true).Padding(0, 2).Underline(true)

	columnStyle = lipgloss.NewStyle().
		Width(30).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderC).
		Padding(0, 1)

	columnHeadStyle = lipgloss.NewStyle().Foreground(fg).Bold(true).Padding(0, 0, 1, 0)
	countStyle      = lipgloss.NewStyle().Foreground(muted)

	cardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderC).
		Background(panelC).
		Padding(0, 1).
		Width(27)

	cardSelectedStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentC).
		Background(lipgloss.Color("#1c1c1c")).
		Padding(0, 1).
		Width(27)

	cardTitleStyle  = lipgloss.NewStyle().Foreground(fg).Bold(true)
	cardMetaStyle   = lipgloss.NewStyle().Foreground(muted)
	mutedStyle      = lipgloss.NewStyle().Foreground(muted)
	cardRefStyle    = lipgloss.NewStyle().Foreground(faint)

	footerStyle = lipgloss.NewStyle().
		Foreground(muted).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(borderC)

	errorStyle   = lipgloss.NewStyle().Foreground(redC).Padding(0, 1)
	statusStyle  = lipgloss.NewStyle().Foreground(greenC).Padding(0, 1)
	overlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderC).
		Background(lipgloss.Color("#0d0d0d")).
		Padding(1, 2)

	sectionTitle = lipgloss.NewStyle().Foreground(fg).Bold(true)
	labelChip    = lipgloss.NewStyle().Foreground(muted)
	helpKey      = lipgloss.NewStyle().Foreground(accentC).Bold(true).Width(14)
	helpText     = lipgloss.NewStyle().Foreground(muted)
	listHeader   = lipgloss.NewStyle().Foreground(muted).Bold(true)
	doneStyle    = lipgloss.NewStyle().Foreground(greenC)
	urgentStyle  = lipgloss.NewStyle().Foreground(redC).Bold(true)
	highStyle    = lipgloss.NewStyle().Foreground(amberC)
)

func priorityColor(p string) lipgloss.Color {
	switch p {
	case "urgent":
		return redC
	case "high":
		return amberC
	case "medium":
		return blueC
	default:
		return faint
	}
}

func priorityStyle(p string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(priorityColor(p)).Bold(p == "urgent" || p == "high")
}
