package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"glplan/internal/api"
	"glplan/internal/model"
)

func TestIsLocalAPI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		server string
		want   bool
	}{
		{"", true},
		{"http://127.0.0.1:9876", true},
		{"http://localhost:9876", true},
		{"http://[::1]:9876", true},
		{"https://board.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			if got := isLocalAPI(tt.server); got != tt.want {
				t.Fatalf("isLocalAPI(%q)=%v, want %v", tt.server, got, tt.want)
			}
		})
	}
}

func TestShouldStartEmbeddedOnlyForUnreachableLocalAPI(t *testing.T) {
	t.Parallel()
	if !shouldStartEmbedded("", fmt.Errorf("connection refused")) {
		t.Fatal("expected network failure on local API to allow embedded backend")
	}
	if shouldStartEmbedded("https://board.example.com", fmt.Errorf("connection refused")) {
		t.Fatal("remote API must not start a local embedded backend")
	}
	if shouldStartEmbedded("", &api.APIError{Status: 401, Body: "unauthorized"}) {
		t.Fatal("reachable API returning an API error must not be replaced")
	}
	if shouldStartEmbedded("", nil) {
		t.Fatal("healthy API must not start embedded backend")
	}
	if shouldStartEmbedded("", errors.New("x")) != true {
		t.Fatal("generic local network error should start embedded backend")
	}
}

func TestEmbeddedAPIServesSeededBoard(t *testing.T) {
	embedded, err := startEmbeddedAPI(filepath.Join(t.TempDir(), "embedded.db"), "", true)
	if err != nil {
		t.Fatalf("start embedded api: %v", err)
	}
	defer embedded.Close()

	client := api.NewClient(embedded.url, "")
	if err := client.Health(); err != nil {
		t.Fatalf("embedded health: %v", err)
	}
	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("expected seeded project from embedded API")
	}
}

func TestClientWithEmbeddedUsesLocalBackendWhenDefaultAPIUnavailable(t *testing.T) {
	client, embedded, err := clientWithEmbedded("http://127.0.0.1:1", "", filepath.Join(t.TempDir(), "embedded.db"), true)
	if err != nil {
		t.Fatalf("client with embedded: %v", err)
	}
	if embedded == nil {
		t.Fatal("expected embedded backend for unreachable local API")
	}
	defer embedded.Close()
	if err := client.Health(); err != nil {
		t.Fatalf("embedded client health: %v", err)
	}
}

func TestOpenStoreBootstrapsEmptyProjectByDefault(t *testing.T) {
	s, err := openStore(filepath.Join(t.TempDir(), "clean.db"), false)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected one bootstrapped project, got %d", len(projects))
	}
	tasks, err := s.ListTasks(model.TaskFilter{ProjectID: projects[0].ID, IncludeDone: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected no demo tasks by default, got %d", len(tasks))
	}
}

func TestOpenStoreWithDemoSeedsSampleTasks(t *testing.T) {
	s, err := openStore(filepath.Join(t.TempDir(), "demo.db"), true)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	tasks, err := s.ListTasks(model.TaskFilter{IncludeDone: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected demo tasks when --demo is enabled")
	}
}
