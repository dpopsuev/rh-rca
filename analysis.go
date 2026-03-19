package rca

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dpopsuev/rh-rca/store"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/origami/format"
	"github.com/dpopsuev/origami/schematics/toolkit"
)

// AnalysisConfig holds configuration for an analysis run.
type AnalysisConfig struct {
	Components      []*framework.Component
	Envelope        *rcatype.Envelope
	Catalog         toolkit.SourceCatalog
	Thresholds      Thresholds
	BasePath        string // root directory for investigation artifacts; defaults to DefaultBasePath
	CircuitData     []byte // circuit definition YAML; required
}

// AnalysisReport is the output of an analysis run.
// Unlike CalibrationReport, there is no ground truth scoring — just investigation results.
type AnalysisReport struct {
	SourceName  string               `json:"source_name"`
	Transformer string               `json:"transformer"`
	TotalCases  int                  `json:"total_cases"`
	CaseResults []AnalysisCaseResult `json:"case_results"`
}

// AnalysisCaseResult captures per-case investigation outcome without ground truth scoring.
type AnalysisCaseResult struct {
	CaseLabel     string   `json:"case_label"`
	TestName      string   `json:"test_name"`
	StoreCaseID   int64    `json:"store_case_id"`
	DefectType    string   `json:"defect_type"`
	Category      string   `json:"category"`
	RCAMessage    string   `json:"rca_message"`
	Component     string   `json:"component"`
	Path          []string `json:"path"`
	RecallHit     bool     `json:"recall_hit"`
	Skip          bool     `json:"skip"`
	Cascade       bool     `json:"cascade"`
	EvidenceRefs  []string `json:"evidence_refs"`
	SelectedRepos []string `json:"selected_repos"`
	Convergence    float64  `json:"convergence"`
	RCAID          int64    `json:"rca_id"`
	SourceIssueType    string   `json:"source_issue_type,omitempty"`
	SourceAutoAnalyzed bool     `json:"source_auto_analyzed,omitempty"`
}

// RunAnalysis drives the F0–F6 circuit for a set of cases using the provided transformer.
// Unlike RunCalibration, there is no ground truth scoring — just investigation results.
// Each case is walked through the circuit graph using WalkCase with store-effect hooks.
func RunAnalysis(st store.Store, cases []*store.Case, suiteID int64, cfg AnalysisConfig) (*AnalysisReport, error) {
	transformerName := "unknown"
	if len(cfg.Components) > 0 {
		transformerName = cfg.Components[0].Name
	}
	report := &AnalysisReport{
		Transformer: transformerName,
		TotalCases: len(cases),
	}

	logger := slog.Default().With("component", "analyze")

	for i, caseData := range cases {
		caseLabel := fmt.Sprintf("A%d", i+1)
		logger.Info("processing case",
			"label", caseLabel, "index", i+1, "total", len(cases), "test", caseData.Name)

		result, err := walkAnalysisCase(st, caseData, caseLabel, cfg)
		if err != nil {
			logger.Error("case circuit failed", "label", caseLabel, "error", err)
			result = &AnalysisCaseResult{
				CaseLabel:   caseLabel,
				TestName:    caseData.Name,
				StoreCaseID: caseData.ID,
			}
		}
		report.CaseResults = append(report.CaseResults, *result)
	}

	return report, nil
}

// walkAnalysisCase runs a single case through the RCA circuit via a framework
// graph walk. Store effects fire automatically via hooks declared in the circuit YAML.
func walkAnalysisCase(
	st store.Store,
	caseData *store.Case,
	caseLabel string,
	cfg AnalysisConfig,
) (*AnalysisCaseResult, error) {
	result := &AnalysisCaseResult{
		CaseLabel:   caseLabel,
		TestName:    caseData.Name,
		StoreCaseID: caseData.ID,
	}

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = DefaultBasePath
	}
	caseDir, _ := EnsureCaseDir(basePath, 0, caseData.ID)

	hooksComp := &framework.Component{
		Namespace: "store",
		Name:      "rca-store-hooks",
		Hooks:     StoreHooks(st, caseData),
	}
	injectComp := &framework.Component{
		Namespace: "inject",
		Name:      "rca-inject-hooks",
		Hooks: InjectHooksWithOpts(InjectHookOpts{
			Store:           st,
			CaseData:        caseData,
			Envelope:        cfg.Envelope,
			Catalog:         cfg.Catalog,
			CaseDir:         caseDir,
		}),
	}
	comps := append(cfg.Components, hooksComp, injectComp)

	walkCfg := WalkConfig{
		Store:       st,
		CaseData:    caseData,
		Envelope:    cfg.Envelope,
		Catalog:     cfg.Catalog,
		CaseDir:     caseDir,
		CaseLabel:   caseLabel,
		Thresholds:  cfg.Thresholds,
		CircuitData: cfg.CircuitData,
		Components:  comps,
	}

	walkResult, err := WalkCase(context.Background(), walkCfg)
	if err != nil {
		return result, fmt.Errorf("walk: %w", err)
	}

	result.Path = walkResult.Path

	for nodeName, art := range walkResult.StepArtifacts {
		extractAnalysisStepData(result, nodeName, art.Raw())
	}

	updated, err := st.GetCase(caseData.ID)
	if err == nil && updated != nil {
		result.RCAID = updated.RCAID
		if updated.RCAID != 0 {
			rcaRec, err := st.GetRCA(updated.RCAID)
			if err == nil && rcaRec != nil {
				result.DefectType = rcaRec.DefectType
				result.RCAMessage = rcaRec.Description
				result.Component = rcaRec.Component
				result.Convergence = rcaRec.ConvergenceScore
			}
		}
	}

	return result, nil
}

// extractAnalysisStepData captures per-step results without ground truth comparison.
func extractAnalysisStepData(result *AnalysisCaseResult, nodeName string, artifact any) {
	m := asMap(artifact)
	if m == nil {
		return
	}
	switch nodeName {
	case "recall":
		result.RecallHit = mapBool(m, "match") && mapFloat(m, "confidence") >= 0.80
	case "triage":
		result.Category = mapStr(m, "symptom_category")
		result.Skip = mapBool(m, "skip_investigation")
		result.Cascade = mapBool(m, "cascade_suspected")
		candidates := mapStrSlice(m, "candidate_repos")
		if len(candidates) == 1 && !mapBool(m, "skip_investigation") {
			result.SelectedRepos = append(result.SelectedRepos, candidates[0])
		}
	case "resolve":
		for _, r := range mapSlice(m, "selected_repos") {
			if rm, ok := r.(map[string]any); ok {
				if name := mapStr(rm, "name"); name != "" {
					result.SelectedRepos = append(result.SelectedRepos, name)
				}
			}
		}
	case "investigate":
		result.DefectType = mapStr(m, "defect_type")
		result.RCAMessage = mapStr(m, "rca_message")
		result.EvidenceRefs = mapStrSlice(m, "evidence_refs")
		result.Convergence = mapFloat(m, "convergence_score")
	}
}

// FormatAnalysisReport produces a human-readable analysis report.
func FormatAnalysisReport(report *AnalysisReport) string {
	var b strings.Builder

	b.WriteString("=== Asterisk Analysis Report ===\n")
	if report.SourceName != "" {
		b.WriteString(fmt.Sprintf("Launch:  %s\n", report.SourceName))
	}
	b.WriteString(fmt.Sprintf("Transformer: %s\n", report.Transformer))
	b.WriteString(fmt.Sprintf("Cases:   %d\n\n", report.TotalCases))

	recallHits := 0
	skipped := 0
	cascades := 0
	investigated := 0
	for _, cr := range report.CaseResults {
		if cr.RecallHit {
			recallHits++
		}
		if cr.Skip {
			skipped++
		}
		if cr.Cascade {
			cascades++
		}
		if cr.RCAID != 0 {
			investigated++
		}
	}
	b.WriteString(fmt.Sprintf("Recall hits:  %d/%d\n", recallHits, report.TotalCases))
	b.WriteString(fmt.Sprintf("Skipped:      %d/%d\n", skipped, report.TotalCases))
	b.WriteString(fmt.Sprintf("Cascades:     %d/%d\n", cascades, report.TotalCases))
	b.WriteString(fmt.Sprintf("Investigated: %d/%d\n\n", investigated, report.TotalCases))

	b.WriteString("--- Per-case breakdown ---\n")
	tbl := format.NewTable(format.ASCII)
	tbl.Header("Case", "Test", "Defect", "Source", "Category", "Conv", "Path", "Flags")
	tbl.Columns(
		format.ColumnConfig{Number: 2, MaxWidth: 50},
		format.ColumnConfig{Number: 6, Align: format.AlignRight},
	)
	for _, cr := range report.CaseResults {
		path := vocabStagePath(cr.Path)
		if path == "" {
			path = "(no steps)"
		}
		flags := ""
		if cr.RecallHit {
			flags += "[recall]"
		}
		if cr.Skip {
			if flags != "" {
				flags += " "
			}
			flags += "[skip]"
		}
		if cr.Cascade {
			if flags != "" {
				flags += " "
			}
			flags += "[cascade]"
		}
		rpTag := vocabSourceIssueTag(cr.SourceIssueType, cr.SourceAutoAnalyzed)
		if rpTag == "" {
			rpTag = "-"
		}
		tbl.Row(
			cr.CaseLabel,
			format.Truncate(cr.TestName, 50),
			vocabNameWithCode(cr.DefectType),
			rpTag,
			cr.Category,
			fmt.Sprintf("%.2f", cr.Convergence),
			path,
			flags,
		)
	}
	b.WriteString(tbl.String())
	b.WriteString("\n")

	// RCA messages below the table
	for _, cr := range report.CaseResults {
		if cr.RCAMessage != "" {
			b.WriteString(fmt.Sprintf("  %s RCA: %s\n", cr.CaseLabel, format.Truncate(cr.RCAMessage, 80)))
		}
	}

	return b.String()
}
