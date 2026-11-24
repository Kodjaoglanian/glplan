package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"glplan/internal/model"
)

// Client is a small HTTP client for the glplan API, used by the TUI and MCP server.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient builds a client. An empty base URL defaults to http://127.0.0.1:9876.
func NewClient(base, token string) *Client {
	if base == "" {
		base = "http://127.0.0.1:9876"
	}
	return &Client{
		BaseURL: strings.TrimRight(base, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// APIError is a non-2xx response from the server.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(e.Body)
	var parsed struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(body), &parsed) == nil && parsed.Error != "" {
		body = parsed.Error
	}
	return fmt.Sprintf("API %d: %s", e.Status, body)
}

func (c *Client) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach glplan API at %s: %w (is 'glplan serve' running?)", c.BaseURL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{Status: resp.StatusCode, Body: string(data)}
	}
	if out == nil || len(data) == 0 || resp.StatusCode == 204 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Health checks connectivity.
func (c *Client) Health() error { return c.do("GET", "/api/health", nil, nil) }

// ListProjects returns all projects.
func (c *Client) ListProjects() ([]model.Project, error) {
	var out []model.Project
	err := c.do("GET", "/api/projects", nil, &out)
	return out, err
}

// CreateProject creates a workspace.
func (c *Client) CreateProject(name, key, description string) (*model.Project, error) {
	out := &model.Project{}
	err := c.do("POST", "/api/projects", map[string]any{"name": name, "key": key, "description": description}, out)
	return out, err
}

// GetProject looks up a project by id or key.
func (c *Client) GetProject(id string) (*model.Project, error) {
	out := &model.Project{}
	err := c.do("GET", "/api/projects/"+url.PathEscape(id), nil, out)
	return out, err
}

// GetBoard loads the full board for a project.
func (c *Client) GetBoard(id string) (*model.Board, error) {
	out := &model.Board{}
	err := c.do("GET", "/api/projects/"+url.PathEscape(id)+"/board", nil, out)
	return out, err
}

// ListColumns returns a project's lanes.
func (c *Client) ListColumns(projectID string) ([]model.Column, error) {
	var out []model.Column
	err := c.do("GET", "/api/projects/"+url.PathEscape(projectID)+"/columns", nil, &out)
	return out, err
}

// CreateColumn appends a lane.
func (c *Client) CreateColumn(projectID, name, color string, wip int) (*model.Column, error) {
	out := &model.Column{}
	err := c.do("POST", "/api/projects/"+url.PathEscape(projectID)+"/columns", map[string]any{"name": name, "color": color, "wip_limit": wip}, out)
	return out, err
}

// ListLabels returns a project's labels.
func (c *Client) ListLabels(projectID string) ([]model.Label, error) {
	var out []model.Label
	err := c.do("GET", "/api/projects/"+url.PathEscape(projectID)+"/labels", nil, &out)
	return out, err
}

// CreateLabel adds a label.
func (c *Client) CreateLabel(projectID, name, color string) (*model.Label, error) {
	out := &model.Label{}
	err := c.do("POST", "/api/projects/"+url.PathEscape(projectID)+"/labels", map[string]any{"name": name, "color": color}, out)
	return out, err
}

// ListMembers returns the team.
func (c *Client) ListMembers() ([]model.Member, error) {
	var out []model.Member
	err := c.do("GET", "/api/members", nil, &out)
	return out, err
}

// CreateMember adds a person to the team.
func (c *Client) CreateMember(name, email, color string) (*model.Member, error) {
	out := &model.Member{}
	err := c.do("POST", "/api/members", map[string]any{"name": name, "email": email, "color": color}, out)
	return out, err
}

// ListTasks returns tasks, optionally filtered.
func (c *Client) ListTasks(filter model.TaskFilter) ([]model.Task, error) {
	q := url.Values{}
	if filter.ProjectID != "" {
		q.Set("project", filter.ProjectID)
	}
	if filter.ColumnID != "" {
		q.Set("column", filter.ColumnID)
	}
	if filter.AssigneeID != "" {
		q.Set("assignee", filter.AssigneeID)
	}
	if filter.LabelID != "" {
		q.Set("label", filter.LabelID)
	}
	if filter.Priority != "" {
		q.Set("priority", string(filter.Priority))
	}
	if filter.Search != "" {
		q.Set("q", filter.Search)
	}
	if filter.IncludeDone {
		q.Set("done", "true")
	}
	if filter.Limit > 0 {
		q.Set("limit", fmt.Sprint(filter.Limit))
	}
	path := "/api/tasks"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out []model.Task
	err := c.do("GET", path, nil, &out)
	return out, err
}

// GetTask looks up a task by id or reference.
func (c *Client) GetTask(id string) (*model.Task, error) {
	out := &model.Task{}
	err := c.do("GET", "/api/tasks/"+url.PathEscape(id), nil, out)
	return out, err
}

// CreateTaskInput describes a new task.
type CreateTaskInput struct {
	ProjectID   string
	ColumnID    string
	Title       string
	Description string
	Priority    model.Priority
	Estimate    int
	LabelIDs    []string
}

// CreateTask adds a task to a board.
func (c *Client) CreateTask(in CreateTaskInput) (*model.Task, error) {
	payload := map[string]any{
		"project_id": in.ProjectID, "column_id": in.ColumnID, "title": in.Title,
		"description": in.Description, "priority": string(in.Priority), "estimate": in.Estimate,
		"label_ids": in.LabelIDs,
	}
	out := &model.Task{}
	err := c.do("POST", "/api/tasks", payload, out)
	return out, err
}

// UpdateTask sends a partial task update.
func (c *Client) UpdateTask(id string, patch map[string]any) (*model.Task, error) {
	out := &model.Task{}
	err := c.do("PATCH", "/api/tasks/"+url.PathEscape(id), patch, out)
	return out, err
}

// DeleteTask removes a task.
func (c *Client) DeleteTask(id string) error {
	return c.do("DELETE", "/api/tasks/"+url.PathEscape(id), nil, nil)
}

// MoveTask moves a task to a column or position.
func (c *Client) MoveTask(id, columnID, beforeID string, position *float64) (*model.Task, error) {
	out := &model.Task{}
	err := c.do("POST", "/api/tasks/"+url.PathEscape(id)+"/move", map[string]any{
		"column_id": columnID, "before_id": beforeID, "position": position,
	}, out)
	return out, err
}

// AddComment leaves a note on a task.
func (c *Client) AddComment(taskID, author, body string) error {
	return c.do("POST", "/api/tasks/"+url.PathEscape(taskID)+"/comments", map[string]any{"author": author, "body": body}, nil)
}

// ListComments returns a task's notes.
func (c *Client) ListComments(taskID string) ([]model.Comment, error) {
	var out []model.Comment
	err := c.do("GET", "/api/tasks/"+url.PathEscape(taskID)+"/comments", nil, &out)
	return out, err
}

// AddSubtask adds a checklist item.
func (c *Client) AddSubtask(taskID, title string) error {
	return c.do("POST", "/api/tasks/"+url.PathEscape(taskID)+"/subtasks", map[string]any{"title": title}, nil)
}

// UpdateSubtask changes a checklist item.
func (c *Client) UpdateSubtask(id string, patch map[string]any) error {
	return c.do("PATCH", "/api/subtasks/"+url.PathEscape(id), patch, nil)
}

// ListActivity returns recent changes for a project.
func (c *Client) ListActivity(projectID string, limit int) ([]model.Activity, error) {
	var out []model.Activity
	err := c.do("GET", "/api/projects/"+url.PathEscape(projectID)+"/activity?limit="+fmt.Sprint(limit), nil, &out)
	return out, err
}

// Search finds tasks by text.
func (c *Client) Search(projectID, query string) ([]model.Task, error) {
	q := url.Values{}
	q.Set("q", query)
	if projectID != "" {
		q.Set("project", projectID)
	}
	var out []model.Task
	err := c.do("GET", "/api/search?"+q.Encode(), nil, &out)
	return out, err
}
