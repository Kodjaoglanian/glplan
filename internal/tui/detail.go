package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"glplan/internal/model"
)

func (m Model) detailView() string {
	if m.detail == nil {
		return mutedStyle.Render("carregando task...")
	}
	t := *m.detail
	width := min(76, max(34, m.width-16))
	prio := priorityStyle(string(t.Priority)).Render(priorityLabel(string(t.Priority)))
	column := lipgloss.NewStyle().Foreground(accentC).Render(m.colName(t.ColumnID))
	primaryMeta := []string{
		mutedStyle.Render("coluna ") + column,
		mutedStyle.Render("prioridade ") + prio,
	}
	if t.Estimate > 0 {
		primaryMeta = append(primaryMeta, mutedStyle.Render("estimativa ")+sectionTitle.Render(fmt.Sprintf("%dp", t.Estimate)))
	}
	secondaryMeta := []string{}
	if t.AssigneeID != nil {
		if name := m.memberName(*t.AssigneeID); name != "" {
			secondaryMeta = append(secondaryMeta, mutedStyle.Render("responsável ")+sectionTitle.Render(name))
		}
	}
	if t.DueDate != nil {
		secondaryMeta = append(secondaryMeta, mutedStyle.Render("prazo ")+sectionTitle.Render(dueLabel(*t.DueDate)))
	}

	lines := []string{
		cardRefStyle.Render(t.Ref) + "  " + sectionTitle.Render(t.Title),
		strings.Join(primaryMeta, mutedStyle.Render("   ·   ")),
	}
	if len(secondaryMeta) > 0 {
		lines = append(lines, strings.Join(secondaryMeta, mutedStyle.Render("   ·   ")))
	}
	if len(t.Labels) > 0 {
		labels := []string{}
		for _, l := range t.Labels {
			labels = append(labels, labelStyle(l.Color).Render("#"+l.Name))
		}
		lines = append(lines, "", mutedStyle.Render("labels  ")+strings.Join(labels, " "))
	}
	if strings.TrimSpace(t.Description) != "" {
		lines = append(lines, "", sectionTitle.Render("Descrição"))
		for _, line := range wrap(t.Description, width) {
			lines = append(lines, mutedStyle.Render(line))
		}
	}

	done := 0
	for _, s := range t.Subtasks {
		if s.Done {
			done++
		}
	}
	lines = append(lines, "", sectionTitle.Render("Checklist"))
	if len(t.Subtasks) > 0 {
		lines = append(lines, progressBar(done, len(t.Subtasks), 16)+mutedStyle.Render(fmt.Sprintf("  %d/%d concluídos", done, len(t.Subtasks))))
	}
	if len(t.Subtasks) == 0 {
		lines = append(lines, faintStyle.Render("  vazio · t adiciona o primeiro item"))
	}
	for i, s := range t.Subtasks {
		mark := "○"
		title := s.Title
		if s.Done {
			mark = "●"
			title = faintStyle.Render(title)
		}
		cursor := "  "
		if i == m.subIdx {
			cursor = lipgloss.NewStyle().Foreground(accentC).Render("› ")
		}
		markStyle := mutedStyle
		if s.Done {
			markStyle = doneStyle
		}
		lines = append(lines, cursor+markStyle.Render(mark)+" "+title)
	}

	lines = append(lines, "", sectionTitle.Render("Comentários"))
	if len(m.comments) == 0 {
		lines = append(lines, faintStyle.Render("  nenhum · c escreve o primeiro"))
	}
	start := max(0, len(m.comments)-4)
	for _, c := range m.comments[start:] {
		author := c.Author
		if author == "" {
			author = "você"
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(accentC).Render(author)+faintStyle.Render(" · "+c.CreatedAt.Format("02/01 15:04")))
		for _, line := range wrap(c.Body, width-2) {
			lines = append(lines, mutedStyle.Render("  "+line))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) boardLabels() []model.Label {
	if m.board == nil {
		return nil
	}
	return m.board.Labels
}

func (m Model) labelPickerView() string {
	labels := m.boardLabels()
	lines := []string{sectionTitle.Render("Labels da task"), ""}
	for i, label := range labels {
		cursor := "  "
		if i == m.labelIdx {
			cursor = lipgloss.NewStyle().Foreground(accentC).Render("› ")
		}
		mark := faintStyle.Render("○")
		if m.labelSelected[label.ID] {
			mark = doneStyle.Render("●")
		}
		lines = append(lines, cursor+mark+" "+labelStyle(label.Color).Render("#"+label.Name))
	}
	if len(labels) == 0 {
		lines = append(lines, faintStyle.Render("nenhuma label cadastrada"))
	}
	lines = append(lines, "", mutedStyle.Render("espaço alterna · enter salva · esc cancela"))
	return strings.Join(lines, "\n")
}

func (m Model) colName(id string) string {
	if m.board == nil {
		return id
	}
	for _, c := range m.board.Columns {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}

func (m Model) taskFormView() string {
	title := "Nova task"
	if m.editingID != "" {
		title = "Editar task"
	}
	col := "—"
	if m.board != nil && m.formCol >= 0 && m.formCol < len(m.board.Columns) {
		col = m.board.Columns[m.formCol].Name
	}
	prio := string(model.Priorities[m.formPrio])
	if prio == "none" {
		prio = "sem prioridade"
	} else {
		prio = strings.ToUpper(prio)
	}
	estimate := fmt.Sprint(estimateValues[m.formEstimate])
	if estimate == "0" {
		estimate = "sem estimativa"
	}

	lines := []string{
		sectionTitle.Render(title),
		"",
		m.formLabel(0, "Título"),
		m.titleIn.View(),
		"",
		m.formLabel(1, "Descrição"),
		m.descIn.View(),
		"",
		m.formLabel(2, "Coluna"),
		m.choiceLine(2, col),
		"",
		m.formLabel(3, "Prioridade"),
		m.choiceLine(3, prio),
		"",
		m.formLabel(4, "Estimativa"),
		m.choiceLine(4, estimate),
		"",
		mutedStyle.Render("tab campo · ←/→ escolha · enter/ctrl+s salva · esc cancela"),
	}
	return strings.Join(lines, "\n")
}

func (m Model) formLabel(index int, label string) string {
	if m.formField == index {
		return lipgloss.NewStyle().Foreground(accentC).Bold(true).Render("▸ " + label)
	}
	return faintStyle.Render("  " + label)
}

func (m Model) choiceLine(index int, value string) string {
	line := "‹ " + value + " ›"
	if m.formField == index {
		return lipgloss.NewStyle().Foreground(accentC).Bold(true).Render(line)
	}
	return sectionTitle.Render(line)
}

func (m Model) projectView() string {
	lines := []string{sectionTitle.Render("Projetos"), ""}
	for i, p := range m.projects {
		cursor := "  "
		if i == m.projectPick {
			cursor = lipgloss.NewStyle().Foreground(accentC).Render("› ")
		}
		key := lipgloss.NewStyle().Foreground(accentC).Render(fit(p.Key, 6))
		line := cursor + key + " " + p.Name
		if p.Description != "" {
			line += faintStyle.Render("  ·  " + truncate(p.Description, 24))
		}
		lines = append(lines, line)
	}
	if len(m.projects) == 0 {
		lines = append(lines, faintStyle.Render("nenhum projeto ainda"))
	}
	lines = append(lines, "", mutedStyle.Render("enter abre · n cria projeto · esc fecha"))
	return strings.Join(lines, "\n")
}

func (m Model) projectFormView() string {
	lines := []string{
		sectionTitle.Render("Novo projeto"),
		"",
		m.projectLabel(0, "Nome"),
		m.projectNameIn.View(),
		"",
		m.projectLabel(1, "Key"),
		m.projectKeyIn.View(),
		"",
		m.projectLabel(2, "Descrição"),
		m.projectDescIn.View(),
		"",
		mutedStyle.Render("ex: Produto / PROD · enter/ctrl+s cria · esc cancela"),
	}
	return strings.Join(lines, "\n")
}

func (m Model) projectLabel(index int, label string) string {
	if m.projectField == index {
		return lipgloss.NewStyle().Foreground(accentC).Bold(true).Render("▸ " + label)
	}
	return faintStyle.Render("  " + label)
}

func (m Model) helpView() string {
	lines := []string{sectionTitle.Render("glplan — comandos"), ""}
	sections := []struct {
		name    string
		entries [][2]string
	}{
		{"Navegação", [][2]string{
			{"j / k", "move seleção ou rola atividade"},
			{"h / l", "muda de coluna no board"},
			{"g / G", "primeira / última task"},
			{"tab / 1 2 3", "alterna Board, List e Activity"},
		}},
		{"Operar board", [][2]string{
			{"enter", "abre detalhes"},
			{"n", "nova task"},
			{"e", "edita task selecionada"},
			{"J / K", "reordena dentro da coluna"},
			{"> / <", "move entre colunas"},
			{"/", "filtra por texto · esc limpa"},
		}},
		{"Projetos", [][2]string{
			{"p", "troca de projeto"},
			{"N", "cria novo projeto"},
		}},
		{"Detalhe", [][2]string{
			{"x", "marca/desmarca subtask"},
			{"t", "adiciona subtask"},
			{"c", "comenta"},
			{"L", "edita labels"},
			{"d", "cicla prazo: amanhã, 7 dias, limpar"},
			{"p", "cicla prioridade"},
			{"a", "cicla responsável"},
			{"del", "exclui task"},
		}},
		{"Sistema", [][2]string{
			{"r", "sincroniza agora"},
			{"?", "abre esta ajuda"},
			{"q", "sai"},
		}},
	}
	for _, section := range sections {
		lines = append(lines, sectionTitle.Render(section.name))
		for _, entry := range section.entries {
			lines = append(lines, helpKey.Render(entry[0])+helpText.Render(entry[1]))
		}
		lines = append(lines, "")
	}
	lines = append(lines, mutedStyle.Render("esc fecha"))
	return strings.Join(lines, "\n")
}
