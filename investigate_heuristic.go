package rca

import (
	"context"
	"fmt"
	"strings"

	framework "github.com/dpopsuev/origami"
)

type investigateHeuristic struct {
	ht *heuristicTransformer
}

func (t *investigateHeuristic) Name() string        { return "investigate-heuristic" }
func (t *investigateHeuristic) Deterministic() bool { return true }

func (t *investigateHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	text := t.ht.textFromFailure(fp)
	component := t.ht.identifyComponent(text)
	_, defectType, _ := t.ht.classifyDefect(text)
	evidenceRefs := extractEvidenceRefs(fp.errorMessage, component)

	rcaParts := []string{}
	if fp.errorMessage != "" {
		rcaParts = append(rcaParts, fp.errorMessage)
	}
	if fp.name != "" {
		rcaParts = append(rcaParts, fmt.Sprintf("Test: %s", fp.name))
	}
	if component != "unknown" {
		rcaParts = append(rcaParts, fmt.Sprintf("Suspected component: %s", component))
	}
	rcaMessage := strings.Join(rcaParts, " | ")
	if rcaMessage == "" {
		rcaMessage = "investigation pending (no error message available)"
	}

	convergence := t.ht.computeConvergence(text, component)
	gapBrief := t.ht.buildGapBrief(fp, text, component, defectType, convergence)

	m := map[string]any{
		"rca_message":       rcaMessage,
		"defect_type":       defectType,
		"component":         component,
		"convergence_score": convergence,
		"evidence_refs":     evidenceRefs,
	}
	if gapBrief != nil {
		gapItems := make([]any, 0, len(gapBrief.GapItems))
		for _, g := range gapBrief.GapItems {
			gapItems = append(gapItems, map[string]any{
				"category":    g.Category,
				"description": g.Description,
				"would_help":  g.WouldHelp,
				"source":      g.Source,
				"blocked":     g.Blocked,
			})
		}
		m["gap_brief"] = map[string]any{
			"verdict":   gapBrief.Verdict,
			"gap_items": gapItems,
		}
	}
	return m, nil
}
