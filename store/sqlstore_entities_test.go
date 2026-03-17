package store

import (
	"path/filepath"
	"testing"
)

// TestSqlStoreV2_FullHierarchy tests the complete v2 entity tree:
// Suite → Circuit (+ Version) → Launch → Job → Case → Triage
// Plus global knowledge: Symptom, RCA, SymptomRCA.
func TestSqlStoreV2_FullHierarchy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v2.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// --- Suite ---
	suiteID, err := s.CreateSuite(&InvestigationSuite{Name: "PTP Feb 2026", Description: "Regression analysis"})
	if err != nil {
		t.Fatalf("CreateSuite: %v", err)
	}
	suite, err := s.GetSuite(suiteID)
	if err != nil || suite == nil || suite.Name != "PTP Feb 2026" || suite.Status != "open" {
		t.Fatalf("GetSuite: got %+v err %v", suite, err)
	}
	suites, err := s.ListSuites()
	if err != nil || len(suites) != 1 {
		t.Fatalf("ListSuites: got %d err %v", len(suites), err)
	}

	// --- Version ---
	verID, err := s.CreateVersion(&Version{Label: "4.21", BuildID: "4.21.2"})
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	ver, err := s.GetVersion(verID)
	if err != nil || ver == nil || ver.Label != "4.21" {
		t.Fatalf("GetVersion: got %+v err %v", ver, err)
	}
	verByLabel, err := s.GetVersionByLabel("4.21")
	if err != nil || verByLabel == nil || verByLabel.ID != verID {
		t.Fatalf("GetVersionByLabel: got %+v err %v", verByLabel, err)
	}

	// --- Circuit ---
	pipID, err := s.CreateCircuit(&Circuit{
		SuiteID: suiteID, VersionID: verID,
		Name: "telco-ft-ran-ptp-4.21", SourceRunID: "33195", Status: "FAILED",
	})
	if err != nil {
		t.Fatalf("CreateCircuit: %v", err)
	}
	pip, err := s.GetCircuit(pipID)
	if err != nil || pip == nil || pip.Name != "telco-ft-ran-ptp-4.21" {
		t.Fatalf("GetCircuit: got %+v err %v", pip, err)
	}
	pips, err := s.ListCircuitsBySuite(suiteID)
	if err != nil || len(pips) != 1 {
		t.Fatalf("ListCircuitsBySuite: got %d err %v", len(pips), err)
	}

	// --- Launch ---
	launchID, err := s.CreateLaunch(&Launch{
		CircuitID: pipID, SourceRunID: "33195", Name: "test-launch", Status: "FAILED",
	})
	if err != nil {
		t.Fatalf("CreateLaunch: %v", err)
	}
	launch, err := s.GetLaunch(launchID)
	if err != nil || launch == nil || launch.SourceRunID != "33195" {
		t.Fatalf("GetLaunch: got %+v err %v", launch, err)
	}
	launchByRP, err := s.GetLaunchBySourceRunID(pipID, "33195")
	if err != nil || launchByRP == nil || launchByRP.ID != launchID {
		t.Fatalf("GetLaunchBySourceRunID: got %+v err %v", launchByRP, err)
	}

	// --- Job ---
	jobID, err := s.CreateJob(&Job{
		LaunchID: launchID, SourceItemID: "100", Name: "[T-TSC] RAN PTP tests",
		ClockType: "T-TSC", Status: "FAILED",
		StatsTotal: 20, StatsFailed: 5, StatsPassed: 12, StatsSkipped: 3,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	job, err := s.GetJob(jobID)
	if err != nil || job == nil || job.Name != "[T-TSC] RAN PTP tests" || job.StatsTotal != 20 {
		t.Fatalf("GetJob: got %+v err %v", job, err)
	}
	jobs, err := s.ListJobsByLaunch(launchID)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("ListJobsByLaunch: got %d err %v", len(jobs), err)
	}

	// --- Case v2 ---
	caseID, err := s.CreateCase(&Case{
		JobID: jobID, LaunchID: launchID, SourceItemID: "200",
		Name: "PTP Recovery ptp process restart", Status: "open",
		ErrorMessage: "context deadline exceeded",
	})
	if err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
	c, err := s.GetCase(caseID)
	if err != nil || c == nil || c.Name != "PTP Recovery ptp process restart" {
		t.Fatalf("GetCase: got %+v err %v", c, err)
	}
	if c.Status != "open" || c.ErrorMessage != "context deadline exceeded" {
		t.Fatalf("GetCase fields: status=%q err_msg=%q", c.Status, c.ErrorMessage)
	}

	cases, err := s.ListCasesByJob(jobID)
	if err != nil || len(cases) != 1 {
		t.Fatalf("ListCasesByJob: got %d err %v", len(cases), err)
	}

	// Update case status
	if err := s.UpdateCaseStatus(caseID, "triaged"); err != nil {
		t.Fatalf("UpdateCaseStatus: %v", err)
	}
	c, _ = s.GetCase(caseID)
	if c.Status != "triaged" {
		t.Errorf("case status after update: got %q want %q", c.Status, "triaged")
	}

	// --- Symptom ---
	symID, err := s.CreateSymptom(&Symptom{
		Fingerprint: "fp-ptp-sync-timeout",
		Name:        "PTP sync timeout",
		ErrorPattern: "context deadline exceeded",
		Component:   "ptp-operator",
	})
	if err != nil {
		t.Fatalf("CreateSymptom: %v", err)
	}
	sym, err := s.GetSymptom(symID)
	if err != nil || sym == nil || sym.Name != "PTP sync timeout" || sym.OccurrenceCount != 1 {
		t.Fatalf("GetSymptom: got %+v err %v", sym, err)
	}
	if sym.Status != "active" {
		t.Errorf("symptom status: got %q want %q", sym.Status, "active")
	}

	symByFP, err := s.GetSymptomByFingerprint("fp-ptp-sync-timeout")
	if err != nil || symByFP == nil || symByFP.ID != symID {
		t.Fatalf("GetSymptomByFingerprint: got %+v err %v", symByFP, err)
	}

	// Update symptom seen
	if err := s.UpdateSymptomSeen(symID); err != nil {
		t.Fatalf("UpdateSymptomSeen: %v", err)
	}
	sym, _ = s.GetSymptom(symID)
	if sym.OccurrenceCount != 2 {
		t.Errorf("symptom occurrence after update: got %d want 2", sym.OccurrenceCount)
	}

	// Link case to symptom
	if err := s.LinkCaseToSymptom(caseID, symID); err != nil {
		t.Fatalf("LinkCaseToSymptom: %v", err)
	}
	c, _ = s.GetCase(caseID)
	if c.SymptomID != symID {
		t.Errorf("case symptom_id: got %d want %d", c.SymptomID, symID)
	}

	// ListCasesBySymptom
	casesBySym, err := s.ListCasesBySymptom(symID)
	if err != nil || len(casesBySym) != 1 {
		t.Fatalf("ListCasesBySymptom: got %d err %v", len(casesBySym), err)
	}

	// --- Triage ---
	triageID, err := s.CreateTriage(&Triage{
		CaseID:               caseID,
		SymptomCategory:      "timeout",
		DefectTypeHypothesis: "pb001",
		CandidateRepos:       `["ptp-operator","linuxptp-daemon"]`,
	})
	if err != nil {
		t.Fatalf("CreateTriage: %v", err)
	}
	triage, err := s.GetTriageByCase(caseID)
	if err != nil || triage == nil || triage.ID != triageID || triage.SymptomCategory != "timeout" {
		t.Fatalf("GetTriageByCase: got %+v err %v", triage, err)
	}

	// --- RCA v2 ---
	rcaID, err := s.SaveRCA(&RCA{
		Title:            "PTP holdover timeout reduced",
		Description:      "ptp4l fails to acquire lock because holdover timeout was reduced",
		DefectType:       "pb001",
		Category:         "product",
		Component:        "linuxptp-daemon",
		AffectedVersions: `["4.21"]`,
		ConvergenceScore: 0.85,
	})
	if err != nil {
		t.Fatalf("SaveRCA: %v", err)
	}
	rca, err := s.GetRCA(rcaID)
	if err != nil || rca == nil || rca.Title != "PTP holdover timeout reduced" || rca.Status != "open" {
		t.Fatalf("GetRCA: got %+v err %v", rca, err)
	}
	if rca.ConvergenceScore != 0.85 {
		t.Errorf("rca convergence: got %f want 0.85", rca.ConvergenceScore)
	}

	// RCA status lifecycle
	if err := s.UpdateRCAStatus(rcaID, "resolved"); err != nil {
		t.Fatalf("UpdateRCAStatus resolved: %v", err)
	}
	rca, _ = s.GetRCA(rcaID)
	if rca.Status != "resolved" || rca.ResolvedAt == "" {
		t.Errorf("rca resolved: status=%q resolvedAt=%q", rca.Status, rca.ResolvedAt)
	}

	openRCAs, err := s.ListRCAsByStatus("open")
	if err != nil {
		t.Fatalf("ListRCAsByStatus: %v", err)
	}
	resolvedRCAs, err := s.ListRCAsByStatus("resolved")
	if err != nil || len(resolvedRCAs) != 1 {
		t.Fatalf("ListRCAsByStatus resolved: got %d err %v", len(resolvedRCAs), err)
	}
	_ = openRCAs

	// --- SymptomRCA ---
	linkID, err := s.LinkSymptomToRCA(&SymptomRCA{
		SymptomID: symID, RCAID: rcaID, Confidence: 0.9,
		Notes: "High confidence match",
	})
	if err != nil {
		t.Fatalf("LinkSymptomToRCA: %v", err)
	}
	if linkID == 0 {
		t.Error("LinkSymptomToRCA returned 0 id")
	}

	rcasForSym, err := s.GetRCAsForSymptom(symID)
	if err != nil || len(rcasForSym) != 1 || rcasForSym[0].RCAID != rcaID {
		t.Fatalf("GetRCAsForSymptom: got %+v err %v", rcasForSym, err)
	}
	symsForRCA, err := s.GetSymptomsForRCA(rcaID)
	if err != nil || len(symsForRCA) != 1 || symsForRCA[0].SymptomID != symID {
		t.Fatalf("GetSymptomsForRCA: got %+v err %v", symsForRCA, err)
	}

	// --- Close suite ---
	if err := s.CloseSuite(suiteID); err != nil {
		t.Fatalf("CloseSuite: %v", err)
	}
	suite, _ = s.GetSuite(suiteID)
	if suite.Status != "closed" || suite.ClosedAt == "" {
		t.Errorf("closed suite: status=%q closedAt=%q", suite.Status, suite.ClosedAt)
	}
}

// TestSqlStore_FreshInstall verifies that a fresh DB gets the current schema.
func TestSqlStore_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	_, err = s.CreateSuite(&InvestigationSuite{Name: "test"})
	if err != nil {
		t.Fatalf("CreateSuite on fresh db: %v", err)
	}
	_, err = s.CreateSymptom(&Symptom{Fingerprint: "test-fp", Name: "test"})
	if err != nil {
		t.Fatalf("CreateSymptom on fresh db: %v", err)
	}
}
