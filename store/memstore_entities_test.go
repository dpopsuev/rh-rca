package store

import "testing"

func TestMemStoreV2_FullHierarchy(t *testing.T) {
	s := NewMemStore()

	// Suite
	suiteID, err := s.CreateSuite(&InvestigationSuite{Name: "MemTest"})
	if err != nil {
		t.Fatalf("CreateSuite: %v", err)
	}
	suite, _ := s.GetSuite(suiteID)
	if suite == nil || suite.Name != "MemTest" || suite.Status != "open" {
		t.Fatalf("GetSuite: %+v", suite)
	}

	// Version
	verID, err := s.CreateVersion(&Version{Label: "4.21"})
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	ver, _ := s.GetVersionByLabel("4.21")
	if ver == nil || ver.ID != verID {
		t.Fatalf("GetVersionByLabel: %+v", ver)
	}

	// Circuit
	pipID, err := s.CreateCircuit(&Circuit{SuiteID: suiteID, VersionID: verID, Name: "pip1", Status: "FAILED"})
	if err != nil {
		t.Fatalf("CreateCircuit: %v", err)
	}
	pips, _ := s.ListCircuitsBySuite(suiteID)
	if len(pips) != 1 {
		t.Fatalf("ListCircuitsBySuite: %d", len(pips))
	}

	// Launch
	launchID, err := s.CreateLaunch(&Launch{CircuitID: pipID, SourceRunID: "33195", Name: "test"})
	if err != nil {
		t.Fatalf("CreateLaunch: %v", err)
	}
	l, _ := s.GetLaunchBySourceRunID(pipID, "33195")
	if l == nil || l.ID != launchID {
		t.Fatalf("GetLaunchBySourceRunID: %+v", l)
	}

	// Job
	jobID, err := s.CreateJob(&Job{LaunchID: launchID, SourceItemID: "100", Name: "job1"})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	jobs, _ := s.ListJobsByLaunch(launchID)
	if len(jobs) != 1 {
		t.Fatalf("ListJobsByLaunch: %d", len(jobs))
	}

	// Case v2
	caseID, err := s.CreateCase(&Case{
		JobID: jobID, LaunchID: launchID, SourceItemID: "200",
		Name: "test-case", ErrorMessage: "timeout",
	})
	if err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
	c, _ := s.GetCase(caseID)
	if c == nil || c.Name != "test-case" || c.Status != "open" {
		t.Fatalf("GetCase: %+v", c)
	}
	if err := s.UpdateCaseStatus(caseID, "triaged"); err != nil {
		t.Fatalf("UpdateCaseStatus: %v", err)
	}

	// Symptom
	symID, err := s.CreateSymptom(&Symptom{Fingerprint: "fp1", Name: "sym1", Component: "comp1"})
	if err != nil {
		t.Fatalf("CreateSymptom: %v", err)
	}
	sym, _ := s.GetSymptomByFingerprint("fp1")
	if sym == nil || sym.ID != symID || sym.OccurrenceCount != 1 {
		t.Fatalf("GetSymptomByFingerprint: %+v", sym)
	}
	if err := s.UpdateSymptomSeen(symID); err != nil {
		t.Fatalf("UpdateSymptomSeen: %v", err)
	}
	sym, _ = s.GetSymptom(symID)
	if sym.OccurrenceCount != 2 {
		t.Errorf("occurrence after update: %d", sym.OccurrenceCount)
	}

	// Link case to symptom
	if err := s.LinkCaseToSymptom(caseID, symID); err != nil {
		t.Fatalf("LinkCaseToSymptom: %v", err)
	}
	cases, _ := s.ListCasesBySymptom(symID)
	if len(cases) != 1 {
		t.Errorf("ListCasesBySymptom: %d", len(cases))
	}

	// Triage
	_, err = s.CreateTriage(&Triage{CaseID: caseID, SymptomCategory: "timeout"})
	if err != nil {
		t.Fatalf("CreateTriage: %v", err)
	}
	triage, _ := s.GetTriageByCase(caseID)
	if triage == nil || triage.SymptomCategory != "timeout" {
		t.Fatalf("GetTriageByCase: %+v", triage)
	}

	// RCA v2
	rcaID, err := s.SaveRCA(&RCA{
		Title: "test-rca", Description: "desc", DefectType: "pb001",
		ConvergenceScore: 0.9,
	})
	if err != nil {
		t.Fatalf("SaveRCA: %v", err)
	}
	rca, _ := s.GetRCA(rcaID)
	if rca == nil || rca.Status != "open" {
		t.Fatalf("GetRCA: %+v", rca)
	}
	if err := s.UpdateRCAStatus(rcaID, "resolved"); err != nil {
		t.Fatalf("UpdateRCAStatus: %v", err)
	}
	rca, _ = s.GetRCA(rcaID)
	if rca.Status != "resolved" {
		t.Errorf("rca status: %q", rca.Status)
	}

	// SymptomRCA
	_, err = s.LinkSymptomToRCA(&SymptomRCA{SymptomID: symID, RCAID: rcaID, Confidence: 0.95})
	if err != nil {
		t.Fatalf("LinkSymptomToRCA: %v", err)
	}
	links, _ := s.GetRCAsForSymptom(symID)
	if len(links) != 1 {
		t.Errorf("GetRCAsForSymptom: %d", len(links))
	}
	rlinks, _ := s.GetSymptomsForRCA(rcaID)
	if len(rlinks) != 1 {
		t.Errorf("GetSymptomsForRCA: %d", len(rlinks))
	}

	// Close suite
	if err := s.CloseSuite(suiteID); err != nil {
		t.Fatalf("CloseSuite: %v", err)
	}
	suite, _ = s.GetSuite(suiteID)
	if suite.Status != "closed" {
		t.Errorf("suite status: %q", suite.Status)
	}
}
