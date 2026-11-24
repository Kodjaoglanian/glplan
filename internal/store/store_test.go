package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"glplan/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "glplan-test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func createTestProject(t *testing.T, s *Store, key string) *model.Project {
	t.Helper()
	p := &model.Project{Key: key, Name: key + " Project"}
	if err := s.CreateProject(p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	return p
}

func firstColumn(t *testing.T, s *Store, projectID string) model.Column {
	t.Helper()
	cols, err := s.ListColumns(projectID)
	if err != nil {
		t.Fatalf("list columns: %v", err)
	}
	if len(cols) == 0 {
		t.Fatal("project has no columns")
	}
	return cols[0]
}

func TestGetTaskHydratesLabelsAndSubtasks(t *testing.T) {
	s := openTestStore(t)
	p := createTestProject(t, s, "HYD")
	col := firstColumn(t, s, p.ID)

	task, err := s.CreateTask(p.ID, col.ID, "Hidratar detalhe", "desc", model.PriorityHigh, 3)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	label, err := s.CreateLabel(p.ID, "backend", "#3355ff")
	if err != nil {
		t.Fatalf("create label: %v", err)
	}
	if err := s.SetTaskLabels(task.ID, []string{label.ID}); err != nil {
		t.Fatalf("set labels: %v", err)
	}
	if _, err := s.AddSubtask(task.ID, "primeiro checklist"); err != nil {
		t.Fatalf("add subtask: %v", err)
	}

	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if len(got.Labels) != 1 || got.Labels[0].ID != label.ID {
		t.Fatalf("expected hydrated label %q, got %#v", label.ID, got.Labels)
	}
	if len(got.Subtasks) != 1 || got.Subtasks[0].Title != "primeiro checklist" {
		t.Fatalf("expected hydrated subtask, got %#v", got.Subtasks)
	}
}

func TestUpdateTaskRejectsColumnFromAnotherProject(t *testing.T) {
	s := openTestStore(t)
	p1 := createTestProject(t, s, "ONE")
	p2 := createTestProject(t, s, "TWO")
	col1 := firstColumn(t, s, p1.ID)
	col2 := firstColumn(t, s, p2.ID)

	task, err := s.CreateTask(p1.ID, col1.ID, "não cruzar projeto", "", model.PriorityNone, 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	_, err = s.UpdateTask(task.ID, TaskUpdate{ColumnID: &col2.ID})
	if err == nil {
		t.Fatal("expected cross-project column update to fail")
	}
}

func TestMoveTaskSetsAndClearsCompletion(t *testing.T) {
	s := openTestStore(t)
	p := createTestProject(t, s, "MOV")
	cols, err := s.ListColumns(p.ID)
	if err != nil {
		t.Fatalf("list columns: %v", err)
	}
	var backlog, done model.Column
	for _, c := range cols {
		switch c.Name {
		case "Backlog":
			backlog = c
		case "Done":
			done = c
		}
	}
	if backlog.ID == "" || done.ID == "" {
		t.Fatal("default columns not found")
	}

	task, err := s.CreateTask(p.ID, backlog.ID, "completar", "", model.PriorityNone, 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	moved, err := s.MoveTask(task.ID, done.ID, "", nil)
	if err != nil {
		t.Fatalf("move to done: %v", err)
	}
	if moved.CompletedAt == nil {
		t.Fatal("expected completed_at when moved to done column")
	}
	moved, err = s.MoveTask(task.ID, backlog.ID, "", nil)
	if err != nil {
		t.Fatalf("move back: %v", err)
	}
	if moved.CompletedAt != nil {
		t.Fatal("expected completed_at to be cleared outside done column")
	}
}

func TestCreateTaskDirectlyInDoneColumnSetsCompletion(t *testing.T) {
	s := openTestStore(t)
	p := createTestProject(t, s, "DIRECT")
	cols, err := s.ListColumns(p.ID)
	if err != nil {
		t.Fatalf("list columns: %v", err)
	}
	var done model.Column
	for _, c := range cols {
		if c.IsDone {
			done = c
		}
	}
	if done.ID == "" {
		t.Fatal("done column not found")
	}
	task, err := s.CreateTask(p.ID, done.ID, "já concluída", "", model.PriorityNone, 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.CompletedAt == nil {
		t.Fatal("expected task created in done column to be completed")
	}
}

func TestSeedCreatesRichDemoBoard(t *testing.T) {
	s := openTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	tasks, err := s.ListTasks(model.TaskFilter{IncludeDone: true})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) < 6 {
		t.Fatalf("expected rich seeded board, got %d tasks", len(tasks))
	}
	var withLabels, withSubtasks, withAssignee, done int
	for _, task := range tasks {
		if len(task.Labels) > 0 {
			withLabels++
		}
		if len(task.Subtasks) > 0 {
			withSubtasks++
		}
		if task.AssigneeID != nil {
			withAssignee++
		}
		if task.Done() {
			done++
		}
	}
	if withLabels == 0 || withSubtasks == 0 || withAssignee == 0 {
		t.Fatalf("seed lacks demo metadata: labels=%d subtasks=%d assignees=%d", withLabels, withSubtasks, withAssignee)
	}
	if done == 0 {
		t.Fatal("expected at least one completed seeded task")
	}
}

func TestCreateTaskAcceptsProjectKey(t *testing.T) {
	s := openTestStore(t)
	p := createTestProject(t, s, "KEY")
	task, err := s.CreateTask(p.Key, "", "criada pela key", "", model.PriorityNone, 0)
	if err != nil {
		t.Fatalf("create task by project key: %v", err)
	}
	if task.ProjectID != p.ID {
		t.Fatalf("expected canonical project id %q, got %q", p.ID, task.ProjectID)
	}
	if task.Ref == "" || !strings.HasPrefix(task.Ref, p.Key+"-") {
		t.Fatalf("unexpected task ref %q", task.Ref)
	}
}

func TestReorderingDoneTaskPreservesCompletionTimestamp(t *testing.T) {
	s := openTestStore(t)
	p := createTestProject(t, s, "DONEORDER")
	cols, err := s.ListColumns(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var done *model.Column
	for i := range cols {
		if cols[i].IsDone {
			done = &cols[i]
		}
	}
	if done == nil {
		t.Fatal("missing done column")
	}
	task, err := s.CreateTask(p.ID, done.ID, "já feita", "", model.PriorityNone, 0)
	if err != nil {
		t.Fatal(err)
	}
	before := task.CompletedAt
	if before == nil {
		t.Fatal("expected completed timestamp")
	}
	time.Sleep(time.Millisecond)
	moved, err := s.MoveTask(task.ID, done.ID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved.CompletedAt == nil || !moved.CompletedAt.Equal(*before) {
		t.Fatalf("expected completion preserved at %v, got %v", before, moved.CompletedAt)
	}
}
