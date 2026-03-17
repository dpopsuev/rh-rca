package rp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Launch tests ---

func TestLaunchScope_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/launch/33195" && r.Method == "GET" {
			json.NewEncoder(w).Encode(LaunchResource{
				ID:   33195,
				UUID: "abc-uuid",
				Name: "telco-ft-ran-ptp-4.21",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}

	launch, err := client.Project("ecosystem-qe").Launches().Get(context.Background(), 33195)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if launch.ID != 33195 || launch.Name != "telco-ft-ran-ptp-4.21" {
		t.Errorf("unexpected launch: %+v", launch)
	}
}

func TestLaunchScope_Get_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorRS{ErrorCode: 40410, Message: "Launch not found"})
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	_, err := client.Project("ecosystem-qe").Launches().Get(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got: %v", err)
	}
}

func TestLaunchScope_List(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/launch" {
			json.NewEncoder(w).Encode(PagedLaunches{
				Content: []LaunchResource{
					{ID: 1, Name: "launch-1"},
					{ID: 2, Name: "launch-2"},
				},
				Page: PageInfo{Number: 1, Size: 20, TotalElements: 2, TotalPages: 1},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	result, err := client.Project("ecosystem-qe").Launches().List(context.Background(),
		WithLaunchStatus("FAILED"), WithPageSize(20))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Content) != 2 {
		t.Errorf("expected 2 launches, got %d", len(result.Content))
	}
}

// --- Item tests ---

func TestItemScope_List(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/item" {
			json.NewEncoder(w).Encode(PagedItems{
				Content: []TestItemResource{
					{ID: 100, Name: "test-1", Status: "FAILED"},
					{ID: 101, Name: "test-2", Status: "FAILED"},
				},
				Page: PageInfo{TotalElements: 2},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	result, err := client.Project("ecosystem-qe").Items().List(context.Background(),
		WithLaunchID(33195), WithStatus("FAILED"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Content) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Content))
	}
}

func TestItemScope_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/item/1697136" {
			json.NewEncoder(w).Encode(TestItemResource{
				ID:      1697136,
				Name:    "[T-TSC] RAN PTP tests",
				Status:  "FAILED",
				CodeRef: "ptp_recovery_test.go:121",
				Issue:   &Issue{IssueType: "ti001", Comment: "To investigate"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	item, err := client.Project("ecosystem-qe").Items().Get(context.Background(), 1697136)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if item.CodeRef != "ptp_recovery_test.go:121" {
		t.Errorf("unexpected CodeRef: %q", item.CodeRef)
	}
	if item.Issue == nil || item.Issue.IssueType != "ti001" {
		t.Errorf("unexpected Issue: %+v", item.Issue)
	}
}

func TestItemScope_UpdateDefect(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/item/100/update" && r.Method == "PUT" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	err := client.Project("ecosystem-qe").Items().UpdateDefect(context.Background(), 100, "pb001")
	if err != nil {
		t.Fatalf("UpdateDefect: %v", err)
	}
	if receivedBody == nil {
		t.Error("expected request body")
	}
}

func TestItemScope_UpdateDefectBulk(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/item" && r.Method == "PUT" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	defs := []IssueDefinition{
		{Issue: Issue{IssueType: "pb001"}, TestItemID: 100},
		{Issue: Issue{IssueType: "pb001"}, TestItemID: 101},
	}
	err := client.Project("ecosystem-qe").Items().UpdateDefectBulk(context.Background(), defs)
	if err != nil {
		t.Fatalf("UpdateDefectBulk: %v", err)
	}
}

// --- Envelope mapping test ---

func TestFetchEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ecosystem-qe/launch/33195":
			json.NewEncoder(w).Encode(LaunchResource{
				ID:   33195,
				UUID: "launch-uuid",
				Name: "telco-ft-ran-ptp-4.21",
			})
		case "/api/v1/ecosystem-qe/item":
			json.NewEncoder(w).Encode(PagedItems{
				Content: []TestItemResource{
					{
						ID:       1697136,
						UUID:     "item-uuid-1",
						Name:     "[T-TSC] RAN PTP tests",
						Type:     "TEST",
						Status:   "FAILED",
						CodeRef:  "ptp_test.go:50",
						Parent:   1697100,
						Issue:    &Issue{IssueType: "ti001", Comment: "needs analysis"},
					},
					{
						ID:     1697139,
						UUID:   "item-uuid-2",
						Name:   "[T-BC] RAN PTP tests",
						Type:   "TEST",
						Status: "FAILED",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	env, err := client.Project("ecosystem-qe").FetchEnvelope(context.Background(), 33195)
	if err != nil {
		t.Fatalf("FetchEnvelope: %v", err)
	}
	if env.RunID != "33195" || env.Name != "telco-ft-ran-ptp-4.21" {
		t.Errorf("unexpected envelope: run_id=%q name=%q", env.RunID, env.Name)
	}
	if len(env.FailureList) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(env.FailureList))
	}

	// Check enriched fields on first item
	f := env.FailureList[0]
	if f.ID != 1697136 || f.Name != "[T-TSC] RAN PTP tests" {
		t.Errorf("unexpected first failure: %+v", f)
	}
	if f.CodeRef != "ptp_test.go:50" {
		t.Errorf("expected CodeRef=ptp_test.go:50, got %q", f.CodeRef)
	}
	if f.ParentID != 1697100 {
		t.Errorf("expected ParentID=1697100, got %d", f.ParentID)
	}
	if f.IssueType != "ti001" {
		t.Errorf("expected IssueType=ti001, got %q", f.IssueType)
	}
	if f.IssueComment != "needs analysis" {
		t.Errorf("expected IssueComment, got %q", f.IssueComment)
	}

	// Second item: no enriched fields
	f2 := env.FailureList[1]
	if f2.CodeRef != "" || f2.IssueType != "" {
		t.Errorf("expected empty enriched fields for item 2: %+v", f2)
	}
}

// --- Error predicate tests ---

func TestAPIError_Predicates(t *testing.T) {
	err404 := newAPIError("get launch", 404, 40410, "not found")
	err401 := newAPIError("list", 401, 0, "unauthorized")
	err403 := newAPIError("update", 403, 0, "forbidden")

	if !IsNotFound(err404) {
		t.Error("expected IsNotFound for 404")
	}
	if IsNotFound(err401) {
		t.Error("did not expect IsNotFound for 401")
	}
	if !IsUnauthorized(err401) {
		t.Error("expected IsUnauthorized for 401")
	}
	if !IsForbidden(err403) {
		t.Error("expected IsForbidden for 403")
	}
	if !HasStatusCode(err404, 404) {
		t.Error("expected HasStatusCode(404)")
	}
	if !HasErrorCode(err404, 40410) {
		t.Error("expected HasErrorCode(40410)")
	}
}

func TestAPIError_ErrorString(t *testing.T) {
	err := newAPIError("get launch", 404, 40410, "Launch not found")
	expected := "get launch: HTTP 404: [40410] Launch not found"
	if err.Error() != expected {
		t.Errorf("error string: got %q, want %q", err.Error(), expected)
	}

	errNoCode := newAPIError("list", 500, 0, "Internal Server Error")
	expectedNoCode := "list: HTTP 500: Internal Server Error"
	if errNoCode.Error() != expectedNoCode {
		t.Errorf("error string: got %q, want %q", errNoCode.Error(), expectedNoCode)
	}
}

// --- Client construction tests ---

func TestNew_EmptyBaseURL(t *testing.T) {
	_, err := New("", "token")
	if err == nil {
		t.Error("expected error for empty baseURL")
	}
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(LaunchResource{ID: 1})
	}))
	defer server.Close()

	client, err := New(server.URL+"/", "token", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if client.baseURL != server.URL {
		t.Errorf("baseURL not trimmed: %q", client.baseURL)
	}
}

func TestReadAPIKey_FileNotFound(t *testing.T) {
	_, err := ReadAPIKey("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- EpochMillis test ---

func TestEpochMillis_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		year  int
	}{
		{"milliseconds", "1771104069000", 2026},
		{"microseconds", "1771104069000000", 2026},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e EpochMillis
			if err := e.UnmarshalJSON([]byte(tt.input)); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if e.Time().Year() != tt.year {
				t.Errorf("expected year %d, got %d (time=%v)", tt.year, e.Time().Year(), e.Time())
			}
		})
	}
}
