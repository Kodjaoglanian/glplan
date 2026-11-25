package store

import (
	"sync"

	"glplan/internal/model"
)

// EventListener receives domain events after successful mutations.
type EventListener func(model.Event)

var (
	listenersMu sync.RWMutex
	listeners   = map[*Store][]EventListener{}
)

// OnEvent registers a listener called synchronously after each event.
// Listeners must be fast and non-blocking; heavy work belongs in a queue.
func (s *Store) OnEvent(l EventListener) {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	listeners[s] = append(listeners[s], l)
}

func (s *Store) emit(evType, projectID string, taskID *string, actor, detail string) {
	listenersMu.RLock()
	ls := listeners[s]
	listenersMu.RUnlock()
	if len(ls) == 0 {
		return
	}
	ev := model.Event{
		ID:        model.NewID("evt"),
		Type:      evType,
		ProjectID: projectID,
		Actor:     actor,
		Detail:    detail,
		CreatedAt: nowTime(),
	}
	if taskID != nil {
		ev.TaskID = *taskID
	}
	for _, l := range ls {
		l(ev)
	}
}

// CreateWebhook registers a webhook endpoint. projectID may be empty for all projects.
func (s *Store) CreateWebhook(projectID, url, secret, events string) (*model.Webhook, error) {
	if projectID != "" {
		if _, err := s.GetProject(projectID); err != nil {
			return nil, err
		}
	}
	if events == "" {
		events = "*"
	}
	wh := &model.Webhook{ID: model.NewID("whk"), ProjectID: projectID, URL: url, Secret: secret, Events: events, Active: true, CreatedAt: nowTime()}
	var project any
	if wh.ProjectID != "" {
		project = wh.ProjectID
	}
	_, err := s.db.Exec(`INSERT INTO webhooks(id,project_id,url,secret,events,active,created_at) VALUES(?,?,?,?,?,1,?)`,
		wh.ID, project, wh.URL, wh.Secret, wh.Events, fmtTime(wh.CreatedAt))
	if err != nil {
		return nil, err
	}
	return wh, nil
}

func scanWebhook(row interface{ Scan(...any) error }) (model.Webhook, error) {
	var wh model.Webhook
	var projectID, created string
	var active int
	err := row.Scan(&wh.ID, &projectID, &wh.URL, &wh.Secret, &wh.Events, &active, &created)
	wh.ProjectID = projectID
	wh.Active = active != 0
	wh.CreatedAt = parseStringTime(created)
	return wh, err
}

// ListWebhooks returns all registered webhooks, optionally filtered by project.
func (s *Store) ListWebhooks() ([]model.Webhook, error) {
	rows, err := s.db.Query(`SELECT id,COALESCE(project_id,''),url,secret,events,active,created_at FROM webhooks ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Webhook{}
	for rows.Next() {
		wh, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wh)
	}
	return out, rows.Err()
}

// GetWebhook returns one webhook by ID.
func (s *Store) GetWebhook(id string) (*model.Webhook, error) {
	wh, err := scanWebhook(s.db.QueryRow(`SELECT id,COALESCE(project_id,''),url,secret,events,active,created_at FROM webhooks WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return &wh, nil
}

// WebhookUpdate carries optional webhook changes.
type WebhookUpdate struct {
	URL    *string
	Secret *string
	Events *string
	Active *bool
}

// UpdateWebhook applies partial changes to a webhook.
func (s *Store) UpdateWebhook(id string, u WebhookUpdate) (*model.Webhook, error) {
	wh, err := s.GetWebhook(id)
	if err != nil {
		return nil, err
	}
	if u.URL != nil {
		wh.URL = *u.URL
	}
	if u.Secret != nil {
		wh.Secret = *u.Secret
	}
	if u.Events != nil {
		wh.Events = *u.Events
	}
	if u.Active != nil {
		wh.Active = *u.Active
	}
	if _, err := s.db.Exec(`UPDATE webhooks SET url=?,secret=?,events=?,active=? WHERE id=?`,
		wh.URL, wh.Secret, wh.Events, boolInt(wh.Active), id); err != nil {
		return nil, err
	}
	return wh, nil
}

// DeleteWebhook removes a webhook registration.
func (s *Store) DeleteWebhook(id string) error {
	res, err := s.db.Exec(`DELETE FROM webhooks WHERE id=?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}
