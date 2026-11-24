package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"glplan/internal/api"
	"glplan/internal/model"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	switch m.mode {
	case modeTaskForm:
		return m.updateTaskForm(msg)
	case modeProjectForm:
		return m.updateProjectForm(msg)
	case modeSearch:
		return m.updateSearch(msg)
	case modeProjects:
		return m.updateProjects(msg)
	case modeHelp:
		if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" {
			m.mode = modeNormal
		}
		return m, nil
	case modeConfirm:
		return m.updateConfirm(msg)
	case modeComment:
		return m.updateComment(msg)
	case modeSubtask:
		return m.updateSubtaskInput(msg)
	case modeLabels:
		return m.updateLabels(msg)
	case modeDetail:
		return m.updateDetail(msg)
	}

	return m.updateNormal(msg)
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "p":
		m.projectPick = m.projectIdx
		m.mode = modeProjects
	case "N":
		return m.openProjectForm()
	case "/":
		m.mode = modeSearch
		m.searchIn.SetValue(m.search)
		m.searchIn.Focus()
		return m, m.searchIn.Focus()
	case "r":
		m.loading = true
		m.status = ""
		return m, m.refresh()
	case "1":
		m.view = viewBoard
	case "2":
		m.view = viewList
		m.ensureListVisible()
	case "3":
		m.view = viewActivity
		return m, m.loadActivity(m.currentProjectID())
	case "tab":
		m.view = (m.view + 1) % 3
		if m.view == viewActivity {
			return m, m.loadActivity(m.currentProjectID())
		}
	case "n":
		return m.openNewForm()
	case "e":
		return m.openEditForm()
	case "h", "left":
		if m.view == viewBoard && m.colIdx > 0 {
			m.colIdx--
			m.ensureColumnVisible()
			m.ensureCursorVisible()
		}
	case "l", "right":
		if m.view == viewBoard && m.board != nil && m.colIdx < len(m.board.Columns)-1 {
			m.colIdx++
			m.ensureColumnVisible()
			m.ensureCursorVisible()
		}
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "J":
		return m.reorderSelected(1)
	case "K":
		return m.reorderSelected(-1)
	case "g":
		m.setCursor(0)
	case "G":
		m.setCursor(1 << 30)
	case "enter":
		return m.openSelected()
	case ">", "]":
		return m.quickMove(1)
	case "<", "[":
		return m.quickMove(-1)
	case "esc":
		if m.search != "" {
			m.search = ""
			m.clampCursors()
		}
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.searchIn.Blur()
		return m, nil
	case "enter":
		m.search = strings.TrimSpace(m.searchIn.Value())
		m.mode = modeNormal
		m.searchIn.Blur()
		m.clampCursors()
		return m, nil
	case "ctrl+u":
		m.searchIn.SetValue("")
		return m, nil
	default:
		var cmd tea.Cmd
		m.searchIn, cmd = m.searchIn.Update(msg)
		return m, cmd
	}
}

func (m Model) updateProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
	case "j", "down":
		if m.projectPick < len(m.projects)-1 {
			m.projectPick++
		}
	case "k", "up":
		if m.projectPick > 0 {
			m.projectPick--
		}
	case "n":
		return m.openProjectForm()
	case "enter":
		if m.projectPick >= 0 && m.projectPick < len(m.projects) {
			m.projectIdx = m.projectPick
			m.mode = modeNormal
			m.loading = true
			m.detail = nil
			m.comments = nil
			m.search = ""
			m.colIdx, m.colStart = 0, 0
			m.cursors = map[string]int{}
			m.colScroll = map[string]int{}
			m.listIdx, m.listStart = 0, 0
			return m, m.refresh()
		}
	}
	return m, nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		if m.detail != nil {
			id := m.detail.ID
			m.mode = modeNormal
			m.detail = nil
			m.comments = nil
			return m, func() tea.Msg {
				if err := m.client.DeleteTask(id); err != nil {
					return errMsg(err)
				}
				return savedMsg("task excluída")
			}
		}
		m.mode = modeNormal
	case "n", "esc":
		if m.detail != nil {
			m.mode = modeDetail
		} else {
			m.mode = modeNormal
		}
	}
	return m, nil
}

func (m Model) updateComment(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDetail
		m.commentIn.Blur()
	case "enter":
		body := strings.TrimSpace(m.commentIn.Value())
		id := ""
		if m.detail != nil {
			id = m.detail.ID
		}
		m.commentIn.SetValue("")
		m.commentIn.Blur()
		m.mode = modeDetail
		if body != "" && id != "" {
			return m, func() tea.Msg {
				if err := m.client.AddComment(id, "", body); err != nil {
					return errMsg(err)
				}
				return savedMsg("comentário adicionado")
			}
		}
	default:
		var cmd tea.Cmd
		m.commentIn, cmd = m.commentIn.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateSubtaskInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDetail
		m.subIn.Blur()
	case "enter":
		title := strings.TrimSpace(m.subIn.Value())
		id := ""
		if m.detail != nil {
			id = m.detail.ID
		}
		m.subIn.SetValue("")
		m.subIn.Blur()
		m.mode = modeDetail
		if title != "" && id != "" {
			return m, func() tea.Msg {
				if err := m.client.AddSubtask(id, title); err != nil {
					return errMsg(err)
				}
				return savedMsg("subtask adicionada")
			}
		}
	default:
		var cmd tea.Cmd
		m.subIn, cmd = m.subIn.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || m.view == viewActivity {
		return m, nil
	}
	m.mode = modeDetail
	m.subIdx = 0
	return m, m.loadTask(t.ID)
}

func (m Model) quickMove(delta int) (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || m.board == nil {
		return m, nil
	}
	current := -1
	for i, c := range m.board.Columns {
		if c.ID == t.ColumnID {
			current = i
			break
		}
	}
	next := current + delta
	if current < 0 || next < 0 || next >= len(m.board.Columns) {
		return m, nil
	}
	newCol := m.board.Columns[next]
	m.colIdx = next
	m.ensureColumnVisible()
	id := t.ID
	return m, func() tea.Msg {
		if _, err := m.client.MoveTask(id, newCol.ID, "", nil); err != nil {
			return errMsg(err)
		}
		return savedMsg("movido para " + newCol.Name)
	}
}

func (m Model) reorderSelected(delta int) (tea.Model, tea.Cmd) {
	if m.view != viewBoard || m.search != "" {
		return m, nil
	}
	col := m.currentColumn()
	if col == nil {
		return m, nil
	}
	tasks := m.filteredTasks(col.ID)
	idx := m.cursors[col.ID]
	pos, ok := reorderPosition(tasks, idx, delta)
	if !ok {
		return m, nil
	}
	t := tasks[idx]
	m.cursors[col.ID] = idx + delta
	m.ensureCursorVisible()
	return m, func() tea.Msg {
		if _, err := m.client.MoveTask(t.ID, col.ID, "", &pos); err != nil {
			return errMsg(err)
		}
		return savedMsg(t.Ref + " reordenada")
	}
}

func (m Model) openNewForm() (tea.Model, tea.Cmd) {
	if m.board == nil || len(m.board.Columns) == 0 {
		m.err = "carregue um board antes de criar tasks"
		return m, nil
	}
	m.mode = modeTaskForm
	m.editingID = ""
	m.formReturnDetail = false
	m.formField = 0
	m.formCol = clamp(m.colIdx, 0, len(m.board.Columns)-1)
	m.formPrio = 0
	m.formEstimate = 0
	m.titleIn.SetValue("")
	m.descIn.SetValue("")
	return m, m.focusTaskForm()
}

func (m Model) openEditForm() (tea.Model, tea.Cmd) {
	t := m.detail
	if t == nil {
		t = m.selectedTask()
	}
	if t == nil || m.board == nil {
		return m, nil
	}
	m.mode = modeTaskForm
	m.editingID = t.ID
	m.formReturnDetail = m.detail != nil
	m.formField = 0
	m.formCol = 0
	for i, col := range m.board.Columns {
		if col.ID == t.ColumnID {
			m.formCol = i
			break
		}
	}
	m.formPrio = 0
	for i, p := range model.Priorities {
		if p == t.Priority {
			m.formPrio = i
			break
		}
	}
	m.formEstimate = 0
	for i, v := range estimateValues {
		if v == t.Estimate {
			m.formEstimate = i
			break
		}
	}
	m.titleIn.SetValue(t.Title)
	m.descIn.SetValue(t.Description)
	return m, m.focusTaskForm()
}

func (m *Model) focusTaskForm() tea.Cmd {
	m.titleIn.Blur()
	m.descIn.Blur()
	switch m.formField {
	case 0:
		m.titleIn.Focus()
		return m.titleIn.Focus()
	case 1:
		m.descIn.Focus()
		return m.descIn.Focus()
	}
	return nil
}

func (m Model) updateTaskForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fields := 5
	switch msg.String() {
	case "esc":
		if m.formReturnDetail {
			m.mode = modeDetail
		} else {
			m.mode = modeNormal
		}
		m.titleIn.Blur()
		m.descIn.Blur()
		return m, nil
	case "ctrl+s":
		return m.submitTaskForm()
	case "tab":
		m.formField = (m.formField + 1) % fields
		return m, m.focusTaskForm()
	case "shift+tab":
		m.formField = (m.formField + fields - 1) % fields
		return m, m.focusTaskForm()
	case "enter":
		if m.formField == 0 {
			m.formField = 1
			return m, m.focusTaskForm()
		}
		if m.formField >= 2 {
			return m.submitTaskForm()
		}
	case "h", "left":
		if m.formField >= 2 {
			m.adjustTaskFormChoice(-1)
			return m, nil
		}
	case "l", "right":
		if m.formField >= 2 {
			m.adjustTaskFormChoice(1)
			return m, nil
		}
	}
	if m.formField == 0 {
		var cmd tea.Cmd
		m.titleIn, cmd = m.titleIn.Update(msg)
		return m, cmd
	}
	if m.formField == 1 {
		var cmd tea.Cmd
		m.descIn, cmd = m.descIn.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) adjustTaskFormChoice(delta int) {
	if m.board == nil {
		return
	}
	switch m.formField {
	case 2:
		m.formCol = clamp(m.formCol+delta, 0, max(0, len(m.board.Columns)-1))
	case 3:
		m.formPrio = clamp(m.formPrio+delta, 0, len(model.Priorities)-1)
	case 4:
		m.formEstimate = clamp(m.formEstimate+delta, 0, len(estimateValues)-1)
	}
}

func (m Model) submitTaskForm() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.titleIn.Value())
	if title == "" {
		m.err = "título é obrigatório"
		return m, nil
	}
	colID := ""
	if m.board != nil && m.formCol >= 0 && m.formCol < len(m.board.Columns) {
		colID = m.board.Columns[m.formCol].ID
	}
	priority := model.Priorities[m.formPrio]
	estimate := estimateValues[m.formEstimate]
	description := m.descIn.Value()
	projectID := m.currentProjectID()
	editingID := m.editingID
	returnDetail := m.formReturnDetail
	m.titleIn.Blur()
	m.descIn.Blur()
	if returnDetail {
		m.mode = modeDetail
	} else {
		m.mode = modeNormal
	}
	m.err = ""
	if editingID == "" {
		return m, func() tea.Msg {
			t, err := m.client.CreateTask(api.CreateTaskInput{
				ProjectID: projectID, ColumnID: colID, Title: title, Description: description,
				Priority: priority, Estimate: estimate,
			})
			if err != nil {
				return errMsg(err)
			}
			return savedMsg("criada " + t.Ref)
		}
	}
	return m, func() tea.Msg {
		t, err := m.client.UpdateTask(editingID, map[string]any{
			"title": title, "description": description, "column_id": colID,
			"priority": string(priority), "estimate": estimate,
		})
		if err != nil {
			return errMsg(err)
		}
		return savedMsg("atualizada " + t.Ref)
	}
}

func (m Model) openProjectForm() (tea.Model, tea.Cmd) {
	m.mode = modeProjectForm
	m.projectField = 0
	m.projectNameIn.SetValue("")
	m.projectKeyIn.SetValue("")
	m.projectDescIn.SetValue("")
	return m, m.focusProjectForm()
}

func (m *Model) focusProjectForm() tea.Cmd {
	m.projectNameIn.Blur()
	m.projectKeyIn.Blur()
	m.projectDescIn.Blur()
	switch m.projectField {
	case 0:
		m.projectNameIn.Focus()
		return m.projectNameIn.Focus()
	case 1:
		m.projectKeyIn.Focus()
		return m.projectKeyIn.Focus()
	case 2:
		m.projectDescIn.Focus()
		return m.projectDescIn.Focus()
	}
	return nil
}

func (m Model) updateProjectForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "ctrl+s":
		return m.submitProjectForm()
	case "tab":
		m.projectField = (m.projectField + 1) % 3
		return m, m.focusProjectForm()
	case "shift+tab":
		m.projectField = (m.projectField + 2) % 3
		return m, m.focusProjectForm()
	case "enter":
		if m.projectField < 2 {
			m.projectField++
			return m, m.focusProjectForm()
		}
		return m.submitProjectForm()
	}
	var cmd tea.Cmd
	switch m.projectField {
	case 0:
		m.projectNameIn, cmd = m.projectNameIn.Update(msg)
	case 1:
		m.projectKeyIn, cmd = m.projectKeyIn.Update(msg)
	case 2:
		m.projectDescIn, cmd = m.projectDescIn.Update(msg)
	}
	return m, cmd
}

func (m Model) submitProjectForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.projectNameIn.Value())
	if name == "" {
		m.err = "nome do projeto é obrigatório"
		return m, nil
	}
	key := strings.ToUpper(strings.TrimSpace(m.projectKeyIn.Value()))
	if key == "" {
		key = projectKeyFromName(name)
	}
	desc := strings.TrimSpace(m.projectDescIn.Value())
	m.mode = modeNormal
	m.err = ""
	return m, func() tea.Msg {
		project, err := m.client.CreateProject(name, key, desc)
		if err != nil {
			return errMsg(err)
		}
		return projectCreatedMsg(project)
	}
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.detail == nil {
		m.mode = modeNormal
		return m, nil
	}
	t := *m.detail
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.detail = nil
		m.comments = nil
		return m, nil
	case "j", "down":
		if m.subIdx < len(t.Subtasks)-1 {
			m.subIdx++
		}
	case "k", "up":
		if m.subIdx > 0 {
			m.subIdx--
		}
	case "x", " ":
		if m.subIdx < len(t.Subtasks) {
			sub := t.Subtasks[m.subIdx]
			done := !sub.Done
			id := sub.ID
			return m, func() tea.Msg {
				if err := m.client.UpdateSubtask(id, map[string]any{"done": done}); err != nil {
					return errMsg(err)
				}
				return savedMsg("checklist atualizado")
			}
		}
	case "t":
		m.mode = modeSubtask
		m.subIn.Focus()
		return m, m.subIn.Focus()
	case "c":
		m.mode = modeComment
		m.commentIn.Focus()
		return m, m.commentIn.Focus()
	case "e":
		return m.openEditForm()
	case "p":
		idx := 0
		for i, p := range model.Priorities {
			if p == t.Priority {
				idx = i
			}
		}
		next := model.Priorities[(idx+1)%len(model.Priorities)]
		id := t.ID
		return m, func() tea.Msg {
			if _, err := m.client.UpdateTask(id, map[string]any{"priority": string(next)}); err != nil {
				return errMsg(err)
			}
			return savedMsg("prioridade: " + string(next))
		}
	case "a":
		return m.cycleAssignee(t)
	case "d":
		return m.cycleDueDate(t)
	case "L":
		return m.openLabelPicker(t)
	case ">", "]":
		return m.moveDetail(t, 1)
	case "<", "[":
		return m.moveDetail(t, -1)
	case "delete", "backspace":
		m.mode = modeConfirm
		return m, nil
	}
	return m, nil
}

func (m Model) openLabelPicker(t model.Task) (tea.Model, tea.Cmd) {
	if m.board == nil || len(m.board.Labels) == 0 {
		m.err = "este projeto não tem labels"
		return m, nil
	}
	m.mode = modeLabels
	m.labelTaskID = t.ID
	m.labelIdx = 0
	m.labelSelected = map[string]bool{}
	for _, label := range t.Labels {
		m.labelSelected[label.ID] = true
	}
	return m, nil
}

func (m Model) updateLabels(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.board == nil || len(m.board.Labels) == 0 {
		m.mode = modeDetail
		return m, nil
	}
	switch msg.String() {
	case "esc", "q":
		m.mode = modeDetail
	case "j", "down":
		if m.labelIdx < len(m.board.Labels)-1 {
			m.labelIdx++
		}
	case "k", "up":
		if m.labelIdx > 0 {
			m.labelIdx--
		}
	case "x", " ":
		label := m.board.Labels[m.labelIdx]
		m.labelSelected[label.ID] = !m.labelSelected[label.ID]
	case "enter":
		ids := []string{}
		for _, label := range m.board.Labels {
			if m.labelSelected[label.ID] {
				ids = append(ids, label.ID)
			}
		}
		id := m.labelTaskID
		m.mode = modeDetail
		return m, func() tea.Msg {
			if _, err := m.client.UpdateTask(id, map[string]any{"label_ids": ids}); err != nil {
				return errMsg(err)
			}
			return savedMsg("labels atualizadas")
		}
	}
	return m, nil
}

func (m Model) cycleDueDate(t model.Task) (tea.Model, tea.Cmd) {
	var next *time.Time
	label := "sem prazo"
	switch {
	case t.DueDate == nil || daysUntil(*t.DueDate) <= 0:
		due := time.Now().AddDate(0, 0, 1)
		next = &due
		label = "amanhã"
	case daysUntil(*t.DueDate) < 7:
		due := time.Now().AddDate(0, 0, 7)
		next = &due
		label = "em 7 dias"
	}
	var value any
	if next != nil {
		value = next.Format(time.RFC3339)
	}
	id := t.ID
	return m, func() tea.Msg {
		if _, err := m.client.UpdateTask(id, map[string]any{"due_date": value}); err != nil {
			return errMsg(err)
		}
		return savedMsg("prazo: " + label)
	}
}

func (m Model) cycleAssignee(t model.Task) (tea.Model, tea.Cmd) {
	members := []string{}
	if m.board != nil {
		for _, mem := range m.board.Members {
			if mem.Active {
				members = append(members, mem.ID)
			}
		}
	}
	if len(members) == 0 {
		m.err = "sem membros ativos cadastrados"
		return m, nil
	}
	idx := -1
	if t.AssigneeID != nil {
		for i, id := range members {
			if id == *t.AssigneeID {
				idx = i
			}
		}
	}
	var assignee any
	if idx+1 < len(members) {
		assignee = members[idx+1]
	}
	id := t.ID
	return m, func() tea.Msg {
		if _, err := m.client.UpdateTask(id, map[string]any{"assignee_id": assignee}); err != nil {
			return errMsg(err)
		}
		return savedMsg("responsável atualizado")
	}
}

func (m Model) moveDetail(t model.Task, delta int) (tea.Model, tea.Cmd) {
	if m.board == nil {
		return m, nil
	}
	idx := -1
	for i, c := range m.board.Columns {
		if c.ID == t.ColumnID {
			idx = i
		}
	}
	next := idx + delta
	if idx < 0 || next < 0 || next >= len(m.board.Columns) {
		return m, nil
	}
	col := m.board.Columns[next]
	id := t.ID
	return m, func() tea.Msg {
		if _, err := m.client.MoveTask(id, col.ID, "", nil); err != nil {
			return errMsg(err)
		}
		return savedMsg("movido para " + col.Name)
	}
}
