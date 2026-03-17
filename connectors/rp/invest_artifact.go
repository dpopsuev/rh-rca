package rp

// Artifact is the output of analyze: written to FS for push.
// Contract: mock-investigation — launch_id, case refs, rca_message, defect_type, convergence_score, evidence refs.
// See docs/artifact-schema.mdc (or data-io.mdc).
type Artifact struct {
	LaunchID          string   `json:"launch_id"`
	CaseIDs           []int    `json:"case_ids"`
	RCAMessage       string   `json:"rca_message"`        // RCA summary / root-cause message
	DefectType       string   `json:"defect_type"`
	ConvergenceScore float64  `json:"convergence_score"`
	EvidenceRefs      []string `json:"evidence_refs"`
}
