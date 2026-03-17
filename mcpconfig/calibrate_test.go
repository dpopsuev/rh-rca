package mcpconfig_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	framework "github.com/dpopsuev/origami"
	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/rh-rca"
	"github.com/dpopsuev/rh-rca/scenarios"
)

func calibrateScenarioName() string {
	if v := os.Getenv("CALIBRATE_SCENARIO"); v != "" {
		return v
	}
	return "ptp-mock"
}

func calibrateBackend() string {
	if v := os.Getenv("CALIBRATE_BACKEND"); v != "" {
		return v
	}
	return "stub"
}

func calibrateMode() string {
	if v := os.Getenv("CALIBRATE_MODE"); v != "" {
		return v
	}
	return "offline"
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
	scenario, err := scenarios.LoadScenario(scenarioFS, calibrateScenarioName())
	if err != nil {
		t.Fatalf("load scenario %s: %v", calibrateScenarioName(), err)
	}
	return scenario
}

func buildCalibrationComponents(t *testing.T, scenario *rca.Scenario) ([]*framework.Component, rca.IDMappable) {
	t.Helper()
	backend := calibrateBackend()
	switch backend {
	case "stub":
		stub := rca.NewStubTransformer(scenario)
		return []*framework.Component{rca.TransformerComponent(stub)}, stub
	default:
		t.Fatalf("test wrapper only supports stub backend (got %q); use MCP server for llm", backend)
		return nil, nil
	}
}

func TestCalibrate(t *testing.T) {
	domainFS := testDomainFS(t)
	scenario := loadCalibrationScenario(t, domainFS)
	comps, idMapper := buildCalibrationComponents(t, scenario)

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
		TokenTracker:   dispatch.NewTokenTracker(),
		ReportTemplate: calReportTemplate,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	harnessConfig := cal.HarnessConfig{
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
		OnCaseComplete: adapter.OnCaseComplete(),
	}

	if res := calibrateResolution(); res != "" {
		resolution, err := cal.ParseResolution(res)
		if err != nil {
			t.Fatalf("parse resolution: %v", err)
		}
		harnessConfig.Resolution = resolution

		// Load port stubs for isolated resolutions from domain FS.
		stubsDir := fmt.Sprintf("stubs/%s", res)
		if domainFS != nil {
			stubFS, fsErr := fs.Sub(domainFS, stubsDir)
			if fsErr == nil {
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
				if len(ps) > 0 {
					harnessConfig.PortStubs = ps
				}
			}
		}
		t.Logf("calibration resolution: %s (port stubs: %d)", res, len(harnessConfig.PortStubs))
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
	if passed < total {
		t.Errorf("calibration: %d/%d metrics passed", passed, total)
	}
}
