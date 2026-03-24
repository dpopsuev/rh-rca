// Package calibrate implements the E2E calibration framework for Asterisk.
// It drives the F0–F6 circuit against known ground truth (synthetic or real)
// and measures how closely the agent's conclusions match the known answers.
package rca

import (
	"reflect"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/schematics/toolkit"
)

// Metric is an alias for the generic calibrate.Metric type.
type Metric = cal.Metric

// MetricSet is an alias for the generic calibrate.MetricSet type.
type MetricSet = cal.MetricSet

// CalibrationMode controls how data is sourced during calibration.
type CalibrationMode string

const (
	ModeOnline  CalibrationMode = "online"
	ModeOffline CalibrationMode = "offline"
)

// ParseCalibrationMode converts a string to CalibrationMode, defaulting to online.
func ParseCalibrationMode(s string) CalibrationMode {
	if s == "offline" {
		return ModeOffline
	}
	return ModeOnline
}

// Scenario defines a complete calibration scenario with ground truth data.
type Scenario struct {
	Name             string               `json:"name" yaml:"name"`
	Description      string               `json:"description" yaml:"description"`
	Defaults         *GroundTruthCase     `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	RCAs             []GroundTruthRCA     `json:"rcas" yaml:"rcas"`
	Symptoms         []GroundTruthSymptom `json:"symptoms" yaml:"symptoms"`
	Cases            []GroundTruthCase    `json:"cases" yaml:"cases"`
	Candidates       []GroundTruthCase    `json:"candidates,omitempty" yaml:"candidates,omitempty"` // unverified cases tracked for dataset growth, never scored
	SourcePack       SourcePackConfig     `json:"source_pack" yaml:"source_pack"`
	DryCappedMetrics []string             `json:"dry_capped_metrics,omitempty" yaml:"dry_capped_metrics,omitempty"` // metrics structurally unsolvable without real repo content
}

// ApplyDefaults merges Defaults into each Case, filling zero-value fields.
func (s *Scenario) ApplyDefaults() {
	if s.Defaults == nil {
		return
	}
	for i := range s.Cases {
		mergeDefaults(&s.Cases[i], s.Defaults)
	}
	for i := range s.Candidates {
		mergeDefaults(&s.Candidates[i], s.Defaults)
	}
}

// mergeDefaults copies non-zero fields from defaults into dst where dst has zero values.
func mergeDefaults(dst, defaults *GroundTruthCase) {
	dv := reflect.ValueOf(dst).Elem()
	sv := reflect.ValueOf(defaults).Elem()
	for i := 0; i < dv.NumField(); i++ {
		df := dv.Field(i)
		sf := sv.Field(i)
		if df.IsZero() && !sf.IsZero() {
			df.Set(sf)
		}
	}
}

// GroundTruthRCA is a known root cause for calibration scoring.
type GroundTruthRCA struct {
	ID               string   `json:"id" yaml:"id"`                // e.g. "R1"
	Title            string   `json:"title" yaml:"title"`
	Description      string   `json:"description" yaml:"description"`
	DefectType       string   `json:"defect_type" yaml:"defect_type"`       // e.g. "pb001"
	Category         string   `json:"category" yaml:"category"`           // product / automation / infra
	Component        string   `json:"component" yaml:"component"`
	AffectedVersions []string `json:"affected_versions" yaml:"affected_versions"`
	JiraID           string   `json:"jira_id,omitempty" yaml:"jira_id,omitempty"`
	RequiredKeywords []string `json:"required_keywords" yaml:"required_keywords"`  // for stub-mode semantic match
	KeywordThreshold int      `json:"keyword_threshold" yaml:"keyword_threshold"`  // min keywords needed
	RelevantRepos    []string `json:"relevant_repos" yaml:"relevant_repos"`     // repos that should be selected for this RCA
	FixPRs           []string `json:"fix_prs,omitempty" yaml:"fix_prs,omitempty"`
	Verified         bool     `json:"verified" yaml:"verified"`                 // true = PR-proven ground truth; false = candidate (not scored)
	SmokingGun       string   `json:"smoking_gun,omitempty" yaml:"smoking_gun,omitempty"`    // key phrase from the fix PR proving the root cause
}

// GroundTruthSymptom is a known symptom pattern.
type GroundTruthSymptom struct {
	ID           string `json:"id" yaml:"id"`            // e.g. "S1"
	Name         string `json:"name" yaml:"name"`
	ErrorPattern string `json:"error_pattern" yaml:"error_pattern"` // regex-like pattern for matching
	Component    string `json:"component" yaml:"component"`
	MapsToRCA    string `json:"maps_to_rca" yaml:"maps_to_rca"`   // GroundTruthRCA.ID
}

// GroundTruthCase is a known test failure with expected outcomes.
type GroundTruthCase struct {
	ID           string   `json:"id" yaml:"id"`           // e.g. "C1"
	Version      string   `json:"version" yaml:"version"`      // e.g. "4.20"
	Job          string   `json:"job" yaml:"job"`           // e.g. "[T-TSC]"
	TestName     string   `json:"test_name" yaml:"test_name"`
	TestID       string   `json:"test_id,omitempty" yaml:"test_id,omitempty"` // RP item ID
	ErrorMessage string   `json:"error_message" yaml:"error_message"` // planted error message
	LogSnippet   string   `json:"log_snippet" yaml:"log_snippet"`   // planted log snippet
	SymptomID    string   `json:"symptom_id" yaml:"symptom_id"`    // expected GroundTruthSymptom.ID
	RCAID        string   `json:"rca_id" yaml:"rca_id"`        // expected GroundTruthRCA.ID
	ExpectedPath []string `json:"expected_path" yaml:"expected_path"` // expected circuit steps, e.g. ["F0","F1","F2","F3","F4","F5","F6"]

	// Expected per-step outcomes (for stub backend responses)
	ExpectedRecall    *ExpectedRecall    `json:"expected_recall,omitempty" yaml:"expected_recall,omitempty"`
	ExpectedTriage    *ExpectedTriage    `json:"expected_triage,omitempty" yaml:"expected_triage,omitempty"`
	ExpectedResolve   *ExpectedResolve   `json:"expected_resolve,omitempty" yaml:"expected_resolve,omitempty"`
	ExpectedInvest    *ExpectedInvest    `json:"expected_invest,omitempty" yaml:"expected_invest,omitempty"`
	ExpectedCorrelate *ExpectedCorrelate `json:"expected_correlate,omitempty" yaml:"expected_correlate,omitempty"`
	ExpectedReview    *ExpectedReview    `json:"expected_review,omitempty" yaml:"expected_review,omitempty"`

	// Flags for metric computation
	ExpectRecallHit   bool `json:"expect_recall_hit" yaml:"expect_recall_hit"`
	ExpectSkip        bool `json:"expect_skip" yaml:"expect_skip"`         // infra/flake skip
	ExpectCascade     bool `json:"expect_cascade" yaml:"expect_cascade"`
	ExpectedLoops     int  `json:"expected_loops" yaml:"expected_loops"`       // expected F3→F2→F3 loops

	// Source fields (optional). When SourceLaunchID > 0, the calibration runner
	// fetches real failure data from RP at runtime instead of using the embedded
	// ErrorMessage/LogSnippet. Ground truth expectations remain embedded.
	SourceLaunchID     int    `json:"source_launch_id,omitempty" yaml:"source_launch_id,omitempty"`
	SourceItemID       int    `json:"source_item_id,omitempty" yaml:"source_item_id,omitempty"`
	SourceIssueType    string `json:"source_issue_type,omitempty" yaml:"source_issue_type,omitempty"`    // populated at runtime by ResolveRPCases
	SourceAutoAnalyzed bool   `json:"source_auto_analyzed,omitempty" yaml:"source_auto_analyzed,omitempty"` // populated at runtime by ResolveRPCases

	// Antithesis dialectic expectations (optional)
	ExpectedSynthesis string `json:"expected_synthesis,omitempty" yaml:"expected_synthesis,omitempty"` // expected SynthesisDecision if dialectic activates
}

// ExpectedRecall defines the ideal F0 output for a case.
type ExpectedRecall struct {
	Match      bool    `json:"match" yaml:"match"`
	PriorRCAID int64   `json:"prior_rca_id,omitempty" yaml:"prior_rca_id,omitempty"`
	SymptomID  int64   `json:"symptom_id,omitempty" yaml:"symptom_id,omitempty"`
	Confidence float64 `json:"confidence" yaml:"confidence"`
}

// ExpectedTriage defines the ideal F1 output.
type ExpectedTriage struct {
	SymptomCategory      string   `json:"symptom_category" yaml:"symptom_category"`
	Severity             string   `json:"severity" yaml:"severity"`
	DefectTypeHypothesis string   `json:"defect_type_hypothesis" yaml:"defect_type_hypothesis"`
	CandidateRepos       []string `json:"candidate_repos" yaml:"candidate_repos"`
	SkipInvestigation    bool     `json:"skip_investigation" yaml:"skip_investigation"`
	CascadeSuspected     bool     `json:"cascade_suspected" yaml:"cascade_suspected"`
}

// ExpectedResolve defines the ideal F2 output.
type ExpectedResolve struct {
	SelectedRepos []ExpectedResolveRepo `json:"selected_repos" yaml:"selected_repos"`
}

// ExpectedResolveRepo is a simplified repo selection for ground truth.
type ExpectedResolveRepo struct {
	Name   string `json:"name" yaml:"name"`
	Reason string `json:"reason" yaml:"reason"`
}

// ExpectedInvest defines the ideal F3 output.
type ExpectedInvest struct {
	RCAMessage       string   `json:"rca_message" yaml:"rca_message"`
	DefectType       string   `json:"defect_type" yaml:"defect_type"`
	Component        string   `json:"component" yaml:"component"`
	ConvergenceScore float64  `json:"convergence_score" yaml:"convergence_score"`
	EvidenceRefs     []string `json:"evidence_refs" yaml:"evidence_refs"`
}

// ExpectedCorrelate defines the ideal F4 output.
type ExpectedCorrelate struct {
	IsDuplicate       bool    `json:"is_duplicate" yaml:"is_duplicate"`
	LinkedRCAID       int64   `json:"linked_rca_id,omitempty" yaml:"linked_rca_id,omitempty"`
	Confidence        float64 `json:"confidence" yaml:"confidence"`
	CrossVersionMatch bool    `json:"cross_version_match" yaml:"cross_version_match"`
}

// ExpectedReview defines the ideal F5 output.
type ExpectedReview struct {
	Decision string `json:"decision" yaml:"decision"` // approve
}

// SourcePackConfig describes the source repositories and docs for F2/F3.
type SourcePackConfig struct {
	Repos   []RepoConfig       `json:"repos" yaml:"repos"`
	Sources []toolkit.Source  `json:"sources,omitempty" yaml:"sources,omitempty"`
}

// RepoConfig describes one repo in the source pack.
type RepoConfig struct {
	Name    string `json:"name" yaml:"name"`
	Path    string `json:"path" yaml:"path"`
	Purpose string `json:"purpose" yaml:"purpose"`
	Branch  string `json:"branch" yaml:"branch"`

	// Ground truth: is this repo relevant to any RCA?
	RelevantToRCAs []string `json:"relevant_to_rcas,omitempty" yaml:"relevant_to_rcas,omitempty"`
	IsRedHerring   bool     `json:"is_red_herring,omitempty" yaml:"is_red_herring,omitempty"`
}

// DatasetHealth summarizes the ground truth dataset composition.
type DatasetHealth struct {
	VerifiedCount  int             `json:"verified_count" yaml:"verified_count"`
	CandidateCount int             `json:"candidate_count" yaml:"candidate_count"`
	Candidates     []CandidateInfo `json:"candidates,omitempty" yaml:"candidates,omitempty"`
}

// CandidateInfo describes an unverified candidate case.
type CandidateInfo struct {
	CaseID string `json:"case_id" yaml:"case_id"`
	RCAID  string `json:"rca_id" yaml:"rca_id"`
	JiraID string `json:"jira_id,omitempty" yaml:"jira_id,omitempty"`
	Reason string `json:"reason" yaml:"reason"`
}

// CalibrationReport is the final output of a calibration run.
// It embeds the generic cal.CalibrationReport (Scenario, Transformer, Runs,
// Metrics, RunMetrics, Tokens) and adds domain-specific fields.
type CalibrationReport struct {
	cal.CalibrationReport
	SuiteID     int64                   `json:"suite_id" yaml:"suite_id"`
	BasePath    string                  `json:"-" yaml:"-"`
	CaseResults []CaseResult            `json:"case_results" yaml:"case_results"`
	Dataset     *DatasetHealth          `json:"dataset,omitempty" yaml:"dataset,omitempty"`
}

// CaseResult captures the per-case investigation outcome.
type CaseResult struct {
	CaseID       string   `json:"case_id" yaml:"case_id"`       // ground truth ID, e.g. "C1"
	TestName     string   `json:"test_name" yaml:"test_name"`
	Version      string   `json:"version" yaml:"version"`
	Job          string   `json:"job" yaml:"job"`
	StoreCaseID  int64    `json:"store_case_id" yaml:"store_case_id"`  // internal store ID

	// Actual outcomes
	ActualDefectType  string   `json:"actual_defect_type" yaml:"actual_defect_type"`
	ActualCategory    string   `json:"actual_category" yaml:"actual_category"`
	ActualRCAMessage  string   `json:"actual_rca_message" yaml:"actual_rca_message"`
	ActualComponent   string   `json:"actual_component" yaml:"actual_component"`
	ActualPath        []string `json:"actual_path" yaml:"actual_path"`         // actual circuit steps taken
	ActualRecallHit   bool     `json:"actual_recall_hit" yaml:"actual_recall_hit"`
	ActualSkip        bool     `json:"actual_skip" yaml:"actual_skip"`
	ActualCascade     bool     `json:"actual_cascade" yaml:"actual_cascade"`
	ActualLoops       int      `json:"actual_loops" yaml:"actual_loops"`
	ActualEvidenceRefs []string `json:"actual_evidence_refs" yaml:"actual_evidence_refs"`
	ActualSelectedRepos []string `json:"actual_selected_repos" yaml:"actual_selected_repos"`
	ActualRCAID       int64    `json:"actual_rca_id" yaml:"actual_rca_id"`
	ActualConvergence float64  `json:"actual_convergence" yaml:"actual_convergence"`

	// Source-provided classification (populated for source-sourced cases)
	SourceIssueType    string `json:"source_issue_type,omitempty" yaml:"source_issue_type,omitempty"`
	SourceAutoAnalyzed bool   `json:"source_auto_analyzed,omitempty" yaml:"source_auto_analyzed,omitempty"`

	// Token tracking (populated when billing.Tracker is present)
	PromptTokensTotal   int   `json:"prompt_tokens_total,omitempty" yaml:"prompt_tokens_total,omitempty"`
	ArtifactTokensTotal int   `json:"artifact_tokens_total,omitempty" yaml:"artifact_tokens_total,omitempty"`
	StepCount           int   `json:"step_count,omitempty" yaml:"step_count,omitempty"`
	WallClockMs         int64 `json:"wall_clock_ms,omitempty" yaml:"wall_clock_ms,omitempty"`

	// Per-case scoring
	DefectTypeCorrect  bool    `json:"defect_type_correct" yaml:"defect_type_correct"`
	CategoryCorrect    bool    `json:"category_correct" yaml:"category_correct"`
	PathCorrect        bool    `json:"path_correct" yaml:"path_correct"`
	ComponentCorrect   bool    `json:"component_correct" yaml:"component_correct"`
	SemanticScore      float64 `json:"semantic_score" yaml:"semantic_score"` // 0-1

	// Evidence gap analysis
	VerdictConfidence string        `json:"verdict_confidence,omitempty" yaml:"verdict_confidence,omitempty"`
	EvidenceGaps      []EvidenceGap `json:"evidence_gaps,omitempty" yaml:"evidence_gaps,omitempty"`

	// Circuit error (non-empty when the case failed during execution)
	CircuitError string `json:"circuit_error,omitempty" yaml:"circuit_error,omitempty"`
}
