// Package api exposes the board over a small JSON HTTP API.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"glplan/internal/model"
	"glplan/internal/store"
)

// Server serves the REST API.
type Server struct {
	store *store.Store
	token string
}

// New creates an API server. token, when set, is required as a bearer token.
func New(s *store.Store, token string) *Server { return &Server{store: s, token: token} }

// Handler builds the HTTP router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "version": version})
	})

	mux.HandleFunc("GET /api/projects", s.listProjects)
	mux.HandleFunc("POST /api/projects", s.createProject)
	mux.HandleFunc("GET /api/projects/{id}", s.getProject)
	mux.HandleFunc("PATCH /api/projects/{id}", s.updateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)
	mux.HandleFunc("GET /api/projects/{id}/board", s.getBoard)

	mux.HandleFunc("GET /api/projects/{id}/columns", s.listColumns)
	mux.HandleFunc("POST /api/projects/{id}/columns", s.createColumn)
	mux.HandleFunc("PATCH /api/projects/{id}/columns", s.reorderColumns)
	mux.HandleFunc("PATCH /api/columns/{id}", s.updateColumn)
	mux.HandleFunc("DELETE /api/columns/{id}", s.deleteColumn)

	mux.HandleFunc("GET /api/projects/{id}/labels", s.listLabels)
	mux.HandleFunc("POST /api/projects/{id}/labels", s.createLabel)
	mux.HandleFunc("PATCH /api/labels/{id}", s.updateLabel)
	mux.HandleFunc("DELETE /api/labels/{id}", s.deleteLabel)

	mux.HandleFunc("GET /api/members", s.listMembers)
	mux.HandleFunc("POST /api/members", s.createMember)
	mux.HandleFunc("PATCH /api/members/{id}", s.updateMember)
	mux.HandleFunc("DELETE /api/members/{id}", s.deleteMember)

	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("POST /api/tasks", s.createTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.getTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", s.updateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.deleteTask)
	mux.HandleFunc("POST /api/tasks/{id}/move", s.moveTask)
	mux.HandleFunc("POST /api/tasks/{id}/labels", s.setLabels)
	mux.HandleFunc("GET /api/tasks/{id}/comments", s.listComments)
	mux.HandleFunc("POST /api/tasks/{id}/comments", s.addComment)
	mux.HandleFunc("POST /api/tasks/{id}/subtasks", s.addSubtask)
	mux.HandleFunc("PATCH /api/subtasks/{id}", s.updateSubtask)
	mux.HandleFunc("DELETE /api/subtasks/{id}", s.deleteSubtask)

	mux.HandleFunc("GET /api/projects/{id}/activity", s.listActivity)
	mux.HandleFunc("GET /api/search", s.search)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			writeJSON(w, 200, map[string]string{"name": "glplan", "api": "/api", "version": version})
			return
		}
		writeErr(w, store.ErrNotFound)
	})

	return s.auth(mux)
}

// version can be overridden at build time with -ldflags.
var version = "dev"

// SetVersion sets the version reported by the API.
func SetVersion(v string) { version = v }

func (s *Server) auth(next http.Handler) http.Handler {
	if s.token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h != "Bearer "+s.token {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	code := 500
	switch {
	case errors.Is(err, store.ErrNotFound):
		code = 404
	case isBadRequest(err):
		code = 400
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

type badRequest string

func (b badRequest) Error() string { return string(b) }

func isBadRequest(err error) bool {
	var b badRequest
	return errors.As(err, &b)
}

func bad(format string, args ...any) error { return badRequest(fmt.Sprintf(format, args...)) }

func decode(r *http.Request, dst any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return bad("read body: %v", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return bad("invalid JSON: %v", err)
	}
	return nil
}

func decodeRaw(r *http.Request) (map[string]json.RawMessage, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	raw := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(body))) == 0 {
		return raw, nil
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, bad("invalid JSON: %v", err)
	}
	return raw, nil
}

func str(raw map[string]json.RawMessage, key string) *string {
	v, ok := raw[key]
	if !ok || string(v) == "null" {
		return nil
	}
	var s string
	if json.Unmarshal(v, &s) != nil {
		return nil
	}
	return &s
}

func intVal(raw map[string]json.RawMessage, key string) *int {
	v, ok := raw[key]
	if !ok || string(v) == "null" {
		return nil
	}
	var n int
	if json.Unmarshal(v, &n) != nil {
		return nil
	}
	return &n
}

func boolVal(raw map[string]json.RawMessage, key string) *bool {
	v, ok := raw[key]
	if !ok || string(v) == "null" {
		return nil
	}
	var b bool
	if json.Unmarshal(v, &b) != nil {
		return nil
	}
	return &b
}

func floatVal(raw map[string]json.RawMessage, key string) *float64 {
	v, ok := raw[key]
	if !ok || string(v) == "null" {
		return nil
	}
	var f float64
	if json.Unmarshal(v, &f) != nil {
		return nil
	}
	return &f
}

func stringsVal(raw map[string]json.RawMessage, key string) *[]string {
	v, ok := raw[key]
	if !ok || string(v) == "null" {
		return nil
	}
	var out []string
	if json.Unmarshal(v, &out) != nil {
		return nil
	}
	return &out
}

func present(raw map[string]json.RawMessage, key string) bool {
	_, ok := raw[key]
	return ok
}

func isNull(raw map[string]json.RawMessage, key string) bool {
	v, ok := raw[key]
	return ok && string(v) == "null"
}

func parseDate(raw map[string]json.RawMessage, key string) (*time.Time, error) {
	if !present(raw, key) {
		return nil, nil
	}
	if isNull(raw, key) {
		return nil, nil
	}
	s := str(raw, key)
	if s == nil {
		return nil, bad("invalid %s", key)
	}
	if strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, *s); err == nil {
			utc := t.UTC()
			return &utc, nil
		}
	}
	return nil, bad("invalid date %q", *s)
}

func require(raw map[string]json.RawMessage, key string) (string, error) {
	s := str(raw, key)
	if s == nil || strings.TrimSpace(*s) == "" {
		return "", bad("%s is required", key)
	}
	return *s, nil
}

func projectID(r *http.Request) string { return r.PathValue("id") }

func (s *Server) resolveProjectID(idOrKey string) (string, error) {
	p, err := s.store.GetProject(idOrKey)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func (s *Server) resolveProjectFilter(idOrKey string) string {
	if idOrKey == "" {
		return ""
	}
	if p, err := s.store.GetProject(idOrKey); err == nil {
		return p.ID
	}
	return idOrKey
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListProjects()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	name, err := require(raw, "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	p := &model.Project{Name: name, Key: valueOr(str(raw, "key"), ""), Description: valueOr(str(raw, "description"), "")}
	if err := s.store.CreateProject(p); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, p)
}

func valueOr(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	p, err := s.store.UpdateProject(projectID(r), str(raw, "name"), str(raw, "description"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteProject(projectID(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) getBoard(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	cols, err := s.store.ListColumns(p.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	tasks, err := s.store.ListTasks(model.TaskFilter{ProjectID: p.ID, IncludeDone: true})
	if err != nil {
		writeErr(w, err)
		return
	}
	labels, err := s.store.ListLabels(p.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	members, err := s.store.ListMembers()
	if err != nil {
		writeErr(w, err)
		return
	}
	counts := map[string]int{}
	for _, t := range tasks {
		counts[t.ColumnID]++
	}
	writeJSON(w, 200, model.Board{Project: *p, Columns: cols, Counts: counts, Labels: labels, Members: members})
}

func (s *Server) listColumns(w http.ResponseWriter, r *http.Request) {
	project, err := s.resolveProjectID(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	cols, err := s.store.ListColumns(project)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, cols)
}

func (s *Server) createColumn(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	name, err := require(raw, "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	project, err := s.resolveProjectID(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	c, err := s.store.CreateColumn(project, name, valueOr(str(raw, "color"), ""), intOr(intVal(raw, "wip_limit"), 0))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, c)
}

func queryInt(q url.Values, key string, fallback int) int {
	n, err := strconv.Atoi(q.Get(key))
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func intOr(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}

func (s *Server) reorderColumns(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ids := stringsVal(raw, "ids")
	if ids == nil {
		writeErr(w, bad("ids is required"))
		return
	}
	project, err := s.resolveProjectID(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.store.ReorderColumns(project, *ids); err != nil {
		writeErr(w, err)
		return
	}
	cols, _ := s.store.ListColumns(project)
	writeJSON(w, 200, cols)
}

func (s *Server) updateColumn(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	c, err := s.store.UpdateColumn(projectID(r), str(raw, "name"), str(raw, "color"), intVal(raw, "wip_limit"), boolVal(raw, "is_done"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, c)
}

func (s *Server) deleteColumn(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteColumn(projectID(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) listLabels(w http.ResponseWriter, r *http.Request) {
	project, err := s.resolveProjectID(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	out, err := s.store.ListLabels(project)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) createLabel(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	name, err := require(raw, "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	project, err := s.resolveProjectID(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	l, err := s.store.CreateLabel(project, name, valueOr(str(raw, "color"), ""))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, l)
}

func (s *Server) updateLabel(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	l, err := s.store.UpdateLabel(projectID(r), str(raw, "name"), str(raw, "color"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, l)
}

func (s *Server) deleteLabel(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteLabel(projectID(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListMembers()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) createMember(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	name, err := require(raw, "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	m, err := s.store.CreateMember(name, valueOr(str(raw, "email"), ""), valueOr(str(raw, "color"), ""))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, m)
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	m, err := s.store.UpdateMember(projectID(r), str(raw, "name"), str(raw, "email"), str(raw, "color"), boolVal(raw, "active"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) deleteMember(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteMember(projectID(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := model.TaskFilter{
		ProjectID:   s.resolveProjectFilter(q.Get("project")),
		ColumnID:    q.Get("column"),
		AssigneeID:  q.Get("assignee"),
		LabelID:     q.Get("label"),
		Priority:    model.Priority(q.Get("priority")),
		Search:      q.Get("q"),
		IncludeDone: q.Get("done") == "true",
		Limit:       queryInt(q, "limit", 0),
	}
	tasks, err := s.store.ListTasks(f)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, tasks)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	title, err := require(raw, "title")
	if err != nil {
		writeErr(w, err)
		return
	}
	project := valueOr(str(raw, "project_id"), "")
	if project == "" {
		projects, err := s.store.ListProjects()
		if err != nil || len(projects) == 0 {
			writeErr(w, bad("no project available"))
			return
		}
		project = projects[0].ID
	}
	priority := model.Priority(valueOr(str(raw, "priority"), string(model.PriorityNone)))
	t, err := s.store.CreateTask(project, valueOr(str(raw, "column_id"), ""), title,
		valueOr(str(raw, "description"), ""), priority, intOr(intVal(raw, "estimate"), 0))
	if err != nil {
		writeErr(w, err)
		return
	}
	if ids := stringsVal(raw, "label_ids"); ids != nil {
		if err := s.store.SetTaskLabels(t.ID, *ids); err != nil {
			writeErr(w, err)
			return
		}
		t, _ = s.store.GetTask(t.ID)
	}
	writeJSON(w, 201, t)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetTask(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, t)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	u := store.TaskUpdate{
		Title:       str(raw, "title"),
		Description: str(raw, "description"),
		ColumnID:    str(raw, "column_id"),
		Position:    floatVal(raw, "position"),
		LabelIDs:    stringsVal(raw, "label_ids"),
		Actor:       valueOr(str(raw, "actor"), "system"),
	}
	if p := str(raw, "priority"); p != nil {
		priority := model.Priority(*p)
		u.Priority = &priority
	}
	u.Estimate = intVal(raw, "estimate")
	if present(raw, "assignee_id") {
		u.SetAssignee = true
		if !isNull(raw, "assignee_id") {
			u.AssigneeID = str(raw, "assignee_id")
		}
	}
	if present(raw, "due_date") {
		u.SetDueDate = true
		u.DueDate, err = parseDate(raw, "due_date")
		if err != nil {
			writeErr(w, err)
			return
		}
	}
	t, err := s.store.UpdateTask(projectID(r), u)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, t)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTask(projectID(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) moveTask(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	t, err := s.store.MoveTask(projectID(r), valueOr(str(raw, "column_id"), ""),
		valueOr(str(raw, "before_id"), ""), floatVal(raw, "position"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, t)
}

func (s *Server) setLabels(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	ids := stringsVal(raw, "label_ids")
	if ids == nil {
		writeErr(w, bad("label_ids is required"))
		return
	}
	if err := s.store.SetTaskLabels(projectID(r), *ids); err != nil {
		writeErr(w, err)
		return
	}
	t, _ := s.store.GetTask(projectID(r))
	writeJSON(w, 200, t)
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListComments(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	body, err := require(raw, "body")
	if err != nil {
		writeErr(w, err)
		return
	}
	c, err := s.store.AddComment(projectID(r), valueOr(str(raw, "author"), ""), body)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, c)
}

func (s *Server) addSubtask(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	title, err := require(raw, "title")
	if err != nil {
		writeErr(w, err)
		return
	}
	sub, err := s.store.AddSubtask(projectID(r), title)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, sub)
}

func (s *Server) updateSubtask(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeRaw(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	sub, err := s.store.UpdateSubtask(projectID(r), str(raw, "title"), boolVal(raw, "done"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, sub)
}

func (s *Server) deleteSubtask(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteSubtask(projectID(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) listActivity(w http.ResponseWriter, r *http.Request) {
	project, err := s.resolveProjectID(projectID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	out, err := s.store.ListActivity(project, queryInt(r.URL.Query(), "limit", 30))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.ListTasks(model.TaskFilter{
		ProjectID:   s.resolveProjectFilter(r.URL.Query().Get("project")),
		Search:      r.URL.Query().Get("q"),
		IncludeDone: true,
		Limit:       100,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, tasks)
}

// Listen starts the HTTP server on addr.
func (s *Server) Listen(addr string) error {
	if addr == "" {
		addr = ":9876"
	}
	fmt.Fprintf(os.Stderr, "glplan API listening on http://%s\n", addr)
	return http.ListenAndServe(addr, s.Handler())
}
