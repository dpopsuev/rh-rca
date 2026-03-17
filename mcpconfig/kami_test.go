package mcpconfig_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/kami"
	mcpserver "github.com/dpopsuev/rh-rca/mcpconfig"
)

func newTestServerWithKami(t *testing.T) (*mcpserver.Server, string) {
	t.Helper()
	srv := mcpserver.NewServer("test-rca",
		mcpserver.WithDomainFS(testDomainFS(t)),
		mcpserver.WithStateDir(t.TempDir()),
	)
	srv.ProjectRoot = projectRoot(t)

	bridge := kami.NewEventBridge(nil)
	kamiSrv := kami.NewServer(kami.Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())

	httpAddr, _, err := kamiSrv.StartOnAvailablePort(ctx)
	if err != nil {
		cancel()
		t.Fatalf("kami start: %v", err)
	}

	srv.Observer = kami.NewSessionObserver(kamiSrv)
	kami.RegisterMCPTools(srv.MCPServer, nil, kamiSrv)

	t.Cleanup(func() {
		srv.Shutdown()
		cancel()
		bridge.Close()
	})

	return srv, httpAddr
}

func TestKamiWiring_SessionCreatesStore(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Before session: SSE should return 503.
	resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
	if err != nil {
		t.Fatalf("GET SSE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before session, got %d", resp.StatusCode)
	}

	startCircuit(t, ctx, session, "ptp-mock", "stub", 0)

	// After session: SSE should return 200.
	resp, err = http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
	if err != nil {
		t.Fatalf("GET SSE after session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after session, got %d", resp.StatusCode)
	}
}

func TestKamiWiring_StepDispatchEmitsNodeEnter(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := srv.SessionID()

	evtCh := make(chan kami.Event, 10)
	go readSSEEvents(ctx, httpAddr, evtCh)

	time.Sleep(100 * time.Millisecond)

	callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})

	timeout := time.After(5 * time.Second)
	for {
		select {
		case evt := <-evtCh:
			if evt.Type == kami.EventNodeEnter {
				t.Logf("received node_enter for node=%q", evt.Node)
				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for node_enter event on SSE")
		}
	}
}

func TestKamiWiring_StepSubmitEmitsNodeExit(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := srv.SessionID()

	evtCh := make(chan kami.Event, 10)
	go readSSEEvents(ctx, httpAddr, evtCh)

	time.Sleep(100 * time.Millisecond)

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})
	stepName, _ := step["step"].(string)
	dispatchID, _ := step["dispatch_id"].(float64)

	callTool(t, ctx, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": int64(dispatchID),
		"step":        stepName,
		"fields":      fieldsForStep(stepName, 0),
	})

	timeout := time.After(5 * time.Second)
	for {
		select {
		case evt := <-evtCh:
			if evt.Type == kami.EventNodeExit {
				t.Logf("received node_exit for node=%q", evt.Node)
				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for node_exit event on SSE")
		}
	}
}

func TestKamiWiring_SequentialSessions(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	for i := 0; i < 3; i++ {
		start := startCircuit(t, ctx, session, "ptp-mock", "stub", 0)
		sessionID := start["session_id"].(string)

		time.Sleep(500 * time.Millisecond)

		step := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
		})
		if done, _ := step["done"].(bool); !done {
			t.Fatalf("session %d: expected done=true for stub", i)
		}

		// Verify SSE is accessible after each session.
		resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
		if err != nil {
			t.Fatalf("session %d: GET SSE: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("session %d: expected 200, got %d", i, resp.StatusCode)
		}

		t.Logf("session %d (%s) completed, SSE accessible", i, sessionID)
	}
}

func TestKamiWiring_BridgeCleanup(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start session 1 and connect SSE reader.
	startCircuit(t, ctx, session, "ptp-mock", "stub", 0)
	time.Sleep(500 * time.Millisecond)

	s1Done := make(chan struct{})
	go func() {
		defer close(s1Done)
		resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
		if err != nil {
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Start session 2 (force), which should close session 1's store.
	callTool(t, ctx, session, "start_circuit", map[string]any{
		"extra": map[string]any{"scenario": "ptp-mock", "backend": "stub"},
		"force": true,
	})

	select {
	case <-s1Done:
		t.Log("SSE reader from session 1 exited cleanly after session 2 started")
	case <-time.After(5 * time.Second):
		t.Fatal("SSE reader from session 1 did not exit after session 2 started")
	}
}

func readSSEEvents(ctx context.Context, httpAddr string, out chan<- kami.Event) {
	for {
		if ctx.Err() != nil {
			return
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/events/stream", httpAddr), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var evt kami.Event
			if err := json.Unmarshal([]byte(line[6:]), &evt); err != nil {
				continue
			}
			select {
			case out <- evt:
			case <-ctx.Done():
				resp.Body.Close()
				return
			}
		}
		resp.Body.Close()
	}
}

func TestKamiWiring_ToolDiscovery(t *testing.T) {
	srv, _ := newTestServerWithKami(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	registered := make(map[string]bool)
	for _, tool := range tools.Tools {
		registered[tool.Name] = true
	}

	circuitTools := []string{
		"start_circuit", "get_next_step", "submit_step",
		"get_report", "emit_signal", "get_signals", "get_worker_health",
	}
	kamiTools := []string{
		"sumi_get_view",
		"kami_get_selection",
		"kami_highlight_nodes", "kami_highlight_zone", "kami_zoom_to_zone",
		"kami_place_marker", "kami_clear_all", "kami_set_speed",
	}

	for _, name := range circuitTools {
		if !registered[name] {
			t.Errorf("circuit tool %q missing", name)
		}
	}
	for _, name := range kamiTools {
		if !registered[name] {
			t.Errorf("kami tool %q missing — wiring gap in serve command", name)
		}
	}

	debugTools := []string{
		"kami_get_circuit_state", "kami_get_snapshot", "kami_get_assertions",
		"kami_pause", "kami_resume", "kami_advance_node",
		"kami_set_breakpoint", "kami_clear_breakpoint",
	}
	for _, name := range debugTools {
		if registered[name] {
			t.Errorf("debug tool %q should not be registered (dc=nil in serve mode)", name)
		}
	}

	t.Logf("tool discovery: %d circuit + %d kami tools registered, %d debug tools correctly absent",
		len(circuitTools), len(kamiTools), len(debugTools))
}

// Suppress unused import warning.
var _ sync.Mutex
