package rca

import (
	"strings"
	"testing"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/dispatch"
)

func TestBuildCostBill_NilTokens(t *testing.T) {
	report := &CalibrationReport{CalibrationReport: cal.CalibrationReport{Scenario: "test", Transformer: "stub"}}
	bill := BuildCostBill(report)
	if bill != nil {
		t.Fatal("expected nil bill when Tokens is nil")
	}
}

func TestBuildCostBill_Basic(t *testing.T) {
	report := &CalibrationReport{
		CalibrationReport: cal.CalibrationReport{
			Scenario:    "ptp",
			Transformer: "llm",
			Tokens: &dispatch.TokenSummary{
				TotalPromptTokens:   100_000,
				TotalArtifactTokens: 5_000,
				TotalTokens:         105_000,
				TotalCostUSD:        0.375,
				TotalSteps:          12,
				TotalWallClockMs:    60_000,
				PerCase: map[string]dispatch.CaseTokenSummary{
					"C1": {PromptTokens: 60000, ArtifactTokens: 3000, TotalTokens: 63000, Steps: 7, WallClockMs: 35000},
					"C2": {PromptTokens: 40000, ArtifactTokens: 2000, TotalTokens: 42000, Steps: 5, WallClockMs: 25000},
				},
				PerStep: map[string]dispatch.StepTokenSummary{
					"F0_RECALL":      {PromptTokens: 20000, ArtifactTokens: 1000, TotalTokens: 21000, Invocations: 2},
					"F1_TRIAGE":      {PromptTokens: 30000, ArtifactTokens: 2000, TotalTokens: 32000, Invocations: 2},
					"F3_INVESTIGATE": {PromptTokens: 50000, ArtifactTokens: 2000, TotalTokens: 52000, Invocations: 8},
				},
			},
		},
		CaseResults: []CaseResult{
			{CaseID: "C1", TestName: "TestPTP/sync_loss", Version: "4.16", Job: "e2e"},
			{CaseID: "C2", TestName: "TestPTP/holdover", Version: "4.16", Job: "e2e"},
		},
	}

	bill := BuildCostBill(report)
	if bill == nil {
		t.Fatal("expected non-nil bill")
	}

	if bill.CaseCount != 2 {
		t.Errorf("CaseCount: got %d, want 2", bill.CaseCount)
	}
	if len(bill.CaseLines) != 2 {
		t.Errorf("CaseLines: got %d, want 2", len(bill.CaseLines))
	}
	if len(bill.StepLines) != 3 {
		t.Errorf("StepLines: got %d, want 3", len(bill.StepLines))
	}

	if bill.StepLines[0].Step != "F0_RECALL" {
		t.Errorf("first step: got %s, want F0_RECALL", bill.StepLines[0].Step)
	}
	if bill.StepLines[1].Step != "F1_TRIAGE" {
		t.Errorf("second step: got %s, want F1_TRIAGE", bill.StepLines[1].Step)
	}
	if bill.StepLines[2].Step != "F3_INVESTIGATE" {
		t.Errorf("third step: got %s, want F3_INVESTIGATE", bill.StepLines[2].Step)
	}

	cost := dispatch.DefaultCostConfig()
	for _, cl := range bill.CaseLines {
		if cl.CaseID == "C1" {
			expectedCost := float64(60000)/1e6*cost.InputPricePerMToken + float64(3000)/1e6*cost.OutputPricePerMToken
			if cl.CostUSD != expectedCost {
				t.Errorf("C1 cost: got %f, want %f", cl.CostUSD, expectedCost)
			}
		}
	}
}

func TestFormatCostBill_Nil(t *testing.T) {
	out := dispatch.FormatCostBill(nil)
	if out != "" {
		t.Errorf("expected empty string for nil bill, got %d bytes", len(out))
	}
}

func TestFormatCostBill_Markdown(t *testing.T) {
	report := &CalibrationReport{
		CalibrationReport: cal.CalibrationReport{
			Scenario:    "ptp",
			Transformer: "llm",
			Tokens: &dispatch.TokenSummary{
				TotalPromptTokens:   100_000,
				TotalArtifactTokens: 5_000,
				TotalTokens:         105_000,
				TotalCostUSD:        0.375,
				TotalSteps:          12,
				TotalWallClockMs:    90_000,
				PerCase: map[string]dispatch.CaseTokenSummary{
					"C1": {PromptTokens: 60000, ArtifactTokens: 3000, TotalTokens: 63000, Steps: 7, WallClockMs: 50000},
					"C2": {PromptTokens: 40000, ArtifactTokens: 2000, TotalTokens: 42000, Steps: 5, WallClockMs: 40000},
				},
				PerStep: map[string]dispatch.StepTokenSummary{
					"F1_TRIAGE": {PromptTokens: 30000, ArtifactTokens: 2000, TotalTokens: 32000, Invocations: 2},
				},
			},
		},
		CaseResults: []CaseResult{
			{CaseID: "C1", TestName: "TestPTP/sync_loss", Version: "4.16", Job: "e2e"},
			{CaseID: "C2", TestName: "TestPTP/holdover_timeout_very_long_name_test", Version: "4.16", Job: "e2e"},
		},
	}

	bill := BuildCostBill(report)
	md := dispatch.FormatCostBill(bill)

	checks := []string{
		"# TokiMeter",
		"## Summary",
		"## Per-case costs",
		"## Per-step costs",
		"| Case |",
		"| Step |",
		"| **TOTAL**",
		"ptp",
		"llm",
		"C1",
		"C2",
		"Triage (F1)",
		"105.0K",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("markdown missing: %q", check)
		}
	}
}
