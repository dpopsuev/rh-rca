package rca_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	cal "github.com/dpopsuev/origami/calibrate"
	rca "github.com/dpopsuev/origami-rca"
)

// testDomainFS returns the testdata_mcp directory as an fs.FS.
func testDomainFS(t *testing.T) fs.FS {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	return os.DirFS(filepath.Join(filepath.Dir(f), "testdata_mcp"))
}

// mustLoadScenario loads a scenario from the test domain FS.
func mustLoadScenario(t *testing.T, name string) *rca.Scenario {
	t.Helper()
	domainFS := testDomainFS(t)
	scenarioFS, err := fs.Sub(domainFS, "scenarios")
	if err != nil {
		t.Fatalf("sub scenarios: %v", err)
	}
	s, err := rca.LoadScenario(scenarioFS, name)
	if err != nil {
		t.Fatalf("load scenario %s: %v", name, err)
	}
	return s
}

// loadTestScoreCard loads the RCA scorecard from the test domain FS.
func loadTestScoreCard(t *testing.T) *cal.ScoreCard {
	t.Helper()
	data, err := fs.ReadFile(testDomainFS(t), "scorecards/rca.yaml")
	if err != nil {
		t.Fatalf("read scorecard: %v", err)
	}
	sc, err := cal.ParseScoreCard(data)
	if err != nil {
		t.Fatalf("parse scorecard: %v", err)
	}
	return sc
}

// testCircuitData reads the RCA circuit YAML from the test domain FS.
func testCircuitData(t *testing.T) []byte {
	t.Helper()
	data, err := fs.ReadFile(testDomainFS(t), "circuits/rca.yaml")
	if err != nil {
		t.Fatalf("read circuit: %v", err)
	}
	return data
}
