package store

import (
	"time"

	"glplan/internal/model"
)

// AddSubtask appends a checklist item to a task.
func (s *Store) AddSubtask(taskID, title string) (*model.Subtask, error) {
	if _, err := s.GetTask(taskID); err != nil {
		return nil, err
	}
	var pos int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(position),-1)+1 FROM subtasks WHERE task_id=?`, taskID).Scan(&pos); err != nil {
		return nil, err
	}
	sub := &model.Subtask{ID: model.NewID("sub"), TaskID: taskID, Title: title, Position: pos}
	if _, err := s.db.Exec(`INSERT INTO subtasks(id,task_id,title,done,position) VALUES(?,?,?,0,?)`, sub.ID, sub.TaskID, sub.Title, sub.Position); err != nil {
		return nil, err
	}
	return sub, nil
}

// UpdateSubtask changes a checklist item's title and/or done state.
func (s *Store) UpdateSubtask(id string, title *string, done *bool) (*model.Subtask, error) {
	var sub model.Subtask
	var d int
	if err := s.db.QueryRow(`SELECT id,task_id,title,done,position FROM subtasks WHERE id=?`, id).Scan(&sub.ID, &sub.TaskID, &sub.Title, &d, &sub.Position); err != nil {
		return nil, notFound(err)
	}
	sub.Done = d != 0
	if title != nil {
		sub.Title = *title
	}
	if done != nil {
		sub.Done = *done
	}
	if _, err := s.db.Exec(`UPDATE subtasks SET title=?,done=? WHERE id=?`, sub.Title, boolInt(sub.Done), sub.ID); err != nil {
		return nil, err
	}
	return &sub, nil
}

// DeleteSubtask removes a checklist item.
func (s *Store) DeleteSubtask(id string) error {
	res, err := s.db.Exec(`DELETE FROM subtasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}

// AddComment leaves a note on a task.
func (s *Store) AddComment(taskID, author, body string) (*model.Comment, error) {
	t, err := s.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	if author == "" {
		author = "you"
	}
	c := &model.Comment{ID: model.NewID("cmt"), TaskID: taskID, Author: author, Body: body, CreatedAt: nowTime()}
	if _, err := s.db.Exec(`INSERT INTO comments(id,task_id,author,body,created_at) VALUES(?,?,?,?,?)`,
		c.ID, c.TaskID, c.Author, c.Body, fmtTime(c.CreatedAt)); err != nil {
		return nil, err
	}
	if err := s.LogActivity(t.ProjectID, &t.ID, author, "comment.added", ""); err != nil {
		return nil, err
	}
	return c, nil
}

// ListComments returns a task's notes oldest first.
func (s *Store) ListComments(taskID string) ([]model.Comment, error) {
	rows, err := s.db.Query(`SELECT id,task_id,author,body,created_at FROM comments WHERE task_id=? ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Comment{}
	for rows.Next() {
		var c model.Comment
		var created string
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Body, &created); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Seed creates sample data for first-time use.
func (s *Store) Seed() error {
	empty, err := s.Empty()
	if err != nil || !empty {
		return err
	}
	p := &model.Project{Key: "GL", Name: "Glplan", Description: "Board de exemplo — edite à vontade"}
	if err := s.CreateProject(p); err != nil {
		return err
	}
	cols, err := s.ListColumns(p.ID)
	if err != nil {
		return err
	}
	byName := map[string]string{}
	for _, c := range cols {
		byName[c.Name] = c.ID
	}
	for _, m := range []string{"Ana", "Bruno", "Carla"} {
		if _, err := s.CreateMember(m, "", "#7c3aed"); err != nil {
			return err
		}
	}
	for _, l := range []struct{ name, color string }{
		{"bug", "#ef4444"}, {"feature", "#3b82f6"}, {"design", "#a855f7"}, {"infra", "#64748b"},
	} {
		if _, err := s.CreateLabel(p.ID, l.name, l.color); err != nil {
			return err
		}
	}
	samples := []struct {
		title, desc, column string
		priority            model.Priority
	}{
		{"Desenhar fluxo de onboarding", "Mapear os primeiros 5 minutos do usuário no TUI.", "Todo", model.PriorityHigh},
		{"Implementar sincronização via API", "Cliente HTTP com retry e cache local.", "In Progress", model.PriorityUrgent},
		{"Criar installer one-line", "curl | bash e irm | iex publicados no release.", "In Progress", model.PriorityHigh},
		{"Revisar atalhos de teclado", "Padronizar navegação hjkl e setas.", "Review", model.PriorityMedium},
		{"Estruturar schema do SQLite", "Tabelas de projects, tasks, labels e activity.", "Done", model.PriorityNone},
	}
	for _, sample := range samples {
		t, err := s.CreateTask(p.ID, byName[sample.column], sample.title, sample.desc, sample.priority, 0)
		if err != nil {
			return err
		}
		if sample.title == "Criar installer one-line" {
			_ = t
		}
	}
	return nil
}
