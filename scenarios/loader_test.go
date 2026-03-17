package scenarios_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dpopsuev/rh-rca/scenarios"

	"github.com/google/go-cmp/cmp"
)

func scenarioTestFS() fs.FS {
	_, f, _, _ := runtime.Caller(0)
	return os.DirFS(filepath.Join(filepath.Dir(f), "testdata"))
}

func TestLoadScenario_AllValid(t *testing.T) {
	fsys := scenarioTestFS()
	for _, name := range scenarios.ListScenarios(fsys) {
		t.Run(name, func(t *testing.T) {
			s, err := scenarios.LoadScenario(fsys, name)
			if err != nil {
				t.Fatalf("LoadScenario(%q): %v", name, err)
			}
			if s.Name != name {
				t.Errorf("Name = %q, want %q", s.Name, name)
			}
			if len(s.Cases) == 0 {
				t.Error("expected at least one case")
			}
			if len(s.RCAs) == 0 {
				t.Error("expected at least one RCA")
			}
		})
	}
}

func TestListScenarios(t *testing.T) {
	names := scenarios.ListScenarios(scenarioTestFS())
	if len(names) != 3 {
		t.Fatalf("expected 3 scenarios, got %d: %v", len(names), names)
	}
	want := []string{"daemon-mock", "ptp", "ptp-mock"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("ListScenarios mismatch:\n%s", diff)
	}
}

func TestLoadScenario_NotFound(t *testing.T) {
	_, err := scenarios.LoadScenario(scenarioTestFS(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent scenario")
	}
}
