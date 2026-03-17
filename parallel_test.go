package rca_test

import (
	"context"
	"testing"

	framework "github.com/dpopsuev/origami"

	"github.com/dpopsuev/rh-rca"
)

func TestParallel_ResultsMatch(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := mustLoadScenario(t, "ptp-mock")
	stub := rca.NewStubTransformer(scenario)

	serialCfg := rca.RunConfig{
		Scenario:    scenario,
		Components:    []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:    stub,
		Runs:        1,
		Thresholds: rca.DefaultThresholds(),
		Parallel:   1,
		BasePath:   tmpDir,
		ScoreCard:   loadTestScoreCard(t),
		CircuitData: testCircuitData(t),
	}
	serialReport, err := rca.RunCalibration(context.Background(), serialCfg)
	if err != nil {
		t.Fatalf("serial run failed: %v", err)
	}

	parallelCfg := rca.RunConfig{
		Scenario:    scenario,
		Components:    []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:    stub,
		Runs:        1,
		Thresholds: rca.DefaultThresholds(),
		Parallel:   4,
		BasePath:   tmpDir,
		ScoreCard:   loadTestScoreCard(t),
		CircuitData: testCircuitData(t),
	}
	parallelReport, err := rca.RunCalibration(context.Background(), parallelCfg)
	if err != nil {
		t.Fatalf("parallel run failed: %v", err)
	}

	if len(serialReport.CaseResults) != len(parallelReport.CaseResults) {
		t.Fatalf("case count mismatch: serial=%d parallel=%d",
			len(serialReport.CaseResults), len(parallelReport.CaseResults))
	}

	for i := range serialReport.CaseResults {
		sc := serialReport.CaseResults[i]
		pc := parallelReport.CaseResults[i]
		if sc.CaseID != pc.CaseID {
			t.Errorf("case %d: ID mismatch serial=%s parallel=%s", i, sc.CaseID, pc.CaseID)
		}
	}

	parallelM19 := findMetricByID(parallelReport.Metrics, "M19")
	if parallelM19 == nil {
		t.Fatal("M19 metric not found in parallel report")
	}
	if parallelM19.Value < 0.50 {
		t.Errorf("M19 too low in parallel mode: %.3f (want >= 0.50)", parallelM19.Value)
	}
	t.Logf("M19: serial=%.3f parallel=%.3f",
		findMetricByID(serialReport.Metrics, "M19").Value, parallelM19.Value)
}

func TestParallel_NoRace(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := mustLoadScenario(t, "ptp-mock")
	stub := rca.NewStubTransformer(scenario)
	cfg := rca.RunConfig{
		Scenario:    scenario,
		Components:    []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:    stub,
		Runs:        1,
		Thresholds: rca.DefaultThresholds(),
		Parallel:   4,
		BasePath:   tmpDir,
		ScoreCard:   loadTestScoreCard(t),
		CircuitData: testCircuitData(t),
	}

	report, err := rca.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("parallel run failed: %v", err)
	}

	if len(report.CaseResults) != 12 {
		t.Errorf("expected 12 case results, got %d", len(report.CaseResults))
	}
}

func TestParallel_AllCasesComplete(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := mustLoadScenario(t, "ptp-mock")
	stub := rca.NewStubTransformer(scenario)
	cfg := rca.RunConfig{
		Scenario:    scenario,
		Components:    []*framework.Component{rca.TransformerComponent(stub)},
		TransformerName: "stub",
		IDMapper:    stub,
		Runs:        1,
		Thresholds: rca.DefaultThresholds(),
		Parallel:   4,
		BasePath:   tmpDir,
		ScoreCard:   loadTestScoreCard(t),
		CircuitData: testCircuitData(t),
	}

	report, err := rca.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("parallel run failed: %v", err)
	}

	for _, cr := range report.CaseResults {
		if len(cr.ActualPath) == 0 {
			t.Errorf("case %s has empty path", cr.CaseID)
		}
	}
}

func findMetricByID(ms rca.MetricSet, id string) *rca.Metric {
	for _, m := range ms.AllMetrics() {
		if m.ID == id {
			return &m
		}
	}
	return nil
}
