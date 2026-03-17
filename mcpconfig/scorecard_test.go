package mcpconfig

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestLoadScorecard_DomainFSFirst(t *testing.T) {
	scorecardYAML := `scorecard: test
version: 1
metrics: []
`
	domainFS := fstest.MapFS{
		"scorecards/rca.yaml": &fstest.MapFile{Data: []byte(scorecardYAML)},
	}

	s := &Server{DomainFS: domainFS}

	sc, err := s.loadScorecard("scorecards/rca.yaml", "/nonexistent")
	if err != nil {
		t.Fatalf("loadScorecard: %v", err)
	}
	if sc == nil {
		t.Fatal("expected non-nil scorecard")
	}
}

func TestLoadScorecard_FallbackToDisk(t *testing.T) {
	scorecardYAML := `scorecard: test
version: 1
metrics: []
`
	tmpDir := t.TempDir()
	scPath := filepath.Join(tmpDir, "scorecards", "rca.yaml")
	os.MkdirAll(filepath.Dir(scPath), 0755)
	os.WriteFile(scPath, []byte(scorecardYAML), 0644)

	s := &Server{DomainFS: nil}

	sc, err := s.loadScorecard("scorecards/rca.yaml", tmpDir)
	if err != nil {
		t.Fatalf("loadScorecard: %v", err)
	}
	if sc == nil {
		t.Fatal("expected non-nil scorecard from disk fallback")
	}
}

func TestLoadScorecard_DomainFSMiss_FallbackToDisk(t *testing.T) {
	scorecardYAML := `scorecard: test
version: 1
metrics: []
`
	domainFS := fstest.MapFS{}

	tmpDir := t.TempDir()
	scPath := filepath.Join(tmpDir, "scorecards", "rca.yaml")
	os.MkdirAll(filepath.Dir(scPath), 0755)
	os.WriteFile(scPath, []byte(scorecardYAML), 0644)

	s := &Server{DomainFS: fs.FS(domainFS)}

	sc, err := s.loadScorecard("scorecards/rca.yaml", tmpDir)
	if err != nil {
		t.Fatalf("loadScorecard: %v", err)
	}
	if sc == nil {
		t.Fatal("expected non-nil scorecard from disk after DomainFS miss")
	}
}

func TestLoadScorecard_BothMissing(t *testing.T) {
	domainFS := fstest.MapFS{}
	s := &Server{DomainFS: fs.FS(domainFS)}

	_, err := s.loadScorecard("scorecards/rca.yaml", "/nonexistent")
	if err == nil {
		t.Fatal("expected error when scorecard missing from both sources")
	}
}
