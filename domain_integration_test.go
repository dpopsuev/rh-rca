package rca_test

import (
	"context"
	"io/fs"
	"net/http/httptest"
	"testing"
	"time"

	mcpserver "github.com/dpopsuev/rh-rca/mcpconfig"
	"github.com/dpopsuev/rh-rca/scenarios"

	"github.com/dpopsuev/origami/domainfs"
	"github.com/dpopsuev/origami/domainserve"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// domainSessionCaller adapts *sdkmcp.ClientSession to subprocess.ToolCaller.
type domainSessionCaller struct {
	session *sdkmcp.ClientSession
}

func (s *domainSessionCaller) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	return s.session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
}

// startDomainServer starts a domain data server backed by testdata and returns
// an MCPRemoteFS connected to it via HTTP/MCP.
func startDomainServer(t *testing.T) *domainfs.MCPRemoteFS {
	t.Helper()

	handler := domainserve.New(testDomainFS(t), domainserve.Config{
		Name:    "asterisk",
		Version: "v0.1.0-test",
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-engine", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to domain server: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	return domainfs.New(&domainSessionCaller{session: session}).
		WithTimeout(5 * time.Second)
}

// TestDomainIntegration_ScenarioLoadingOverMCP verifies the full domain
// separation pipeline: domain data served via domainserve over HTTP/MCP,
// fetched by the engine via MCPRemoteFS, scenarios loaded, circuit started.
func TestDomainIntegration_ScenarioLoadingOverMCP(t *testing.T) {
	remoteFS := startDomainServer(t)

	// Verify scenario listing and loading over MCP
	scenarioFS, err := fs.Sub(remoteFS, "scenarios")
	if err != nil {
		t.Fatalf("fs.Sub scenarios: %v", err)
	}
	names := scenarios.ListScenarios(scenarioFS)
	if len(names) == 0 {
		t.Fatal("expected at least one scenario via MCPRemoteFS")
	}
	t.Logf("scenarios via MCPRemoteFS: %v", names)

	scenario, err := scenarios.LoadScenario(scenarioFS, "ptp-mock")
	if err != nil {
		t.Fatalf("LoadScenario via MCPRemoteFS: %v", err)
	}
	if scenario.Name != "ptp-mock" {
		t.Errorf("scenario.Name = %q, want ptp-mock", scenario.Name)
	}
	if len(scenario.Cases) == 0 {
		t.Fatal("scenario has zero cases")
	}
	t.Logf("loaded %q: %d cases, %d RCAs", scenario.Name, len(scenario.Cases), len(scenario.RCAs))

	// Verify other domain data is accessible
	for _, path := range []string{"heuristics.yaml", "vocabulary.yaml", "circuits/rca.yaml"} {
		data, err := fs.ReadFile(remoteFS, path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", path)
		}
	}
}

// TestDomainIntegration_StubCalibrationOverMCP runs a full stub calibration
// where the engine loads all domain data (scenarios, prompts, heuristics,
// vocabulary, circuit def, scorecard, report template) from a remote domain
// server via MCPRemoteFS.
func TestDomainIntegration_StubCalibrationOverMCP(t *testing.T) {
	remoteFS := startDomainServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	engineSrv := mcpserver.NewServer("test-rca",
		mcpserver.WithDomainFS(remoteFS),
		mcpserver.WithStateDir(t.TempDir()),
	)
	engineSrv.ProjectRoot = projectRoot(t)
	t.Cleanup(engineSrv.Shutdown)

	session := connectInMemory(t, ctx, engineSrv)
	defer session.Close()

	startResult := startCircuit(t, ctx, session, "ptp-mock", "stub", 0)
	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("start_circuit returned no session_id: %v", startResult)
	}
	totalCases, _ := startResult["total_cases"].(float64)
	if totalCases < 1 {
		t.Fatalf("expected total_cases >= 1, got %v", totalCases)
	}
	t.Logf("started session %s with %v cases", sessionID, totalCases)

	// Stub backend completes synchronously
	time.Sleep(500 * time.Millisecond)
	stepResult := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	done, _ := stepResult["done"].(bool)
	if !done {
		t.Fatalf("expected done=true for stub backend, got %v", stepResult)
	}
	if errMsg, _ := stepResult["error"].(string); errMsg != "" {
		t.Fatalf("circuit failed: %s", errMsg)
	}

	// Verify the report is available
	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected report status=done, got %s", status)
	}
	reportStr, _ := reportResult["report"].(string)
	if reportStr == "" {
		t.Fatal("expected non-empty report")
	}
	t.Logf("stub calibration over MCP succeeded (report: %d bytes)", len(reportStr))
}
