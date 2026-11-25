package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"glplan/internal/model"
)

func renderTestModel() Model {
	m := New(nil)
	m.width = 120
	m.height = 32
	m.loading = false
	m.projects = []model.Project{{ID: "prj_1", Key: "GL", Name: "Glplan"}}
	m.projectIdx = 0
	cols := []model.Column{
		{ID: "col_backlog", ProjectID: "prj_1", Name: "Backlog", Position: 0},
		{ID: "col_todo", ProjectID: "prj_1", Name: "Todo", Position: 1, WIPLimit: 3},
		{ID: "col_done", ProjectID: "prj_1", Name: "Done", Position: 2, IsDone: true},
	}
	m.board = &model.Board{
		Project: m.projects[0], Columns: cols,
		Members: []model.Member{{ID: "mem_ana", Name: "Ana Lima", Active: true}},
		Labels:  []model.Label{{ID: "lbl_bug", ProjectID: "prj_1", Name: "bug", Color: "#ef4444"}},
	}
	now := time.Now()
	m.tasks = []model.Task{
		{ID: "tsk_1", Ref: "GL-1", ProjectID: "prj_1", ColumnID: "col_backlog", Title: "Estruturar board", Priority: model.PriorityHigh, Position: 100, CreatedAt: now, UpdatedAt: now, Labels: m.board.Labels, Subtasks: []model.Subtask{{ID: "sub_1", Title: "check", Done: true}}},
		{ID: "tsk_2", Ref: "GL-2", ProjectID: "prj_1", ColumnID: "col_todo", Title: "Revisar atalhos", Priority: model.PriorityUrgent, Position: 100, CreatedAt: now, UpdatedAt: now},
		{ID: "tsk_3", Ref: "GL-3", ProjectID: "prj_1", ColumnID: "col_done", Title: "Schema SQLite", Priority: model.PriorityNone, Position: 100, CreatedAt: now, UpdatedAt: now, CompletedAt: &now},
	}
	return m
}

func TestBoardRenderSmoke(t *testing.T) {
	m := renderTestModel()
	out := m.View()
	for _, want := range []string{"glplan", "Glplan", "Backlog", "Todo", "Done", "GL-1", "Estruturar board", "J/K ordenar"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render does not contain %q:\n%s", want, out)
		}
	}
}

func TestRenderNarrowTerminalDoesNotPanic(t *testing.T) {
	m := renderTestModel()
	for _, size := range [][2]int{{30, 8}, {48, 14}, {80, 20}, {160, 40}} {
		m.width, m.height = size[0], size[1]
		if out := m.View(); out == "" {
			t.Fatalf("empty render at %dx%d", size[0], size[1])
		}
	}
}

func TestReorderPosition(t *testing.T) {
	tasks := []model.Task{{Position: 100}, {Position: 200}, {Position: 300}}
	if pos, ok := reorderPosition(tasks, 1, -1); !ok || pos != 0 {
		t.Fatalf("move up at middle: pos=%v ok=%v", pos, ok)
	}
	if pos, ok := reorderPosition(tasks, 2, -1); !ok || pos != 150 {
		t.Fatalf("move up between tasks: pos=%v ok=%v", pos, ok)
	}
	if pos, ok := reorderPosition(tasks, 1, 1); !ok || pos != 400 {
		t.Fatalf("move down at end: pos=%v ok=%v", pos, ok)
	}
	if pos, ok := reorderPosition(tasks, 0, 1); !ok || pos != 250 {
		t.Fatalf("move down between tasks: pos=%v ok=%v", pos, ok)
	}
	if _, ok := reorderPosition(tasks, 0, -1); ok {
		t.Fatal("first task cannot move up")
	}
	if _, ok := reorderPosition(tasks, 2, 1); ok {
		t.Fatal("last task cannot move down")
	}
}

func TestDueLabelUsesLocalCalendarDay(t *testing.T) {
	if got := dueLabel(time.Now().UTC().Add(24 * time.Hour)); got != "amanhã" {
		t.Fatalf("expected UTC+24h to render as amanhã in local calendar, got %q", got)
	}
}

func TestDetailOverlayKeepsBoardContext(t *testing.T) {
	m := renderTestModel()
	m.detail = &m.tasks[0]
	m.mode = modeDetail
	out := m.View()
	for _, want := range []string{"Backlog", "GL-1", "Checklist", "Comentários"} {
		if !strings.Contains(out, want) {
			t.Fatalf("detail overlay does not contain %q:\n%s", want, out)
		}
	}
}

func TestLabelPickerRender(t *testing.T) {
	m := renderTestModel()
	m.detail = &m.tasks[0]
	opened, _ := m.openLabelPicker(m.tasks[0])
	m = opened.(Model)
	if m.mode != modeLabels {
		t.Fatalf("expected labels mode, got %v", m.mode)
	}
	out := m.View()
	for _, want := range []string{"Labels da task", "#bug", "enter salva"} {
		if !strings.Contains(out, want) {
			t.Fatalf("label picker does not contain %q:\n%s", want, out)
		}
	}
}

func TestFilteredTasksInListUseColumnOrderBeforePosition(t *testing.T) {
	m := renderTestModel()
	m.tasks[0].Position = 100 // backlog
	m.tasks[1].Position = 50  // todo: would come first in a global position sort
	got := m.filteredTasks("")
	if len(got) != 3 || got[0].Ref != "GL-1" || got[1].Ref != "GL-2" || got[2].Ref != "GL-3" {
		refs := []string{}
		for _, task := range got {
			refs = append(refs, task.Ref)
		}
		t.Fatalf("expected list grouped in column order, got %v", refs)
	}
}

func TestNewTaskFormFocusesTitleInput(t *testing.T) {
	m := renderTestModel()
	opened, _ := m.openNewForm()
	m = opened.(Model)
	if !m.titleIn.Focused() {
		t.Fatal("expected title input to be focused when opening task form")
	}
}

func TestProjectFormFocusesNameInput(t *testing.T) {
	m := renderTestModel()
	opened, _ := m.openProjectForm()
	m = opened.(Model)
	if !m.projectNameIn.Focused() {
		t.Fatal("expected project name input to be focused when opening project form")
	}
}

func TestProjectKeyFromName(t *testing.T) {
	tests := map[string]string{
		"Produto":          "PROD",
		"Customer Success": "CS",
		"API Platform":     "AP",
		"nova interface":   "NI",
	}
	for name, want := range tests {
		if got := projectKeyFromName(name); got != want {
			t.Fatalf("projectKeyFromName(%q)=%q, want %q", name, got, want)
		}
	}
}

func TestHeaderShowsUrgentAndCompletedCountsTogether(t *testing.T) {
	m := renderTestModel()
	out := m.View()
	for _, want := range []string{"2 abertas", "1 urgente", "1 concluída"} {
		if !strings.Contains(out, want) {
			t.Fatalf("header does not contain %q:\n%s", want, out)
		}
	}
}

func TestTruncateUsesDisplayWidth(t *testing.T) {
	if got := truncate("ab你cd", 4); got != "ab…" {
		t.Fatalf("unexpected wide-character truncation %q", got)
	}
	if got := ansiWidth(truncate("abcdef", 4)); got > 4 {
		t.Fatalf("truncated string wider than limit: width=%d", got)
	}
}

func ansiWidth(s string) int {
	return lipgloss.Width(s)
}

func TestTaskFormTextFieldsAcceptHLAndArrowKeys(t *testing.T) {
	m := renderTestModel()
	opened, _ := m.openNewForm()
	m = opened.(Model)
	updated, _ := m.updateTaskForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(Model)
	if got := m.titleIn.Value(); got != "h" {
		t.Fatalf("expected h to reach title input, got %q", got)
	}
	updated, _ = m.updateTaskForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if got := m.titleIn.Value(); got != "hl" {
		t.Fatalf("expected l to reach title input, got %q", got)
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiPattern.ReplaceAllString(s, "") }

func TestCardUsesReadableMetadataAndWrapsTitle(t *testing.T) {
	m := renderTestModel()
	m.width, m.height = 150, 50
	m.board.Members = append(m.board.Members, model.Member{ID: "mem_carla", Name: "Carla Nunes", Active: true})
	assignee := "mem_carla"
	due := time.Now().Add(24 * time.Hour)
	task := m.tasks[0]
	task.Title = "Desenhar fluxo de onboarding para a primeira execução"
	task.AssigneeID = &assignee
	task.Estimate = 3
	task.DueDate = &due
	task.Subtasks = append(task.Subtasks, model.Subtask{ID: "sub_2", Title: "outro"})
	out := stripANSI(m.card(task, false, 34))
	for _, want := range []string{"est 3p", "resp Carla", "chk", "prazo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected card metadata %q, got:\n%s", want, out)
		}
	}
	if strings.Count(out, "\n") < 3 {
		t.Fatalf("expected multi-line readable card, got:\n%s", out)
	}
}

func TestBoardMetricsCapVisibleColumnsAtFour(t *testing.T) {
	m := renderTestModel()
	m.width = 190
	metrics := m.boardMetrics()
	if metrics.visibleCols != 4 {
		t.Fatalf("visibleCols=%d, want 4", metrics.visibleCols)
	}
	if metrics.colWidth < 40 {
		t.Fatalf("colWidth=%d, want at least 40", metrics.colWidth)
	}
}

func TestPulseMeterExplainsLoad(t *testing.T) {
	out := stripANSI(pulseMeter(1, 3, 3, 18, true, ""))
	if !strings.Contains(out, "carga") || !strings.Contains(out, "33%") {
		t.Fatalf("expected labeled load meter, got %q", out)
	}
}
