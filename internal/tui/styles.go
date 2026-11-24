package tui

import "github.com/charmbracelet/lipgloss"

var (
	ink     = lipgloss.Color("#E9F1F2")
	muted   = lipgloss.Color("#91A3AA")
	faint   = lipgloss.Color("#5E7077")
	borderC = lipgloss.Color("#26363C")
	panelC  = lipgloss.Color("#0B1113")
	panel2C = lipgloss.Color("#10191D")
	accentC = lipgloss.Color("#3BE4D8")
	greenC  = lipgloss.Color("#48D597")
	redC    = lipgloss.Color("#FF5263")
	amberC  = lipgloss.Color("#FFB454")
	blueC   = lipgloss.Color("#63A9FF")
	violetC = lipgloss.Color("#A98BFF")

	headerStyle = lipgloss.NewStyle().
			Foreground(ink).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(borderC)

	brandStyle = lipgloss.NewStyle().Foreground(accentC).Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(muted)
	faintStyle = lipgloss.NewStyle().Foreground(faint)

	tabStyle  = lipgloss.NewStyle().Foreground(muted).Padding(0, 2)
	activeTab = lipgloss.NewStyle().Foreground(accentC).Bold(true).Padding(0, 2).Underline(true)

	columnStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderC).
			Padding(0, 1)

	columnActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentC).
				Padding(0, 1)

	columnHeadStyle = lipgloss.NewStyle().Foreground(ink).Bold(true)
	countStyle      = lipgloss.NewStyle().Foreground(muted)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderC).
			Background(panelC).
			Padding(0, 1)

	cardSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentC).
				Background(panel2C).
				Padding(0, 1)

	cardTitleStyle = lipgloss.NewStyle().Foreground(ink).Bold(true)
	cardMetaStyle  = lipgloss.NewStyle().Foreground(muted)
	cardRefStyle   = lipgloss.NewStyle().Foreground(faint)

	footerStyle = lipgloss.NewStyle().
			Foreground(muted).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(borderC)

	errorStyle   = lipgloss.NewStyle().Foreground(redC)
	statusStyle  = lipgloss.NewStyle().Foreground(greenC)
	loadingStyle = lipgloss.NewStyle().Foreground(amberC)

	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentC).
			Background(lipgloss.Color("#071012")).
			Padding(1, 2)

	sectionTitle = lipgloss.NewStyle().Foreground(ink).Bold(true)
	labelChip    = lipgloss.NewStyle().Foreground(muted)
	helpKey      = lipgloss.NewStyle().Foreground(accentC).Bold(true).Width(15)
	helpText     = lipgloss.NewStyle().Foreground(muted)
	listHeader   = lipgloss.NewStyle().Foreground(faint).Bold(true)
	selectedRow  = lipgloss.NewStyle().Background(panel2C).Foreground(ink)
	doneStyle    = lipgloss.NewStyle().Foreground(greenC)
)

func priorityColor(p string) lipgloss.Color {
	switch p {
	case "urgent":
		return redC
	case "high":
		return amberC
	case "medium":
		return blueC
	case "low":
		return violetC
	default:
		return faint
	}
}

func priorityStyle(p string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(priorityColor(p)).Bold(p == "urgent" || p == "high")
}

func priorityLabel(p string) string {
	switch p {
	case "urgent":
		return "URG"
	case "high":
		return "HIGH"
	case "medium":
		return "MED"
	case "low":
		return "LOW"
	default:
		return "—"
	}
}

func colorValue(color string, fallback lipgloss.Color) lipgloss.Color {
	if len(color) == 7 && color[0] == '#' {
		return lipgloss.Color(color)
	}
	return fallback
}

func labelStyle(color string) lipgloss.Style {
	if len(color) == 7 && color[0] == '#' {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	}
	return labelChip
}
