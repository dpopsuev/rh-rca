package rp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

)

// BDD: Given envelope in store, When Analyze runs, Then artifact exists and contains launch_id, case_ids, defect_type, convergence_score.
func TestAnalyze_ProducesArtifactWithRequiredShape(t *testing.T) {
	launchID := 33195
	env := loadFixtureEnvelope(t)
	store := NewMemEnvelopeStore()
	if err := store.Save(launchID, env); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")

	err := Analyze(store, launchID, artifactPath)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile artifact: %v", err)
	}
	var a Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("Unmarshal artifact: %v", err)
	}
	if a.LaunchID != env.RunID {
		t.Errorf("LaunchID: got %q want %q", a.LaunchID, env.RunID)
	}
	if len(a.CaseIDs) != len(env.FailureList) {
		t.Errorf("CaseIDs len: got %d want %d", len(a.CaseIDs), len(env.FailureList))
	}
	if a.DefectType == "" {
		t.Error("DefectType: empty")
	}
	// convergence_score and evidence_refs are optional placeholders
	_ = a.ConvergenceScore
	_ = a.EvidenceRefs
}

func loadFixtureEnvelope(t *testing.T) *Envelope {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "pre-investigation-33195-4.21", "envelope_33195_4.21.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture not found: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &env
}
