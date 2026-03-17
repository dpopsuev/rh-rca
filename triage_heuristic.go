package rca

import (
	"context"

	framework "github.com/dpopsuev/origami"
)

type triageHeuristic struct {
	ht *heuristicTransformer
}

func (t *triageHeuristic) Name() string        { return "triage-heuristic" }
func (t *triageHeuristic) Deterministic() bool { return true }

func (t *triageHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	text := t.ht.textFromFailure(fp)
	category, hypothesis, skip := t.ht.classifyDefect(text)
	component := t.ht.identifyComponent(text)

	var candidateRepos []any
	if component != "unknown" {
		candidateRepos = []any{component}
	} else {
		for _, r := range t.ht.repos {
			candidateRepos = append(candidateRepos, r)
		}
	}

	cascade := matchCount(text, cascadeKeywords()) > 0

	return map[string]any{
		"symptom_category":       category,
		"severity":               "medium",
		"defect_type_hypothesis": hypothesis,
		"candidate_repos":        candidateRepos,
		"skip_investigation":     skip,
		"cascade_suspected":      cascade,
	}, nil
}
