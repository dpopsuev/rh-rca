package rca

import (
	"testing"

	"github.com/dpopsuev/rh-rca/store"
)

// --- applyStoreEffects tests ---

func TestStoreEffects_F0Recall_NoMatch(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := map[string]any{"match": false, "confidence": 0.1}
	err := applyStoreEffects(st, caseData, "recall", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	// No links should be created
	updated, err := st.GetCase(caseData.ID)
	if err != nil {
		t.Fatalf("GetCase: %v", err)
	}
	if updated.SymptomID != 0 {
		t.Errorf("expected no symptom link, got %d", updated.SymptomID)
	}
	if updated.RCAID != 0 {
		t.Errorf("expected no RCA link, got %d", updated.RCAID)
	}
}

func TestStoreEffects_F0Recall_Match(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Create a symptom and RCA to link to
	sym := &store.Symptom{Name: "test", Fingerprint: "fp123", Status: "active", OccurrenceCount: 1}
	symID, err := st.CreateSymptom(sym)
	if err != nil {
		t.Fatalf("create symptom: %v", err)
	}

	rca := &store.RCA{Title: "test rca", Status: "open"}
	rcaID, err := st.SaveRCA(rca)
	if err != nil {
		t.Fatalf("save rca: %v", err)
	}

	artifact := map[string]any{"match": true, "prior_rca_id": float64(rcaID), "symptom_id": float64(symID), "confidence": 0.85}
	err = applyStoreEffects(st, caseData, "recall", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	if caseData.SymptomID != symID {
		t.Errorf("case SymptomID = %d, want %d", caseData.SymptomID, symID)
	}
	if caseData.RCAID != rcaID {
		t.Errorf("case RCAID = %d, want %d", caseData.RCAID, rcaID)
	}
}

func TestStoreEffects_F1Triage(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := map[string]any{
		"symptom_category":       "product",
		"severity":               "high",
		"defect_type_hypothesis": "pb001",
		"candidate_repos":        []any{"linuxptp-daemon"},
	}
	err := applyStoreEffects(st, caseData, "triage", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	// Case should be triaged
	if caseData.Status != "triaged" {
		t.Errorf("status = %q, want 'triaged'", caseData.Status)
	}

	// Symptom should be created and linked
	if caseData.SymptomID == 0 {
		t.Error("expected symptom to be created and linked")
	}
}

func TestStoreEffects_F3Investigate(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)
	// Pre-set symptom for the symptom->RCA link test
	sym := &store.Symptom{Name: "test", Fingerprint: "fp1", Status: "active", OccurrenceCount: 1}
	symID, _ := st.CreateSymptom(sym)
	caseData.SymptomID = symID

	artifact := map[string]any{
		"rca_message":        "PTP clock offset exceeded threshold",
		"defect_type":        "pb001",
		"component":          "linuxptp-daemon",
		"convergence_score":  0.85,
		"evidence_refs":      []any{"src/ptp.c"},
	}
	err := applyStoreEffects(st, caseData, "investigate", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	// RCA should be created
	if caseData.RCAID == 0 {
		t.Fatal("expected RCA to be created")
	}

	// Case should be investigated
	if caseData.Status != "investigated" {
		t.Errorf("status = %q, want 'investigated'", caseData.Status)
	}

	// RCA fields should be correct
	rca, err := st.GetRCA(caseData.RCAID)
	if err != nil {
		t.Fatalf("get rca: %v", err)
	}
	if rca.DefectType != "pb001" {
		t.Errorf("rca defect = %q, want 'pb001'", rca.DefectType)
	}
	if rca.Component != "linuxptp-daemon" {
		t.Errorf("rca component = %q, want 'linuxptp-daemon'", rca.Component)
	}
	if rca.ConvergenceScore != 0.85 {
		t.Errorf("rca convergence = %f, want 0.85", rca.ConvergenceScore)
	}
}

func TestStoreEffects_F4Correlate_Duplicate(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Create a pre-existing RCA
	rca := &store.RCA{Title: "existing rca", Status: "open"}
	rcaID, _ := st.SaveRCA(rca)

	artifact := map[string]any{
		"is_duplicate":   true,
		"linked_rca_id":  float64(rcaID),
		"confidence":     0.90,
	}
	err := applyStoreEffects(st, caseData, "correlate", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	if caseData.RCAID != rcaID {
		t.Errorf("case RCAID = %d, want %d", caseData.RCAID, rcaID)
	}
}

func TestStoreEffects_F4Correlate_NotDuplicate(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := map[string]any{"is_duplicate": false}
	err := applyStoreEffects(st, caseData, "correlate", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	if caseData.RCAID != 0 {
		t.Errorf("expected no RCA link for non-duplicate, got %d", caseData.RCAID)
	}
}

func TestStoreEffects_F5Review_Approve(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := map[string]any{"decision": "approve"}
	err := applyStoreEffects(st, caseData, "review", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	if caseData.Status != "reviewed" {
		t.Errorf("status = %q, want 'reviewed'", caseData.Status)
	}
}

func TestStoreEffects_F5Review_Overturn(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Create an RCA to overturn
	rca := &store.RCA{Title: "original", Description: "original rca", DefectType: "pb001", Status: "open"}
	rcaID, _ := st.SaveRCA(rca)
	caseData.RCAID = rcaID

	artifact := map[string]any{
		"decision": "overturn",
		"human_override": map[string]any{
			"defect_type": "au001",
			"rca_message": "human corrected: automation issue",
		},
	}
	err := applyStoreEffects(st, caseData, "review", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}

	if caseData.Status != "reviewed" {
		t.Errorf("status = %q, want 'reviewed'", caseData.Status)
	}

	// RCA should be updated with human correction
	updated, err := st.GetRCA(rcaID)
	if err != nil {
		t.Fatalf("get rca: %v", err)
	}
	if updated.DefectType != "au001" {
		t.Errorf("rca defect after overturn = %q, want 'au001'", updated.DefectType)
	}
	if updated.Description != "human corrected: automation issue" {
		t.Errorf("rca description = %q", updated.Description)
	}
}

func TestStoreEffects_F2Resolve_NoOp(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := map[string]any{"selected_repos": []any{map[string]any{"name": "repo"}}}
	err := applyStoreEffects(st, caseData, "resolve", artifact)
	if err != nil {
		t.Fatalf("applyStoreEffects: %v", err)
	}
	// F2 has no store effects, this just verifies no error
}

func TestStoreEffects_NilArtifact(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// nil artifact should be safely handled
	err := applyStoreEffects(st, caseData, "recall", nil)
	if err != nil {
		t.Fatalf("expected nil artifact to be safe, got: %v", err)
	}
}

func TestStoreEffects_WrongType(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Wrong artifact type should be silently ignored
	err := applyStoreEffects(st, caseData, "recall", "not a recall result")
	if err != nil {
		t.Fatalf("expected wrong type to be safe, got: %v", err)
	}
}

// --- ComputeFingerprint ---

func TestComputeFingerprint_Effects(t *testing.T) {
	fp1 := ComputeFingerprint("test1", "error1", "comp1")
	fp2 := ComputeFingerprint("test1", "error1", "comp1")
	fp3 := ComputeFingerprint("test2", "error1", "comp1")

	if fp1 != fp2 {
		t.Error("identical inputs should produce identical fingerprints")
	}
	if fp1 == fp3 {
		t.Error("different inputs should produce different fingerprints")
	}
	if len(fp1) != 16 {
		t.Errorf("expected 16-char hex fingerprint, got %d chars", len(fp1))
	}
}

// --- Helpers ---

func createTestCase(t *testing.T, st store.Store) *store.Case {
	t.Helper()
	suite := &store.InvestigationSuite{Name: "test", Status: "active"}
	suiteID, err := st.CreateSuite(suite)
	if err != nil {
		t.Fatalf("create suite: %v", err)
	}
	v := &store.Version{Label: "1.0"}
	vid, err := st.CreateVersion(v)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	pipe := &store.Circuit{SuiteID: suiteID, VersionID: vid, Name: "CI", Status: "complete"}
	pipeID, err := st.CreateCircuit(pipe)
	if err != nil {
		t.Fatalf("create circuit: %v", err)
	}
	launch := &store.Launch{CircuitID: pipeID, Name: "Launch", Status: "complete"}
	launchID, err := st.CreateLaunch(launch)
	if err != nil {
		t.Fatalf("create launch: %v", err)
	}
	job := &store.Job{LaunchID: launchID, Name: "Job", Status: "complete"}
	jobID, err := st.CreateJob(job)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	c := &store.Case{
		JobID:        jobID,
		LaunchID:     launchID,
		Name:         "test case",
		Status:       "open",
		ErrorMessage: "test error",
	}
	caseID, err := st.CreateCase(c)
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	c.ID = caseID
	return c
}
