package rca

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	framework "github.com/dpopsuev/origami"
)

type echoTransformer struct{ name string }

func (e *echoTransformer) Name() string { return e.name }
func (e *echoTransformer) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	return map[string]any{"node": tc.NodeName}, nil
}

func TestRoutingRecorder_DelegatesToInner(t *testing.T) {
	inner := &echoTransformer{name: "inner"}
	rec := NewRoutingRecorder(inner, "blue")

	if rec.Name() != "inner" {
		t.Errorf("Name() = %q, want %q", rec.Name(), "inner")
	}

	ws := framework.NewWalkerState("C1")
	ws.Context[KeyCaseLabel] = "C1"
	tc := &framework.TransformerContext{
		NodeName:    "recall",
		WalkerState: ws,
	}

	result, err := rec.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["node"] != "recall" {
		t.Errorf("expected inner result, got %v", result)
	}
}

func TestRoutingRecorder_LogsEntries(t *testing.T) {
	inner := &echoTransformer{name: "test"}
	rec := NewRoutingRecorder(inner, "red")

	ws := framework.NewWalkerState("C1")
	ws.Context[KeyCaseLabel] = "C1"

	steps := []string{"recall", "triage", "review"}
	for _, step := range steps {
		tc := &framework.TransformerContext{NodeName: step, WalkerState: ws}
		if _, err := rec.Transform(context.Background(), tc); err != nil {
			t.Fatalf("Transform(%s): %v", step, err)
		}
	}

	log := rec.Log()
	if log.Len() != 3 {
		t.Fatalf("log.Len() = %d, want 3", log.Len())
	}
	for _, e := range log {
		if e.CaseID != "C1" {
			t.Errorf("CaseID = %q, want C1", e.CaseID)
		}
		if e.Color != "red" {
			t.Errorf("Color = %q, want red", e.Color)
		}
	}

	byCase := log.ForCase("C1")
	if byCase.Len() != 3 {
		t.Errorf("ForCase(C1).Len() = %d, want 3", byCase.Len())
	}
	byStep := log.ForStep("recall")
	if byStep.Len() != 1 {
		t.Errorf("ForStep(F0_RECALL).Len() = %d, want 1", byStep.Len())
	}
}

func TestRoutingRecorder_ThreadSafe(t *testing.T) {
	inner := &echoTransformer{name: "test"}
	rec := NewRoutingRecorder(inner, "green")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ws := framework.NewWalkerState("C1")
			ws.Context[KeyCaseLabel] = "C1"
			tc := &framework.TransformerContext{NodeName: "recall", WalkerState: ws}
			rec.Transform(context.Background(), tc)
		}(i)
	}
	wg.Wait()

	if rec.Log().Len() != 10 {
		t.Errorf("expected 10 entries, got %d", rec.Log().Len())
	}
}

type idMappableTransformer struct {
	echoTransformer
	rcaIDs     map[string]int64
	symptomIDs map[string]int64
}

func (t *idMappableTransformer) SetRCAID(gtID string, storeID int64)     { t.rcaIDs[gtID] = storeID }
func (t *idMappableTransformer) SetSymptomID(gtID string, storeID int64) { t.symptomIDs[gtID] = storeID }

func TestRoutingRecorder_IDMappableDelegation(t *testing.T) {
	inner := &idMappableTransformer{
		echoTransformer: echoTransformer{name: "stub"},
		rcaIDs:          make(map[string]int64),
		symptomIDs:      make(map[string]int64),
	}
	rec := NewRoutingRecorder(inner, "gold")

	rec.SetRCAID("C1", 42)
	rec.SetSymptomID("C2", 99)

	if inner.rcaIDs["C1"] != 42 {
		t.Errorf("SetRCAID not delegated: got %d", inner.rcaIDs["C1"])
	}
	if inner.symptomIDs["C2"] != 99 {
		t.Errorf("SetSymptomID not delegated: got %d", inner.symptomIDs["C2"])
	}
}

func TestRoutingRecorder_IDMappable_NoopWithoutInterface(t *testing.T) {
	inner := &echoTransformer{name: "plain"}
	rec := NewRoutingRecorder(inner, "gray")

	rec.SetRCAID("C1", 1)
	rec.SetSymptomID("C2", 2)
}

func TestSaveLoadRoutingLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")

	original := RoutingLog{
		{CaseID: "C1", Step: "F0_RECALL", Color: "blue", DispatchID: 1},
		{CaseID: "C1", Step: "F5_REVIEW", Color: "blue", DispatchID: 2},
	}

	if err := SaveRoutingLog(path, original); err != nil {
		t.Fatalf("SaveRoutingLog: %v", err)
	}

	loaded, err := LoadRoutingLog(path)
	if err != nil {
		t.Fatalf("LoadRoutingLog: %v", err)
	}

	if len(loaded) != len(original) {
		t.Fatalf("loaded %d entries, want %d", len(loaded), len(original))
	}
	for i, e := range loaded {
		if e.CaseID != original[i].CaseID || e.Step != original[i].Step {
			t.Errorf("entry %d mismatch: got %+v, want %+v", i, e, original[i])
		}
	}
}

func TestLoadRoutingLog_Missing(t *testing.T) {
	_, err := LoadRoutingLog("/tmp/nonexistent-routing-log.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSaveRoutingLog_BadDir(t *testing.T) {
	err := SaveRoutingLog("/nonexistent/dir/routing.json", RoutingLog{})
	if err == nil {
		t.Fatal("expected error for bad directory")
	}
}

func TestCompareRoutingLogs(t *testing.T) {
	expected := RoutingLog{
		{CaseID: "C1", Step: "F0_RECALL", Color: "blue"},
		{CaseID: "C1", Step: "F5_REVIEW", Color: "blue"},
	}
	actual := RoutingLog{
		{CaseID: "C1", Step: "F0_RECALL", Color: "red"},
		{CaseID: "C2", Step: "F0_RECALL", Color: "red"},
	}

	diffs := CompareRoutingLogs(expected, actual)

	if len(diffs) != 3 {
		t.Fatalf("expected 3 diffs, got %d: %+v", len(diffs), diffs)
	}

	has := func(caseID, step, exp, act string) bool {
		for _, d := range diffs {
			if d.CaseID == caseID && d.Step == step && d.Expected == exp && d.Actual == act {
				return true
			}
		}
		return false
	}
	if !has("C1", "F0_RECALL", "blue", "red") {
		t.Error("missing color mismatch diff for C1/F0_RECALL")
	}
	if !has("C1", "F5_REVIEW", "blue", "<missing>") {
		t.Error("missing diff for C1/F5_REVIEW")
	}
	if !has("C2", "F0_RECALL", "<missing>", "red") {
		t.Error("missing extra diff for C2/F0_RECALL")
	}
}

func TestCompareRoutingLogs_Identical(t *testing.T) {
	log := RoutingLog{
		{CaseID: "C1", Step: "F0_RECALL", Color: "blue"},
	}
	diffs := CompareRoutingLogs(log, log)
	if len(diffs) != 0 {
		t.Errorf("identical logs should have 0 diffs, got %d", len(diffs))
	}
}

func TestSaveRoutingLog_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.json")

	log := RoutingLog{{CaseID: "X", Step: "F1_TRIAGE", Color: "purple", DispatchID: 7}}
	if err := SaveRoutingLog(path, log); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("file is empty after save")
	}

	loaded, err := LoadRoutingLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].DispatchID != 7 {
		t.Errorf("DispatchID = %d, want 7", loaded[0].DispatchID)
	}
}
