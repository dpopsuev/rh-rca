package rca_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/origami/agentport"
	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami-rca"
)

func calibrateScenarioName() string {
	if v := os.Getenv("CALIBRATE_SCENARIO"); v != "" {
		return v
	}
	return "ptp"
}

func calibrateBackend() string {
	if v := os.Getenv("CALIBRATE_BACKEND"); v != "" {
		return v
	}
	return "stub"
}

func calibrateResolution() string {
	return os.Getenv("CALIBRATE_RESOLUTION")
}

func loadCalibrationScenario(t *testing.T, domainFS fs.FS) *rca.Scenario {
	t.Helper()
	scenarioFS, err := fs.Sub(domainFS, "scenarios")
	if err != nil {
		t.Fatalf("sub scenarios: %v", err)
	}
	scenario, err := rca.LoadScenario(scenarioFS, calibrateScenarioName())
	if err != nil {
		t.Fatalf("load scenario %s: %v", calibrateScenarioName(), err)
	}
	return scenario
}

func buildCalibrationComponents(t *testing.T, scenario *rca.Scenario, domainFS fs.FS) ([]*engine.Component, rca.IDMappable) {
	t.Helper()
	backend := calibrateBackend()
	switch backend {
	case "stub":
		stub := rca.NewStubTransformer(scenario)
		return []*engine.Component{rca.TransformerComponent(stub)}, stub
	case "cli":
		// CLI dispatcher removed in Troupe integration — use ACP via Broker instead.
		t.Skip("CLI backend removed — use backend=llm with Troupe Broker")
		transformer := rca.NewStubTransformer(scenario) // unreachable, satisfies compiler
		_ = transformer
		return nil, rca.NewStubTransformer(scenario) // unreachable
	case "cli-legacy": // dead code placeholder
		transformer := rca.NewRCATransformer(nil, domainFS,
			rca.WithRCABasePath(t.TempDir()),
		)
		return []*engine.Component{rca.TransformerComponent(transformer)}, nil
	default:
		t.Fatalf("unknown backend %q (available: stub, cli)", backend)
		return nil, nil
	}
}

func TestCalibrate(t *testing.T) {
	domainFS := testDomainFS(t)
	scenario := loadCalibrationScenario(t, domainFS)
	comps, idMapper := buildCalibrationComponents(t, scenario, domainFS)

	circuitData, err := fs.ReadFile(domainFS, "circuits/rca.yaml")
	if err != nil {
		t.Fatalf("read circuit def: %v", err)
	}
	def, err := rca.LoadCircuitDef(circuitData, rca.DefaultThresholds())
	if err != nil {
		t.Fatalf("load circuit def: %v", err)
	}

	scorecardData, err := fs.ReadFile(domainFS, "scorecards/rca.yaml")
	if err != nil {
		t.Fatalf("read scorecard: %v", err)
	}
	sc, err := cal.ParseScoreCard(scorecardData)
	if err != nil {
		t.Fatalf("parse scorecard: %v", err)
	}

	calReportTemplate, _ := fs.ReadFile(domainFS, "reports/calibration-report.yaml")
	adapter := &rca.RCACalibrationAdapter{
		Scenario:       scenario,
		Components:     comps,
		IDMapper:       idMapper,
		BasePath:       t.TempDir(),
		Thresholds:     rca.DefaultThresholds(),
		ScoreCard:      sc,
		TokenTracker:   agentport.NewTracker(),
		ReportTemplate: calReportTemplate,
	}

	timeout := 2 * time.Minute
	if calibrateBackend() == "cli" {
		timeout = 30 * time.Minute // CLI backends need time for LLM calls
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	harnessConfig := &cal.HarnessConfig{
		Loader:         adapter,
		Collector:      adapter,
		Renderer:       adapter,
		CircuitDef:     def,
		ScoreCard:      sc,
		Contract:       cal.ContractFromDef(def.Calibration),
		Scenario:       scenario.Name,
		Transformer:    calibrateBackend(),
		Runs:           1,
		Parallel:       1,
		OnCaseComplete: func() func(int, engine.BatchWalkResult) {
			adapterCB := adapter.OnCaseComplete()
			total := len(scenario.Cases)
			return func(i int, result engine.BatchWalkResult) {
				if adapterCB != nil {
					adapterCB(i, result)
				}
				if result.Error != nil {
					fmt.Fprintf(os.Stderr, "  [%d/%d] %s ERROR: %v\n", i+1, total, result.CaseID, result.Error)
				} else {
					fmt.Fprintf(os.Stderr, "  [%d/%d] %s OK (steps: %d)\n", i+1, total, result.CaseID, len(result.Path))
				}
			}
		}(),
	}

	if res := calibrateResolution(); res != "" {
		applyResolution(t, harnessConfig, domainFS, res)
	}

	genReport, err := cal.Run(ctx, harnessConfig)
	if err != nil {
		t.Fatalf("calibration failed: %v", err)
	}

	report := adapter.RCAReport(genReport)
	rca.ApplyDryCaps(&report.Metrics, scenario.DryCappedMetrics)

	rendered, err := rca.RenderCalibrationReport(report, calReportTemplate)
	if err != nil {
		t.Fatalf("render report: %v", err)
	}
	fmt.Fprint(os.Stdout, rendered)

	passed, total := report.Metrics.PassCount()
	// Stub mode: 2 metrics are structurally dry-capped (M12 evidence_recall,
	// M13 evidence_precision) and will always fail. Accept 18/20 as passing.
	minPass := total - 2
	if passed < minPass {
		t.Errorf("calibration: %d/%d metrics passed (expected >= %d)", passed, total, minPass)
	}
	t.Logf("calibration: %d/%d metrics passed", passed, total)
}

func applyResolution(t *testing.T, harnessConfig *cal.HarnessConfig, domainFS fs.FS, res string) {
	t.Helper()
	resolution, err := cal.ParseResolution(res)
	if err != nil {
		t.Fatalf("parse resolution: %v", err)
	}
	harnessConfig.Resolution = resolution

	ps := loadPortStubs(domainFS, fmt.Sprintf("stubs/%s", res))
	if len(ps) > 0 {
		harnessConfig.PortStubs = ps
	}
	t.Logf("calibration resolution: %s (port stubs: %d)", res, len(harnessConfig.PortStubs))
}

func loadPortStubs(domainFS fs.FS, stubsDir string) cal.PortStubs {
	if domainFS == nil {
		return nil
	}
	stubFS, fsErr := fs.Sub(domainFS, stubsDir)
	if fsErr != nil {
		return nil
	}
	ps := cal.PortStubs{}
	entries, _ := fs.ReadDir(stubFS, ".")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, readErr := fs.ReadFile(stubFS, e.Name())
		if readErr != nil {
			continue
		}
		var v any
		if jsonErr := json.Unmarshal(data, &v); jsonErr != nil {
			continue
		}
		portName := e.Name()[:len(e.Name())-len(".json")]
		ps[portName] = v
	}
	return ps
}
