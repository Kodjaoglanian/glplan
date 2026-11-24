// Package model contains the core domain types shared by the API, TUI and MCP server.
package model

import (
	"crypto/rand"
	"time"
)

// Priority is a task's importance level.
type Priority string

const (
	PriorityNone   Priority = "none"
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

var Priorities = []Priority{
	PriorityNone, PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent,
}

// PriorityRank orders priorities from least to most urgent.
func PriorityRank(p Priority) int {
	switch p {
	case PriorityLow:
		return 1
	case PriorityMedium:
		return 2
	case PriorityHigh:
		return 3
	case PriorityUrgent:
		return 4
	default:
		return 0
	}
}

// Project is a workspace containing one kanban board.
type Project struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Columns     []Column  `json:"columns,omitempty"`
}

// Column is a lane on a project's board.
type Column struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Position  int    `json:"position"`
	WIPLimit  int    `json:"wip_limit"`
	IsDone    bool   `json:"is_done"`
}

// Label is a tag that can be attached to tasks.
type Label struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
}

// Member is a person who can be assigned to tasks.
type Member struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Color  string `json:"color"`
	Active bool   `json:"active"`
}

// Task is a unit of work on the board.
type Task struct {
	ID           string     `json:"id"`
	Ref          string     `json:"ref"`
	ProjectID    string     `json:"project_id"`
	ColumnID     string     `json:"column_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Priority     Priority   `json:"priority"`
	AssigneeID   *string    `json:"assignee_id"`
	DueDate      *time.Time `json:"due_date"`
	Position     float64    `json:"position"`
	Estimate     int        `json:"estimate"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	Labels       []Label    `json:"labels"`
	Subtasks     []Subtask  `json:"subtasks"`
	CommentCount int        `json:"comment_count"`
}

// Done reports whether the task sits in a column flagged as completed.
func (t Task) Done() bool { return t.CompletedAt != nil }

// Subtask is a checklist item on a task.
type Subtask struct {
	ID       string `json:"id"`
	TaskID   string `json:"task_id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Position int    `json:"position"`
}

// Comment is a note left on a task.
type Comment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Activity is an audit entry describing a change.
type Activity struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	TaskID    *string    `json:"task_id"`
	Actor     string     `json:"actor"`
	Action    string     `json:"action"`
	Detail    string     `json:"detail"`
	CreatedAt time.Time  `json:"created_at"`
}

// Board is a project together with its lanes and summary counters.
type Board struct {
	Project  Project            `json:"project"`
	Columns  []Column           `json:"columns"`
	Counts   map[string]int     `json:"counts"`
	Members  []Member           `json:"members"`
	Labels   []Label            `json:"labels"`
}

// TaskFilter narrows task listings.
type TaskFilter struct {
	ProjectID  string
	ColumnID   string
	AssigneeID string
	LabelID    string
	Priority   Priority
	Search     string
	IncludeDone bool
	Limit      int
}

// NewID returns a short, URL-safe random identifier.
func NewID(prefix string) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand should never fail on supported platforms; fall back to time.
		for i := range raw {
			raw[i] = byte(time.Now().UnixNano() >> (i * 3))
		}
	}
	id := make([]byte, len(raw))
	for i, v := range raw {
		id[i] = alphabet[int(v)%len(alphabet)]
	}
	if prefix == "" {
		return string(id)
	}
	return prefix + "_" + string(id)
}
