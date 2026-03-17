package mcpconfig_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	mcpserver "github.com/dpopsuev/rh-rca/mcpconfig"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMain(m *testing.M) {
	mcpserver.DefaultGetNextStepTimeout = 1 * time.Second
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
	os.Exit(m.Run())
}

func dumpGoroutines(t *testing.T) {
	t.Helper()
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	t.Logf("=== GOROUTINE DUMP ===\n%s", buf[:n])
}

// projectRoot returns a temp directory seeded with read-only fixtures
// from testdata/ that the server needs at ProjectRoot (e.g. scorecard).
// All writes (.asterisk/calibrate/, datasets/, candidates/) go to temp
// and are cleaned up automatically.
func projectRoot(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	_, f, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(f), "testdata")

	seedPaths := []string{
		"scorecards/rca.yaml",
	}
	for _, rel := range seedPaths {
		data, err := os.ReadFile(filepath.Join(src, rel))
		if err != nil {
			t.Fatalf("seed projectRoot: read %s: %v", rel, err)
		}
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("seed projectRoot: mkdir %s: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("seed projectRoot: write %s: %v", dst, err)
		}
	}
	return tmp
}

func testDomainFS(t *testing.T) fs.FS {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	return os.DirFS(filepath.Join(filepath.Dir(f), "testdata"))
}

func newTestServer(t *testing.T) *mcpserver.Server {
	t.Helper()
	srv := mcpserver.NewServer("test-rca",
		mcpserver.WithDomainFS(testDomainFS(t)),
		mcpserver.WithStateDir(t.TempDir()),
	)
	srv.ProjectRoot = projectRoot(t)
	t.Cleanup(srv.Shutdown)
	return srv
}

func connectInMemory(t *testing.T, ctx context.Context, srv *mcpserver.Server) *sdkmcp.ClientSession {
	t.Helper()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := srv.MCPServer.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return session
}

func callTool(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("CallTool(%s) returned error: %s", name, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s) returned error", name)
	}
	result := make(map[string]any)
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				t.Fatalf("unmarshal tool result: %v (text: %s)", err, tc.Text)
			}
			return result
		}
	}
	t.Fatalf("no text content in tool result")
	return nil
}

func callToolE(ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) (map[string]any, error) {
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("CallTool(%s): %w", name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				return nil, fmt.Errorf("CallTool(%s) error: %s", name, tc.Text)
			}
		}
		return nil, fmt.Errorf("CallTool(%s) returned error", name)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			result := make(map[string]any)
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				return nil, fmt.Errorf("unmarshal %s result: %w", name, err)
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("no text content in %s result", name)
}

// startCircuit is a helper that wraps start_circuit with Asterisk-specific
// extra params (scenario, backend) to reduce boilerplate in domain tests.
func startCircuit(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, scenario, backend string, parallel int) map[string]any {
	t.Helper()
	args := map[string]any{
		"extra": map[string]any{
			"scenario": scenario,
			"backend":  backend,
		},
	}
	if parallel > 0 {
		args["parallel"] = parallel
	}
	return callTool(t, ctx, session, "start_circuit", args)
}

// requireStep calls get_next_step and fatals with the circuit error if done=true.
func requireStep(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, sessionID string) map[string]any {
	t.Helper()
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		if errMsg, _ := step["error"].(string); errMsg != "" {
			t.Fatalf("circuit failed: %s", errMsg)
		}
		t.Fatal("expected a step, got done=true (no error)")
	}
	return step
}

// --- Artifact helpers ---

func artifactForStepViaResolve(step string, subagentID int) string {
	switch step {
	case "F1_TRIAGE":
		return `{"symptom_category":"product","severity":"high","defect_type_hypothesis":"pb001","candidate_repos":["repo-a","repo-b"],"skip_investigation":false,"cascade_suspected":false}`
	default:
		return artifactForStep(step, subagentID)
	}
}

func artifactForStep(step string, subagentID int) string {
	switch step {
	case "recall":
		return fmt.Sprintf(`{"match":false,"confidence":0.0,"reasoning":"subagent-%d"}`, subagentID)
	case "triage":
		return `{"symptom_category":"product","severity":"high","defect_type_hypothesis":"pb001","candidate_repos":["test-repo"],"skip_investigation":false,"cascade_suspected":false}`
	case "resolve":
		return `{"selected_repos":[{"name":"test-repo","reason":"test"}]}`
	case "investigate":
		return fmt.Sprintf(`{"rca_message":"root cause from subagent-%d","defect_type":"pb001","component":"test-component","convergence_score":0.85,"evidence_refs":["ref-1"]}`, subagentID)
	case "correlate":
		return `{"is_duplicate":false,"confidence":0.1}`
	case "review":
		return `{"decision":"approve"}`
	case "report":
		return fmt.Sprintf(`{"defect_type":"pb001","case_id":"auto","summary":"test summary","subagent":%d}`, subagentID)
	default:
		return fmt.Sprintf(`{"defect_type":"pb001","subagent":%d}`, subagentID)
	}
}

// fieldsForStep parses artifactForStep JSON into a map for submit_step.
func fieldsForStep(step string, subagentID int) map[string]any {
	artifact := artifactForStep(step, subagentID)
	var fields map[string]any
	if err := json.Unmarshal([]byte(artifact), &fields); err != nil {
		panic(err)
	}
	return fields
}

// fieldsForStepViaResolve parses artifactForStepViaResolve JSON into a map for submit_step.
func fieldsForStepViaResolve(step string, subagentID int) map[string]any {
	artifact := artifactForStepViaResolve(step, subagentID)
	var fields map[string]any
	if err := json.Unmarshal([]byte(artifact), &fields); err != nil {
		panic(err)
	}
	return fields
}

type stepRecord struct {
	CaseID     string
	Step       string
	DispatchID int64
}

// --- Domain-specific tests ---

func TestServer_ToolDiscovery(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{
		"start_circuit": false,
		"get_next_step":  false,
		"submit_step":    false,
		"get_report":     false,
		"emit_signal":    false,
		"get_signals":    false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not found in ListTools", name)
		}
	}
}

func TestServer_StubCalibration_FullLoop(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "stub", 0)

	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("expected non-empty session_id, got %v", startResult["session_id"])
	}
	totalCases, _ := startResult["total_cases"].(float64)
	if totalCases < 1 {
		t.Fatalf("expected total_cases >= 1, got %v", totalCases)
	}
	t.Logf("started session %s with %v cases", sessionID, totalCases)

	time.Sleep(500 * time.Millisecond)
	stepResult := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	done, _ := stepResult["done"].(bool)
	if !done {
		t.Fatalf("expected done=true for stub backend, got %v", stepResult)
	}

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})

	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}

	reportStr, _ := reportResult["report"].(string)
	if reportStr == "" {
		t.Fatal("expected non-empty report string")
	}
	t.Logf("report preview: %.200s...", reportStr)
}

func TestServer_GetNextStep_NoSession(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "get_next_step",
		Arguments: map[string]any{"session_id": "nonexistent"},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for missing session")
	}
}

func TestServer_SubmitStep_UnknownStep(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	t.Logf("start_circuit result: %+v", startResult)
	sessionID, _ := startResult["session_id"].(string)

	time.Sleep(200 * time.Millisecond)
	stepResult := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": float64(3000),
	})
	t.Logf("get_next_step result: %+v", stepResult)
	if done, _ := stepResult["done"].(bool); done {
		if errMsg, _ := stepResult["error"].(string); errMsg != "" {
			t.Fatalf("circuit failed: %s", errMsg)
		}
		t.Fatal("expected a step, got done=true (no error)")
	}
	dispatchID, _ := stepResult["dispatch_id"].(float64)

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "submit_step",
		Arguments: map[string]any{
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        "INVALID_STEP",
			"fields":      map[string]any{"x": 1},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for unknown step")
	}
}

func TestServer_DoubleStart_WhileRunning(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 0)

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "start_circuit",
		Arguments: map[string]any{
			"extra": map[string]any{"scenario": "ptp-mock", "backend": "llm"},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for double start while running")
	}
}

func TestServer_StartAfterDone(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "stub", 0)
	time.Sleep(500 * time.Millisecond)

	startResult := startCircuit(t, ctx, session, "ptp-mock", "stub", 0)
	if _, ok := startResult["session_id"].(string); !ok {
		t.Fatalf("second start failed: %v", startResult)
	}
}

func TestServer_SignalBus_EmitAndGet(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "stub", 0)
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	emitResult := callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "dispatch",
		"agent":      "main",
		"case_id":    "C01",
		"step":       "F1",
		"meta":       map[string]any{"detail": "test"},
	})
	if emitResult["ok"] != "signal emitted" {
		t.Fatalf("expected ok='signal emitted', got %v", emitResult)
	}

	getResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	total, _ := getResult["total"].(float64)
	if total < 3 {
		t.Fatalf("expected at least 3 signals (server auto-emits), got %v", total)
	}
}

func TestServer_Parallel_GetNextStep_TwoConcurrent(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 2)
	sessionID := startResult["session_id"].(string)

	type stepResult struct {
		caseID string
		step   string
		err    error
	}

	results := make(chan stepResult, 2)
	for i := 0; i < 2; i++ {
		go func() {
			res := callTool(t, ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
			})
			results <- stepResult{
				caseID: res["case_id"].(string),
				step:   res["step"].(string),
			}
		}()
	}

	var steps []stepResult
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			steps = append(steps, r)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for get_next_step %d/2 (only %d returned)", i+1, len(steps))
		}
	}

	if steps[0].caseID == steps[1].caseID && steps[0].step == steps[1].step {
		t.Fatalf("expected 2 different steps, got same: %s/%s", steps[0].caseID, steps[0].step)
	}
	t.Logf("got 2 concurrent steps: %s/%s and %s/%s", steps[0].caseID, steps[0].step, steps[1].caseID, steps[1].step)
}

func TestServer_FourSubagents_FullDrain(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 4)
	sessionID := startResult["session_id"].(string)
	t.Logf("started session %s", sessionID)

	var mu sync.Mutex
	workLog := make(map[int][]stepRecord)

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d get_next_step: %w", subID, err)
					return
				}

				if done, _ := res["done"].(bool); done {
					return
				}

				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      fieldsForStep(step, subID),
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d submit_step(%s/%s): %w", subID, caseID, step, err)
					return
				}

				mu.Lock()
				workLog[subID] = append(workLog[subID], stepRecord{
					CaseID:     caseID,
					Step:       step,
					DispatchID: int64(dispatchID),
				})
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("subagent error: %v", err)
	}

	for i := 0; i < 4; i++ {
		if len(workLog[i]) == 0 {
			t.Errorf("subagent-%d got zero steps (starvation)", i)
		} else {
			t.Logf("subagent-%d processed %d steps", i, len(workLog[i]))
		}
	}

	seenIDs := make(map[int64]bool)
	totalSteps := 0
	for _, records := range workLog {
		for _, r := range records {
			if seenIDs[r.DispatchID] {
				t.Errorf("duplicate dispatch_id %d", r.DispatchID)
			}
			seenIDs[r.DispatchID] = true
			totalSteps++
		}
	}
	if totalSteps == 0 {
		t.Fatal("circuit produced zero steps")
	}
	t.Logf("total steps processed: %d across 4 subagents", totalSteps)

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
}

func TestServer_FourSubagents_ViaResolve(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 4)
	sessionID := startResult["session_id"].(string)

	var mu sync.Mutex
	stepLog := make(map[string][]string)

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d get_next_step: %w", subID, err)
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				t.Logf("[sub-%d] pull case=%s step=%s dispatch=%.0f", subID, caseID, step, dispatchID)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      fieldsForStepViaResolve(step, subID),
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d submit_step(%s/%s): %w", subID, caseID, step, err)
					return
				}

				mu.Lock()
				stepLog[caseID] = append(stepLog[caseID], step)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("subagent error: %v", err)
	}

	var f2Count, f3Count int
	for caseID, steps := range stepLog {
		t.Logf("case %s: %v", caseID, steps)
		for _, s := range steps {
			if s == "resolve" {
				f2Count++
			}
			if s == "investigate" {
				f3Count++
			}
		}
	}

	if f3Count == 0 {
		t.Error("no cases reached investigate")
	}
	t.Logf("resolve dispatches: %d, investigate dispatches: %d", f2Count, f3Count)

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	t.Logf("report status=%s", status)
	if structured, ok := reportResult["structured"].(map[string]any); ok {
		if cases, ok := structured["case_results"].([]any); ok {
			for _, c := range cases {
				if cm, ok := c.(map[string]any); ok {
					if ce, ok := cm["circuit_error"].(string); ok && ce != "" {
						t.Logf("  case %v circuit_error: %s", cm["case_id"], ce)
					}
				}
			}
		}
	}
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
}

func TestServer_FourSubagents_NoDuplicateDispatch(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "daemon-mock", "llm", 4)
	sessionID := startResult["session_id"].(string)

	var mu sync.Mutex
	var allRecords []stepRecord

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d: %w", subID, err)
					return
				}

				if done, _ := res["done"].(bool); done {
					return
				}

				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      fieldsForStep(step, subID),
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d submit_step(%s/%s): %w", subID, caseID, step, err)
					return
				}

				mu.Lock()
				allRecords = append(allRecords, stepRecord{
					CaseID:     caseID,
					Step:       step,
					DispatchID: int64(dispatchID),
				})
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("subagent error: %v", err)
	}

	type caseStep struct{ CaseID, Step string }
	seen := make(map[caseStep]int)
	for _, r := range allRecords {
		key := caseStep{r.CaseID, r.Step}
		seen[key]++
		if seen[key] > 1 {
			t.Errorf("duplicate dispatch: case=%s step=%s appeared %d times", r.CaseID, r.Step, seen[key])
		}
	}

	if len(allRecords) == 0 {
		t.Fatal("circuit produced zero steps")
	}
	t.Logf("daemon-mock: %d unique (case, step) pairs processed, 0 duplicates", len(seen))

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if status, _ := reportResult["status"].(string); status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
}

func TestServer_FourSubagents_ConcurrencyTiming(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 4)
	sessionID := startResult["session_id"].(string)

	const perStepDelay = 20 * time.Millisecond
	var mu sync.Mutex
	var totalSteps int64

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	start := time.Now()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- err
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}

				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				time.Sleep(perStepDelay)

				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      fieldsForStep(step, subID),
				})
				if err != nil {
					errCh <- err
					return
				}
				mu.Lock()
				totalSteps++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	elapsed := time.Since(start)

	for err := range errCh {
		t.Fatalf("subagent error: %v", err)
	}

	serialEstimate := time.Duration(totalSteps) * perStepDelay
	speedup := float64(serialEstimate) / float64(elapsed)

	t.Logf("steps=%d, elapsed=%v, serial_estimate=%v, speedup=%.2fx",
		totalSteps, elapsed, serialEstimate, speedup)

	if elapsed > time.Duration(float64(serialEstimate)*0.75) {
		t.Errorf("concurrent execution too slow: elapsed=%v > 75%% of serial=%v (speedup=%.2fx)",
			elapsed, serialEstimate, speedup)
	}

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if status, _ := reportResult["status"].(string); status != "done" {
		t.Fatalf("expected done, got %s", status)
	}
}

func TestGetNextStep_InlinePrompt(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := startResult["session_id"].(string)

	step := requireStep(t, ctx, session, sessionID)

	promptContent, _ := step["prompt_content"].(string)
	promptPath, _ := step["prompt_path"].(string)

	if promptContent == "" {
		t.Fatal("expected non-empty prompt_content")
	}
	if promptPath == "" {
		t.Fatal("expected non-empty prompt_path")
	}

	fileContent, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt file %s: %v", promptPath, err)
	}

	if promptContent != string(fileContent) {
		t.Errorf("prompt_content does not match file content.\nprompt_content len=%d, file len=%d",
			len(promptContent), len(fileContent))
	}
	t.Logf("inline prompt verified: %d bytes, path=%s", len(promptContent), promptPath)
}

func TestGetNextStep_Timeout(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := startResult["session_id"].(string)

	step1 := requireStep(t, ctx, session, sessionID)
	_ = step1

	start := time.Now()
	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 100,
	})
	elapsed := time.Since(start)

	if done, _ := res["done"].(bool); done {
		t.Fatal("expected done=false")
	}
	if avail, _ := res["available"].(bool); avail {
		t.Fatal("expected available=false")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout should return within ~100ms, took %v", elapsed)
	}
	t.Logf("timeout returned in %v with available=false", elapsed)
}

func TestSession_TTL_Abort(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := startResult["session_id"].(string)

	step := requireStep(t, ctx, session, sessionID)
	_ = step

	srv.SetSessionTTL(2 * time.Second)

	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			dumpGoroutines(t)
			t.Fatal("session did not abort within 10s after TTL=2s")
		case <-ticker.C:
			res, err := callToolE(ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
				"timeout_ms": 100,
			})
			if err != nil {
				t.Logf("get_next_step error (may be abort): %v", err)
				continue
			}
			if done, _ := res["done"].(bool); done {
				t.Logf("session transitioned to done (aborted)")
				return
			}
		}
	}
}

func TestServer_StaleSession_StartReplacesStuck(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start1 := startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sid1 := start1["session_id"].(string)

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sid1,
		"timeout_ms": 500,
	})
	if avail, _ := step["available"].(bool); !avail {
		t.Fatal("expected first step to be available")
	}

	_, err := callToolE(ctx, session, "start_circuit", map[string]any{
		"extra": map[string]any{"scenario": "ptp-mock", "backend": "stub"},
	})
	if err == nil {
		t.Fatal("expected error starting second session without force")
	}
	t.Logf("without force: %v (expected)", err)

	start3 := callTool(t, ctx, session, "start_circuit", map[string]any{
		"extra": map[string]any{"scenario": "ptp-mock", "backend": "stub"},
		"force": true,
	})
	sid3 := start3["session_id"].(string)
	if sid3 == "" {
		t.Fatal("expected new session_id from force-start")
	}
	if sid3 == sid1 {
		t.Fatal("force-started session should have a different ID")
	}
	t.Logf("force-started session 3: %s (replaced %s)", sid3, sid1)

	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sid3,
	})
	if status, _ := report["status"].(string); status != "done" {
		t.Fatalf("expected done, got %s", status)
	}
}

func TestServer_StaleSession_TTLAutoAbort(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sid := srv.SessionID()

	srv.SetSessionTTL(200 * time.Millisecond)

	callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sid,
		"timeout_ms": 1000,
	})

	time.Sleep(500 * time.Millisecond)

	start2 := startCircuit(t, ctx, session, "ptp-mock", "stub", 0)
	sid2 := start2["session_id"].(string)
	if sid2 == "" || sid2 == sid {
		t.Fatalf("expected new session after TTL abort, got %q", sid2)
	}
	t.Logf("TTL-aborted session %s replaced by %s", sid, sid2)
}

func TestGetNextStep_OverPull_Draining(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "llm", 2)
	sessionID := startResult["session_id"].(string)

	type pullResult struct {
		res map[string]any
		err error
	}
	results := make(chan pullResult, 4)
	start := time.Now()
	for i := 0; i < 4; i++ {
		go func() {
			res, err := callToolE(ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
				"timeout_ms": 500,
			})
			results <- pullResult{res, err}
		}()
	}

	var gotSteps, gotTimeout, gotDone int
	for i := 0; i < 4; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Fatalf("unexpected error: %v", r.err)
			}
			if done, _ := r.res["done"].(bool); done {
				gotDone++
			} else if avail, _ := r.res["available"].(bool); avail {
				gotSteps++
			} else {
				gotTimeout++
			}
		case <-ctx.Done():
			t.Fatalf("DEADLOCK: only %d of 4 calls returned", gotSteps+gotTimeout+gotDone)
		}
	}
	elapsed := time.Since(start)

	if gotSteps != 2 {
		t.Errorf("expected 2 steps, got %d", gotSteps)
	}
	if gotTimeout < 2 {
		t.Errorf("expected at least 2 timeouts, got %d (done=%d)", gotTimeout, gotDone)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("over-pull took %v; should resolve within ~500ms", elapsed)
	}
	t.Logf("draining resolved: %d steps, %d timeouts, %d done in %v", gotSteps, gotTimeout, gotDone, elapsed)
}

// Suppress unused import warning for dumpGoroutines.
var _ = dumpGoroutines

func TestLoadStepSchemas_LegacyFormat(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`
name: F0_RECALL
fields:
  match: "bool"
  confidence: "float"
defs:
  - name: match
    type: bool
    required: true
  - name: confidence
    type: float
    required: true
`)
	if err := os.WriteFile(filepath.Join(dir, "F0_RECALL.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	schemas, err := mcpserver.LoadStepSchemas(os.DirFS(dir))
	if err != nil {
		t.Fatalf("LoadStepSchemas: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0].Name != "F0_RECALL" {
		t.Errorf("name = %q, want F0_RECALL", schemas[0].Name)
	}
	if len(schemas[0].Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(schemas[0].Fields))
	}
	if len(schemas[0].Defs) != 2 {
		t.Errorf("expected 2 defs, got %d", len(schemas[0].Defs))
	}
}

func TestLoadStepSchemas_UnifiedFormat(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`
kind: artifact-schema
version: v1
metadata:
  name: F0_RECALL
  description: "Recall step"
fields:
  match:
    type: bool
    required: true
  confidence:
    type: float
    required: true
`)
	if err := os.WriteFile(filepath.Join(dir, "F0_RECALL.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	schemas, err := mcpserver.LoadStepSchemas(os.DirFS(dir))
	if err != nil {
		t.Fatalf("LoadStepSchemas: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	s := schemas[0]
	if s.Name != "F0_RECALL" {
		t.Errorf("name = %q, want F0_RECALL (from metadata)", s.Name)
	}
	if len(s.Fields) != 2 {
		t.Errorf("expected 2 flat fields, got %d", len(s.Fields))
	}
	if s.Fields["match"] != "bool" {
		t.Errorf("fields[match] = %q, want bool", s.Fields["match"])
	}
	if len(s.Defs) != 2 {
		t.Errorf("expected 2 derived defs, got %d", len(s.Defs))
	}
	for _, d := range s.Defs {
		if d.Type == "" {
			t.Errorf("def %q has empty type", d.Name)
		}
		if !d.Required {
			t.Errorf("def %q should be required", d.Name)
		}
	}
}
