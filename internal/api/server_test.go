package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"glplan/internal/api"
	"glplan/internal/model"
	"glplan/internal/store"
)

func newTestAPI(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "glplan-api-test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.Seed(); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	ts := httptest.NewServer(api.New(st, "").Handler())
	t.Cleanup(func() {
		ts.Close()
		_ = st.Close()
	})
	return ts, st
}

func getJSON[T any](t *testing.T, url string, out *T) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

func TestListTasksAcceptsProjectKey(t *testing.T) {
	ts, _ := newTestAPI(t)
	var tasks []model.Task
	getJSON(t, ts.URL+"/api/tasks?project=GL", &tasks)
	if len(tasks) == 0 {
		t.Fatal("expected seeded tasks when filtering by project key GL")
	}
}

func TestSearchAcceptsProjectKey(t *testing.T) {
	ts, _ := newTestAPI(t)
	var tasks []model.Task
	getJSON(t, ts.URL+"/api/search?project=GL&q=sqlite", &tasks)
	if len(tasks) == 0 {
		t.Fatal("expected search by project key GL to find seeded sqlite task")
	}
}

func TestCreateTaskPersistsEstimate(t *testing.T) {
	ts, st := newTestAPI(t)
	projects, err := st.ListProjects()
	if err != nil || len(projects) == 0 {
		t.Fatalf("list projects: %v", err)
	}
	body := strings.NewReader(`{"project_id":"` + projects[0].ID + `","title":"task com estimativa","estimate":8}`)
	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", body)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var task model.Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if task.Estimate != 8 {
		t.Fatalf("expected estimate 8, got %d", task.Estimate)
	}
}

func TestEmptyDueDateClearsExistingDate(t *testing.T) {
	ts, st := newTestAPI(t)
	tasks, err := st.ListTasks(model.TaskFilter{IncludeDone: true, Limit: 1})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("list tasks: %v", err)
	}
	due := time.Now().UTC().Add(24 * time.Hour)
	if _, err := st.UpdateTask(tasks[0].ID, store.TaskUpdate{SetDueDate: true, DueDate: &due}); err != nil {
		t.Fatalf("set due date: %v", err)
	}

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/tasks/"+tasks[0].ID, strings.NewReader(`{"due_date":""}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch task: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var task model.Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if task.DueDate != nil {
		t.Fatalf("expected empty due_date to clear date, got %v", task.DueDate)
	}
}

func TestBearerAuth(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "glplan-auth-test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	ts := httptest.NewServer(api.New(st, "secret").Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/projects")
	if err != nil {
		t.Fatalf("unauthenticated request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/projects", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", resp.StatusCode)
	}
}

func TestClientUpdateTaskCanAssignAndClearAssignee(t *testing.T) {
	ts, st := newTestAPI(t)
	client := api.NewClient(ts.URL, "")
	tasks, err := client.ListTasks(model.TaskFilter{IncludeDone: true, Limit: 1})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("list tasks: %v", err)
	}
	member, err := st.CreateMember("Marina", "", "#3355ff")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	updated, err := client.UpdateTask(tasks[0].ID, map[string]any{"assignee_id": member.ID})
	if err != nil {
		t.Fatalf("assign task: %v", err)
	}
	if updated.AssigneeID == nil || *updated.AssigneeID != member.ID {
		t.Fatalf("expected assignee %q, got %#v", member.ID, updated.AssigneeID)
	}
	updated, err = client.UpdateTask(tasks[0].ID, map[string]any{"assignee_id": nil})
	if err != nil {
		t.Fatalf("clear assignee: %v", err)
	}
	if updated.AssigneeID != nil {
		t.Fatalf("expected assignee to be cleared, got %#v", updated.AssigneeID)
	}
}

func TestMoveTaskThroughAPIUpdatesCompletion(t *testing.T) {
	ts, st := newTestAPI(t)
	client := api.NewClient(ts.URL, "")
	projects, err := st.ListProjects()
	if err != nil || len(projects) == 0 {
		t.Fatalf("list projects: %v", err)
	}
	cols, err := st.ListColumns(projects[0].ID)
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
	tasks, err := client.ListTasks(model.TaskFilter{ProjectID: projects[0].ID, IncludeDone: true})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("list tasks: %v", err)
	}
	var task model.Task
	for _, candidate := range tasks {
		if !candidate.Done() {
			task = candidate
			break
		}
	}
	moved, err := client.MoveTask(task.ID, done.ID, "", nil)
	if err != nil {
		t.Fatalf("move to done: %v", err)
	}
	if moved.CompletedAt == nil {
		t.Fatal("expected completed_at after moving to done")
	}
	moved, err = client.MoveTask(task.ID, backlog.ID, "", nil)
	if err != nil {
		t.Fatalf("move back: %v", err)
	}
	if moved.CompletedAt != nil {
		t.Fatal("expected completed_at to be cleared after leaving done")
	}
}

func TestNestedProjectRoutesAcceptProjectKey(t *testing.T) {
	ts, _ := newTestAPI(t)
	var cols []model.Column
	getJSON(t, ts.URL+"/api/projects/GL/columns", &cols)
	if len(cols) == 0 {
		t.Fatal("expected columns when using project key GL")
	}
	var labels []model.Label
	getJSON(t, ts.URL+"/api/projects/GL/labels", &labels)
	if len(labels) == 0 {
		t.Fatal("expected labels when using project key GL")
	}
	var activity []model.Activity
	getJSON(t, ts.URL+"/api/projects/GL/activity", &activity)
	if len(activity) == 0 {
		t.Fatal("expected activity when using project key GL")
	}
}

func TestAPIErrorUnwrapsJSONErrorBody(t *testing.T) {
	err := &api.APIError{Status: 400, Body: `{"error":"title is required"}`}
	if got := err.Error(); got != "API 400: title is required" {
		t.Fatalf("unexpected error text %q", got)
	}
}
