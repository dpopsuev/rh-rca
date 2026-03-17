package rca

import (
	"context"

	framework "github.com/dpopsuev/origami"
)

type reviewHeuristic struct{}

func (t *reviewHeuristic) Name() string        { return "review-heuristic" }
func (t *reviewHeuristic) Deterministic() bool { return true }

func (t *reviewHeuristic) Transform(_ context.Context, _ *framework.TransformerContext) (any, error) {
	return map[string]any{"decision": "approve"}, nil
}
