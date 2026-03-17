package rp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

)

// BDD: Given artifact file, When Push runs, Then mock store records defect type (and optional Jira).
func TestPush_RecordsDefectTypeAndJiraInStore(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	artifact := pushArtifact{
		RunID:            "33195",
		CaseIDs:          []string{"1697136", "1697139"},
		DefectType:        "ti001",
		ConvergenceScore:  0.85,
		EvidenceRefs:      nil,
	}
	data, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(artifactPath, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store := NewMemPushStore()
	err := Push(artifactPath, store, "PROJ-123", "https://jira.example.com/PROJ-123")
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	got := store.LastPushed()
	if got == nil {
		t.Fatal("LastPushed: nil")
	}
	if got.DefectType != artifact.DefectType {
		t.Errorf("DefectType: got %q want %q", got.DefectType, artifact.DefectType)
	}
	if got.RunID != artifact.RunID {
		t.Errorf("RunID: got %q want %q", got.RunID, artifact.RunID)
	}
	if got.JiraTicketID != "PROJ-123" {
		t.Errorf("JiraTicketID: got %q want PROJ-123", got.JiraTicketID)
	}
}
