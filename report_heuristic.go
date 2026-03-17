package rca

import (
	"context"

	framework "github.com/dpopsuev/origami"
)

type reportHeuristic struct{}

func (t *reportHeuristic) Name() string        { return "report-heuristic" }
func (t *reportHeuristic) Deterministic() bool { return true }

func (t *reportHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	caseLabel, _ := tc.WalkerState.Context[KeyCaseLabel].(string)
	return map[string]any{
		"case_id":   caseLabel,
		"test_name": fp.name,
		"summary":   "automated baseline analysis",
	}, nil
}
