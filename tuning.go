package rca

import "github.com/dpopsuev/origami/toolkit"

// QuickWin is an alias to the generic toolkit type.
type QuickWin = toolkit.QuickWin

// TuningResult is an alias for toolkit.TuningResult.
type TuningResult = toolkit.TuningResult

// TuningReport is an alias for toolkit.TuningReport.
type TuningReport = toolkit.TuningReport

// LoadQuickWins delegates to toolkit.
func LoadQuickWins(data []byte) []QuickWin {
	return toolkit.LoadQuickWins(data)
}

// TuningRunner adapts the generic toolkit runner to include RCA's RunConfig.
type TuningRunner struct {
	Config       RunConfig
	QuickWins    []QuickWin
	TargetM19    float64
	MaxNoImprove int
}

// NewTuningRunner creates a runner with default stop conditions.
func NewTuningRunner(cfg RunConfig, qws []QuickWin) *TuningRunner {
	return &TuningRunner{
		Config:       cfg,
		QuickWins:    qws,
		TargetM19:    0.90,
		MaxNoImprove: 3,
	}
}

// Run executes the tuning loop via the toolkit runner.
func (r *TuningRunner) Run(baselineM19 float64) TuningReport {
	runner := toolkit.NewTuningRunner(r.QuickWins, r.TargetM19)
	runner.MaxNoImprove = r.MaxNoImprove
	return runner.Run(baselineM19)
}
