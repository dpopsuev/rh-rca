package rca

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 2, 18, 14, 30, 0, 0, time.UTC)

func rcaReportTemplate(t *testing.T) []byte {
	t.Helper()
	return readInternalTestdata(t, "reports/rca-report.yaml")
}

func TestRenderAnalysisReport_EmptyReport(t *testing.T) {
	got, err := RenderAnalysisReport(nil, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "No failures analyzed") {
		t.Errorf("expected empty-report message, got:\n%s", got)
	}

	got, err = RenderAnalysisReport(&AnalysisReport{}, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "No failures analyzed") {
		t.Errorf("expected empty-report message for zero cases, got:\n%s", got)
	}
}

func TestRenderAnalysisReport_SingleCase(t *testing.T) {
	report := &AnalysisReport{
		SourceName: "test-launch-4.20",
		Transformer: "basic",
		TotalCases: 1,
		CaseResults: []AnalysisCaseResult{
			{
				CaseLabel:      "A1",
				TestName:       "should have ptp4l in UP state",
				DefectType:     "pb001",
				Category:       "product",
				RCAMessage:     "Suspected component: linuxptp-daemon",
				Component:      "linuxptp-daemon",
				Path:           []string{"F0", "F1", "F3", "F4", "F5", "F6"},
				EvidenceRefs:   []string{"linuxptp-daemon:relevant_source_file"},
				SelectedRepos:  []string{"linuxptp-daemon"},
				Convergence:    0.80,
				RCAID:          1,
				SourceIssueType:    "ti_abc123",
				SourceAutoAnalyzed: false,
			},
		},
	}

	got, err := RenderAnalysisReport(report, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"RCA Report",
		"2026-02-18 14:30 UTC",
		"basic",
		"Product Bug (pb001)",
		"linuxptp-daemon",
		"80%",
		"[human]",
		"Suspected component: linuxptp-daemon",
		"linuxptp-daemon:relevant_source_file",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in report:\n%s", want, got)
		}
	}
}

func TestRenderAnalysisReport_MultipleComponentsGrouped(t *testing.T) {
	report := &AnalysisReport{
		SourceName: "test-launch",
		Transformer: "basic",
		TotalCases: 3,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "test-1", DefectType: "pb001", Component: "comp-a", Convergence: 0.70, RCAID: 1},
			{CaseLabel: "A2", TestName: "test-2", DefectType: "pb001", Component: "comp-b", Convergence: 0.80, RCAID: 2},
			{CaseLabel: "A3", TestName: "test-3", DefectType: "au001", Component: "comp-a", Convergence: 0.75, RCAID: 3},
		},
	}

	got, err := RenderAnalysisReport(report, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "comp-a (2 failures)") {
		t.Errorf("expected comp-a grouped with 2 failures, got:\n%s", got)
	}
	if !strings.Contains(got, "comp-b (1 failure)") {
		t.Errorf("expected comp-b grouped with 1 failure, got:\n%s", got)
	}

	compAIdx := strings.Index(got, "comp-a (2 failures)")
	compBIdx := strings.Index(got, "comp-b (1 failure)")
	if compAIdx > compBIdx {
		t.Error("expected comp-a before comp-b (alphabetical)")
	}
}

func TestRenderAnalysisReport_RPTags(t *testing.T) {
	report := &AnalysisReport{
		SourceName: "rp-test",
		Transformer: "basic",
		TotalCases: 2,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "comp",
				SourceIssueType: "ti_human", SourceAutoAnalyzed: false, Convergence: 0.80, RCAID: 1},
			{CaseLabel: "A2", TestName: "t2", DefectType: "au001", Component: "comp",
				SourceIssueType: "ti_auto", SourceAutoAnalyzed: true, Convergence: 0.70, RCAID: 2},
		},
	}

	got, err := RenderAnalysisReport(report, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "[human]") {
		t.Errorf("expected [human] tag, got:\n%s", got)
	}
	if !strings.Contains(got, "[auto]") {
		t.Errorf("expected [auto] tag, got:\n%s", got)
	}
}

func TestRenderAnalysisReport_Flags(t *testing.T) {
	report := &AnalysisReport{
		SourceName: "flags-test",
		Transformer: "basic",
		TotalCases: 3,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "c", RecallHit: true, RCAID: 1},
			{CaseLabel: "A2", TestName: "t2", DefectType: "au001", Component: "c", Skip: true},
			{CaseLabel: "A3", TestName: "t3", DefectType: "pb001", Component: "c", Cascade: true, RCAID: 3},
		},
	}

	got, err := RenderAnalysisReport(report, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "recall-hit") {
		t.Errorf("expected recall-hit flag, got:\n%s", got)
	}
	if !strings.Contains(got, "skipped") {
		t.Errorf("expected skipped flag, got:\n%s", got)
	}
	if !strings.Contains(got, "cascade") {
		t.Errorf("expected cascade flag, got:\n%s", got)
	}
}

func TestRenderAnalysisReport_ConvergenceRounding(t *testing.T) {
	report := &AnalysisReport{
		SourceName: "conv-test",
		Transformer: "basic",
		TotalCases: 1,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "c",
				Convergence: 0.7999999999999999, RCAID: 1},
		},
	}

	got, err := RenderAnalysisReport(report, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "80%") {
		t.Errorf("expected convergence rounded to 80%%, got:\n%s", got)
	}
}

func TestRenderAnalysisReport_UnknownComponent(t *testing.T) {
	report := &AnalysisReport{
		SourceName: "unknown-test",
		Transformer: "basic",
		TotalCases: 1,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "en001", Component: "", Skip: true},
		},
	}

	got, err := RenderAnalysisReport(report, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "unknown") {
		t.Errorf("expected 'unknown' for empty component, got:\n%s", got)
	}
}

func TestRenderAnalysisReport_EvidenceDeduplication(t *testing.T) {
	report := &AnalysisReport{
		SourceName: "evidence-test",
		Transformer: "basic",
		TotalCases: 2,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "comp",
				EvidenceRefs: []string{"comp:file_a", "comp:file_b"}, RCAID: 1},
			{CaseLabel: "A2", TestName: "t2", DefectType: "pb001", Component: "comp",
				EvidenceRefs: []string{"comp:file_a", "comp:file_c"}, RCAID: 2},
		},
	}

	got, err := RenderAnalysisReport(report, testTime, rcaReportTemplate(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := strings.Count(got, "comp:file_a")
	if count < 2 {
		t.Errorf("expected comp:file_a to appear in component section and case details, count=%d", count)
	}
}
