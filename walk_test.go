package rca

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami-rca/store"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// fullCircuitTransformer returns deterministic typed results for all steps,
// driving the circuit through recall-hit → review-approve → report-done.
type fullCircuitTransformer struct{}

func (f *fullCircuitTransformer) Name() string { return "test-full" }
func (f *fullCircuitTransformer) Transform(_ context.Context, tc *engine.TransformerContext) (any, error) {
	switch tc.NodeName {
	case "recall":
		return map[string]any{"match": true, "confidence": 0.95, "reasoning": "known failure"}, nil
	case "review":
		return map[string]any{"decision": "approve"}, nil
	case "report":
		return map[string]any{"summary": "done"}, nil
	default:
		return map[string]any{}, nil
	}
}

func TestWalkCase_RecallHitPath(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 1, Name: "test-case"}

	storeComp := &engine.Component{
		Namespace: "store", Name: "test-store",
		Hooks: StoreHooks(ms, c),
	}
	transComp := TransformerComponent(&fullCircuitTransformer{})
	circuitData := readInternalTestdata(t, "circuit_rca.yaml")

	result, err := WalkCase(context.Background(), WalkConfig{
		Store:       ms,
		CaseData:    c,
		CaseLabel:   "T1",
		CircuitData: circuitData,
		Components:  []*engine.Component{transComp, storeComp},
	})
	if err != nil {
		t.Fatalf("WalkCase: %v", err)
	}

	if len(result.Path) == 0 {
		t.Fatal("expected non-empty path")
	}
	if result.Path[0] != "recall" {
		t.Errorf("first step = %q, want recall", result.Path[0])
	}

	expectedPath := []string{"recall", "review", "report"}
	if len(result.Path) != len(expectedPath) {
		t.Errorf("path = %v, want %v", result.Path, expectedPath)
	} else {
		for i, step := range expectedPath {
			if result.Path[i] != step {
				t.Errorf("path[%d] = %q, want %q", i, result.Path[i], step)
			}
		}
	}

	if result.State == nil {
		t.Fatal("expected non-nil State in WalkResult")
	}
}

// triageInvestigateTransformer drives: recall-miss → triage → investigate → correlate → review → report
type triageInvestigateTransformer struct{}

func (f *triageInvestigateTransformer) Name() string { return "test-triage" }
func (f *triageInvestigateTransformer) Transform(_ context.Context, tc *engine.TransformerContext) (any, error) {
	switch tc.NodeName {
	case "recall":
		return map[string]any{"match": false, "confidence": 0.1}, nil
	case "triage":
		return map[string]any{"symptom_category": "product_bug", "candidate_repos": []any{"repo-a"}}, nil
	case "investigate":
		return map[string]any{"convergence_score": 0.8, "evidence_refs": []any{"commit-abc"}, "defect_type": "product_bug"}, nil
	case "correlate":
		return map[string]any{"is_duplicate": false, "confidence": 0.3}, nil
	case "review":
		return map[string]any{"decision": "approve"}, nil
	case "report":
		return map[string]any{"summary": "done"}, nil
	default:
		return map[string]any{}, nil
	}
}

func TestWalkCase_TriageInvestigatePath(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 2, Name: "test-deep"}

	storeComp := &engine.Component{
		Namespace: "store", Name: "test-store",
		Hooks: StoreHooks(ms, c),
	}
	transComp := TransformerComponent(&triageInvestigateTransformer{})
	circuitData := readInternalTestdata(t, "circuit_rca.yaml")

	result, err := WalkCase(context.Background(), WalkConfig{
		Store:       ms,
		CaseData:    c,
		CaseLabel:   "T2",
		CircuitData: circuitData,
		Components:  []*engine.Component{transComp, storeComp},
	})
	if err != nil {
		t.Fatalf("WalkCase: %v", err)
	}

	if len(result.Path) < 4 {
		t.Errorf("expected at least 4 steps, got %d: %v", len(result.Path), result.Path)
	}
	if result.Path[0] != "recall" {
		t.Errorf("first step = %q, want recall", result.Path[0])
	}
	if result.Path[1] != "triage" {
		t.Errorf("second step = %q, want triage", result.Path[1])
	}
}

func TestWalkCase_HITL_Fallback(t *testing.T) {
	hitlComp := HITLComponent()
	th := DefaultThresholds()
	circuitData := readInternalTestdata(t, "circuit_rca.yaml")
	runner, err := BuildRunner(circuitData, th, hitlComp)
	if err != nil {
		t.Fatalf("BuildRunner: %v", err)
	}

	walker := circuit.NewProcessWalker("test")

	def, err := LoadCircuitDef(circuitData, th)
	if err != nil {
		t.Fatalf("LoadCircuitDef: %v", err)
	}

	err = runner.Walk(context.Background(), walker, def.Start)
	if err == nil {
		t.Fatal("expected error for HITL fallback (no prompt dir)")
	}
}
