// Package store persists the board in SQLite.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is a handle to the SQLite database.
type Store struct {
	db *sql.DB
}

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;

CREATE TABLE IF NOT EXISTS projects (
	id          TEXT PRIMARY KEY,
	key         TEXT NOT NULL UNIQUE,
	name        TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS columns (
	id         TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	color      TEXT NOT NULL DEFAULT '',
	position   INTEGER NOT NULL DEFAULT 0,
	wip_limit  INTEGER NOT NULL DEFAULT 0,
	is_done    INTEGER NOT NULL DEFAULT 0,
	UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS labels (
	id         TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	color      TEXT NOT NULL DEFAULT '#64748b',
	UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS members (
	id    TEXT PRIMARY KEY,
	name  TEXT NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	color TEXT NOT NULL DEFAULT '#64748b',
	active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS task_seq (
	project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
	n INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tasks (
	id           TEXT PRIMARY KEY,
	ref          TEXT NOT NULL UNIQUE,
	project_id   TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	column_id    TEXT NOT NULL REFERENCES columns(id),
	title        TEXT NOT NULL,
	description  TEXT NOT NULL DEFAULT '',
	priority     TEXT NOT NULL DEFAULT 'none',
	assignee_id  TEXT REFERENCES members(id) ON DELETE SET NULL,
	due_date     TEXT,
	position     REAL NOT NULL DEFAULT 0,
	estimate     INTEGER NOT NULL DEFAULT 0,
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL,
	completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_column ON tasks(column_id);

CREATE TABLE IF NOT EXISTS task_labels (
	task_id  TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	label_id TEXT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
	PRIMARY KEY (task_id, label_id)
);

CREATE TABLE IF NOT EXISTS subtasks (
	id        TEXT PRIMARY KEY,
	task_id   TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	title     TEXT NOT NULL,
	done      INTEGER NOT NULL DEFAULT 0,
	position  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS comments (
	id        TEXT PRIMARY KEY,
	task_id   TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	author    TEXT NOT NULL DEFAULT '',
	body      TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS activities (
	id         TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	task_id    TEXT REFERENCES tasks(id) ON DELETE SET NULL,
	actor      TEXT NOT NULL DEFAULT '',
	action     TEXT NOT NULL,
	detail     TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activities_project ON activities(project_id, created_at DESC);
`

// Open creates or opens the database at path and migrates the schema.
func Open(path string) (*Store, error) {
	if path == "" {
		path = "glplan.db"
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite writes are serialized; keeps things simple and safe.
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Empty reports whether there are no projects yet.
func (s *Store) Empty() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&n)
	return n == 0, err
}

func now() string { return fmtTime(time.Now()) }

func nowTime() time.Time { return time.Now().UTC() }

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseStringTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseTime(v sql.NullString) *time.Time {
	if !v.Valid || v.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, v.String)
	if err != nil {
		return nil
	}
	return &t
}

func mustTime(v string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, v)
	return t
}

func nullString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullTime(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.UTC().Format(time.RFC3339Nano)
}

func like(q string) string {
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return "%" + q + "%"
}
