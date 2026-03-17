package rca

import (
	"fmt"

	cal "github.com/dpopsuev/origami/calibrate"
)

// computeMetrics calculates all calibration metrics from case results using
// the framework's batch scorer patterns. Domain-specific Go scorers are gone;
// the scorecard YAML declares pattern + params for each metric.
func computeMetrics(scenario *Scenario, results []CaseResult, sc *cal.ScoreCard) MetricSet {
	reg := cal.DefaultScorerRegistry()

	batchItems, batchCtx := PrepareBatchInput(results, scenario)
	values, details, err := sc.ScoreCase(batchItems, batchCtx, reg)
	if err != nil {
		values = make(map[string]float64)
		details = make(map[string]string)
	}

	ms := sc.Evaluate(values, details)

	if sc.Aggregate != nil {
		agg, err := sc.ComputeAggregate(ms)
		if err == nil {
			ms.Metrics = append(ms.Metrics, agg)
		}
	}

	m20def := sc.FindDef("M20")
	if m20def != nil {
		ms.Metrics = append(ms.Metrics, m20def.ToMetric(0, "single run"))
	}

	ApplyDryCaps(&ms, scenario.DryCappedMetrics)
	return ms
}

// ApplyDryCaps marks metrics that are structurally unsolvable in dry calibration.
func ApplyDryCaps(ms *MetricSet, capped []string) {
	if len(capped) == 0 {
		return
	}
	set := make(map[string]bool, len(capped))
	for _, id := range capped {
		set[id] = true
	}
	for i := range ms.Metrics {
		if set[ms.Metrics[i].ID] {
			ms.Metrics[i].DryCapped = true
		}
	}
}

// PrepareBatchInput converts typed Asterisk structs into the generic
// []map[string]any batch format consumed by Origami's batch scorer patterns.
// Each item merges CaseResult fields + GroundTruthCase fields + joined RCA fields.
// The second return value is batch-level context (red herring repos, etc.).
func PrepareBatchInput(results []CaseResult, scenario *Scenario) ([]map[string]any, map[string]any) {
	caseMap := make(map[string]*GroundTruthCase, len(scenario.Cases))
	for i := range scenario.Cases {
		caseMap[scenario.Cases[i].ID] = &scenario.Cases[i]
	}
	rcaMap := make(map[string]*GroundTruthRCA, len(scenario.RCAs))
	for i := range scenario.RCAs {
		rcaMap[scenario.RCAs[i].ID] = &scenario.RCAs[i]
	}
	repoRelevance := buildRepoRelevanceMap(scenario)

	batch := make([]map[string]any, 0, len(results))
	for _, r := range results {
		item := map[string]any{
			"case_id":              r.CaseID,
			"actual_defect_type":   r.ActualDefectType,
			"actual_category":      r.ActualCategory,
			"actual_recall_hit":    r.ActualRecallHit,
			"actual_skip":          r.ActualSkip,
			"actual_cascade":       r.ActualCascade,
			"actual_convergence":   r.ActualConvergence,
			"actual_selected_repos": r.ActualSelectedRepos,
			"actual_evidence_refs": r.ActualEvidenceRefs,
			"actual_rca_message":   r.ActualRCAMessage,
			"actual_component":     r.ActualComponent,
			"actual_path":          r.ActualPath,
			"actual_loops":         r.ActualLoops,
			"actual_rca_id":        r.ActualRCAID,
			"prompt_tokens_total":  r.PromptTokensTotal,
			"actual_path_length":   len(r.ActualPath),
			"defect_type_correct":  r.DefectTypeCorrect,
			"evidence_gap_count":   len(r.EvidenceGaps),
		}

		// Derived: wrong prediction with gap briefs (for M22)
		wrongWithGaps := 0
		if !r.DefectTypeCorrect && r.ActualDefectType != "" && len(r.EvidenceGaps) > 0 {
			wrongWithGaps = 1
		}
		item["wrong_with_gaps"] = wrongWithGaps

		gt := caseMap[r.CaseID]
		if gt != nil {
			item["rca_id"] = gt.RCAID
			item["expect_recall_hit"] = gt.ExpectRecallHit
			item["expect_skip"] = gt.ExpectSkip
			item["expect_cascade"] = gt.ExpectCascade
			item["expected_path"] = gt.ExpectedPath
			item["expected_loops"] = gt.ExpectedLoops
			item["has_expected_resolve"] = gt.ExpectedResolve != nil

			if gt.ExpectedTriage != nil {
				item["expected_symptom_category"] = gt.ExpectedTriage.SymptomCategory
			}
			if gt.ExpectedInvest != nil {
				item["expected_evidence_refs"] = gt.ExpectedInvest.EvidenceRefs
			}

			if rca := rcaMap[gt.RCAID]; rca != nil {
				item["rca_defect_type"] = rca.DefectType
				item["rca_component"] = rca.Component
				item["rca_relevant_repos"] = rca.RelevantRepos
				item["rca_required_keywords"] = rca.RequiredKeywords
				item["rca_keyword_threshold"] = rca.KeywordThreshold
				item["rca_smoking_gun"] = rca.SmokingGun
				item["rca_smoking_gun_words"] = cal.SmokingGunWords(rca.SmokingGun)

				correct := 0.0
				if r.ActualDefectType == rca.DefectType {
					correct = 1.0
				}
				item["defect_type_correct_float"] = correct
			}

			// Repo relevance for M9/M10
			if rel, ok := repoRelevance[gt.RCAID]; ok {
				repos := make([]string, 0, len(rel))
				for repo := range rel {
					repos = append(repos, repo)
				}
				item["rca_relevant_repos"] = repos
			}
		}

		// has_convergence filter for M8
		item["has_convergence"] = r.ActualConvergence > 0

		batch = append(batch, item)
	}

	// Batch-level context
	var redHerringRepos []string
	for _, repo := range scenario.SourcePack.Repos {
		if repo.IsRedHerring {
			redHerringRepos = append(redHerringRepos, repo.Name)
		}
	}
	batchCtx := map[string]any{
		"red_herring_repos": redHerringRepos,
	}

	return batch, batchCtx
}

// AggregateRunMetrics computes the mean and variance across multiple runs.
func AggregateRunMetrics(runs []MetricSet, sc *cal.ScoreCard) MetricSet {
	if len(runs) == 0 {
		return MetricSet{}
	}
	if len(runs) == 1 {
		return runs[0]
	}

	agg := cal.AggregateRunMetrics(runs, func(m Metric) bool {
		if def := sc.FindDef(m.ID); def != nil {
			return def.Evaluate(m.Value)
		}
		return m.Value >= m.Threshold
	})

	var m19vals []float64
	for _, run := range runs {
		for _, m := range run.AllMetrics() {
			if m.ID == "M19" {
				m19vals = append(m19vals, m.Value)
			}
		}
	}
	m19mean := cal.Mean(m19vals)
	variance := cal.Stddev(m19vals)

	m19threshold := 0.70
	if sc.Aggregate != nil {
		m19threshold = sc.Aggregate.Threshold
	}

	m20def := sc.FindDef("M20")
	m20threshold := 0.15
	if m20def != nil {
		m20threshold = m20def.Threshold
	}

	for i := range agg.Metrics {
		switch agg.Metrics[i].ID {
		case "M19":
			agg.Metrics[i] = Metric{
				ID: "M19", Name: "overall_accuracy", Value: m19mean, Threshold: m19threshold,
				Pass: m19mean >= m19threshold, Detail: fmt.Sprintf("mean of %d runs", len(runs)),
				Tier: cal.TierMeta,
			}
		case "M20":
			agg.Metrics[i] = Metric{
				ID: "M20", Name: "run_variance", Value: variance, Threshold: m20threshold,
				Pass: variance <= m20threshold, Detail: fmt.Sprintf("stddev=%.3f over %d runs", variance, len(runs)),
				Tier: cal.TierMeta,
			}
		}
	}

	return agg
}

// buildRepoRelevanceMap creates a map from RCA ID → set of relevant repo names.
func buildRepoRelevanceMap(scenario *Scenario) map[string]map[string]bool {
	m := make(map[string]map[string]bool)
	for _, rca := range scenario.RCAs {
		m[rca.ID] = make(map[string]bool)
		for _, repo := range rca.RelevantRepos {
			m[rca.ID][repo] = true
		}
	}
	return m
}
