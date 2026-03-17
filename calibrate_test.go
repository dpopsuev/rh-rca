package rca_test

import (
	"context"
	"io/fs"
	"math"
	"os"
	"testing"

	cal "github.com/dpopsuev/origami/calibrate"
	framework "github.com/dpopsuev/origami"

	"github.com/dpopsuev/rh-rca"
	"github.com/dpopsuev/rh-rca/scenarios"
)

func testCircuitData(t *testing.T) []byte {
	t.Helper()
	return readTestdata(t, "circuit_rca.yaml")
}

func loadTestScoreCard(t *testing.T) *cal.ScoreCard {
	t.Helper()
	sc, err := cal.LoadScoreCard("testdata/scorecard.yaml")
	if err != nil {
		t.Fatalf("load scorecard: %v", err)
	}
	return sc
}

func scenarioFS() fs.FS {
	sub, _ := fs.Sub(os.DirFS(testdataDir()), "scenarios")
	return sub
}

func mustLoadScenario(t *testing.T, name string) *rca.Scenario {
	t.Helper()
	s, err := scenarios.LoadScenario(scenarioFS(), name)
	if err != nil {
		t.Fatalf("LoadScenario(%q): %v", name, err)
	}
	return s
}

func TestStubCalibration_AllMetricsPass(t *testing.T) {
	// Override the orchestrate base path to a temp dir
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := mustLoadScenario(t, "ptp-mock")
	stub := rca.NewStubTransformer(scenario)
	cfg := rca.RunConfig{
		Scenario:    scenario,
		Components:    []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:    stub,
		Runs:        1,
		Thresholds:  rca.DefaultThresholds(),
		BasePath:    tmpDir,
		ScoreCard:   loadTestScoreCard(t),
		CircuitData: testCircuitData(t),
	}

	report, err := rca.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	// All 20 metrics should pass
	passed, total := report.Metrics.PassCount()
	if passed != total {
		t.Errorf("metrics: %d/%d passed; expected all to pass", passed, total)
		for _, m := range report.Metrics.AllMetrics() {
			if !m.Pass {
				t.Errorf("  FAIL: %s (%s) = %.2f (threshold %.2f) detail=%s",
					m.ID, m.Name, m.Value, m.Threshold, m.Detail)
			}
		}
	}

	// Verify case count
	if len(report.CaseResults) != 12 {
		t.Errorf("expected 12 case results, got %d", len(report.CaseResults))
	}

	// All 12 paths should be correct
	correctPaths := 0
	for _, cr := range report.CaseResults {
		if cr.PathCorrect {
			correctPaths++
		}
	}
	if correctPaths != 12 {
		t.Errorf("expected 12 correct paths, got %d", correctPaths)
		for _, cr := range report.CaseResults {
			if !cr.PathCorrect {
				t.Logf("  %s: actual=%v", cr.CaseID, cr.ActualPath)
			}
		}
	}

	// Spot-check serial killer detection (R1 across versions)
	r1Cases := map[string]bool{"C1": true, "C2": true, "C3": true, "C6": true, "C9": true, "C10": true}
	var r1RCAIDs []int64
	for _, cr := range report.CaseResults {
		if r1Cases[cr.CaseID] {
			if cr.ActualRCAID == 0 {
				t.Errorf("case %s should have an RCA link", cr.CaseID)
			}
			r1RCAIDs = append(r1RCAIDs, cr.ActualRCAID)
		}
	}
	if len(r1RCAIDs) > 1 {
		first := r1RCAIDs[0]
		for _, id := range r1RCAIDs[1:] {
			if id != first {
				t.Errorf("serial killer: not all R1 cases linked to same RCA (%v)", r1RCAIDs)
				break
			}
		}
	}
}

func TestStubCalibration_MultiRun(t *testing.T) {
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := mustLoadScenario(t, "ptp-mock")
	stub := rca.NewStubTransformer(scenario)
	cfg := rca.RunConfig{
		Scenario:    scenario,
		Components:    []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:    stub,
		Runs:        3,
		Thresholds:  rca.DefaultThresholds(),
		BasePath:    tmpDir,
		ScoreCard:   loadTestScoreCard(t),
		CircuitData: testCircuitData(t),
	}

	report, err := rca.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	// Variance (M20) should be ~0 for deterministic stub
	for _, m := range report.Metrics.AllMetrics() {
		if m.ID == "M20" && math.Abs(m.Value) > 1e-9 {
			t.Errorf("M20 run_variance should be ~0 for deterministic stub, got %e", m.Value)
		}
	}

	// All metrics should still pass
	passed, total := report.Metrics.PassCount()
	if passed != total {
		t.Errorf("multi-run: %d/%d passed", passed, total)
	}
}

func TestFormatReport(t *testing.T) {
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := mustLoadScenario(t, "ptp-mock")
	stub := rca.NewStubTransformer(scenario)
	cfg := rca.DefaultRunConfig(scenario, []*framework.Component{rca.TransformerComponent(stub)}, "stub")
	cfg.IDMapper = stub
	cfg.Thresholds = rca.DefaultThresholds()
	cfg.BasePath = tmpDir
	cfg.ScoreCard = loadTestScoreCard(t)
	cfg.CircuitData = testCircuitData(t)

	report, err := rca.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	output, renderErr := rca.RenderCalibrationReport(report, readTestdata(t, "reports/calibration-report.yaml"))
	if renderErr != nil {
		t.Fatalf("RenderCalibrationReport: %v", renderErr)
	}
	if len(output) == 0 {
		t.Fatal("RenderCalibrationReport returned empty string")
	}

	checks := []string{
		"Asterisk Calibration Report",
		"ptp-mock",
		"stub",
		"Outcome",
		"Investigation",
		"Detection",
		"Efficiency",
		"Meta",
		"RESULT: PASS",
		"Per-case breakdown",
	}
	for _, check := range checks {
		if !containsStr(output, check) {
			t.Errorf("report missing expected text: %q", check)
		}
	}
}

func TestStubCalibration_DaemonMock(t *testing.T) {
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := mustLoadScenario(t, "daemon-mock")
	stub := rca.NewStubTransformer(scenario)
	cfg := rca.RunConfig{
		Scenario:    scenario,
		Components:    []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:    stub,
		Runs:        1,
		Thresholds:  rca.DefaultThresholds(),
		BasePath:    tmpDir,
		ScoreCard:   loadTestScoreCard(t),
		CircuitData: testCircuitData(t),
	}

	report, err := rca.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	passed, total := report.Metrics.PassCount()
	if passed != total {
		t.Errorf("daemon-mock metrics: %d/%d passed", passed, total)
		for _, m := range report.Metrics.AllMetrics() {
			if !m.Pass {
				t.Errorf("  FAIL: %s (%s) = %.2f (threshold %.2f) detail=%s",
					m.ID, m.Name, m.Value, m.Threshold, m.Detail)
			}
		}
	}

	if len(report.CaseResults) != 8 {
		t.Errorf("expected 8 case results, got %d", len(report.CaseResults))
	}

	// All paths should be correct
	for _, cr := range report.CaseResults {
		if !cr.PathCorrect {
			t.Errorf("case %s path incorrect: actual=%v", cr.CaseID, cr.ActualPath)
		}
	}

	// Verify cascade detected for C6
	for _, cr := range report.CaseResults {
		if cr.CaseID == "C6" && !cr.ActualCascade {
			t.Error("C6 should have cascade detected")
		}
	}
}

func TestStubCalibration_PTP(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := mustLoadScenario(t, "ptp")
	stub := rca.NewStubTransformer(scenario)
	cfg := rca.RunConfig{
		Scenario:        scenario,
		Components:      []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:        stub,
		Runs:            1,
		Thresholds:      rca.DefaultThresholds(),
		BasePath:        tmpDir,
		ScoreCard:       loadTestScoreCard(t),
		CircuitData:     testCircuitData(t),
	}

	report, err := rca.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	if len(report.CaseResults) != 18 {
		t.Errorf("expected 18 verified case results, got %d", len(report.CaseResults))
	}

	passed, total := report.Metrics.PassCount()
	t.Logf("ptp stub calibration: %d/%d metrics passed", passed, total)
}

func TestScenarioCoverage(t *testing.T) {
	testCases := []struct {
		name     string
		scenario string
		rcas     int
		symptoms int
		cases    int
		repos    int
	}{
		{"ptp-mock", "ptp-mock", 3, 4, 12, 5},
		{"daemon-mock", "daemon-mock", 2, 3, 8, 5},
		{"ptp", "ptp", 30, 30, 18, 6},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustLoadScenario(t, tc.scenario)

			if len(s.RCAs) != tc.rcas {
				t.Errorf("expected %d RCAs, got %d", tc.rcas, len(s.RCAs))
			}
			if len(s.Symptoms) != tc.symptoms {
				t.Errorf("expected %d symptoms, got %d", tc.symptoms, len(s.Symptoms))
			}
			if len(s.Cases) != tc.cases {
				t.Errorf("expected %d cases, got %d", tc.cases, len(s.Cases))
			}
			if len(s.SourcePack.Repos) != tc.repos {
				t.Errorf("expected %d source pack repos, got %d", tc.repos, len(s.SourcePack.Repos))
			}

			hasRedHerring := false
			for _, r := range s.SourcePack.Repos {
				if r.IsRedHerring {
					hasRedHerring = true
				}
			}
			if !hasRedHerring {
				t.Error("workspace should have at least one red herring repo")
			}

			// Check all cases reference valid RCAs and symptoms
			rcaSet := make(map[string]bool)
			for _, r := range s.RCAs {
				rcaSet[r.ID] = true
			}
			symSet := make(map[string]bool)
			for _, sym := range s.Symptoms {
				symSet[sym.ID] = true
			}
			for _, c := range s.Cases {
				if c.RCAID != "" && !rcaSet[c.RCAID] {
					t.Errorf("case %s references unknown RCA %q", c.ID, c.RCAID)
				}
				if c.SymptomID != "" && !symSet[c.SymptomID] {
					t.Errorf("case %s references unknown symptom %q", c.ID, c.SymptomID)
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
