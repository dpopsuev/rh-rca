package rca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/dpopsuev/origami/engine"
	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/agentport"

	"github.com/dpopsuev/origami-rca/rcatype"
	"github.com/dpopsuev/origami-rca/store"
)

// IDMappable is implemented by transformers that track ground-truth-to-store
// ID mappings (e.g. stubTransformer). Used by calibration to wire recall/correlate
// cross-case references.
type IDMappable interface {
	SetRCAID(gtID string, storeID int64)
	SetSymptomID(gtID string, storeID int64)
}

// RunConfig holds configuration for a calibration run.
type RunConfig struct {
	Scenario     *Scenario
	Components     []*engine.Component // transformer component(s) for the circuit
	TransformerName string             // label for reports
	IDMapper     IDMappable           // optional; stub cross-case references
	Runs         int
	Thresholds   Thresholds
	TokenTracker agentport.Tracker // optional; when set, records per-step token usage
	Parallel     int          // number of parallel workers (default 1 = serial)
	TokenBudget  int          // max concurrent dispatches (token semaphore); 0 = Parallel
	BatchSize    int          // max signals per batch for batch-file dispatch mode; 0 = Parallel
	BasePath     string       // root directory for investigation artifacts; defaults to DefaultBasePath
	SourceFetcher rcatype.EnvelopeFetcher // optional; when set, source-linked cases fetch real failure data
	ScoreCard    *cal.ScoreCard // declarative metric definitions; loaded from YAML at startup

	GapConfidentThreshold    float64 // convergence >= this → confident (no gap brief); 0 uses default 0.80
	GapInconclusiveThreshold float64 // convergence < this → inconclusive (gap brief required); 0 uses default 0.50
	CircuitData              []byte  // circuit definition YAML; required
}

// DefaultRunConfig returns defaults for calibration.
func DefaultRunConfig(scenario *Scenario, comps []*engine.Component, transformerName string) RunConfig {
	return RunConfig{
		Scenario:                 scenario,
		Components:               comps,
		TransformerName:         transformerName,
		Runs:                     1,
		Thresholds:               DefaultThresholds(),
		BasePath:                 DefaultBasePath,
		GapConfidentThreshold:    DefaultGapConfidentThreshold,
		GapInconclusiveThreshold: DefaultGapInconclusiveThreshold,
	}
}

// ResolvedGapConfidentThreshold returns the gap confident threshold,
// falling back to the default if zero.
func (c RunConfig) ResolvedGapConfidentThreshold() float64 {
	if c.GapConfidentThreshold > 0 {
		return c.GapConfidentThreshold
	}
	return DefaultGapConfidentThreshold
}

// ResolvedGapInconclusiveThreshold returns the gap inconclusive threshold,
// falling back to the default if zero.
func (c RunConfig) ResolvedGapInconclusiveThreshold() float64 {
	if c.GapInconclusiveThreshold > 0 {
		return c.GapInconclusiveThreshold
	}
	return DefaultGapInconclusiveThreshold
}

// RunCalibration executes the full calibration loop using the generic
// calibrate.Run() harness with RCA adapters. This is a compatibility
// wrapper — new code should use calibrate.Run() with RCACalibrationAdapter
// directly.
func RunCalibration(ctx context.Context, cfg RunConfig) (*CalibrationReport, error) {
	if cfg.BasePath == "" {
		cfg.BasePath = DefaultBasePath
	}
	if cfg.ScoreCard == nil {
		return nil, fmt.Errorf("RunConfig.ScoreCard is required (set it directly or declare scorecard: in your circuit YAML)")
	}

	adapter := &RCACalibrationAdapter{
		Scenario:     cfg.Scenario,
		Components:   cfg.Components,
		IDMapper:     cfg.IDMapper,
		BasePath:     cfg.BasePath,
		Thresholds:   cfg.Thresholds,
		ScoreCard:    cfg.ScoreCard,
		TokenTracker: cfg.TokenTracker,
	}

	def, err := LoadCircuitDef(cfg.CircuitData, cfg.Thresholds)
	if err != nil {
		return nil, fmt.Errorf("load circuit def: %w", err)
	}

	genReport, err := cal.Run(ctx, cal.HarnessConfig{
		Loader:         adapter,
		Collector:      adapter,
		CircuitDef:     def,
		ScoreCard:      cfg.ScoreCard,
		Contract:       cal.ContractFromDef(def.Calibration),
		Scenario:       cfg.Scenario.Name,
		Transformer:    cfg.TransformerName,
		Runs:           cfg.Runs,
		Parallel:       cfg.Parallel,
		OnCaseComplete: adapter.OnCaseComplete(),
	})
	if err != nil {
		return nil, err
	}

	report := adapter.RCAReport(genReport)

	// Apply domain-specific metric post-processing.
	ApplyDryCaps(&report.Metrics, cfg.Scenario.DryCappedMetrics)

	m20def := cfg.ScoreCard.FindDef("M20")
	if m20def != nil {
		if cfg.Runs == 1 {
			report.Metrics.Metrics = append(report.Metrics.Metrics,
				m20def.ToMetric(0, "single run"))
		} else {
			report.Metrics = AggregateRunMetrics(
				report.RunMetrics, cfg.ScoreCard,
			)
			ApplyDryCaps(&report.Metrics, cfg.Scenario.DryCappedMetrics)
		}
	}

	return report, nil
}

// applyContractFields overlays contract-extracted values onto a CaseResult.
// Fields extracted via the calibration contract take precedence over store-based
// extraction for fields the contract declares. Store-specific fields (RCAID,
// store-persisted state) remain untouched.
func applyContractFields(r *CaseResult, fields map[string]any) {
	if s, ok := fields["actual_defect_type"].(string); ok && s != "" {
		r.ActualDefectType = s
	} else if s, ok := fields["rca_defect_type"].(string); ok && s != "" {
		r.ActualDefectType = s
	}
	if s, ok := fields["actual_category"].(string); ok && s != "" {
		r.ActualCategory = s
	}
	if s, ok := fields["actual_component"].(string); ok && s != "" {
		r.ActualComponent = s
	}
	if s, ok := fields["actual_rca_message"].(string); ok && s != "" {
		r.ActualRCAMessage = s
	}
	if v, ok := fields["actual_convergence"].(float64); ok {
		r.ActualConvergence = v
	}
	if refs, ok := fields["actual_evidence_refs"].([]any); ok {
		strs := make([]string, 0, len(refs))
		for _, ref := range refs {
			if s, ok := ref.(string); ok {
				strs = append(strs, s)
			}
		}
		if len(strs) > 0 {
			r.ActualEvidenceRefs = strs
		}
	}
	if path, ok := fields["_path"].([]string); ok && len(path) > 0 {
		r.ActualPath = path
	}
}

// scoreCaseResult sets the CategoryCorrect, DefectTypeCorrect, PathCorrect,
// and ComponentCorrect flags on a CaseResult by comparing against ground truth.
func scoreCaseResult(r *CaseResult, scenario *Scenario) {
	var gt *GroundTruthCase
	for j := range scenario.Cases {
		if scenario.Cases[j].ID == r.CaseID {
			gt = &scenario.Cases[j]
			break
		}
	}
	if gt == nil {
		return
	}

	// Path accuracy
	r.PathCorrect = cal.PathsEqual(r.ActualPath, gt.ExpectedPath)

	// Category accuracy
	if gt.ExpectedTriage != nil {
		r.CategoryCorrect = (r.ActualCategory == gt.ExpectedTriage.SymptomCategory)
	}

	// Defect type and component — look up ground truth RCA
	if gt.RCAID != "" {
		for _, gtRCA := range scenario.RCAs {
			if gtRCA.ID == gt.RCAID {
				r.DefectTypeCorrect = (r.ActualDefectType == gtRCA.DefectType)
				r.ComponentCorrect = (r.ActualComponent == gtRCA.Component) ||
					(r.ActualRCAMessage != "" && strings.Contains(
						strings.ToLower(r.ActualRCAMessage),
						strings.ToLower(gtRCA.Component)))
				break
			}
		}
	}
}

// collectCaseResult builds a CaseResult from a BatchWalkResult, extracting
// step metrics, writing artifacts, and reading final store state.
func collectCaseResult(
	br engine.BatchWalkResult,
	gtCase GroundTruthCase,
	caseData *store.Case,
	caseDir string,
	suiteID int64,
	st store.Store,
	cfg RunConfig,
) CaseResult {
	result := CaseResult{
		CaseID:         gtCase.ID,
		TestName:       gtCase.TestName,
		Version:        gtCase.Version,
		Job:            gtCase.Job,
		StoreCaseID:    caseData.ID,
		SourceIssueType:    gtCase.SourceIssueType,
		SourceAutoAnalyzed: gtCase.SourceAutoAnalyzed,
	}

	if br.Error != nil {
		result.CircuitError = br.Error.Error()
		return result
	}

	for _, nodeName := range br.Path {
		result.ActualPath = append(result.ActualPath, nodeName)
	}

	for nodeName, art := range br.StepArtifacts {
		extractStepMetrics(&result, nodeName, art.Raw(), gtCase)
		if err := WriteArtifact(caseDir, NodeArtifactFilename(nodeName), art.Raw()); err != nil {
			slog.Warn("write artifact", "component", "calibrate", "node", nodeName, "error", err)
		}
	}

	if br.State != nil {
		ws := br.State
		history := make([]StepRecord, 0, len(ws.History))
		for _, h := range ws.History {
			history = append(history, StepRecord{
				Step:        h.Node,
				Outcome:     h.Outcome,
				HeuristicID: h.EdgeID,
				Timestamp:   h.Timestamp,
			})
		}
		caseState := &CaseState{
			CaseID:      caseData.ID,
			SuiteID:     suiteID,
			CurrentStep: ws.CurrentNode,
			Status:      ws.Status,
			LoopCounts:  ws.LoopCounts,
			History:     history,
		}
		if err := WriteArtifact(caseDir, "state.json", caseState); err != nil {
			slog.Warn("save final state", "component", "calibrate", "error", err)
		}
		result.ActualLoops = ws.LoopCounts["investigate"]
	}

	updatedCase, err := st.GetCase(caseData.ID)
	if err == nil && updatedCase != nil {
		result.ActualRCAID = updatedCase.RCAID
		if updatedCase.RCAID != 0 {
			rcaRec, err := st.GetRCA(updatedCase.RCAID)
			if err == nil && rcaRec != nil {
				result.ActualDefectType = rcaRec.DefectType
				result.ActualRCAMessage = rcaRec.Description
				result.ActualComponent = rcaRec.Component
				result.ActualConvergence = rcaRec.ConvergenceScore
			}
		}
	}

	return result
}


// extractStepMetrics populates CaseResult fields from per-step artifacts.
func extractStepMetrics(result *CaseResult, nodeName string, artifact any, gt GroundTruthCase) {
	m := asMap(artifact)
	if m == nil {
		return
	}
	switch nodeName {
	case "recall":
		result.ActualRecallHit = mapBool(m, "match") && mapFloat(m, "confidence") >= 0.80
	case "triage":
		result.ActualCategory = mapStr(m, "symptom_category")
		cat := mapStr(m, "symptom_category")
		result.ActualSkip = mapBool(m, "skip_investigation") ||
			cat == "infra" || cat == "flake"
		result.ActualCascade = mapBool(m, "cascade_suspected")
		if hyp := mapStr(m, "defect_type_hypothesis"); hyp != "" && result.ActualDefectType == "" {
			result.ActualDefectType = hyp
		}
		candidates := mapStrSlice(m, "candidate_repos")
		if len(candidates) == 1 && !mapBool(m, "skip_investigation") {
			result.ActualSelectedRepos = append(result.ActualSelectedRepos, candidates[0])
		}
	case "resolve":
		result.ActualSelectedRepos = result.ActualSelectedRepos[:0]
		for _, r := range mapSlice(m, "selected_repos") {
			if rm, ok := r.(map[string]any); ok {
				if name := mapStr(rm, "name"); name != "" {
					result.ActualSelectedRepos = append(result.ActualSelectedRepos, name)
				}
			}
		}
	case "investigate":
		result.ActualDefectType = mapStr(m, "defect_type")
		result.ActualRCAMessage = mapStr(m, "rca_message")
		result.ActualEvidenceRefs = mapStrSlice(m, "evidence_refs")
		result.ActualConvergence = mapFloat(m, "convergence_score")
		if comp := mapStr(m, "component"); comp != "" {
			result.ActualComponent = comp
		}
		if gb := mapMap(m, "gap_brief"); gb != nil {
			result.VerdictConfidence = mapStr(gb, "verdict")
			for _, item := range mapSlice(gb, "gap_items") {
				if eg, ok := item.(map[string]any); ok {
					result.EvidenceGaps = append(result.EvidenceGaps, EvidenceGap{
						Category:    mapStr(eg, "category"),
						Description: mapStr(eg, "description"),
						WouldHelp:   mapStr(eg, "would_help"),
						Source:      mapStr(eg, "source"),
						Blocked:     mapStr(eg, "blocked"),
					})
				}
			}
		}
	}
}

// selectRepoByHypothesis maps a defect_type_hypothesis to workspace repos
// using Purpose keyword matching. Returns nil if no match is found (caller
// should fall through to the AI-driven Resolve step).
func selectRepoByHypothesis(hypothesis string, repos []RepoConfig) []string {
	if hypothesis == "" || len(repos) == 0 {
		return nil
	}

	type rule struct {
		include []string
		exclude []string
	}
	prefix := strings.ToLower(hypothesis)

	var r rule
	switch {
	case strings.HasPrefix(prefix, "pb"):
		r = rule{
			include: []string{"operator", "daemon", "product"},
			exclude: []string{"test", "framework", "e2e", "deploy", "manifests"},
		}
	case strings.HasPrefix(prefix, "au"):
		r = rule{
			include: []string{"test", "framework", "e2e"},
			exclude: []string{},
		}
	case strings.HasPrefix(prefix, "en"):
		r = rule{
			include: []string{"config", "infra", "ci "},
			exclude: []string{},
		}
	default:
		return nil
	}

	var matched []string
	for _, repo := range repos {
		if repo.IsRedHerring {
			continue
		}
		purpose := strings.ToLower(repo.Purpose)

		excluded := false
		for _, kw := range r.exclude {
			if strings.Contains(purpose, kw) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		for _, kw := range r.include {
			if strings.Contains(purpose, kw) {
				matched = append(matched, repo.Name)
				break
			}
		}
	}
	if len(matched) == 0 {
		return nil
	}
	return matched
}

// updateIDMaps updates the transformer's RCA/symptom ID maps after a case
// completes, so subsequent cases can reference prior RCAs/symptoms by store ID.
func updateIDMaps(mapper IDMappable, st store.Store, caseData *store.Case, gtCase GroundTruthCase, scenario *Scenario) {
	updated, err := st.GetCase(caseData.ID)
	if err != nil || updated == nil {
		return
	}

	// Map ground truth RCA ID to store RCA ID
	if updated.RCAID != 0 && gtCase.RCAID != "" {
		mapper.SetRCAID(gtCase.RCAID, updated.RCAID)
	}

	// Map ground truth symptom ID to store symptom ID
	if updated.SymptomID != 0 && gtCase.SymptomID != "" {
		mapper.SetSymptomID(gtCase.SymptomID, updated.SymptomID)
	}
}

// pipeKey uniquely identifies a (version, job) combination for circuit/launch/job mapping.
type pipeKey struct{ version, job string }


func parseJSON[T any](data json.RawMessage) (*T, error) {
	cleaned := cleanJSON(data)
	var result T
	if err := json.Unmarshal(cleaned, &result); err != nil {
		// Fallback: sanitize lone backslashes and retry.
		sanitized := sanitizeBackslashes(cleaned)
		if !bytes.Equal(sanitized, cleaned) {
			if err2 := json.Unmarshal(sanitized, &result); err2 == nil {
				return &result, nil
			}
		}
		return nil, err
	}
	return &result, nil
}

// loneBackslashRe matches a backslash NOT followed by a valid JSON escape
// character (", \, /, b, f, n, r, t, or uXXXX).
var loneBackslashRe = regexp.MustCompile(`\\([^"\\/bfnrtu])`)

// sanitizeBackslashes removes lone backslashes that break json.Unmarshal.
func sanitizeBackslashes(data []byte) []byte {
	return loneBackslashRe.ReplaceAll(data, []byte("$1"))
}

// cleanJSON strips markdown code fences and leading/trailing whitespace from
// LLM responses. Models often wrap JSON in ```json ... ``` blocks. This
// handles: ```json\n{...}\n```, ```\n{...}\n```, and bare JSON.
func cleanJSON(data []byte) []byte {
	s := bytes.TrimSpace(data)
	if len(s) == 0 {
		return s
	}

	if bytes.HasPrefix(s, []byte("```")) {
		// Strip opening fence line
		if idx := bytes.IndexByte(s, '\n'); idx >= 0 {
			s = s[idx+1:]
		}
		// Strip closing fence
		if bytes.HasSuffix(s, []byte("```")) {
			s = s[:len(s)-3]
		}
		s = bytes.TrimSpace(s)
	}

	return s
}


// buildDatasetHealth creates a dataset health summary from the scenario.
func buildDatasetHealth(s *Scenario) *DatasetHealth {
	rcaMap := make(map[string]*GroundTruthRCA, len(s.RCAs))
	for i := range s.RCAs {
		rcaMap[s.RCAs[i].ID] = &s.RCAs[i]
	}

	dh := &DatasetHealth{
		VerifiedCount:  len(s.Cases),
		CandidateCount: len(s.Candidates),
	}
	for _, c := range s.Candidates {
		ci := CandidateInfo{
			CaseID: c.ID,
			RCAID:  c.RCAID,
		}
		if rcaRec, ok := rcaMap[c.RCAID]; ok {
			ci.JiraID = rcaRec.JiraID
			if len(rcaRec.FixPRs) == 0 {
				ci.Reason = "no fix PR"
			} else {
				ci.Reason = "disputed/unverified"
			}
		}
		dh.Candidates = append(dh.Candidates, ci)
	}
	return dh
}

