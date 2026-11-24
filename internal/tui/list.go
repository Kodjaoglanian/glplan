package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) listPageSize() int {
	return max(3, m.height-9)
}

func (m Model) activityPageSize() int {
	return max(3, m.height-8)
}

func (m Model) listView() string {
	tasks := m.filteredTasks("")
	if len(tasks) == 0 {
		if m.search != "" {
			return emptyState("Nada encontrado", "esc limpa o filtro atual")
		}
		return emptyState("Nenhuma task", "pressione n para criar a primeira")
	}

	page := m.listPageSize()
	start := clamp(m.listStart, 0, max(0, len(tasks)-page))
	end := min(len(tasks), start+page)

	refW := 9
	colW := 14
	prioW := 7
	assigneeW := 12
	dueW := 10
	fixed := refW + colW + prioW + assigneeW + dueW + 5
	titleW := max(16, m.width-fixed-4)

	header := listHeader.Render(fit("REF", refW) + " " + fit("TITLE", titleW) + " " + fit("COLUMN", colW) + " " + fit("PRIO", prioW) + " " + fit("ASSIGNEE", assigneeW) + " " + fit("DUE", dueW))
	rows := []string{header, faintStyle.Render(strings.Repeat("─", min(m.width-4, fixed+titleW)))}
	for i := start; i < end; i++ {
		t := tasks[i]
		assignee := "—"
		if t.AssigneeID != nil {
			if name := m.memberName(*t.AssigneeID); name != "" {
				assignee = name
			}
		}
		due := "—"
		if t.DueDate != nil {
			due = dueLabel(*t.DueDate)
		}
		prioCell := lipgloss.NewStyle().Foreground(priorityColor(string(t.Priority))).Render(fit(priorityLabel(string(t.Priority)), prioW))
		title := t.Title
		if t.Done() {
			title = "✓ " + title
		}
		title = truncate(title, titleW)
		row := fit(t.Ref, refW) + " " + fit(title, titleW) + " " + fit(m.colName(t.ColumnID), colW) + " " +
			prioCell + " " + fit(assignee, assigneeW) + " " + fit(due, dueW)
		if i == m.listIdx && m.mode == modeNormal {
			row = selectedRow.Render(row)
		} else if t.Done() {
			row = faintStyle.Render(row)
		}
		rows = append(rows, row)
	}
	if start > 0 {
		rows = append(rows, faintStyle.Render(fmt.Sprintf("↑ %d acima", start)))
	}
	if end < len(tasks) {
		rows = append(rows, faintStyle.Render(fmt.Sprintf("↓ %d abaixo", len(tasks)-end)))
	}
	return strings.Join(rows, "\n")
}

func (m Model) activityView() string {
	if len(m.activity) == 0 {
		return emptyState("Sem atividade recente", "ações em tasks, comentários e movimentos aparecem aqui")
	}
	page := m.activityPageSize()
	start := clamp(m.activityStart, 0, max(0, len(m.activity)-page))
	end := min(len(m.activity), start+page)
	rows := []string{listHeader.Render(fit("QUANDO", 12) + " " + fit("AÇÃO", 18) + " DETALHE")}
	for _, a := range m.activity[start:end] {
		when := a.CreatedAt.Format("02/01 15:04")
		action := strings.TrimPrefix(a.Action, "task.")
		line := fit(when, 12) + " " + fit(action, 18) + " " + truncate(a.Detail, max(20, m.width-38))
		rows = append(rows, mutedStyle.Render(line))
	}
	if start > 0 {
		rows = append(rows, faintStyle.Render(fmt.Sprintf("↑ %d mais recentes", start)))
	}
	if end < len(m.activity) {
		rows = append(rows, faintStyle.Render(fmt.Sprintf("↓ %d mais antigas", len(m.activity)-end)))
	}
	return strings.Join(rows, "\n")
}
