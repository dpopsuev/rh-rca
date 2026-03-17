package rca

import "testing"

func TestDefaultQuickWins(t *testing.T) {
	qws := LoadQuickWins(readInternalTestdata(t, "tuning-quickwins.yaml"))
	if len(qws) != 4 {
		t.Fatalf("expected 4 quick wins, got %d", len(qws))
	}
	ids := []string{"QW-1", "QW-2", "QW-3", "QW-4"}
	for i, qw := range qws {
		if qw.ID != ids[i] {
			t.Errorf("qw[%d].ID = %q, want %q", i, qw.ID, ids[i])
		}
		if qw.Name == "" {
			t.Errorf("qw[%d].Name is empty", i)
		}
		if qw.Description == "" {
			t.Errorf("qw[%d].Description is empty", i)
		}
		if qw.MetricTarget == "" {
			t.Errorf("qw[%d].MetricTarget is empty", i)
		}
	}
}

func TestTuningRunner_SkeletonAllSkipped(t *testing.T) {
	qws := LoadQuickWins(readInternalTestdata(t, "tuning-quickwins.yaml"))
	runner := NewTuningRunner(RunConfig{}, qws)
	report := runner.Run(0.83)

	if report.BaselineVal != 0.83 {
		t.Errorf("BaselineVal = %f, want 0.83", report.BaselineVal)
	}
	if report.FinalVal != 0.83 {
		t.Errorf("FinalVal = %f, want 0.83 (no QWs applied)", report.FinalVal)
	}
	if report.QWsApplied != 0 {
		t.Errorf("QWsApplied = %d, want 0", report.QWsApplied)
	}
	if len(report.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3 (stops after MaxNoImprove=3)", len(report.Results))
	}
	for _, r := range report.Results {
		if r.Error != "not yet implemented" {
			t.Errorf("QW %s error = %q, want 'not yet implemented'", r.QWID, r.Error)
		}
	}
	if report.StopReason != "no improvement for 3 consecutive QWs" {
		t.Errorf("StopReason = %q, want no improvement streak", report.StopReason)
	}
}

func TestTuningRunner_StopsAtTarget(t *testing.T) {
	qws := []QuickWin{
		{ID: "T1", Apply: func() error { return nil }},
		{ID: "T2", Apply: func() error { return nil }},
	}
	runner := &TuningRunner{
		QuickWins:    qws,
		TargetM19:    0.90,
		MaxNoImprove: 3,
	}

	report := runner.Run(0.95)
	if report.StopReason != "target 0.90 reached" {
		t.Errorf("StopReason = %q, want target reached", report.StopReason)
	}
	if len(report.Results) != 0 {
		t.Errorf("should not process any QWs when already above target, got %d results", len(report.Results))
	}
}

func TestTuningRunner_StopsAfterNoImprove(t *testing.T) {
	qws := []QuickWin{
		{ID: "T1"},
		{ID: "T2"},
		{ID: "T3"},
		{ID: "T4"},
	}
	runner := &TuningRunner{
		QuickWins:    qws,
		TargetM19:    0.99,
		MaxNoImprove: 3,
	}

	report := runner.Run(0.50)
	if report.StopReason != "no improvement for 3 consecutive QWs" {
		t.Errorf("StopReason = %q, want no improvement streak", report.StopReason)
	}
	if len(report.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3 (stops after MaxNoImprove)", len(report.Results))
	}
}

func TestTuningReport_CumulativeDelta(t *testing.T) {
	runner := &TuningRunner{
		QuickWins:    nil,
		TargetM19:    0.99,
		MaxNoImprove: 3,
	}
	report := runner.Run(0.83)
	if report.CumulativeDelta != 0 {
		t.Errorf("CumulativeDelta = %f, want 0", report.CumulativeDelta)
	}
}
