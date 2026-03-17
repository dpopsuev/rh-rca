package rp

import (
	"encoding/json"
	"os"
)

// pushArtifact is the JSON shape written by the investigation step.
// Local copy to avoid importing internal/investigate (cycle-breaking).
type pushArtifact struct {
	RunID            string   `json:"run_id"`
	CaseIDs          []string `json:"case_ids"`
	RCAMessage       string   `json:"rca_message"`
	DefectType       string   `json:"defect_type"`
	ConvergenceScore float64  `json:"convergence_score"`
	EvidenceRefs     []string `json:"evidence_refs"`
}

// DefectPusher pushes artifact content to a store (mock or real RP).
type DefectPusher interface {
	Push(artifactPath string, store PushStore, jiraTicketID, jiraLink string) error
}

// DefaultDefectPusher is the mock pusher used in tests and wiring.
type DefaultDefectPusher struct{}

// Push implements DefectPusher.
func (DefaultDefectPusher) Push(artifactPath string, store PushStore, jiraTicketID, jiraLink string) error {
	return Push(artifactPath, store, jiraTicketID, jiraLink)
}

// Push reads the artifact at path and records defect type (and optional Jira fields) to the store.
// Artifact format: same as investigate.Artifact (JSON from mock investigation).
// Contract: .cursor/contracts/mock-post-investigation.md
func Push(artifactPath string, store PushStore, jiraTicketID, jiraLink string) error {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	var a pushArtifact
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	return store.RecordPushed(PushedRecord{
		RunID:        a.RunID,
		CaseIDs:      a.CaseIDs,
		DefectType:   a.DefectType,
		JiraTicketID: jiraTicketID,
		JiraLink:     jiraLink,
	})
}
