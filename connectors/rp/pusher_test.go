package rp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

)

func TestPusher_Push_Success(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}

	pusher := NewPusher(client, "test-project", "jane.doe", "")

	artifact := pushArtifact{
		RunID:        "12345",
		CaseIDs:      []string{"100", "101"},
		DefectType:   "pb001",
		RCAMessage:   "Clock sync failure in NTP subsystem",
		EvidenceRefs: []string{"https://github.com/org/repo/commit/abc123", "some-log-file.txt"},
	}
	data, _ := json.Marshal(artifact)
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "artifact.json")
	if err := os.WriteFile(artifactPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	store := NewMemPushStore()
	err = pusher.Push(artifactPath, store, "JIRA-123", "https://jira.example.com/JIRA-123")
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	if receivedBody == nil {
		t.Fatal("expected request body")
	}
	issues, ok := receivedBody["issues"].([]any)
	if !ok {
		t.Fatal("expected issues array in request body")
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issue definitions, got %d", len(issues))
	}

	def := issues[0].(map[string]any)
	issue := def["issue"].(map[string]any)
	comment, _ := issue["comment"].(string)
	if !strings.Contains(comment, "Clock sync failure in NTP subsystem") {
		t.Errorf("comment should contain RCA message, got %q", comment)
	}
	if !strings.Contains(comment, "github.com/org/repo/commit/abc123") {
		t.Errorf("comment should contain commit link, got %q", comment)
	}
	if !strings.Contains(comment, "Analysis was submitted by jane.doe (via Origami)") {
		t.Errorf("comment should contain attribution, got %q", comment)
	}
	if strings.Contains(comment, "some-log-file.txt") {
		t.Errorf("comment should not contain non-commit evidence refs, got %q", comment)
	}

	last := store.LastPushed()
	if last == nil {
		t.Fatal("expected pushed record")
	}
	if last.DefectType != "pb001" {
		t.Errorf("defect type = %q, want pb001", last.DefectType)
	}
	if last.JiraTicketID != "JIRA-123" {
		t.Errorf("jira ticket = %q, want JIRA-123", last.JiraTicketID)
	}
	if len(last.CaseIDs) != 2 {
		t.Errorf("expected 2 case IDs, got %d", len(last.CaseIDs))
	}
}

func TestPusher_Push_AttributionWithoutSubmitter(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	pusher := NewPusher(client, "test-project", "", "")

	artifact := pushArtifact{
		RunID:      "12345",
		CaseIDs:    []string{"100"},
		DefectType: "pb001",
	}
	data, _ := json.Marshal(artifact)
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "artifact.json")
	os.WriteFile(artifactPath, data, 0644)

	store := NewMemPushStore()
	if err := pusher.Push(artifactPath, store, "", ""); err != nil {
		t.Fatalf("Push: %v", err)
	}

	issues := receivedBody["issues"].([]any)
	def := issues[0].(map[string]any)
	issue := def["issue"].(map[string]any)
	comment, _ := issue["comment"].(string)
	if comment != "Analysis was submitted (via Origami)" {
		t.Errorf("unexpected comment without submitter: %q", comment)
	}
}

func TestPusher_Push_MissingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	pusher := NewPusher(client, "test-project", "", "")
	store := NewMemPushStore()

	err := pusher.Push("/nonexistent/path.json", store, "", "")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestPusher_Push_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	pusher := NewPusher(client, "test-project", "", "")
	store := NewMemPushStore()

	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(artifactPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	err := pusher.Push(artifactPath, store, "", "")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPusher_Push_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorRS{ErrorCode: 50000, Message: "Internal Server Error"})
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	pusher := NewPusher(client, "test-project", "jane.doe", "")

	artifact := pushArtifact{
		RunID:      "12345",
		CaseIDs:    []string{"100"},
		DefectType: "pb001",
	}
	data, _ := json.Marshal(artifact)
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "artifact.json")
	os.WriteFile(artifactPath, data, 0644)

	store := NewMemPushStore()
	err := pusher.Push(artifactPath, store, "", "")
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestPusher_BuildComment(t *testing.T) {
	tests := []struct {
		name         string
		submittedBy  string
		rcaMessage   string
		evidenceRefs []string
		wantContain  []string
		wantAbsent   []string
	}{
		{
			name:         "with submitter, RCA, and commit link",
			submittedBy:  "jane.doe",
			rcaMessage:   "Clock sync failure in NTP subsystem",
			evidenceRefs: []string{"https://github.com/org/repo/commit/abc123", "log.txt"},
			wantContain:  []string{"Clock sync failure in NTP subsystem", "Suspected commit(s)", "github.com/org/repo/commit/abc123", "by jane.doe (via Origami)"},
			wantAbsent:   []string{"log.txt"},
		},
		{
			name:         "gitlab commit link",
			submittedBy:  "alice",
			rcaMessage:   "config error",
			evidenceRefs: []string{"https://gitlab.com/org/repo/-/commit/def456"},
			wantContain:  []string{"config error", "Suspected commit(s)", "gitlab.com/org/repo/-/commit/def456"},
		},
		{
			name:         "no commit links in evidence",
			submittedBy:  "bob",
			rcaMessage:   "some failure",
			evidenceRefs: []string{"log.txt", "/var/log/messages"},
			wantContain:  []string{"some failure", "by bob (via Origami)"},
			wantAbsent:   []string{"Suspected commit"},
		},
		{
			name:        "no submitter, no RCA, no evidence",
			submittedBy: "",
			wantContain: []string{"Analysis was submitted (via Origami)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pusher{submittedBy: tt.submittedBy, appName: "Origami"}
			got := p.buildComment(tt.rcaMessage, tt.evidenceRefs)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("buildComment() = %q, want to contain %q", got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("buildComment() = %q, should NOT contain %q", got, absent)
				}
			}
		})
	}
}

func TestIsCommitLink(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{"https://github.com/org/repo/commit/abc123", true},
		{"https://gitlab.com/org/repo/-/commit/abc123", true},
		{"https://bitbucket.org/org/repo/commits/abc123", true},
		{"https://github.com/org/repo/blob/main/file.go", false},
		{"log.txt", false},
		{"/var/log/messages", false},
		{"http://example.com/commit/abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := isCommitLink(tt.ref)
			if got != tt.want {
				t.Errorf("isCommitLink(%q) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}
