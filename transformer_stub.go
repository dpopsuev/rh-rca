package rca

import (
	"context"
	"fmt"
	"sync"

	framework "github.com/dpopsuev/origami"
)

type stubTransformer struct {
	scenario     *Scenario
	mu           sync.RWMutex
	rcaIDMap     map[string]int64
	symptomIDMap map[string]int64
}

func NewStubTransformer(scenario *Scenario) *stubTransformer {
	return &stubTransformer{
		scenario:     scenario,
		rcaIDMap:     make(map[string]int64),
		symptomIDMap: make(map[string]int64),
	}
}

func (t *stubTransformer) Name() string        { return "stub" }
func (t *stubTransformer) Deterministic() bool { return true }

func (t *stubTransformer) SetRCAID(gtID string, storeID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rcaIDMap[gtID] = storeID
}

func (t *stubTransformer) SetSymptomID(gtID string, storeID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.symptomIDMap[gtID] = storeID
}

func (t *stubTransformer) getRCAID(gtID string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.rcaIDMap[gtID]
}

func (t *stubTransformer) getSymptomID(gtID string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.symptomIDMap[gtID]
}

func (t *stubTransformer) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	nodeName := tc.NodeName
	caseID := tc.WalkerState.ID
	gtCase := t.findCase(caseID)
	if gtCase == nil {
		return nil, fmt.Errorf("stub transformer: unknown case %q", caseID)
	}

	switch nodeName {
	case "recall":
		return t.buildRecall(gtCase), nil
	case "triage":
		return t.buildTriage(gtCase), nil
	case "resolve":
		return t.buildResolve(gtCase), nil
	case "investigate":
		return t.buildInvestigate(gtCase), nil
	case "correlate":
		return t.buildCorrelate(gtCase), nil
	case "review":
		return t.buildReview(gtCase), nil
	case "report":
		return t.buildReport(gtCase), nil
	default:
		return nil, fmt.Errorf("stub transformer: no response for node %s", nodeName)
	}
}

func (t *stubTransformer) findCase(id string) *GroundTruthCase {
	for i := range t.scenario.Cases {
		if t.scenario.Cases[i].ID == id {
			return &t.scenario.Cases[i]
		}
	}
	return nil
}

func (t *stubTransformer) findRCA(id string) *GroundTruthRCA {
	for i := range t.scenario.RCAs {
		if t.scenario.RCAs[i].ID == id {
			return &t.scenario.RCAs[i]
		}
	}
	return nil
}

func (t *stubTransformer) buildRecall(c *GroundTruthCase) map[string]any {
	if c.ExpectedRecall != nil {
		m := map[string]any{
			"match":      c.ExpectedRecall.Match,
			"confidence": c.ExpectedRecall.Confidence,
		}
		if c.ExpectedRecall.Match {
			m["reasoning"] = fmt.Sprintf("Recalled prior RCA for symptom matching case %s", c.ID)
			if c.RCAID != "" {
				m["prior_rca_id"] = float64(t.getRCAID(c.RCAID))
			}
			if c.SymptomID != "" {
				m["symptom_id"] = float64(t.getSymptomID(c.SymptomID))
			}
		} else {
			m["reasoning"] = "No prior RCA found matching this failure pattern"
		}
		return m
	}
	return map[string]any{"match": false, "confidence": 0.0, "reasoning": "no recall data"}
}

func (t *stubTransformer) buildTriage(c *GroundTruthCase) map[string]any {
	if c.ExpectedTriage != nil {
		return map[string]any{
			"symptom_category":       c.ExpectedTriage.SymptomCategory,
			"severity":               c.ExpectedTriage.Severity,
			"defect_type_hypothesis": c.ExpectedTriage.DefectTypeHypothesis,
			"candidate_repos":        c.ExpectedTriage.CandidateRepos,
			"skip_investigation":     c.ExpectedTriage.SkipInvestigation,
			"cascade_suspected":      c.ExpectedTriage.CascadeSuspected,
		}
	}
	return map[string]any{"symptom_category": "unknown"}
}

func (t *stubTransformer) buildResolve(c *GroundTruthCase) map[string]any {
	if c.ExpectedResolve != nil {
		repos := make([]any, 0, len(c.ExpectedResolve.SelectedRepos))
		for _, r := range c.ExpectedResolve.SelectedRepos {
			repos = append(repos, map[string]any{"name": r.Name, "reason": r.Reason})
		}
		return map[string]any{"selected_repos": repos}
	}
	return map[string]any{"selected_repos": []any{}}
}

func (t *stubTransformer) buildInvestigate(c *GroundTruthCase) map[string]any {
	if c.ExpectedInvest != nil {
		return map[string]any{
			"rca_message":       c.ExpectedInvest.RCAMessage,
			"defect_type":       c.ExpectedInvest.DefectType,
			"component":         c.ExpectedInvest.Component,
			"convergence_score": c.ExpectedInvest.ConvergenceScore,
			"evidence_refs":     c.ExpectedInvest.EvidenceRefs,
		}
	}
	return map[string]any{"convergence_score": 0.5}
}

func (t *stubTransformer) buildCorrelate(c *GroundTruthCase) map[string]any {
	if c.ExpectedCorrelate != nil {
		m := map[string]any{
			"is_duplicate":        c.ExpectedCorrelate.IsDuplicate,
			"confidence":          c.ExpectedCorrelate.Confidence,
			"cross_version_match": c.ExpectedCorrelate.CrossVersionMatch,
		}
		if c.ExpectedCorrelate.IsDuplicate && c.RCAID != "" {
			m["linked_rca_id"] = float64(t.getRCAID(c.RCAID))
		}
		return m
	}
	return map[string]any{"is_duplicate": false}
}

func (t *stubTransformer) buildReview(c *GroundTruthCase) map[string]any {
	if c.ExpectedReview != nil {
		return map[string]any{"decision": c.ExpectedReview.Decision}
	}
	return map[string]any{"decision": "approve"}
}

func (t *stubTransformer) buildReport(c *GroundTruthCase) map[string]any {
	rcaDef := t.findRCA(c.RCAID)
	report := map[string]any{"case_id": c.ID, "test_name": c.TestName, "defect_type": "nd001"}
	if rcaDef != nil {
		report["defect_type"] = rcaDef.DefectType
		report["jira_id"] = rcaDef.JiraID
		report["component"] = rcaDef.Component
		report["summary"] = rcaDef.Title
	}
	return report
}
