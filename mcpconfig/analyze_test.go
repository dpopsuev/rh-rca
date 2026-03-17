package mcpconfig_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/rh-rca"
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/rh-rca/store"
)

func loadEnvelope(t *testing.T, domainFS fs.FS) *rcatype.Envelope {
	t.Helper()
	path := os.Getenv("ANALYZE_ENVELOPE")
	var data []byte
	var err error
	if path != "" {
		data, err = os.ReadFile(path)
	} else {
		data, err = fs.ReadFile(domainFS, "internal/testdata/envelope.json")
	}
	if err != nil {
		t.Fatalf("load envelope: %v", err)
	}
	var env rcatype.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	return &env
}

func buildAnalysisScaffolding(t *testing.T, st store.Store, env *rcatype.Envelope) (int64, []*store.Case) {
	t.Helper()
	suiteID, err := st.CreateSuite(&store.InvestigationSuite{
		Name:   fmt.Sprintf("Analysis %s", env.Name),
		Status: "active",
	})
	if err != nil {
		t.Fatalf("create suite: %v", err)
	}

	vID, _ := st.CreateVersion(&store.Version{Label: "unknown"})
	if vID == 0 {
		v, _ := st.GetVersionByLabel("unknown")
		if v != nil {
			vID = v.ID
		}
	}
	pID, _ := st.CreateCircuit(&store.Circuit{SuiteID: suiteID, VersionID: vID, Name: "CI", Status: "active"})
	lID, _ := st.CreateLaunch(&store.Launch{CircuitID: pID, Name: env.Name, Status: "active"})
	jID, _ := st.CreateJob(&store.Job{LaunchID: lID, Name: "analysis", Status: "active"})

	var cases []*store.Case
	for _, f := range env.FailureList {
		c := &store.Case{
			JobID:        jID,
			LaunchID:     lID,
			Name:         f.Name,
			Status:       "open",
			ErrorMessage: f.ErrorMessage,
			LogSnippet:   f.LogSnippet,
		}
		cID, err := st.CreateCase(c)
		if err != nil {
			t.Fatalf("create case %s: %v", f.Name, err)
		}
		c.ID = cID
		cases = append(cases, c)
	}
	return suiteID, cases
}

func TestAnalyze_Heuristic(t *testing.T) {
	domainFS := testDomainFS(t)
	env := loadEnvelope(t, domainFS)
	if len(env.FailureList) == 0 {
		t.Skip("envelope has no failures")
	}

	st := store.NewMemStore()
	suiteID, cases := buildAnalysisScaffolding(t, st, env)

	circuitData, err := fs.ReadFile(domainFS, "circuits/rca.yaml")
	if err != nil {
		t.Fatalf("read circuit def: %v", err)
	}

	heuristicsData, _ := fs.ReadFile(domainFS, "heuristics.yaml")

	cfg := rca.AnalysisConfig{
		Components:  []*framework.Component{rca.HeuristicComponent(st, nil, heuristicsData)},
		Thresholds:  rca.DefaultThresholds(),
		BasePath:    t.TempDir(),
		CircuitData: circuitData,
		Envelope:    env,
	}

	report, err := rca.RunAnalysis(st, cases, suiteID, cfg)
	if err != nil {
		t.Fatalf("analysis failed: %v", err)
	}
	report.SourceName = env.Name

	fmt.Fprint(os.Stdout, rca.FormatAnalysisReport(report))

	rcaTemplate, _ := fs.ReadFile(domainFS, "reports/rca-report.yaml")
	if len(rcaTemplate) > 0 {
		rendered, renderErr := rca.RenderAnalysisReport(report, time.Now(), rcaTemplate)
		if renderErr == nil {
			fmt.Fprintln(os.Stdout, "\n--- Markdown Report ---")
			fmt.Fprint(os.Stdout, rendered)
		}
	}

	if report.TotalCases != len(env.FailureList) {
		t.Errorf("expected %d cases, got %d", len(env.FailureList), report.TotalCases)
	}
}
