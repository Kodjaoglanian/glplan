package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"glplan/internal/model"
	"glplan/internal/store"
)

// webhookPayload is the JSON body delivered to webhook endpoints.
type webhookPayload struct {
	model.Event
	Task *model.Task `json:"task,omitempty"`
}

// startWebhookDispatcher registers the store listener and delivery workers.
func (s *Server) startWebhookDispatcher() {
	s.webhookQueue = make(chan webhookJob, 256)
	s.store.OnEvent(func(ev model.Event) {
		webhooks, err := s.store.ListWebhooks()
		if err != nil {
			return
		}
		for _, wh := range webhooks {
			if !wh.Active || !webhookMatches(wh, ev) {
				continue
			}
			job := webhookJob{webhook: wh, event: ev, attempt: 1}
			select {
			case s.webhookQueue <- job:
			default: // queue full: drop rather than block mutations
			}
		}
	})
	for i := 0; i < 2; i++ {
		go s.webhookWorker()
	}
}

type webhookJob struct {
	webhook model.Webhook
	event   model.Event
	attempt int
}

// webhookMatches reports whether a webhook subscribes to the event.
func webhookMatches(wh model.Webhook, ev model.Event) bool {
	if wh.ProjectID != "" && wh.ProjectID != ev.ProjectID {
		return false
	}
	if wh.Events == "" || wh.Events == "*" {
		return true
	}
	for _, e := range strings.Split(wh.Events, ",") {
		if strings.TrimSpace(e) == ev.Type {
			return true
		}
	}
	return false
}

func (s *Server) webhookWorker() {
	client := &http.Client{Timeout: 10 * time.Second}
	for job := range s.webhookQueue {
		if job.attempt > 1 {
			time.Sleep(time.Duration(job.attempt*2) * time.Second)
		}
		if err := deliverWebhook(client, s.store, job); err == nil {
			continue
		}
		if job.attempt < 3 {
			job.attempt++
			select {
			case s.webhookQueue <- job:
			default:
			}
		}
	}
}

func deliverWebhook(client *http.Client, st taskGetter, job webhookJob) error {
	payload := webhookPayload{Event: job.event}
	if job.event.TaskID != "" {
		if t, err := st.GetTask(job.event.TaskID); err == nil {
			payload.Task = t
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", job.webhook.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "glplan-webhook")
	req.Header.Set("X-Glplan-Event", job.event.Type)
	req.Header.Set("X-Glplan-Delivery", job.event.ID)
	if job.webhook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(job.webhook.Secret))
		mac.Write(body)
		req.Header.Set("X-Glplan-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

type taskGetter interface {
	GetTask(id string) (*model.Task, error)
}

type webhookInput struct {
	ProjectID string `json:"project_id"`
	URL       string `json:"url"`
	Secret    string `json:"secret"`
	Events    string `json:"events"`
	Active    *bool  `json:"active"`
}

func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks, err := s.store.ListWebhooks()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, webhooks)
}

func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) {
	var in webhookInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, err)
		return
	}
	if in.URL == "" || (!strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://")) {
		writeJSON(w, 400, map[string]string{"error": "url http(s) é obrigatória"})
		return
	}
	wh, err := s.store.CreateWebhook(in.ProjectID, in.URL, in.Secret, in.Events)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, wh)
}

func (s *Server) getWebhook(w http.ResponseWriter, r *http.Request) {
	wh, err := s.store.GetWebhook(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, wh)
}

func (s *Server) updateWebhook(w http.ResponseWriter, r *http.Request) {
	var in webhookInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, err)
		return
	}
	u := store.WebhookUpdate{}
	if in.URL != "" {
		u.URL = &in.URL
	}
	if in.Secret != "" {
		u.Secret = &in.Secret
	}
	if in.Events != "" {
		u.Events = &in.Events
	}
	if in.Active != nil {
		u.Active = in.Active
	}
	wh, err := s.store.UpdateWebhook(r.PathValue("id"), u)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, wh)
}

func (s *Server) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteWebhook(r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// testWebhook fires a synthetic webhook.test event at one endpoint.
func (s *Server) testWebhook(w http.ResponseWriter, r *http.Request) {
	wh, err := s.store.GetWebhook(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	ev := model.Event{
		ID:        model.NewID("evt"),
		Type:      "webhook.test",
		ProjectID: wh.ProjectID,
		Actor:     "glplan",
		Detail:    "evento de teste",
		CreatedAt: time.Now().UTC(),
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if err := deliverWebhook(client, s.store, webhookJob{webhook: *wh, event: ev}); err != nil {
		writeJSON(w, 502, map[string]string{"error": "entrega falhou: " + err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "delivery": ev.ID})
}
