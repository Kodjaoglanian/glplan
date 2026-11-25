package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"glplan/internal/model"
)

// TaskUpdate describes a partial task update. Nil fields are left unchanged.
type TaskUpdate struct {
	Title       *string
	Description *string
	ColumnID    *string
	Priority    *model.Priority
	Estimate    *int
	Position    *float64
	SetAssignee bool
	AssigneeID  *string
	SetDueDate  bool
	DueDate     *time.Time
	LabelIDs    *[]string
	Actor       string
}

const taskSelect = `SELECT t.id,t.ref,t.project_id,t.column_id,t.title,t.description,t.priority,
	COALESCE(t.assignee_id,''),COALESCE(t.due_date,''),t.position,t.estimate,
	t.created_at,t.updated_at,COALESCE(t.completed_at,''),
	(SELECT COUNT(*) FROM comments c WHERE c.task_id=t.id) FROM tasks t`

type scanner interface{ Scan(dest ...any) error }

func scanTask(s scanner) (model.Task, error) {
	var t model.Task
	var assignee, due, completed, created, updated string
	err := s.Scan(&t.ID, &t.Ref, &t.ProjectID, &t.ColumnID, &t.Title, &t.Description, &t.Priority,
		&assignee, &due, &t.Position, &t.Estimate, &created, &updated, &completed, &t.CommentCount)
	if err != nil {
		return t, err
	}
	if assignee != "" {
		t.AssigneeID = &assignee
	}
	if due != "" {
		v := parseStringTime(due)
		t.DueDate = &v
	}
	if completed != "" {
		v := parseStringTime(completed)
		t.CompletedAt = &v
	}
	t.CreatedAt = parseStringTime(created)
	t.UpdatedAt = parseStringTime(updated)
	t.Labels = []model.Label{}
	t.Subtasks = []model.Subtask{}
	return t, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// hydrate fills labels and subtasks for a set of tasks in two queries.
func (s *Store) hydrate(tasks []model.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]any, len(tasks))
	index := make(map[string]*model.Task, len(tasks))
	for i := range tasks {
		ids[i] = tasks[i].ID
		index[tasks[i].ID] = &tasks[i]
	}
	ph := placeholders(len(ids))

	rows, err := s.db.Query(`SELECT tl.task_id,l.id,l.project_id,l.name,l.color FROM task_labels tl
		JOIN labels l ON l.id=tl.label_id WHERE tl.task_id IN (`+ph+`) ORDER BY l.name`, ids...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var taskID string
		var l model.Label
		if err := rows.Scan(&taskID, &l.ID, &l.ProjectID, &l.Name, &l.Color); err != nil {
			rows.Close()
			return err
		}
		if t := index[taskID]; t != nil {
			t.Labels = append(t.Labels, l)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT id,task_id,title,done,position FROM subtasks WHERE task_id IN (`+ph+`) ORDER BY position,id`, ids...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var sub model.Subtask
		var done int
		if err := rows.Scan(&sub.ID, &sub.TaskID, &sub.Title, &done, &sub.Position); err != nil {
			rows.Close()
			return err
		}
		sub.Done = done != 0
		if t := index[sub.TaskID]; t != nil {
			t.Subtasks = append(t.Subtasks, sub)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	return rows.Close()
}

// ListTasks returns tasks matching the filter.
func (s *Store) ListTasks(f model.TaskFilter) ([]model.Task, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.ProjectID != "" {
		where = append(where, "t.project_id=?")
		args = append(args, f.ProjectID)
	}
	if f.ColumnID != "" {
		where = append(where, "t.column_id=?")
		args = append(args, f.ColumnID)
	}
	if f.AssigneeID != "" {
		where = append(where, "t.assignee_id=?")
		args = append(args, f.AssigneeID)
	}
	if f.LabelID != "" {
		where = append(where, "EXISTS(SELECT 1 FROM task_labels tl WHERE tl.task_id=t.id AND tl.label_id=?)")
		args = append(args, f.LabelID)
	}
	if f.Priority != "" && f.Priority != model.PriorityNone {
		where = append(where, "t.priority=?")
		args = append(args, f.Priority)
	}
	if !f.IncludeDone {
		where = append(where, "t.completed_at IS NULL")
	}
	if f.Search != "" {
		where = append(where, "(t.title LIKE ? ESCAPE '\\' OR t.description LIKE ? ESCAPE '\\' OR t.ref LIKE ? ESCAPE '\\')")
		q := like(f.Search)
		args = append(args, q, q, q)
	}
	query := taskSelect + ` WHERE ` + strings.Join(where, " AND ") + ` ORDER BY t.position,t.created_at`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.hydrate(out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTask looks up a task by id or human-readable reference.
func (s *Store) GetTask(idOrRef string) (*model.Task, error) {
	t, err := scanTask(s.db.QueryRow(taskSelect+` WHERE t.id=? OR upper(t.ref)=upper(?) LIMIT 1`, idOrRef, idOrRef))
	if err != nil {
		return nil, notFound(err)
	}
	tasks := []model.Task{t}
	if err := s.hydrate(tasks); err != nil {
		return nil, err
	}
	return &tasks[0], nil
}

// CreateTask adds a task to a board, assigning the next sequential reference.
func (s *Store) CreateTask(projectID, columnID, title, description string, priority model.Priority, estimate int) (*model.Task, error) {
	p, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	cols, err := s.ListColumns(p.ID)
	if err != nil {
		return nil, err
	}
	if columnID == "" {
		for _, c := range cols {
			if !c.IsDone {
				columnID = c.ID
				break
			}
		}
	}
	var selected *model.Column
	for i := range cols {
		if cols[i].ID == columnID {
			selected = &cols[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("unknown column %q", columnID)
	}
	if priority == "" {
		priority = model.PriorityNone
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`UPDATE task_seq SET n=n+1 WHERE project_id=? RETURNING n`, p.ID).Scan(&n); err != nil {
		return nil, err
	}
	var pos float64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(position),0)+100 FROM tasks WHERE column_id=?`, columnID).Scan(&pos); err != nil {
		return nil, err
	}
	id := model.NewID("tsk")
	ref := fmt.Sprintf("%s-%d", p.Key, n)
	ts := now()
	var completed any
	if selected.IsDone {
		completed = ts
	}
	if _, err := tx.Exec(`INSERT INTO tasks(id,ref,project_id,column_id,title,description,priority,position,estimate,created_at,updated_at,completed_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, ref, p.ID, columnID, title, description, priority, pos, estimate, ts, ts, completed); err != nil {
		return nil, err
	}
	if err := s.logActivityTx(tx, p.ID, &id, "system", "task.created", fmt.Sprintf("%s · %s", ref, title)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.emit("task.created", p.ID, &id, "system", fmt.Sprintf("%s · %s", ref, title))
	return s.GetTask(id)
}

// UpdateTask applies a partial update and records an activity entry.
func (s *Store) UpdateTask(id string, u TaskUpdate) (*model.Task, error) {
	current, err := s.GetTask(id)
	if err != nil {
		return nil, err
	}
	sets := []string{"updated_at=?"}
	args := []any{now()}
	changes := []string{}
	if u.Title != nil && *u.Title != current.Title {
		sets = append(sets, "title=?")
		args = append(args, *u.Title)
		changes = append(changes, "title")
	}
	if u.Description != nil && *u.Description != current.Description {
		sets = append(sets, "description=?")
		args = append(args, *u.Description)
		changes = append(changes, "description")
	}
	if u.Priority != nil && *u.Priority != current.Priority {
		sets = append(sets, "priority=?")
		args = append(args, *u.Priority)
		changes = append(changes, "priority")
	}
	if u.Estimate != nil && *u.Estimate != current.Estimate {
		sets = append(sets, "estimate=?")
		args = append(args, *u.Estimate)
		changes = append(changes, "estimate")
	}
	if u.Position != nil {
		sets = append(sets, "position=?")
		args = append(args, *u.Position)
	}
	if u.SetAssignee {
		sets = append(sets, "assignee_id=?")
		args = append(args, nullString(u.AssigneeID))
		changes = append(changes, "assignee")
	}
	if u.SetDueDate {
		sets = append(sets, "due_date=?")
		args = append(args, nullTime(u.DueDate))
		changes = append(changes, "due date")
	}
	if u.ColumnID != nil && *u.ColumnID != current.ColumnID {
		col, err := s.getColumn(*u.ColumnID)
		if err != nil {
			return nil, err
		}
		if col.ProjectID != current.ProjectID {
			return nil, fmt.Errorf("column belongs to another project")
		}
		sets = append(sets, "column_id=?", "completed_at=?")
		args = append(args, col.ID, nullTime(dueTimeFor(col.IsDone)))
		changes = append(changes, "column")
	}
	args = append(args, id)
	if _, err := s.db.Exec(`UPDATE tasks SET `+strings.Join(sets, ",")+` WHERE id=?`, args...); err != nil {
		return nil, err
	}
	if u.LabelIDs != nil {
		if err := s.setTaskLabels(id, *u.LabelIDs, current.ProjectID); err != nil {
			return nil, err
		}
		changes = append(changes, "labels")
	}
	actor := u.Actor
	if actor == "" {
		actor = "system"
	}
	if len(changes) > 0 {
		detail := current.Ref + " · " + strings.Join(changes, ", ")
		if err := s.LogActivity(current.ProjectID, &current.ID, actor, "task.updated", detail); err != nil {
			return nil, err
		}
	}
	return s.GetTask(id)
}

func dueTimeFor(done bool) *time.Time {
	if !done {
		return nil
	}
	t := nowTime()
	return &t
}

func (s *Store) setTaskLabels(taskID string, labelIDs []string, projectID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM task_labels WHERE task_id=?`, taskID); err != nil {
		return err
	}
	for _, id := range labelIDs {
		var owner string
		if err := tx.QueryRow(`SELECT project_id FROM labels WHERE id=?`, id).Scan(&owner); err != nil {
			return notFound(err)
		}
		if owner != projectID {
			return fmt.Errorf("label %s belongs to another project", id)
		}
		if _, err := tx.Exec(`INSERT INTO task_labels(task_id,label_id) VALUES(?,?)`, taskID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MoveTask places a task in a column, optionally before another task.
func (s *Store) MoveTask(id, columnID, beforeID string, position *float64) (*model.Task, error) {
	t, err := s.GetTask(id)
	if err != nil {
		return nil, err
	}
	if columnID == "" {
		columnID = t.ColumnID
	}
	col, err := s.getColumn(columnID)
	if err != nil {
		return nil, err
	}
	if col.ProjectID != t.ProjectID {
		return nil, fmt.Errorf("column belongs to another project")
	}
	var pos float64
	if position != nil {
		pos = *position
	} else if beforeID != "" {
		before, err := s.GetTask(beforeID)
		if err != nil {
			return nil, err
		}
		var prev sql.NullFloat64
		if err := s.db.QueryRow(`SELECT MAX(position) FROM tasks WHERE column_id=? AND position<?`, columnID, before.Position).Scan(&prev); err != nil {
			return nil, err
		}
		if prev.Valid {
			pos = (prev.Float64 + before.Position) / 2
		} else {
			pos = before.Position - 100
		}
	} else {
		if err := s.db.QueryRow(`SELECT COALESCE(MAX(position),0)+100 FROM tasks WHERE column_id=? AND id<>?`, columnID, id).Scan(&pos); err != nil {
			return nil, err
		}
	}
	ts := now()
	var completed any
	if col.IsDone {
		completed = ts
		if t.ColumnID == columnID && t.CompletedAt != nil {
			completed = fmtTime(*t.CompletedAt)
		}
	}
	if _, err := s.db.Exec(`UPDATE tasks SET column_id=?,position=?,completed_at=?,updated_at=? WHERE id=?`, columnID, pos, completed, ts, id); err != nil {
		return nil, err
	}
	from := t.ColumnID
	if oldCol, err := s.getColumn(t.ColumnID); err == nil {
		from = oldCol.Name
	}
	detail := fmt.Sprintf("%s · %s → %s", t.Ref, from, col.Name)
	if t.ColumnID == columnID {
		detail = t.Ref + " · reordered"
	}
	if err := s.LogActivity(t.ProjectID, &t.ID, "system", "task.moved", detail); err != nil {
		return nil, err
	}
	return s.GetTask(id)
}

// DeleteTask removes a task permanently.
func (s *Store) DeleteTask(id string) error {
	t, err := s.GetTask(id)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	if err := affected(res); err != nil {
		return err
	}
	return s.LogActivity(t.ProjectID, nil, "system", "task.deleted", t.Ref+" · "+t.Title)
}

// SetTaskLabels replaces a task's labels.
func (s *Store) SetTaskLabels(id string, labelIDs []string) error {
	t, err := s.GetTask(id)
	if err != nil {
		return err
	}
	return s.setTaskLabels(id, labelIDs, t.ProjectID)
}

// LogActivity appends an entry to the project's activity feed.
func (s *Store) LogActivity(projectID string, taskID *string, actor, action, detail string) error {
	if err := s.logActivityTx(s.db, projectID, taskID, actor, action, detail); err != nil {
		return err
	}
	s.emit(action, projectID, taskID, actor, detail)
	return nil
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func (s *Store) logActivityTx(tx execer, projectID string, taskID *string, actor, action, detail string) error {
	_, err := tx.Exec(`INSERT INTO activities(id,project_id,task_id,actor,action,detail,created_at) VALUES(?,?,?,?,?,?,?)`,
		model.NewID("act"), projectID, nullString(taskID), actor, action, detail, now())
	return err
}

// ListActivity returns the most recent entries for a project.
func (s *Store) ListActivity(projectID string, limit int) ([]model.Activity, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.db.Query(`SELECT id,project_id,COALESCE(task_id,''),actor,action,detail,created_at FROM activities
		WHERE project_id=? ORDER BY created_at DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Activity{}
	for rows.Next() {
		var a model.Activity
		var taskID, created string
		if err := rows.Scan(&a.ID, &a.ProjectID, &taskID, &a.Actor, &a.Action, &a.Detail, &created); err != nil {
			return nil, err
		}
		if taskID != "" {
			a.TaskID = &taskID
		}
		a.CreatedAt = parseStringTime(created)
		out = append(out, a)
	}
	return out, rows.Err()
}

var _ = errors.Is
