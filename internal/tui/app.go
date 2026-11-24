// Package tui implements the Bubble Tea terminal interface.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
	modeTaskForm
	modeDetail
	modeSearch
	modeProjects
	modeProjectForm
	modeHelp
	modeConfirm
	modeComment
	modeSubtask
	modeLabels
)

type projectsMsg []model.Project
type boardMsg struct {
	board *model.Board
	tasks []model.Task
}
type activityMsg []model.Activity
type taskMsg *model.Task
type commentsMsg []model.Comment
type projectCreatedMsg *model.Project
type errMsg error
type savedMsg string
type tickMsg time.Time

// Model is the TUI state.
type Model struct {
	client  *api.Client
	backend string

	projects    []model.Project
	projectIdx  int
	projectPick int
	board       *model.Board
	tasks       []model.Task
	activity    []model.Activity

	view view
	mode mode

	colIdx        int
	colStart      int
	cursors       map[string]int
	colScroll     map[string]int
	listIdx       int
	listStart     int
	activityStart int
	subIdx        int

	search   string
	searchIn textinput.Model

	titleIn   textinput.Model
	descIn    textarea.Model
	commentIn textinput.Model
	subIn     textinput.Model

	projectNameIn textinput.Model
	projectKeyIn  textinput.Model
	projectDescIn textinput.Model

	formField        int
	formCol          int
	formPrio         int
	formEstimate     int
	projectField     int
	editingID        string
	formReturnDetail bool
	labelIdx         int
	labelTaskID      string
	labelSelected    map[string]bool

	detail   *model.Task
	comments []model.Comment
	status   string
	statusAt time.Time
	err      string
	width    int
	height   int
	loading  bool
}

var estimateValues = []int{0, 1, 2, 3, 5, 8, 13}

// New creates the TUI model.
func New(client *api.Client) Model { return newModel(client, "api") }

// NewWithBackend creates the TUI model with a human-readable backend label.
func NewWithBackend(client *api.Client, backend string) Model { return newModel(client, backend) }

func newModel(client *api.Client, backend string) Model {
	search := textinput.New()
	search.Placeholder = "filtrar por título, descrição ou ref..."
	search.Prompt = "⌕ "
	search.Width = 52

	title := textinput.New()
	title.Placeholder = "Título objetivo da task"
	title.Prompt = "› "
	title.Width = 56

	desc := textarea.New()
	desc.Placeholder = "Contexto, critérios de aceite, links..."
	desc.ShowLineNumbers = false
	desc.SetWidth(58)
	desc.SetHeight(6)

	comment := textinput.New()
	comment.Placeholder = "Escreva um comentário objetivo..."
	comment.Prompt = "› "
	comment.Width = 58

	sub := textinput.New()
	sub.Placeholder = "Novo item do checklist"
	sub.Prompt = "› "
	sub.Width = 58

	projectName := textinput.New()
	projectName.Placeholder = "Nome do projeto"
	projectName.Prompt = "› "
	projectName.Width = 42
	projectKey := textinput.New()
	projectKey.Placeholder = "KEY curta, ex: API"
	projectKey.Prompt = "› "
	projectKey.Width = 16
	projectDesc := textinput.New()
	projectDesc.Placeholder = "Descrição opcional"
	projectDesc.Prompt = "› "
	projectDesc.Width = 48

	if backend == "" {
		backend = "api"
	}
	return Model{
		client: client, backend: backend,
		cursors: map[string]int{}, colScroll: map[string]int{},
		searchIn: search, titleIn: title, descIn: desc,
		commentIn: comment, subIn: sub,
		projectNameIn: projectName, projectKeyIn: projectKey, projectDescIn: projectDesc,
		formPrio: 0, loading: true,
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
		out, err := m.client.ListActivity(projectID, 80)
		if err != nil {
			return errMsg(err)
		}
		return activityMsg(out)
	}
}

func (m Model) loadTask(id string) tea.Cmd {
	return func() tea.Msg {
		task, err := m.client.GetTask(id)
		if err != nil {
			return errMsg(err)
		}
		return taskMsg(task)
	}
}

func (m Model) loadComments(taskID string) tea.Cmd {
	return func() tea.Msg {
		comments, err := m.client.ListComments(taskID)
		if err != nil {
			return errMsg(err)
		}
		return commentsMsg(comments)
	}
}

func (m Model) currentProjectID() string {
	if len(m.projects) == 0 || m.projectIdx < 0 || m.projectIdx >= len(m.projects) {
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
		inputWidth := clamp(m.width-20, 24, 58)
		m.searchIn.Width = clamp(m.width-16, 24, 52)
		m.titleIn.Width = inputWidth
		m.commentIn.Width = inputWidth
		m.subIn.Width = inputWidth
		m.projectNameIn.Width = clamp(inputWidth, 24, 42)
		m.projectDescIn.Width = clamp(inputWidth, 24, 48)
		m.descIn.SetWidth(clamp(m.width-18, 30, 64))
		m.clampCursors()
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
		m.projectPick = m.projectIdx
		m.err = ""
		if len(m.projects) > 0 {
			return m, m.refresh()
		}
		m.loading = false
		return m, nil

	case boardMsg:
		m.board = msg.board
		m.tasks = msg.tasks
		m.loading = false
		m.err = ""
		m.clampCursors()
		if m.detail != nil {
			return m, m.loadTask(m.detail.ID)
		}
		return m, nil

	case activityMsg:
		m.activity = msg
		if m.activityStart >= len(m.activity) {
			m.activityStart = max(0, len(m.activity)-1)
		}
		return m, nil

	case taskMsg:
		if msg != nil {
			m.detail = msg
			return m, m.loadComments(msg.ID)
		}
		return m, nil

	case commentsMsg:
		m.comments = msg
		return m, nil

	case projectCreatedMsg:
		if msg != nil {
			created := *msg
			replaced := false
			for i := range m.projects {
				if m.projects[i].ID == created.ID {
					m.projects[i] = created
					m.projectIdx = i
					replaced = true
					break
				}
			}
			if !replaced {
				m.projects = append(m.projects, created)
				m.projectIdx = len(m.projects) - 1
			}
			m.projectPick = m.projectIdx
			m.cursors = map[string]int{}
			m.colScroll = map[string]int{}
			m.colIdx, m.colStart = 0, 0
			m.status = "projeto " + created.Key + " pronto"
			m.loading = true
			return m, tea.Batch(m.loadProjects(), m.refresh())
		}
		return m, nil

	case errMsg:
		m.err = msg.Error()
		m.loading = false
		return m, nil

	case savedMsg:
		m.status = string(msg)
		m.statusAt = time.Now()
		return m, m.refresh()
	}
	return m, nil
}
