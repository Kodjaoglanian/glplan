// Package tui implements the Bubble Tea terminal interface.
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"glplan/internal/api"
	"glplan/internal/model"
)

type view int

const (
	viewBoard view = iota
	viewList
	viewActivity
)

type mode int

const (
	modeNormal mode = iota
	modeNewTask
	modeDetail
	modeSearch
	modeProjects
	modeHelp
	modeConfirm
	modeComment
	modeSubtask
)

type projectsMsg []model.Project
type boardMsg struct {
	board *model.Board
	tasks []model.Task
}
type activityMsg []model.Activity
type taskMsg *model.Task
type commentsMsg []model.Comment
type errMsg error
type savedMsg string
type tickMsg time.Time

// Model is the TUI state.
type Model struct {
	client *api.Client

	projects   []model.Project
	projectIdx int
	board      *model.Board
	tasks      []model.Task
	activity   []model.Activity

	view view
	mode mode

	colIdx   int
	colStart int
	cursors  map[string]int
	listIdx  int
	subIdx   int

	search    string
	searchIn  textinput.Model
	titleIn   textinput.Model
	descIn    textarea.Model
	commentIn textinput.Model
	subIn     textinput.Model
	formField int
	formCol   int
	formPrio  int

	detail    *model.Task
	comments  []model.Comment
	status    string
	err       string
	width     int
	height    int
	loading   bool
}

// New creates the TUI model.
func New(client *api.Client) Model {
	search := textinput.New()
	search.Placeholder = "filtrar tasks..."
	title := textinput.New()
	title.Placeholder = "Título da task"
	desc := textarea.New()
	desc.Placeholder = "Descrição (markdown)"
	desc.SetWidth(56)
	desc.SetHeight(6)
	comment := textinput.New()
	comment.Placeholder = "Escreva um comentário..."
	sub := textinput.New()
	sub.Placeholder = "Novo subtask"
	return Model{
		client: client, cursors: map[string]int{},
		searchIn: search, titleIn: title, descIn: desc,
		commentIn: comment, subIn: sub,
		formPrio: 1, loading: true,
	}
}

// Init starts the initial loads and the refresh loop.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadProjects(), m.tick())
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(4*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.client.ListProjects()
		if err != nil {
			return errMsg(err)
		}
		return projectsMsg(projects)
	}
}

func (m Model) loadBoard(projectID string) tea.Cmd {
	return func() tea.Msg {
		board, err := m.client.GetBoard(projectID)
		if err != nil {
			return errMsg(err)
		}
		tasks, err := m.client.ListTasks(model.TaskFilter{ProjectID: projectID, IncludeDone: true})
		if err != nil {
			return errMsg(err)
		}
		return boardMsg{board: board, tasks: tasks}
	}
}

func (m Model) loadActivity(projectID string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.client.ListActivity(projectID, 40)
		if err != nil {
			return errMsg(err)
		}
		return activityMsg(out)
	}
}

func (m Model) currentProjectID() string {
	if len(m.projects) == 0 {
		return ""
	}
	return m.projects[m.projectIdx].ID
}

func (m Model) refresh() tea.Cmd {
	id := m.currentProjectID()
	if id == "" {
		return m.loadProjects()
	}
	return tea.Batch(m.loadBoard(id), m.loadActivity(id))
}

// Update handles messages and key presses.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.descIn.SetWidth(min(56, max(20, m.width-10)))
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		cmds := []tea.Cmd{m.tick()}
		if m.mode == modeNormal && m.currentProjectID() != "" {
			cmds = append(cmds, m.refresh())
		}
		return m, tea.Batch(cmds...)

	case projectsMsg:
		m.projects = msg
		if m.projectIdx >= len(m.projects) {
			m.projectIdx = 0
		}
		m.err = ""
		if len(m.projects) > 0 {
			return m, m.refresh()
		}
		return m, nil

	case boardMsg:
		m.board = msg.board
		m.tasks = msg.tasks
		m.loading = false
		m.err = ""
		m.clampCursors()
		if m.detail != nil {
			if t, err := m.client.GetTask(m.detail.ID); err == nil {
				m.detail = t
			}
		}
		return m, nil

	case activityMsg:
		m.activity = msg
		return m, nil

	case taskMsg:
		if msg != nil {
			m.detail = msg
			m.comments = nil
			return m, func() tea.Msg {
				comments, err := m.client.ListComments(msg.ID)
				if err != nil {
					return errMsg(err)
				}
				return commentsMsg(comments)
			}
		}
		return m, nil

	case commentsMsg:
		m.comments = msg
		return m, nil

	case errMsg:
		m.err = msg.Error()
		m.loading = false
		return m, nil

	case savedMsg:
		m.status = string(msg)
		return m, m.refresh()
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	switch m.mode {
	case modeNewTask:
		return m.updateForm(msg)
	case modeSearch:
		if msg.String() == "esc" {
			m.mode = modeNormal
			m.search = ""
			m.searchIn.Blur()
			return m, nil
		}
		if msg.String() == "enter" {
			m.search = m.searchIn.Value()
			m.mode = modeNormal
			m.searchIn.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.searchIn, cmd = m.searchIn.Update(msg)
		return m, cmd

	case modeProjects:
		switch msg.String() {
		case "esc":
			m.mode = modeNormal
		case "j", "down":
			if m.projectIdx < len(m.projects)-1 {
				m.projectIdx++
			}
		case "k", "up":
			if m.projectIdx > 0 {
				m.projectIdx--
			}
		case "enter":
			m.mode = modeNormal
			m.loading = true
			return m, m.refresh()
		}
		return m, nil

	case modeHelp:
		if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" {
			m.mode = modeNormal
		}
		return m, nil

	case modeConfirm:
		switch msg.String() {
		case "y", "enter":
			if m.detail != nil {
				id := m.detail.ID
				m.mode = modeNormal
				m.detail = nil
				return m, func() tea.Msg {
					if err := m.client.DeleteTask(id); err != nil {
						return errMsg(err)
					}
					return savedMsg("task excluída")
				}
			}
			m.mode = modeNormal
		case "n", "esc":
			m.mode = modeDetail
		}
		return m, nil

	case modeComment:
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
					if err := m.client.AddComment(id, body); err != nil {
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

	case modeSubtask:
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

	case modeDetail:
		return m.updateDetail(msg)
	}

	// normal mode
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "p":
		m.mode = modeProjects
	case "/":
		m.mode = modeSearch
		m.searchIn.SetValue(m.search)
		m.searchIn.Focus()
		return m, m.searchIn.Focus()
	case "r":
		m.loading = true
		return m, m.refresh()
	case "1":
		m.view = viewBoard
	case "2":
		m.view = viewList
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
	case "h", "left":
		if m.colIdx > 0 {
			m.colIdx--
			m.ensureColumnVisible()
		}
	case "l", "right":
		if m.board != nil && m.colIdx < len(m.board.Columns)-1 {
			m.colIdx++
			m.ensureColumnVisible()
		}
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
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
	case "s":
		m.mode = modeSearch
		m.searchIn.SetValue(m.search)
		m.searchIn.Focus()
		return m, m.searchIn.Focus()
	}
	return m, nil
}

func (m Model) filteredTasks(colID string) []model.Task {
	out := []model.Task{}
	for _, t := range m.tasks {
		if colID != "" && t.ColumnID != colID {
			continue
		}
		if m.search != "" {
			q := strings.ToLower(m.search)
			if !strings.Contains(strings.ToLower(t.Title), q) &&
				!strings.Contains(strings.ToLower(t.Description), q) &&
				!strings.Contains(strings.ToLower(t.Ref), q) {
				continue
			}
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out
}

func (m Model) clampCursors() {
	if m.board == nil {
		return
	}
	if m.colIdx >= len(m.board.Columns) {
		m.colIdx = max(0, len(m.board.Columns)-1)
	}
	for k, v := range m.cursors {
		n := len(m.filteredTasks(k))
		if v >= n {
			m.cursors[k] = max(0, n-1)
		}
	}
	if m.view == viewList {
		n := len(m.filteredTasks(""))
		if m.listIdx >= n {
			m.listIdx = max(0, n-1)
		}
	}
	m.ensureColumnVisible()
}

func (m Model) ensureColumnVisible() {
	perCol := 32
	maxCols := max(1, (m.width-4)/perCol)
	if m.colIdx < m.colStart {
		m.colStart = m.colIdx
	}
	if m.colIdx >= m.colStart+maxCols {
		m.colStart = m.colIdx - maxCols + 1
	}
}

func (m Model) cursor() int {
	if m.view == viewList {
		return m.listIdx
	}
	if m.board == nil || len(m.board.Columns) == 0 {
		return 0
	}
	return m.cursors[m.board.Columns[m.colIdx].ID]
}

func (m *Model) setCursor(v int) {
	if m.view == viewList {
		n := len(m.filteredTasks(""))
		m.listIdx = clamp(v, 0, max(0, n-1))
		return
	}
	if m.board == nil || len(m.board.Columns) == 0 {
		return
	}
	id := m.board.Columns[m.colIdx].ID
	n := len(m.filteredTasks(id))
	m.cursors[id] = clamp(v, 0, max(0, n-1))
}

func (m *Model) moveCursor(delta int) {
	cp := *m
	cp.setCursor(m.cursor() + delta)
	*m = cp
}

func (m Model) selectedTask() *model.Task {
	var list []model.Task
	if m.view == viewList {
		list = m.filteredTasks("")
	} else if m.board != nil && len(m.board.Columns) > 0 {
		list = m.filteredTasks(m.board.Columns[m.colIdx].ID)
	}
	i := m.cursor()
	if i < 0 || i >= len(list) {
		return nil
	}
	t := list[i]
	return &t
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	m.mode = modeDetail
	m.subIdx = 0
	id := t.ID
	return m, func() tea.Msg {
		task, err := m.client.GetTask(id)
		if err != nil {
			return errMsg(err)
		}
		return taskMsg(task)
	}
}

func (m Model) quickMove(delta int) (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || m.board == nil {
		return m, nil
	}
	next := m.colIdx + delta
	if next < 0 || next >= len(m.board.Columns) {
		return m, nil
	}
	newCol := m.board.Columns[next]
	id := t.ID
	return m, func() tea.Msg {
		if _, err := m.client.MoveTask(id, newCol.ID, "", nil); err != nil {
			return errMsg(err)
		}
		return savedMsg("movido para " + newCol.Name)
	}
}

func (m Model) openNewForm() (tea.Model, tea.Cmd) {
	m.mode = modeNewTask
	m.formField = 0
	m.formCol = 0
	m.formPrio = 1
	m.titleIn.SetValue("")
	m.descIn.SetValue("")
	m.titleIn.Focus()
	m.descIn.Blur()
	return m, m.titleIn.Focus()
}

func (m Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.titleIn.Blur()
		m.descIn.Blur()
		return m, nil
	case "ctrl+s":
		return m.submitForm()
	case "tab":
		if m.formField == 0 {
			m.titleIn.Blur()
			m.formField = 1
			m.descIn.Focus()
			return m, m.descIn.Focus()
		}
		m.descIn.Blur()
		m.formField = 0
		m.titleIn.Focus()
		return m, m.titleIn.Focus()
	case "h", "left":
		if m.formField == 2 && m.formCol > 0 {
			m.formCol--
		}
		if m.formField == 3 && m.formPrio > 0 {
			m.formPrio--
		}
	case "l", "right":
		if m.formField == 2 && m.board != nil && m.formCol < len(m.board.Columns)-1 {
			m.formCol++
		}
		if m.formField == 3 && m.formPrio < len(model.Priorities)-1 {
			m.formPrio++
		}
	case "ctrl+n":
		m.formField = 2
	case "ctrl+p":
		m.formField = 3
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

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.titleIn.Value())
	if title == "" {
		m.err = "título é obrigatório"
		return m, nil
	}
	colID := ""
	if m.board != nil && m.formCol < len(m.board.Columns) {
		colID = m.board.Columns[m.formCol].ID
	}
	priority := model.Priorities[m.formPrio]
	client, project := m.client, m.currentProjectID()
	m.mode = modeNormal
	m.titleIn.Blur()
	m.descIn.Blur()
	return m, func() tea.Msg {
		t, err := client.CreateTask(project, colID, title, m.descIn.Value(), priority, nil)
		if err != nil {
			return errMsg(err)
		}
		return savedMsg("criada " + t.Ref)
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
				return savedMsg("subtask atualizada")
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
		members := []string{}
		if m.board != nil {
			for _, mem := range m.board.Members {
				members = append(members, mem.ID)
			}
		}
		if len(members) == 0 {
			m.err = "sem membros cadastrados"
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
	case ">", "]":
		if m.board == nil {
			return m, nil
		}
		idx := -1
		for i, c := range m.board.Columns {
			if c.ID == t.ColumnID {
				idx = i
			}
		}
		if idx >= 0 && idx+1 < len(m.board.Columns) {
			col := m.board.Columns[idx+1]
			id := t.ID
			return m, func() tea.Msg {
				if _, err := m.client.MoveTask(id, col.ID, "", nil); err != nil {
					return errMsg(err)
				}
				return savedMsg("movido para " + col.Name)
			}
		}
	case "<", "[":
		if m.board == nil {
			return m, nil
		}
		idx := -1
		for i, c := range m.board.Columns {
			if c.ID == t.ColumnID {
				idx = i
			}
		}
		if idx > 0 {
			col := m.board.Columns[idx-1]
			id := t.ID
			return m, func() tea.Msg {
				if _, err := m.client.MoveTask(id, col.ID, "", nil); err != nil {
					return errMsg(err)
				}
				return savedMsg("movido para " + col.Name)
			}
		}
	case "d":
		if msg.String() == "d" {
			// second 'd' deletes; simple confirmation via modeConfirm below
		}
	case "delete", "backspace":
		m.mode = modeConfirm
		return m, nil
	}
	return m, nil
}

// View renders the interface.
func (m Model) View() string {
	if m.width == 0 {
		return "glplan..."
	}
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.tabs())
	b.WriteString("\n\n")

	switch m.view {
	case viewBoard:
		b.WriteString(m.boardView())
	case viewList:
		b.WriteString(m.listView())
	case viewActivity:
		b.WriteString(m.activityView())
	}
	b.WriteString("\n")
	b.WriteString(m.footer())

	base := b.String()
	switch m.mode {
	case modeNewTask:
		base = m.overlay(base, m.formView(), 66, 22)
	case modeDetail:
		base = m.overlay(base, m.detailView(), min(84, m.width-4), min(28, m.height-4))
	case modeSearch:
		base = m.overlay(base, "Buscar:\n\n"+m.searchIn.View()+"\n\nenter aplica · esc cancela", 54, 7)
	case modeProjects:
		base = m.overlay(base, m.projectView(), 50, min(16, len(m.projects)+6))
	case modeHelp:
		base = m.overlay(base, m.helpView(), 62, 26)
	case modeConfirm:
		base = m.overlay(base, "Excluir esta task? (y/N)", 40, 5)
	}
	return base
}

func (m Model) header() string {
	name := "—"
	if len(m.projects) > 0 {
		name = m.projects[m.projectIdx].Name + "  " + mutedStyle.Render("["+m.projects[m.projectIdx].Key+"]")
	}
	tasks := 0
	for _, t := range m.tasks {
		if !t.Done() {
			tasks++
		}
	}
	status := statusStyle.Render("● online")
	if m.err != "" {
		status = errorStyle.Render("● " + truncate(m.err, 40))
	} else if m.loading {
		status = cardMetaStyle.Render("◌ carregando")
	}
	left := sectionTitle.Render("glplan") + "  " + mutedStyle.Render("/") + "  " + name
	right := status + "  " + mutedStyle.Render(fmt.Sprintf("%d abertas", tasks))
	return lipgloss.JoinHorizontal(lipgloss.Center, headerStyle.Width(max(40, m.width-2)).Render(left), right)
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
	return strings.Join(parts, "")
}

func (m Model) boardView() string {
	if m.board == nil {
		return mutedStyle.Render("carregando board...")
	}
	perCol := 32
	maxCols := max(1, (m.width-4)/perCol)
	lines := []string{}
	end := min(len(m.board.Columns), m.colStart+maxCols)
	for ci := m.colStart; ci < end; ci++ {
		col := m.board.Columns[ci]
		tasks := m.filteredTasks(col.ID)
		style := columnStyle
		if ci == m.colIdx && m.mode == modeNormal {
			style = columnStyle.BorderForeground(accentC)
		}
		head := columnHeadStyle.Render(col.Name) + " " + countStyle.Render(fmt.Sprintf("%d", len(tasks)))
		if col.WIPLimit > 0 {
			head += countStyle.Render(" / " + fmt.Sprint(col.WIPLimit))
		}
		parts := []string{head}
		if len(tasks) == 0 {
			parts = append(parts, mutedStyle.Render("  — vazio —"))
		}
		for i, t := range tasks {
			sel := ci == m.colIdx && i == m.cursors[col.ID] && m.mode == modeNormal
			parts = append(parts, m.card(t, sel))
		}
		lines = append(lines, style.Render(strings.Join(parts, "\n")))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, lines...)
}

func (m Model) card(t model.Task, selected bool) string {
	prio := priorityStyle(string(t.Priority)).Render("●")
	head := cardRefStyle.Render(t.Ref) + " " + prio
	if t.Estimate > 0 {
		head += "  " + cardMetaStyle.Render(fmt.Sprintf("%dp", t.Estimate))
	}
	title := truncate(t.Title, 25)
	lines := []string{head, cardTitleStyle.Render(title)}
	meta := []string{}
	for _, l := range t.Labels {
		meta = append(meta, labelChip.Render("#"+l.Name))
	}
	if t.AssigneeID != nil {
		for _, mem := range m.memberName(*t.AssigneeID) {
			meta = append(meta, cardMetaStyle.Render(initials(mem)))
		}
	}
	if len(t.Subtasks) > 0 {
		done := 0
		for _, s := range t.Subtasks {
			if s.Done {
				done++
			}
		}
		meta = append(meta, cardMetaStyle.Render(fmt.Sprintf("%d/%d", done, len(t.Subtasks))))
	}
	if t.DueDate != nil {
		meta = append(meta, cardMetaStyle.Render(t.DueDate.Format("02/01")))
	}
	if len(meta) > 0 {
		lines = append(lines, cardMetaStyle.Render(truncate(strings.Join(meta, " "), 25)))
	}
	style := cardStyle
	if selected {
		style = cardSelectedStyle
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) memberName(id string) []string {
	if m.board == nil {
		return nil
	}
	for _, mem := range m.board.Members {
		if mem.ID == id {
			return []string{mem.Name}
		}
	}
	return nil
}

func (m Model) listView() string {
	tasks := m.filteredTasks("")
	if len(tasks) == 0 {
		return mutedStyle.Render("nenhuma task")
	}
	colName := func(id string) string {
		if m.board != nil {
			for _, c := range m.board.Columns {
				if c.ID == id {
					return c.Name
				}
			}
		}
		return "—"
	}
	assignee := func(id *string) string {
		if id == nil {
			return "—"
		}
		names := m.memberName(*id)
		if len(names) == 0 {
			return "—"
		}
		return names[0]
	}
	rows := []string{listHeader.Render(fmt.Sprintf("%-8s %-42s %-14s %-8s %-12s %s", "REF", "TITLE", "COLUMN", "PRIO", "ASSIGNEE", "DUE"))}
	for i, t := range tasks {
		due := "—"
		if t.DueDate != nil {
			due = t.DueDate.Format("2006-01-02")
		}
		row := fmt.Sprintf("%-8s %-42s %-14s %-8s %-12s %s",
			t.Ref, truncate(t.Title, 42), truncate(colName(t.ColumnID), 14),
			string(t.Priority), truncate(assignee(t.AssigneeID), 12), due)
		if i == m.listIdx {
			row = lipgloss.NewStyle().Background(lipgloss.Color("#1c1c1c")).Render(row)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func (m Model) activityView() string {
	if len(m.activity) == 0 {
		return mutedStyle.Render("sem atividade recente")
	}
	rows := []string{}
	for _, a := range m.activity {
		ts := a.CreatedAt.Format("01/02 15:04")
		rows = append(rows, cardRefStyle.Render(ts)+"  "+sectionTitle.Render(a.Action)+"  "+mutedStyle.Render(a.Detail))
	}
	return strings.Join(rows, "\n")
}

func (m Model) footer() string {
	keys := "j/k navegar  ·  h/l colunas  ·  enter detalhes  ·  n nova  ·  >/< mover  ·  / buscar  ·  p projetos  ·  ? ajuda  ·  q sair"
	if m.mode == modeDetail {
		keys = "j/k subtasks  ·  x concluir  ·  t novo subtask  ·  c comentar  ·  p prioridade  ·  a responsável  ·  >/< mover  ·  del excluir  ·  esc voltar"
	}
	if m.mode == modeNewTask {
		keys = "tab próximo campo  ·  h/l coluna/prioridade  ·  ctrl+s salvar  ·  esc cancelar"
	}
	return footerStyle.Width(max(40, m.width-2)).Render(truncate(keys, max(20, m.width-4)))
}

func (m Model) overlay(base, content string, w, h int) string {
	w = min(w, max(20, m.width-2))
	h = min(h, max(6, m.height-2))
	box := overlayStyle.Width(w - 4).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "))
}

func (m Model) detailView() string {
	if m.detail == nil {
		return "carregando..."
	}
	t := *m.detail
	lines := []string{
		sectionTitle.Render(t.Ref+"  "+t.Title),
		mutedStyle.Render("coluna: "+m.colName(t.ColumnID)+"  ·  prioridade: "+string(t.Priority)),
		"",
	}
	if t.Description != "" {
		lines = append(lines, wrap(t.Description, 76)...)
		lines = append(lines, "")
	}
	if len(t.Labels) > 0 {
		labels := []string{}
		for _, l := range t.Labels {
			labels = append(labels, "#"+l.Name)
		}
		lines = append(lines, labelChip.Render("labels: "+strings.Join(labels, " ")))
	}
	lines = append(lines, sectionTitle.Render("Checklist"))
	if len(t.Subtasks) == 0 {
		lines = append(lines, mutedStyle.Render("  (vazio)"))
	}
	for i, s := range t.Subtasks {
		mark := "○"
		if s.Done {
			mark = doneStyle.Render("●")
		}
		cursor := "  "
		if i == m.subIdx {
			cursor = "> "
		}
		title := s.Title
		if s.Done {
			title = mutedStyle.Render(title)
		}
		lines = append(lines, cursor+mark+" "+title)
	}
	lines = append(lines, "", sectionTitle.Render("Comentários"))
	if len(m.comments) == 0 {
		lines = append(lines, mutedStyle.Render("  (nenhum)"))
	}
	start := max(0, len(m.comments)-3)
	for _, c := range m.comments[start:] {
		lines = append(lines, mutedStyle.Render(c.Author)+": "+truncate(c.Body, 72))
	}
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

func (m Model) formView() string {
	col := "—"
	if m.board != nil && m.formCol < len(m.board.Columns) {
		col = m.board.Columns[m.formCol].Name
	}
	focus := func(i int, s string) string {
		if m.formField == i {
			return lipgloss.NewStyle().Foreground(accentC).Bold(true).Render("▸ " + s)
		}
		return "  " + s
	}
	lines := []string{
		sectionTitle.Render("Nova task"),
		"",
		focus(0, "Título"),
		m.titleIn.View(),
		"",
		focus(1, "Descrição"),
		m.descIn.View(),
		"",
		focus(2, "Coluna:  < "+col+" >"),
		focus(3, "Prioridade:  < "+string(model.Priorities[m.formPrio])+" >"),
		"",
		mutedStyle.Render("tab alterna · ctrl+s salva · esc cancela"),
	}
	return strings.Join(lines, "\n")
}

func (m Model) projectView() string {
	lines := []string{sectionTitle.Render("Projetos"), ""}
	for i, p := range m.projects {
		prefix := "  "
		if i == m.projectIdx {
			prefix = "> "
		}
		lines = append(lines, prefix+p.Key+"  "+p.Name)
	}
	if len(m.projects) == 0 {
		lines = append(lines, mutedStyle.Render("nenhum projeto"))
	}
	lines = append(lines, "", mutedStyle.Render("j/k seleciona · enter abre · esc fecha"))
	return strings.Join(lines, "\n")
}

func (m Model) helpView() string {
	entries := []struct{ k, d string }{
		{"j / k", "navegar tasks"}, {"h / l", "mudar de coluna"},
		{"enter", "abrir detalhes da task"}, {"n", "nova task"},
		{"> / <", "mover task entre colunas"}, {"/", "buscar"},
		{"p", "trocar projeto"}, {"1 2 3", "board / list / activity"},
		{"r", "atualizar da API"}, {"?", "esta ajuda"}, {"q", "sair"},
		{"----", "---- detalhes ----"},
		{"x", "concluir subtask"}, {"t", "novo subtask"}, {"c", "comentar"},
		{"p", "ciclar prioridade"}, {"a", "ciclar responsável"}, {"del", "excluir task"},
	}
	lines := []string{sectionTitle.Render("glplan — atalhos"), ""}
	for _, e := range entries {
		lines = append(lines, helpKey.Render(e.k)+helpText.Render(e.d))
	}
	lines = append(lines, "", mutedStyle.Render("esc fecha"))
	return strings.Join(lines, "\n")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func wrap(s string, width int) []string {
	words := strings.Fields(s)
	lines := []string{}
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "??"
	}
	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:min(2, len(parts[0]))])
	}
	return strings.ToUpper(parts[0][:1] + parts[len(parts)-1][:1])
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
