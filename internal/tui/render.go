package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View renders the interface.
func (m Model) View() string {
	if m.width == 0 {
		return "glplan..."
	}
	if m.width < 48 || m.height < 18 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			errorStyle.Render("terminal muito pequeno")+"\n"+mutedStyle.Render("use pelo menos 48×18"))
	}

	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.tabs())
	b.WriteString("\n\n")

	var body string
	switch m.view {
	case viewBoard:
		body = m.boardView()
	case viewList:
		body = m.listView()
	case viewActivity:
		body = m.activityView()
	}
	body = fitLines(body, m.bodyHeight())
	b.WriteString(body)
	b.WriteString("\n")
	b.WriteString(m.footer())

	base := b.String()
	switch m.mode {
	case modeTaskForm:
		return m.overlay(base, m.taskFormView(), min(76, m.width-4), min(27, m.height-3))
	case modeDetail:
		return m.overlay(base, m.detailView(), min(88, m.width-4), min(30, m.height-3))
	case modeSearch:
		return m.overlay(base, m.searchView(), min(62, m.width-4), 9)
	case modeProjects:
		return m.overlay(base, m.projectView(), min(58, m.width-4), min(18, len(m.projects)+8))
	case modeProjectForm:
		return m.overlay(base, m.projectFormView(), min(62, m.width-4), 17)
	case modeHelp:
		return m.overlay(base, m.helpView(), min(72, m.width-4), min(30, m.height-3))
	case modeConfirm:
		return m.overlay(base, "Excluir esta task?\n\n"+errorStyle.Render("essa ação não pode ser desfeita")+"\n\n"+mutedStyle.Render("y confirma · n/esc cancela"), 48, 9)
	case modeComment:
		return m.overlay(base, m.inputOverlay("Novo comentário", m.commentIn.View(), "enter envia · esc cancela"), min(68, m.width-4), 9)
	case modeSubtask:
		return m.overlay(base, m.inputOverlay("Novo subtask", m.subIn.View(), "enter adiciona · esc cancela"), min(68, m.width-4), 9)
	case modeLabels:
		return m.overlay(base, m.labelPickerView(), min(56, m.width-4), min(18, len(m.boardLabels())+8))
	}
	return base
}

func (m Model) header() string {
	project := "sem projeto"
	if len(m.projects) > 0 && m.projectIdx < len(m.projects) {
		p := m.projects[m.projectIdx]
		project = p.Name + mutedStyle.Render("  ["+p.Key+"]")
	}

	open, urgent := 0, 0
	for _, t := range m.tasks {
		if !t.Done() {
			open++
		}
		if t.Priority == "urgent" && !t.Done() {
			urgent++
		}
	}

	conn := statusStyle.Render("●") + mutedStyle.Render(" "+m.backend)
	if m.err != "" {
		conn = errorStyle.Render("● " + truncate(m.err, 34))
	} else if m.loading {
		conn = loadingStyle.Render("◌ sincronizando")
	} else if m.status != "" && time.Since(m.statusAt) < 4*time.Second {
		conn = statusStyle.Render("✓ " + truncate(m.status, 34))
	}

	stats := mutedStyle.Render(countLabel(open, "aberta", "abertas"))
	if urgent > 0 {
		stats += "  " + priorityStyle("urgent").Render(countLabel(urgent, "urgente", "urgentes"))
	}
	if done := len(m.tasks) - open; done > 0 {
		stats += "  " + doneStyle.Render(countLabel(done, "concluída", "concluídas"))
	}

	left := brandStyle.Render("glplan") + mutedStyle.Render(" / ") + project
	right := conn + "  " + stats
	width := max(44, m.width)
	space := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if space < 1 {
		right = conn
		space = width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	}
	if space < 1 {
		space = 1
	}
	line := left + strings.Repeat(" ", space) + right
	return headerStyle.Width(width).Render(line)
}

func (m Model) bodyHeight() int {
	return max(4, m.height-6)
}

func fitLines(s string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := clipLines(strings.Split(s, "\n"), height)
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) tabs() string {
	names := []string{"Board", "List", "Activity"}
	parts := []string{}
	for i, n := range names {
		label := fmt.Sprintf("%d %s", i+1, n)
		if int(m.view) == i {
			parts = append(parts, activeTab.Render(label))
		} else {
			parts = append(parts, tabStyle.Render(label))
		}
	}
	if m.search != "" {
		parts = append(parts, mutedStyle.Render("  filtro: ")+priorityStyle("medium").Render(truncate(m.search, 28))+mutedStyle.Render("  esc limpa"))
	}
	return strings.Join(parts, "")
}

func (m Model) footer() string {
	var keys string
	switch m.mode {
	case modeDetail:
		keys = "x check · t sub · c comentar · L label · d prazo · e editar · p prio · a resp · >/< mover · del excluir · esc"
	case modeLabels:
		keys = "j/k navegar · espaço alterna · enter salvar · esc cancelar"
	case modeTaskForm:
		keys = "tab campo · ←/→ ajustar escolha · enter salva nas escolhas · ctrl+s salvar · esc cancelar"
	case modeProjects:
		keys = "j/k selecionar · enter abrir · n novo projeto · esc voltar"
	case modeProjectForm:
		keys = "tab campo · enter avançar · ctrl+s criar · esc cancelar"
	case modeSearch:
		keys = "enter aplica · ctrl+u limpa · esc fecha"
	default:
		if m.view == viewBoard {
			keys = "j/k selecionar · J/K ordenar · h/l coluna · enter abrir · n nova · e editar · >/< mover · / filtro · p projeto · ? ajuda"
		} else if m.view == viewList {
			keys = "j/k selecionar · enter abrir · n nova · e editar · / filtro · tab view · p projeto · ? ajuda"
		} else {
			keys = "j/k rolar atividade · tab view · r atualizar · p projeto · ? ajuda · q sair"
		}
	}
	return footerStyle.Width(max(44, m.width)).Render(truncate(keys, max(20, m.width-4)))
}

func (m Model) overlay(base, content string, w, h int) string {
	w = clamp(w, 30, max(30, m.width-2))
	h = clamp(h, 7, max(7, m.height-2))
	lines := clipLines(strings.Split(content, "\n"), h-4)
	box := overlayStyle.Width(w - 2).Render(strings.Join(lines, "\n"))
	return compositeOverlay(base, box, m.width, m.height)
}

func compositeOverlay(base, overlay string, width, height int) string {
	if width <= 0 || height <= 0 {
		return overlay
	}
	baseLines := strings.Split(base, "\n")
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	overlayLines := strings.Split(overlay, "\n")
	overlayWidth := 0
	for _, line := range overlayLines {
		overlayWidth = max(overlayWidth, lipgloss.Width(line))
	}
	if overlayWidth == 0 || len(overlayLines) == 0 {
		return strings.Join(baseLines, "\n")
	}
	x := max(0, (width-overlayWidth)/2)
	y := max(0, (height-len(overlayLines))/2)
	for i, overlayLine := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		baseLine := baseLines[row]
		if w := lipgloss.Width(baseLine); w < width {
			baseLine += strings.Repeat(" ", width-w)
		}
		left := ansi.Cut(baseLine, 0, x)
		right := ansi.Cut(baseLine, x+overlayWidth, width)
		baseLines[row] = left + overlayLine + right
	}
	return strings.Join(baseLines, "\n")
}

func (m Model) inputOverlay(title, input, hint string) string {
	return sectionTitle.Render(title) + "\n\n" + input + "\n\n" + mutedStyle.Render(hint)
}

func (m Model) searchView() string {
	return sectionTitle.Render("Filtrar board") + "\n\n" + m.searchIn.View() + "\n\n" +
		mutedStyle.Render("procura em título, descrição e referência")
}
