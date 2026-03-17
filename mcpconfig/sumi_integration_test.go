package mcpconfig_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/kami"
	"github.com/dpopsuev/origami/sumi"
	"github.com/dpopsuev/origami/view"
)

// TestSumi_FourWorkerDrain_SSEObservation runs a full 4-worker calibration
// and verifies an SSE client (simulating Sumi) sees all node_enter,
// node_exit, and walk_complete events.
func TestSumi_FourWorkerDrain_SSEObservation(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 4)
	sessionID := srv.SessionID()

	evtCh := make(chan kami.Event, 1024)
	go readSSEEvents(ctx, httpAddr, evtCh)
	time.Sleep(200 * time.Millisecond)

	var wg sync.WaitGroup
	errCh := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 500,
				})
				if err != nil {
					errCh <- fmt.Errorf("worker-%d get_next_step: %w", subID, err)
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      fieldsForStep(step, subID),
				})
				if err != nil {
					errCh <- fmt.Errorf("worker-%d submit_step: %w", subID, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("worker error: %v", err)
	}

	var nodeEnters, nodeExits int
	var gotComplete bool
	walkersSeen := make(map[string]bool)
	nodesSeen := make(map[string]bool)

	deadline := time.After(5 * time.Second)
drain:
	for {
		select {
		case evt := <-evtCh:
			switch evt.Type {
			case kami.EventNodeEnter:
				nodeEnters++
				nodesSeen[evt.Node] = true
				if evt.Agent != "" {
					walkersSeen[evt.Agent] = true
				}
			case kami.EventNodeExit:
				nodeExits++
				if evt.Agent != "" {
					walkersSeen[evt.Agent] = true
				}
			case kami.EventFanOutStart:
				if evt.Agent != "" {
					walkersSeen[evt.Agent] = true
				}
			case kami.EventWalkComplete:
				gotComplete = true
				break drain
			}
		case <-deadline:
			break drain
		}
	}

	if nodeEnters == 0 {
		t.Error("SSE client received zero node_enter events")
	}
	if nodeExits == 0 {
		t.Error("SSE client received zero node_exit events")
	}
	if !gotComplete {
		t.Error("SSE client never received walk_complete event")
	}

	// "resolve" is optional due to H7b hypothesis-based routing bypass.
	requiredNodes := []string{"recall", "triage", "investigate", "correlate", "review", "report"}
	for _, node := range requiredNodes {
		if !nodesSeen[node] {
			t.Errorf("node %q never appeared in SSE events", node)
		}
	}

	if len(walkersSeen) == 0 {
		t.Error("no walker/case IDs appeared in SSE events")
	}

	t.Logf("SSE observed: %d node_enter, %d node_exit, walk_complete=%v, walkers=%d, nodes=%v",
		nodeEnters, nodeExits, gotComplete, len(walkersSeen), len(nodesSeen))
}

// TestSumi_SnapshotMidFlight verifies /api/snapshot returns active nodes
// and walkers during a running circuit.
func TestSumi_SnapshotMidFlight(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := srv.SessionID()

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	defer resp.Body.Close()

	var snap view.CircuitSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	if snap.CircuitName == "" {
		t.Error("snapshot circuit_name is empty")
	}
	if len(snap.Nodes) == 0 {
		t.Error("snapshot has zero nodes")
	}
	if len(snap.Walkers) == 0 {
		t.Error("snapshot has zero walkers mid-flight")
	}
	if snap.Completed {
		t.Error("snapshot should not be completed mid-flight")
	}

	var activeCount int
	for name, ns := range snap.Nodes {
		if ns.State == view.NodeActive {
			activeCount++
			t.Logf("active node: %s", name)
		}
	}
	if activeCount == 0 {
		t.Error("expected at least one active node mid-flight")
	}

	t.Logf("snapshot mid-flight: circuit=%s, nodes=%d, walkers=%d, active=%d",
		snap.CircuitName, len(snap.Nodes), len(snap.Walkers), activeCount)
}

// TestSumi_SnapshotAfterCompletion verifies /api/snapshot shows
// completed=true and zero walkers after a full circuit drain.
func TestSumi_SnapshotAfterCompletion(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 4)
	sessionID := srv.SessionID()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 500,
				})
				if err != nil {
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)
				callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      fieldsForStep(step, subID),
				})
			}
		}(i)
	}
	wg.Wait()

	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	defer resp.Body.Close()

	var snap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&snap)

	if !snap.Completed {
		t.Error("snapshot should report completed=true after full drain")
	}
	if len(snap.Walkers) > 0 {
		t.Errorf("expected 0 walkers after completion, got %d", len(snap.Walkers))
	}

	var completedNodes int
	for _, ns := range snap.Nodes {
		if ns.State == view.NodeCompleted {
			completedNodes++
		}
	}
	t.Logf("snapshot after completion: completed=%v, walkers=%d, completedNodes=%d/%d",
		snap.Completed, len(snap.Walkers), completedNodes, len(snap.Nodes))
}

// TestSumi_SessionRestart_SSEClientSeesNewCircuit simulates what Sumi
// does during a session restart: the SSE client reconnects and must see
// the new session's events, not stale data from the old session.
func TestSumi_SessionRestart_SSEClientSeesNewCircuit(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Session 1: start and dispatch one step.
	startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID1 := srv.SessionID()

	evtCh := make(chan kami.Event, 100)
	go readSSEEvents(ctx, httpAddr, evtCh)
	time.Sleep(200 * time.Millisecond)

	callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID1,
	})

	deadline1 := time.After(3 * time.Second)
	var s1Events int
s1loop:
	for {
		select {
		case <-evtCh:
			s1Events++
			if s1Events >= 1 {
				break s1loop
			}
		case <-deadline1:
			t.Fatal("timeout waiting for session-1 SSE events")
		}
	}

	// Session 2: force restart.
	start2 := callTool(t, ctx, session, "start_circuit", map[string]any{
		"extra":    map[string]any{"scenario": "ptp-mock", "backend": "llm"},
		"force":    true,
		"parallel": 1,
	})
	sessionID2 := start2["session_id"].(string)
	if sessionID2 == sessionID1 {
		t.Fatal("session IDs should differ after force restart")
	}

	time.Sleep(500 * time.Millisecond)

	callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID2,
	})

	deadline2 := time.After(5 * time.Second)
	var s2Events int
s2loop:
	for {
		select {
		case evt := <-evtCh:
			if evt.Type == kami.EventNodeEnter || evt.Type == kami.EventFanOutStart {
				s2Events++
				break s2loop
			}
		case <-deadline2:
			t.Fatal("timeout waiting for session-2 SSE events after restart")
		}
	}

	if s2Events == 0 {
		t.Error("SSE client did not receive any events from session 2")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	defer resp.Body.Close()

	var snap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&snap)

	if snap.Completed {
		t.Error("session 2 should not be completed yet")
	}
	if len(snap.Nodes) == 0 {
		t.Error("session 2 snapshot should have nodes")
	}

	t.Logf("session restart: s1 events=%d, s2 events=%d, snapshot nodes=%d",
		s1Events, s2Events, len(snap.Nodes))
}

// TestSumi_BootstrapAndSSE_FullPipeline simulates what Sumi does on
// startup: bootstrap from /api/snapshot, then connect SSE and verify
// events flow correctly to a local CircuitStore.
func TestSumi_BootstrapAndSSE_FullPipeline(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 2)
	sessionID := srv.SessionID()

	// Bootstrap: fetch snapshot like Sumi does.
	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	var snap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&snap)
	resp.Body.Close()

	if len(snap.Nodes) == 0 {
		t.Fatal("bootstrap snapshot has zero nodes")
	}

	// Build a client-side CircuitStore from the snapshot (like Sumi does).
	clientStore := sumi.BootstrapStoreFromSnapshot(snap)
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	// Connect SSE client loop to feed the client store.
	go sumi.SSEClientLoop(ctx, httpAddr, clientStore)

	time.Sleep(200 * time.Millisecond)

	// Workers drain the circuit.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 500,
				})
				if err != nil {
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)
				callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      fieldsForStep(step, subID),
				})
			}
		}(i)
	}
	wg.Wait()

	// Wait for SSE events to propagate through the client store.
	// Poll the client snapshot until it shows completed or timeout.
	deadline := time.After(5 * time.Second)
	var clientSnap view.CircuitSnapshot
waitComplete:
	for {
		select {
		case <-clientCh:
			clientSnap = clientStore.Snapshot()
			if clientSnap.Completed {
				break waitComplete
			}
		case <-deadline:
			clientSnap = clientStore.Snapshot()
			break waitComplete
		}
	}

	if !clientSnap.Completed {
		t.Error("client snapshot should show completed=true")
	}
	if len(clientSnap.Nodes) == 0 {
		t.Error("client snapshot has zero nodes")
	}

	var completedNodes int
	for _, ns := range clientSnap.Nodes {
		if ns.State == view.NodeCompleted {
			completedNodes++
		}
	}
	if completedNodes == 0 {
		t.Error("client store has zero completed nodes")
	}

	t.Logf("client store pipeline: completed=%v, nodes=%d, completedNodes=%d, walkers=%d",
		clientSnap.Completed, len(clientSnap.Nodes), completedNodes, len(clientSnap.Walkers))
}

// TestStaleness_ForceReplaceCleansPreviousSession starts a circuit,
// dispatches a step (creating a walker), then force-starts a new session.
// The snapshot after the force-replace should have no walkers from the
// old session — OnSessionEnd fires WalkComplete which clears them.
func TestStaleness_ForceReplaceCleansPreviousSession(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := srv.SessionID()

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if step["done"] == true {
		t.Fatal("expected a step, got done=true")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	var midSnap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&midSnap)
	resp.Body.Close()

	if len(midSnap.Walkers) == 0 {
		t.Fatal("expected walkers mid-flight, got 0")
	}
	t.Logf("mid-flight: walkers=%d", len(midSnap.Walkers))

	// Force-replace with a new session. OnSessionEnd should fire.
	callTool(t, ctx, session, "start_circuit", map[string]any{
		"force": true,
		"extra": map[string]any{
			"scenario": "ptp-mock",
			"backend":  "llm",
		},
	})

	// The new session creates a fresh store. Snapshot should be clean.
	resp2, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot after force: %v", err)
	}
	var newSnap view.CircuitSnapshot
	json.NewDecoder(resp2.Body).Decode(&newSnap)
	resp2.Body.Close()

	if len(newSnap.Walkers) > 0 {
		t.Errorf("expected 0 walkers after force-replace, got %d", len(newSnap.Walkers))
	}
	if newSnap.Completed {
		t.Error("new session should not be completed yet")
	}
	t.Logf("after force-replace: walkers=%d, completed=%v, nodes=%d",
		len(newSnap.Walkers), newSnap.Completed, len(newSnap.Nodes))
}

// TestStaleness_HTTPResetEndpoint verifies that POST /api/store/reset
// clears nodes, walkers, and completion from the store.
func TestStaleness_HTTPResetEndpoint(t *testing.T) {
	srv, httpAddr := newTestServerWithKami(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startCircuit(t, ctx, session, "ptp-mock", "llm", 1)
	sessionID := srv.SessionID()

	// Dispatch a step so there's a walker.
	callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})

	// Verify state exists before reset.
	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	var beforeSnap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&beforeSnap)
	resp.Body.Close()

	if len(beforeSnap.Nodes) == 0 {
		t.Fatal("expected nodes before reset")
	}

	// POST /api/store/reset
	resp2, err := http.Post(fmt.Sprintf("http://%s/api/store/reset", httpAddr), "application/json", nil)
	if err != nil {
		t.Fatalf("POST reset: %v", err)
	}
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	// Snapshot after reset should be empty.
	resp3, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot after reset: %v", err)
	}
	var afterSnap view.CircuitSnapshot
	json.NewDecoder(resp3.Body).Decode(&afterSnap)
	resp3.Body.Close()

	if len(afterSnap.Nodes) > 0 {
		t.Errorf("expected 0 nodes after reset, got %d", len(afterSnap.Nodes))
	}
	if len(afterSnap.Walkers) > 0 {
		t.Errorf("expected 0 walkers after reset, got %d", len(afterSnap.Walkers))
	}
	if afterSnap.Completed {
		t.Error("expected completed=false after reset")
	}
	t.Logf("after reset: nodes=%d, walkers=%d, completed=%v",
		len(afterSnap.Nodes), len(afterSnap.Walkers), afterSnap.Completed)
}

// TestStaleness_ResetNilDef verifies CircuitStore.Reset(nil) produces
// an empty store without panicking.
func TestStaleness_ResetNilDef(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "test",
		Nodes:   []framework.NodeDef{{Name: "A"}, {Name: "B"}},
	}
	store := view.NewCircuitStore(def)
	defer store.Close()

	snap := store.Snapshot()
	if len(snap.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(snap.Nodes))
	}

	store.Reset(nil)
	snap = store.Snapshot()
	if len(snap.Nodes) != 0 {
		t.Errorf("expected 0 nodes after Reset(nil), got %d", len(snap.Nodes))
	}
	if len(snap.Walkers) != 0 {
		t.Errorf("expected 0 walkers after Reset(nil), got %d", len(snap.Walkers))
	}
	if snap.CircuitName != "" {
		t.Errorf("expected empty circuit name after Reset(nil), got %q", snap.CircuitName)
	}
}
