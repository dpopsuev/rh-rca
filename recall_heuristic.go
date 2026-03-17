package rca

import (
	"context"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

type recallHeuristic struct {
	ht *heuristicTransformer
}

func (t *recallHeuristic) Name() string        { return "recall-heuristic" }
func (t *recallHeuristic) Deterministic() bool { return true }

func (t *recallHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	fingerprint := ComputeFingerprint(fp.name, fp.errorMessage, "")
	sym, err := t.ht.st.GetSymptomByFingerprint(fingerprint)
	if err != nil || sym == nil {
		return map[string]any{
			"match": false, "confidence": 0.0,
			"reasoning": "no matching symptom in store",
		}, nil
	}
	links, err := t.ht.st.GetRCAsForSymptom(sym.ID)
	if err != nil || len(links) == 0 {
		return map[string]any{
			"match": true, "symptom_id": float64(sym.ID), "confidence": 0.60,
			"reasoning": fmt.Sprintf("matched symptom %q (count=%d) but no linked RCA", sym.Name, sym.OccurrenceCount),
		}, nil
	}
	return map[string]any{
		"match": true, "prior_rca_id": float64(links[0].RCAID), "symptom_id": float64(sym.ID), "confidence": 0.85,
		"reasoning": fmt.Sprintf("recalled symptom %q with RCA #%d", sym.Name, links[0].RCAID),
	}, nil
}
