package api_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"glplan/internal/api"
)

type receivedHook struct {
	body      []byte
	eventType string
	signature string
	delivery  string
}

type hookReceiver struct {
	mu       sync.Mutex
	received []receivedHook
	server   *httptest.Server
}

func newHookReceiver(t *testing.T) *hookReceiver {
	t.Helper()
	r := &hookReceiver{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.received = append(r.received, receivedHook{
			body:      body,
			eventType: req.Header.Get("X-Glplan-Event"),
			signature: req.Header.Get("X-Glplan-Signature"),
			delivery:  req.Header.Get("X-Glplan-Delivery"),
		})
		r.mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *hookReceiver) waitFor(t *testing.T, n int) []receivedHook {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		if len(r.received) >= n {
			out := append([]receivedHook{}, r.received...)
			r.mu.Unlock()
			return out
		}
		r.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d webhook deliveries, got %d", n, len(r.received))
	return nil
}

func TestWebhookDeliveredOnTaskCreated(t *testing.T) {
	ts, st := newTestAPI(t)
	receiver := newHookReceiver(t)
	projects := []struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}{}
	getJSON(t, ts.URL+"/api/projects", &projects)

	body, _ := json.Marshal(map[string]string{
		"url": receiver.server.URL, "secret": "segredo", "events": "task.created,comment.added",
	})
	resp, err := http.Post(ts.URL+"/api/webhooks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("create webhook: status %d: %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	cols := []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{}
	getJSON(t, ts.URL+"/api/projects/"+projects[0].ID+"/columns", &cols)

	taskBody, _ := json.Marshal(map[string]any{
		"project": projects[0].ID, "column_id": cols[0].ID, "title": "Acordar o grokbot", "priority": "high",
	})
	resp, err = http.Post(ts.URL+"/api/tasks", "application/json", bytes.NewReader(taskBody))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	got := receiver.waitFor(t, 1)
	hook := got[0]
	if hook.eventType != "task.created" {
		t.Fatalf("event type %q, want task.created", hook.eventType)
	}
	if hook.delivery == "" {
		t.Fatal("missing X-Glplan-Delivery header")
	}
	mac := hmac.New(sha256.New, []byte("segredo"))
	mac.Write(hook.body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if hook.signature != want {
		t.Fatalf("signature %q, want %q", hook.signature, want)
	}
	var payload struct {
		Type  string `json:"type"`
		Actor string `json:"actor"`
		Task  struct {
			Title    string `json:"title"`
			Priority string `json:"priority"`
			Ref      string `json:"ref"`
		} `json:"task"`
	}
	if err := json.Unmarshal(hook.body, &payload); err != nil {
		t.Fatalf("payload inválido: %v", err)
	}
	if payload.Type != "task.created" || payload.Task.Title != "Acordar o grokbot" || payload.Task.Priority != "high" {
		t.Fatalf("payload inesperado: %+v", payload)
	}
	if payload.Task.Ref == "" {
		t.Fatal("payload task missing ref")
	}
	_ = st
}

func TestWebhookEventFilterSkipsNonMatching(t *testing.T) {
	ts, _ := newTestAPI(t)
	receiver := newHookReceiver(t)

	body, _ := json.Marshal(map[string]string{
		"url": receiver.server.URL, "events": "comment.added",
	})
	resp, err := http.Post(ts.URL+"/api/webhooks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	projects := []struct{ ID string }{}
	getJSON(t, ts.URL+"/api/projects", &projects)
	cols := []struct{ ID string }{}
	getJSON(t, ts.URL+"/api/projects/"+projects[0].ID+"/columns", &cols)
	taskBody, _ := json.Marshal(map[string]any{
		"project": projects[0].ID, "column_id": cols[0].ID, "title": "não deve disparar",
	})
	resp, err = http.Post(ts.URL+"/api/tasks", "application/json", bytes.NewReader(taskBody))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	time.Sleep(400 * time.Millisecond)
	receiver.mu.Lock()
	defer receiver.mu.Unlock()
	if len(receiver.received) != 0 {
		t.Fatalf("webhook com filtro comment.added recebeu %d entregas de task.created", len(receiver.received))
	}
}

func TestWebhookTestEndpointDelivers(t *testing.T) {
	ts, _ := newTestAPI(t)
	receiver := newHookReceiver(t)

	body, _ := json.Marshal(map[string]string{"url": receiver.server.URL})
	resp, err := http.Post(ts.URL+"/api/webhooks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var wh struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&wh)
	resp.Body.Close()

	resp, err = http.Post(ts.URL+"/api/webhooks/"+wh.ID+"/test", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("test endpoint: status %d: %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	got := receiver.waitFor(t, 1)
	if got[0].eventType != "webhook.test" {
		t.Fatalf("event type %q, want webhook.test", got[0].eventType)
	}
}

func TestWebhookCRUD(t *testing.T) {
	ts, _ := newTestAPI(t)
	body, _ := json.Marshal(map[string]string{"url": "http://example.com/hook", "events": "*"})
	resp, _ := http.Post(ts.URL+"/api/webhooks", "application/json", bytes.NewReader(body))
	var wh struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&wh)
	resp.Body.Close()
	if wh.ID == "" {
		t.Fatal("webhook id vazio")
	}

	var list []struct {
		ID string `json:"id"`
	}
	getJSON(t, ts.URL+"/api/webhooks", &list)
	if len(list) != 1 {
		t.Fatalf("list webhooks: got %d, want 1", len(list))
	}

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/webhooks/"+wh.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	getJSON(t, ts.URL+"/api/webhooks", &list)
	if len(list) != 0 {
		t.Fatalf("after delete: got %d webhooks", len(list))
	}
}

var _ = api.NewClient // keep api import used if patterns change
