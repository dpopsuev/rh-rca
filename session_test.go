package rca_test

import (
	"context"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStartCircuit_InvalidScenario(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "start_circuit",
		Arguments: map[string]any{
			"extra": map[string]any{"scenario": "nonexistent", "backend": "stub"},
		},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for invalid scenario")
	}
}

func TestStartCircuit_PTP_VerifiedOnly(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp", "stub", 0)
	sessionID := startResult["session_id"].(string)
	totalCases, _ := startResult["total_cases"].(float64)

	if int(totalCases) != 18 {
		t.Fatalf("expected 18 verified cases, got %v", totalCases)
	}
	t.Logf("ptp: %v verified cases", totalCases)

	time.Sleep(500 * time.Millisecond)

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
	report, _ := reportResult["report"].(string)
	if report == "" {
		t.Fatal("expected non-empty report")
	}
	t.Logf("ptp report: %.200s...", report)
}
