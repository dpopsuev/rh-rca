package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/origami-rca/rcatype"
)

func TestSqlStore_Integration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Envelope (stored on launch via v1 envelope API)
	env := &rcatype.Envelope{
		RunID: "33195",
		Name:  "test-launch",
		FailureList: []rcatype.FailureItem{
			{ID: "1", Name: "fail1", Status: "FAILED"},
			{ID: "2", Name: "fail2", Status: "FAILED"},
		},
	}
	if err := s.SaveEnvelope("33195", env); err != nil {
		t.Fatalf("SaveEnvelope: %v", err)
	}
	got, err := s.GetEnvelope("33195")
	if err != nil {
		t.Fatalf("GetEnvelope: %v", err)
	}
	if got == nil || got.RunID != "33195" || len(got.FailureList) != 2 {
		t.Errorf("GetEnvelope: got %+v", got)
	}

	// Set up v2 hierarchy: Suite -> Version -> Circuit -> Launch -> Job -> Case
	suiteID, err := s.CreateSuite(&InvestigationSuite{Name: "test-suite", Status: "active"})
	if err != nil {
		t.Fatalf("CreateSuite: %v", err)
	}
	versionID, err := s.CreateVersion(&Version{Label: "v1.0"})
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	circuitID, err := s.CreateCircuit(&Circuit{SuiteID: suiteID, VersionID: versionID, Name: "test-circuit"})
	if err != nil {
		t.Fatalf("CreateCircuit: %v", err)
	}
	launchID, err := s.CreateLaunch(&Launch{CircuitID: circuitID, SourceRunID: "33195", Name: "test-launch"})
	if err != nil {
		t.Fatalf("CreateLaunch: %v", err)
	}
	jobID, err := s.CreateJob(&Job{LaunchID: launchID, SourceItemID: "0", Name: "default-job"})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	caseID1, err := s.CreateCase(&Case{JobID: jobID, LaunchID: launchID, SourceItemID: "1", Name: "case-1", Status: "open"})
	if err != nil {
		t.Fatalf("CreateCase 1: %v", err)
	}
	_, err = s.CreateCase(&Case{JobID: jobID, LaunchID: launchID, SourceItemID: "2", Name: "case-2", Status: "open"})
	if err != nil {
		t.Fatalf("CreateCase 2: %v", err)
	}

	cases, err := s.ListCasesByJob(jobID)
	if err != nil {
		t.Fatalf("ListCasesByJob: %v", err)
	}
	if len(cases) != 2 {
		t.Errorf("ListCasesByJob: want 2, got %d", len(cases))
	}

	c, err := s.GetCase(caseID1)
	if err != nil || c == nil || c.SourceItemID != "1" {
		t.Errorf("GetCase: got %+v err %v", c, err)
	}
	if c.Status != "open" {
		t.Errorf("GetCase status: got %q want %q", c.Status, "open")
	}

	// RCA via v2, then link and list
	rcaID, err := s.SaveRCA(&RCA{Title: "R1", Description: "desc", DefectType: "ti001", Status: "open"})
	if err != nil {
		t.Fatalf("SaveRCA: %v", err)
	}
	if err := s.LinkCaseToRCA(caseID1, rcaID); err != nil {
		t.Fatalf("LinkCaseToRCA: %v", err)
	}
	r, err := s.GetRCA(rcaID)
	if err != nil || r == nil || r.Title != "R1" {
		t.Errorf("GetRCA: got %+v err %v", r, err)
	}
	rcas, err := s.ListRCAs()
	if err != nil || len(rcas) != 1 {
		t.Errorf("ListRCAs: got %d err %v", len(rcas), err)
	}
}

func TestSqlStore_OpenCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "asterisk.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("parent dir not created: %v", err)
	}
}
