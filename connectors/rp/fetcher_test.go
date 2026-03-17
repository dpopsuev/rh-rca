package rp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetcher_Fetch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ecosystem-qe/launch/100":
			json.NewEncoder(w).Encode(LaunchResource{
				ID:   100,
				UUID: "uuid-100",
				Name: "test-launch",
			})
		case "/api/v1/ecosystem-qe/item":
			json.NewEncoder(w).Encode(PagedItems{
				Content: []TestItemResource{
					{ID: 200, Name: "test-item", Status: "FAILED", Type: "TEST"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}

	fetcher := NewFetcher(client, "ecosystem-qe")
	env, err := fetcher.Fetch(100)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if env.RunID != "100" {
		t.Errorf("RunID = %q, want '100'", env.RunID)
	}
	if env.Name != "test-launch" {
		t.Errorf("Name = %q, want 'test-launch'", env.Name)
	}
	if len(env.FailureList) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(env.FailureList))
	}
	if env.FailureList[0].ID != 200 {
		t.Errorf("failure ID = %d, want 200", env.FailureList[0].ID)
	}
}

func TestFetcher_Fetch_LaunchNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorRS{ErrorCode: 40410, Message: "Launch not found"})
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	fetcher := NewFetcher(client, "ecosystem-qe")

	_, err := fetcher.Fetch(99999)
	if err == nil {
		t.Error("expected error for missing launch")
	}
}

func TestFetcher_Fetch_NoFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ecosystem-qe/launch/100":
			json.NewEncoder(w).Encode(LaunchResource{ID: 100, Name: "empty-launch"})
		case "/api/v1/ecosystem-qe/item":
			json.NewEncoder(w).Encode(PagedItems{Content: []TestItemResource{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	fetcher := NewFetcher(client, "ecosystem-qe")

	env, err := fetcher.Fetch(100)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(env.FailureList) != 0 {
		t.Errorf("expected 0 failures, got %d", len(env.FailureList))
	}
}
