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
	if err := s.LogActivity(t.ProjectID, &t.ID, author, "comment.added", t.Ref+" · comentário"); err != nil {
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

// Bootstrap creates a clean first project without demo tasks.
func (s *Store) Bootstrap() error {
	empty, err := s.Empty()
	if err != nil || !empty {
		return err
	}
	p := &model.Project{Key: "GL", Name: "Glplan", Description: "Meu board"}
	return s.CreateProject(p)
}

// Seed creates explicit sample data for demos and tests.
func (s *Store) Seed() error {
	projects, err := s.ListProjects()
	if err != nil {
		return err
	}
	var p *model.Project
	if len(projects) == 0 {
		p = &model.Project{Key: "GL", Name: "Glplan", Description: "Board de exemplo — edite à vontade"}
		if err := s.CreateProject(p); err != nil {
			return err
		}
	} else {
		p = &projects[0]
		var taskCount int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=?`, p.ID).Scan(&taskCount); err != nil {
			return err
		}
		if taskCount > 0 {
			return nil
		}
	}
	cols, err := s.ListColumns(p.ID)
	if err != nil {
		return err
	}
	byName := map[string]string{}
	for _, c := range cols {
		byName[c.Name] = c.ID
	}

	members := map[string]string{}
	for _, m := range []struct{ name, color string }{
		{"Ana Lima", "#3BE4D8"}, {"Bruno Reis", "#63A9FF"}, {"Carla Nunes", "#A98BFF"},
	} {
		var existingID string
		err := s.db.QueryRow(`SELECT id FROM members WHERE name=? LIMIT 1`, m.name).Scan(&existingID)
		if err == nil {
			members[m.name] = existingID
			continue
		}
		member, err := s.CreateMember(m.name, "", m.color)
		if err != nil {
			return err
		}
		members[m.name] = member.ID
	}

	labels := map[string]string{}
	if existing, err := s.ListLabels(p.ID); err == nil {
		for _, label := range existing {
			labels[label.Name] = label.ID
		}
	}
	for _, l := range []struct{ name, color string }{
		{"bug", "#FF5263"}, {"feature", "#63A9FF"}, {"design", "#A98BFF"}, {"infra", "#91A3AA"},
	} {
		if labels[l.name] != "" {
			continue
		}
		label, err := s.CreateLabel(p.ID, l.name, l.color)
		if err != nil {
			return err
		}
		labels[l.name] = label.ID
	}

	samples := []struct {
		title, desc, column string
		priority            model.Priority
		estimate            int
		assignee            string
		labels              []string
		subtasks            []string
		dueDays             int
		comment             string
	}{
		{"Desenhar fluxo de onboarding", "Mapear os primeiros cinco minutos: abrir, entender o board e criar a primeira task.", "Backlog", model.PriorityHigh, 3, "Carla Nunes", []string{"design"}, []string{"Mapear primeira execução", "Desenhar estados vazios"}, 4, "A primeira execução precisa funcionar sem servidor manual."},
		{"Definir backup do SQLite", "Decidir formato, frequência e retenção para instalações locais.", "Backlog", model.PriorityMedium, 2, "", []string{"infra"}, nil, 0, ""},
		{"Corrigir hidratação de detalhes", "Garantir que labels e checklist apareçam ao abrir uma task por ID ou referência.", "Todo", model.PriorityUrgent, 1, "Ana Lima", []string{"bug"}, []string{"Reproduzir no smoke test", "Adicionar regressão"}, 1, "Esse bug afeta TUI e MCP."},
		{"Implementar sincronização via API", "Cliente HTTP consistente, mensagens de erro úteis e refresh previsível.", "Todo", model.PriorityHigh, 5, "Bruno Reis", []string{"feature", "infra"}, []string{"Health check", "Listar board", "Retry visual"}, 2, ""},
		{"Criar installer one-line", "curl | bash e irm | iex publicados no release com checksum.", "In Progress", model.PriorityHigh, 3, "Bruno Reis", []string{"infra"}, []string{"Linux amd64", "Windows PowerShell"}, 3, "Validar em máquina limpa antes do release."},
		{"Revisar atalhos de teclado", "Padronizar navegação hjkl, comandos contextuais e ajuda descoberta.", "Review", model.PriorityMedium, 2, "Carla Nunes", []string{"design"}, nil, 0, ""},
		{"Estruturar schema do SQLite", "Tabelas de projects, tasks, labels, comments, subtasks e activity.", "Done", model.PriorityNone, 3, "Ana Lima", []string{"infra"}, []string{"Schema inicial", "Índices"}, 0, ""},
	}
	for _, sample := range samples {
		t, err := s.CreateTask(p.ID, byName[sample.column], sample.title, sample.desc, sample.priority, sample.estimate)
		if err != nil {
			return err
		}
		if len(sample.labels) > 0 {
			ids := make([]string, 0, len(sample.labels))
			for _, name := range sample.labels {
				ids = append(ids, labels[name])
			}
			if err := s.SetTaskLabels(t.ID, ids); err != nil {
				return err
			}
		}
		if sample.assignee != "" || sample.dueDays != 0 {
			u := TaskUpdate{}
			if sample.assignee != "" {
				id := members[sample.assignee]
				u.SetAssignee = true
				u.AssigneeID = &id
			}
			if sample.dueDays != 0 {
				due := time.Now().UTC().Add(time.Duration(sample.dueDays) * 24 * time.Hour)
				u.SetDueDate = true
				u.DueDate = &due
			}
			if _, err := s.UpdateTask(t.ID, u); err != nil {
				return err
			}
		}
		for i, title := range sample.subtasks {
			sub, err := s.AddSubtask(t.ID, title)
			if err != nil {
				return err
			}
			if i == 0 && sample.column == "Done" {
				done := true
				if _, err := s.UpdateSubtask(sub.ID, nil, &done); err != nil {
					return err
				}
			}
		}
		if sample.comment != "" {
			if _, err := s.AddComment(t.ID, "Ana Lima", sample.comment); err != nil {
				return err
			}
		}
	}
	return nil
}
