package rca

import (
	"context"
	"fmt"
	"strings"

	framework "github.com/dpopsuev/origami"
)

type correlateHeuristic struct {
	ht *heuristicTransformer
}

func (t *correlateHeuristic) Name() string        { return "correlate-heuristic" }
func (t *correlateHeuristic) Deterministic() bool { return true }

func (t *correlateHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	rcas, err := t.ht.st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return map[string]any{"is_duplicate": false, "confidence": 0.0}, nil
	}
	text := strings.ToLower(fp.errorMessage)
	if text == "" {
		return map[string]any{"is_duplicate": false, "confidence": 0.0}, nil
	}
	for _, existing := range rcas {
		if existing.Description == "" {
			continue
		}
		rcaText := strings.ToLower(existing.Description)
		if strings.Contains(rcaText, text) || strings.Contains(text, rcaText) {
			return map[string]any{
				"is_duplicate":  true,
				"linked_rca_id": float64(existing.ID),
				"confidence":    0.75,
				"reasoning":     fmt.Sprintf("matched existing RCA #%d: %s", existing.ID, existing.Title),
			}, nil
		}
	}
	return map[string]any{"is_duplicate": false, "confidence": 0.0}, nil
}
