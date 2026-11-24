package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"glplan/internal/model"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

var defaultColumns = []struct {
	name   string
	color  string
	isDone bool
}{
	{"Backlog", "#64748b", false},
	{"Todo", "#3b82f6", false},
	{"In Progress", "#f59e0b", false},
	{"Review", "#a855f7", false},
	{"Done", "#22c55e", true},
}

// CreateProject inserts a project, its numbering sequence and a default board.
func (s *Store) CreateProject(p *model.Project) error {
	if p.ID == "" {
		p.ID = model.NewID("prj")
	}
	if p.Key == "" {
		p.Key = strings.ToUpper(p.ID[len("prj_"):min(len(p.ID), len("prj_")+3)])
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = nowTime()
		p.UpdatedAt = p.CreatedAt
	} else {
		p.UpdatedAt = nowTime()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO projects(id,key,name,description,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		p.ID, p.Key, p.Name, p.Description, fmtTime(p.CreatedAt), fmtTime(p.UpdatedAt)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO task_seq(project_id,n) VALUES(?,0)`, p.ID); err != nil {
		return err
	}
	for i, c := range defaultColumns {
		if _, err := tx.Exec(`INSERT INTO columns(id,project_id,name,color,position,wip_limit,is_done) VALUES(?,?,?,?,?,0,?)`,
			model.NewID("col"), p.ID, c.name, c.color, i, boolInt(c.isDone)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	p.Columns, err = s.ListColumns(p.ID)
	return err
}

// GetProject looks up a project by id or short key.
func (s *Store) GetProject(idOrKey string) (*model.Project, error) {
	row := s.db.QueryRow(`SELECT id,key,name,description,created_at,updated_at FROM projects WHERE id=? OR upper(key)=upper(?) LIMIT 1`, idOrKey, idOrKey)
	p := &model.Project{}
	var created, updated string
	if err := row.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.CreatedAt = parseStringTime(created)
	p.UpdatedAt = parseStringTime(updated)
	cols, err := s.ListColumns(p.ID)
	p.Columns = cols
	return p, err
}

// ListProjects returns every project.
func (s *Store) ListProjects() ([]model.Project, error) {
	rows, err := s.db.Query(`SELECT id,key,name,description,created_at,updated_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Project{}
	for rows.Next() {
		var p model.Project
		var created, updated string
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &created, &updated); err != nil {
			return nil, err
		}
		p.CreatedAt = parseStringTime(created)
		p.UpdatedAt = parseStringTime(updated)
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateProject changes a project's name and/or description.
func (s *Store) UpdateProject(id string, name, description *string) (*model.Project, error) {
	p, err := s.GetProject(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		p.Name = *name
	}
	if description != nil {
		p.Description = *description
	}
	p.UpdatedAt = nowTime()
	_, err = s.db.Exec(`UPDATE projects SET name=?,description=?,updated_at=? WHERE id=?`, p.Name, p.Description, fmtTime(p.UpdatedAt), p.ID)
	if err != nil {
		return nil, err
	}
	return s.GetProject(p.ID)
}

// DeleteProject removes a project and all of its data.
func (s *Store) DeleteProject(id string) error {
	res, err := s.db.Exec(`DELETE FROM projects WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListColumns returns a project's lanes in board order.
func (s *Store) ListColumns(projectID string) ([]model.Column, error) {
	rows, err := s.db.Query(`SELECT id,project_id,name,color,position,wip_limit,is_done FROM columns WHERE project_id=? ORDER BY position,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Column{}
	for rows.Next() {
		var c model.Column
		var done int
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Color, &c.Position, &c.WIPLimit, &done); err != nil {
			return nil, err
		}
		c.IsDone = done != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateColumn appends a lane to a project's board.
func (s *Store) CreateColumn(projectID, name, color string, wipLimit int) (*model.Column, error) {
	if _, err := s.GetProject(projectID); err != nil {
		return nil, err
	}
	var pos int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(position),-1)+1 FROM columns WHERE project_id=?`, projectID).Scan(&pos); err != nil {
		return nil, err
	}
	if color == "" {
		color = "#64748b"
	}
	c := &model.Column{ID: model.NewID("col"), ProjectID: projectID, Name: name, Color: color, Position: pos, WIPLimit: wipLimit}
	_, err := s.db.Exec(`INSERT INTO columns(id,project_id,name,color,position,wip_limit,is_done) VALUES(?,?,?,?,?,?,0)`,
		c.ID, c.ProjectID, c.Name, c.Color, c.Position, c.WIPLimit)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateColumn changes a lane's properties.
func (s *Store) UpdateColumn(id string, name, color *string, wipLimit *int, isDone *bool) (*model.Column, error) {
	c, err := s.getColumn(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		c.Name = *name
	}
	if color != nil {
		c.Color = *color
	}
	if wipLimit != nil {
		c.WIPLimit = *wipLimit
	}
	if isDone != nil {
		c.IsDone = *isDone
	}
	_, err = s.db.Exec(`UPDATE columns SET name=?,color=?,wip_limit=?,is_done=? WHERE id=?`, c.Name, c.Color, c.WIPLimit, boolInt(c.IsDone), c.ID)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) getColumn(id string) (*model.Column, error) {
	row := s.db.QueryRow(`SELECT id,project_id,name,color,position,wip_limit,is_done FROM columns WHERE id=?`, id)
	var c model.Column
	var done int
	if err := row.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Color, &c.Position, &c.WIPLimit, &done); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c.IsDone = done != 0
	return &c, nil
}

// DeleteColumn removes a lane and moves its tasks to the project's first lane.
func (s *Store) DeleteColumn(id string) error {
	c, err := s.getColumn(id)
	if err != nil {
		return err
	}
	cols, err := s.ListColumns(c.ProjectID)
	if err != nil {
		return err
	}
	if len(cols) <= 1 {
		return fmt.Errorf("cannot delete the only column")
	}
	var fallback string
	for _, other := range cols {
		if other.ID != id {
			fallback = other.ID
			break
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE tasks SET column_id=? WHERE column_id=?`, fallback, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM columns WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ReorderColumns sets lane order from the given list of ids.
func (s *Store) ReorderColumns(projectID string, ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		res, err := tx.Exec(`UPDATE columns SET position=? WHERE id=? AND project_id=?`, i, id, projectID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("column %s does not belong to project", id)
		}
	}
	return tx.Commit()
}

// ListLabels returns a project's labels.
func (s *Store) ListLabels(projectID string) ([]model.Label, error) {
	rows, err := s.db.Query(`SELECT id,project_id,name,color FROM labels WHERE project_id=? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Label{}
	for rows.Next() {
		var l model.Label
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CreateLabel adds a label to a project.
func (s *Store) CreateLabel(projectID, name, color string) (*model.Label, error) {
	if _, err := s.GetProject(projectID); err != nil {
		return nil, err
	}
	if color == "" {
		color = "#64748b"
	}
	l := &model.Label{ID: model.NewID("lbl"), ProjectID: projectID, Name: name, Color: color}
	_, err := s.db.Exec(`INSERT INTO labels(id,project_id,name,color) VALUES(?,?,?,?)`, l.ID, l.ProjectID, l.Name, l.Color)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// UpdateLabel changes a label's name and/or color.
func (s *Store) UpdateLabel(id string, name, color *string) (*model.Label, error) {
	var l model.Label
	if err := s.db.QueryRow(`SELECT id,project_id,name,color FROM labels WHERE id=?`, id).Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color); err != nil {
		return nil, notFound(err)
	}
	if name != nil {
		l.Name = *name
	}
	if color != nil {
		l.Color = *color
	}
	_, err := s.db.Exec(`UPDATE labels SET name=?,color=? WHERE id=?`, l.Name, l.Color, l.ID)
	return &l, err
}

// DeleteLabel removes a label from the project and all tasks.
func (s *Store) DeleteLabel(id string) error {
	res, err := s.db.Exec(`DELETE FROM labels WHERE id=?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}

// ListMembers returns all team members.
func (s *Store) ListMembers() ([]model.Member, error) {
	rows, err := s.db.Query(`SELECT id,name,email,color,active FROM members ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Member{}
	for rows.Next() {
		var m model.Member
		var active int
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Color, &active); err != nil {
			return nil, err
		}
		m.Active = active != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateMember adds a member to the team.
func (s *Store) CreateMember(name, email, color string) (*model.Member, error) {
	if color == "" {
		color = "#64748b"
	}
	m := &model.Member{ID: model.NewID("mem"), Name: name, Email: email, Color: color, Active: true}
	_, err := s.db.Exec(`INSERT INTO members(id,name,email,color,active) VALUES(?,?,?,?,1)`, m.ID, m.Name, m.Email, m.Color)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateMember changes a member's details.
func (s *Store) UpdateMember(id string, name, email, color *string, active *bool) (*model.Member, error) {
	var m model.Member
	var a int
	if err := s.db.QueryRow(`SELECT id,name,email,color,active FROM members WHERE id=?`, id).Scan(&m.ID, &m.Name, &m.Email, &m.Color, &a); err != nil {
		return nil, notFound(err)
	}
	m.Active = a != 0
	if name != nil {
		m.Name = *name
	}
	if email != nil {
		m.Email = *email
	}
	if color != nil {
		m.Color = *color
	}
	if active != nil {
		m.Active = *active
	}
	_, err := s.db.Exec(`UPDATE members SET name=?,email=?,color=?,active=? WHERE id=?`, m.Name, m.Email, m.Color, boolInt(m.Active), m.ID)
	return &m, err
}

// DeleteMember removes a member and unassigns their tasks.
func (s *Store) DeleteMember(id string) error {
	res, err := s.db.Exec(`DELETE FROM members WHERE id=?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func affected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
