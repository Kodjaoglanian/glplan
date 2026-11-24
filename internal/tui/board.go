package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"glplan/internal/model"
)

type boardMetrics struct {
	colWidth     int
	visibleCols  int
	visibleCards int
	height       int
}

func (m Model) boardMetrics() boardMetrics {
	width := m.width
	if width <= 0 {
		width = 100
	}
	visibleCols := max(1, (width-2)/32)
	colWidth := clamp((width-2)/visibleCols-2, 25, 38)
	visibleCards := max(1, (m.height-12)/6)
	if m.height <= 0 {
		visibleCards = 5
	}
	return boardMetrics{colWidth: colWidth, visibleCols: visibleCols, visibleCards: visibleCards, height: m.bodyHeight()}
}

func (m Model) boardView() string {
	if m.loading && m.board == nil {
		return mutedStyle.Render("carregando board...")
	}
	if m.board == nil {
		if m.err != "" {
			return emptyState("Não foi possível carregar o board", "verifique a API e pressione r para tentar novamente")
		}
		if len(m.projects) == 0 {
			return emptyState("Nenhum projeto ainda", "pressione N para criar o primeiro projeto")
		}
		return emptyState("Board indisponível", "pressione r para tentar sincronizar novamente")
	}
	if len(m.board.Columns) == 0 {
		return emptyState("Este projeto não tem colunas", "crie colunas pela API ou MCP")
	}

	metrics := m.boardMetrics()
	maxCount := 0
	for _, col := range m.board.Columns {
		if n := len(m.filteredTasks(col.ID)); n > maxCount {
			maxCount = n
		}
	}

	end := min(len(m.board.Columns), m.colStart+metrics.visibleCols)
	if len(m.board.Columns) > metrics.visibleCols {
		metrics.height = max(4, metrics.height-1)
	}
	columns := []string{}
	for ci := m.colStart; ci < end; ci++ {
		if ci > m.colStart {
			columns = append(columns, " ")
		}
		columns = append(columns, m.renderColumn(ci, metrics, maxCount))
	}
	board := lipgloss.JoinHorizontal(lipgloss.Top, columns...)
	if len(m.board.Columns) > metrics.visibleCols {
		indicator := faintStyle.Render(fmt.Sprintf("colunas %d–%d de %d · h/l navega", m.colStart+1, end, len(m.board.Columns)))
		board += "\n" + indicator
	}
	return board
}

func emptyState(title, hint string) string {
	return "\n  " + sectionTitle.Render(title) + "\n  " + mutedStyle.Render(hint)
}

func (m Model) renderColumn(index int, metrics boardMetrics, maxCount int) string {
	col := m.board.Columns[index]
	tasks := m.filteredTasks(col.ID)
	cursor := m.cursors[col.ID]
	scroll := clamp(m.colScroll[col.ID], 0, max(0, len(tasks)-metrics.visibleCards))
	end := min(len(tasks), scroll+metrics.visibleCards)

	head := columnHeadStyle.Render(truncate(col.Name, metrics.colWidth-10))
	count := fmt.Sprintf("%d", len(tasks))
	if col.WIPLimit > 0 {
		count += "/" + fmt.Sprint(col.WIPLimit)
	}
	head += " " + countStyle.Render(count)
	if col.IsDone {
		head += " " + doneStyle.Render("done")
	}

	pulse := pulseMeter(len(tasks), col.WIPLimit, maxCount, min(18, metrics.colWidth-8), index == m.colIdx, col.Color)
	lines := []string{head, pulse}

	if len(tasks) == 0 {
		empty := "sem tasks"
		if index == m.colIdx && m.mode == modeNormal {
			empty = "vazio · n cria task"
		}
		lines = append(lines, "", faintStyle.Render(empty))
		if m.search != "" {
			lines = append(lines, faintStyle.Render("neste filtro"))
		}
	} else {
		if scroll > 0 {
			lines = append(lines, faintStyle.Render(fmt.Sprintf("↑ %d acima", scroll)))
		}
		for i := scroll; i < end; i++ {
			selected := index == m.colIdx && i == cursor && m.mode == modeNormal
			lines = append(lines, m.card(tasks[i], selected, metrics.colWidth-4))
		}
		if end < len(tasks) {
			lines = append(lines, faintStyle.Render(fmt.Sprintf("↓ %d abaixo", len(tasks)-end)))
		}
	}

	style := columnStyle.Width(metrics.colWidth)
	if index == m.colIdx && m.mode == modeNormal {
		style = columnActiveStyle.Width(metrics.colWidth)
	}
	if metrics.height > 2 {
		style = style.Height(metrics.height - 2)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func pulseMeter(count, limit, maxCount, width int, active bool, laneColor string) string {
	if width < 8 {
		width = 8
	}
	ratio := 0.0
	if limit > 0 {
		ratio = float64(count) / float64(limit)
	} else if maxCount > 0 {
		ratio = float64(count) / float64(max(6, maxCount))
	}
	fill := 0
	if count > 0 {
		fill = clamp(int(math.Round(ratio*float64(width))), 2, width)
	}
	wave := []rune("▁▂▃▄▅▄▃▂")
	out := make([]rune, width)
	for i := range out {
		if i < fill {
			out[i] = wave[i%len(wave)]
		} else {
			out[i] = '╌'
		}
	}
	color := faint
	if active {
		color = accentC
	}
	if limit > 0 && count > limit {
		color = redC
	} else if count > 0 && !active {
		color = colorValue(laneColor, muted)
	}
	return lipgloss.NewStyle().Foreground(color).Render(string(out))
}

func (m Model) card(t model.Task, selected bool, width int) string {
	if width < 18 {
		width = 18
	}
	prio := priorityStyle(string(t.Priority)).Render(priorityLabel(string(t.Priority)))
	ref := cardRefStyle.Render(t.Ref)
	head := ref + "  " + prio
	if t.Estimate > 0 {
		head += "  " + cardMetaStyle.Render(fmt.Sprintf("%dp", t.Estimate))
	}

	title := cardTitleStyle.Render(truncate(t.Title, width-5))
	labels := []string{}
	for i, l := range t.Labels {
		if i >= 2 {
			labels = append(labels, faintStyle.Render(fmt.Sprintf("+%d", len(t.Labels)-2)))
			break
		}
		labels = append(labels, labelStyle(l.Color).Render("#"+truncate(l.Name, 8)))
	}

	meta := []string{}
	if t.AssigneeID != nil {
		if name := m.memberName(*t.AssigneeID); name != "" {
			meta = append(meta, cardMetaStyle.Render(initials(name)))
		}
	}
	if len(t.Subtasks) > 0 {
		done := 0
		for _, s := range t.Subtasks {
			if s.Done {
				done++
			}
		}
		meta = append(meta, progressBar(done, len(t.Subtasks), 4)+cardMetaStyle.Render(fmt.Sprintf(" %d/%d", done, len(t.Subtasks))))
	}
	if t.DueDate != nil {
		due := dueLabel(*t.DueDate)
		style := cardMetaStyle
		if time.Now().After(*t.DueDate) && !t.Done() {
			style = lipgloss.NewStyle().Foreground(redC)
		}
		meta = append(meta, style.Render(due))
	}

	lines := []string{head, title}
	if len(labels) > 0 {
		lines = append(lines, strings.Join(labels, " "))
	}
	if len(meta) > 0 {
		lines = append(lines, strings.Join(meta, " "))
	}
	rail := priorityStyle(string(t.Priority)).Render("▌")
	for i := range lines {
		lines[i] = rail + " " + lines[i]
	}
	style := cardStyle.Width(width)
	if selected {
		style = cardSelectedStyle.Width(width)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) memberName(id string) string {
	if m.board == nil {
		return ""
	}
	for _, mem := range m.board.Members {
		if mem.ID == id {
			return mem.Name
		}
	}
	return ""
}
