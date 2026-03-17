// Package orchestrate implements the F0–F6 prompt circuit engine.
// It evaluates heuristics, fills templates, persists intermediate artifacts,
// controls loops, and manages per-case state.
package rca

// Thresholds holds configurable threshold values for circuit edge evaluation.
type Thresholds struct {
	RecallHit             float64 // when to short-circuit on prior RCA (default 0.80)
	RecallUncertain       float64 // below this = definite miss (default 0.40)
	ConvergenceSufficient float64 // when to stop investigating (default 0.70)
	MaxInvestigateLoops   int     // cap on F3→F2→F3 iterations (default 2)
	CorrelateDup          float64 // when to auto-link cases to same RCA (default 0.80)
}

// DefaultThresholds returns conservative default thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		RecallHit:             0.80,
		RecallUncertain:       0.40,
		ConvergenceSufficient: 0.50,
		MaxInvestigateLoops:   1,
		CorrelateDup:          0.80,
	}
}

// CaseState tracks per-case progress through the circuit.
// Persisted to disk (JSON) so the orchestrator can resume across CLI invocations.
type CaseState struct {
	CaseID      int64            `json:"case_id"`
	SuiteID     int64            `json:"suite_id"`
	CurrentStep string           `json:"current_step"`
	LoopCounts  map[string]int   `json:"loop_counts"`  // e.g. "investigate": 2
	Status      string           `json:"status"`        // running, paused, done, error
	History     []StepRecord     `json:"history"`       // log of completed steps
}

// StepRecord logs a completed step with its outcome.
type StepRecord struct {
	Step        string `json:"step"`
	Outcome     string `json:"outcome"`      // e.g. "recall-hit", "triage-investigate"
	HeuristicID string `json:"heuristic_id"` // which heuristic rule matched
	Timestamp   string `json:"timestamp"`    // ISO 8601
}

// Typed intermediate artifacts have been removed. All circuit steps now
// produce and consume map[string]any. See map_access.go for safe accessors.
