package rca

// EvidenceGap describes a specific piece of missing evidence that would
// improve the circuit's confidence in its conclusion.
type EvidenceGap struct {
	Category    string `json:"category"`
	Description string `json:"description"`
	WouldHelp   string `json:"would_help"`
	Source      string `json:"source,omitempty"`
	Blocked     string `json:"blocked,omitempty"`
}

// GapBrief is a structured summary of what evidence the circuit lacked
// when producing its conclusion. Attached to the F3 artifact and surfaced
// in calibration reports.
type GapBrief struct {
	Verdict  string        `json:"verdict"`
	GapItems []EvidenceGap `json:"gap_items"`
}

// Verdict values for GapBrief.
const (
	VerdictConfident     = "confident"
	VerdictLowConfidence = "low-confidence"
	VerdictInconclusive  = "inconclusive"
)

// Evidence gap categories — each maps to an artifact taxonomy domain.
const (
	GapLogDepth    = "log_depth"
	GapSourceCode  = "source_code"
	GapCIContext   = "ci_context"
	GapClusterState = "cluster_state"
	GapVersionInfo = "version_info"
	GapHistorical  = "historical"
	GapJiraContext = "jira_context"
	GapHumanInput  = "human_input"
)

// DefaultGapConfidentThreshold is the convergence score above which
// no gap brief is needed.
const DefaultGapConfidentThreshold = 0.80

// DefaultGapInconclusiveThreshold is the convergence score below which
// the verdict is inconclusive (gap brief required).
const DefaultGapInconclusiveThreshold = 0.50

// ClassifyVerdict determines the verdict based on convergence score and
// defect type. An unknown defect type forces inconclusive regardless of score.
func ClassifyVerdict(convergence float64, defectType string, confidentThreshold, inconclusiveThreshold float64) string {
	if defectType == "unknown" || defectType == "" {
		return VerdictInconclusive
	}
	if convergence >= confidentThreshold {
		return VerdictConfident
	}
	if convergence < inconclusiveThreshold {
		return VerdictInconclusive
	}
	return VerdictLowConfidence
}
