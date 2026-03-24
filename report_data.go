package rca

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/bugle/billing"
	"github.com/dpopsuev/origami/format"
	"github.com/dpopsuev/origami/report"
	"github.com/dpopsuev/origami/schematics/toolkit"
)

// --- Calibration report ---

func formatThreshold(m Metric) string {
	switch m.ID {
	case "M4", "M20":
		return fmt.Sprintf("≤%.2f", m.Threshold)
	case "M17":
		return "0.5–2.0"
	case "M18":
		return fmt.Sprintf("≤%.0f", m.Threshold)
	default:
		return fmt.Sprintf("≥%.2f", m.Threshold)
	}
}

func metricGroup(id string) string {
	switch id {
	case "M1", "M15", "M10":
		return "outcome"
	case "M2", "M3", "M5", "M6", "M7", "M8", "M12", "M13":
		return "investigation"
	case "M9", "M11":
		return "detection"
	case "M16", "M17", "M18":
		return "efficiency"
	default:
		return "meta"
	}
}

func metricRows(ms cal.MetricSet, group string) []map[string]any {
	var rows []map[string]any
	for _, m := range ms.Metrics {
		if metricGroup(m.ID) != group {
			continue
		}
		passMark := format.BoolMark(m.Pass)
		if m.DryCapped {
			passMark = "~"
		}
		rows = append(rows, map[string]any{
			"ID":        m.ID,
			"Metric":    defaultVocab.Name(m.ID),
			"Value":     fmt.Sprintf("%.2f", m.Value),
			"Detail":    m.Detail,
			"Pass":      passMark,
			"Threshold": formatThreshold(m),
		})
	}
	return rows
}

// CalibrationReportData converts a CalibrationReport into the map[string]any
// shape expected by the calibration-report.yaml template.
func CalibrationReportData(r *CalibrationReport) map[string]any {
	data := make(map[string]any)

	data["scenario_name"] = r.CalibrationReport.Scenario
	data["transformer"] = r.CalibrationReport.Transformer
	data["total_cases"] = len(r.CaseResults)

	passed, total := r.Metrics.PassCount()
	result := "PASS"
	if passed < total {
		result = "FAIL"
	}
	data["result_text"] = fmt.Sprintf("%s (%d/%d metrics within threshold)", result, passed, total)

	verifiedCount := 0
	candidateCount := 0
	if r.Dataset != nil {
		verifiedCount = r.Dataset.VerifiedCount
		candidateCount = r.Dataset.CandidateCount
	}
	data["verified_count"] = verifiedCount
	data["candidate_count"] = candidateCount

	metricGroups := map[string]string{
		"outcome":       "outcome_metrics",
		"investigation": "investigation_metrics",
		"detection":     "detection_metrics",
		"efficiency":    "efficiency_metrics",
		"meta":          "meta_metrics",
	}
	for group, key := range metricGroups {
		data[key] = metricRows(r.Metrics, group)
	}

	var candidates []map[string]any
	if r.Dataset != nil {
		for _, c := range r.Dataset.Candidates {
			jira := c.JiraID
			if jira == "" {
				jira = "-"
			}
			candidates = append(candidates, map[string]any{
				"Case":   c.CaseID,
				"RCA":    c.RCAID,
				"Jira":   jira,
				"Reason": c.Reason,
			})
		}
	}
	data["dataset_candidates"] = candidates

	caseRows := make([]map[string]any, 0, len(r.CaseResults))
	for _, cr := range r.CaseResults {
		path := vocabStagePath(cr.ActualPath)
		if path == "" {
			path = "(no steps)"
		}
		srcTag := vocabSourceIssueTag(cr.SourceIssueType, cr.SourceAutoAnalyzed)
		if srcTag == "" {
			srcTag = "-"
		}
		caseRows = append(caseRows, map[string]any{
			"Case":   cr.CaseID,
			"Test":   format.Truncate(cr.TestName, 40),
			"Ver/Job": fmt.Sprintf("%s/%s", cr.Version, cr.Job),
			"Defect": vocabNameWithCode(cr.ActualDefectType),
			"DT":     format.BoolMark(cr.DefectTypeCorrect),
			"Source": srcTag,
			"Comp":   format.BoolMark(cr.ComponentCorrect),
			"Path":   path,
			"PathOK": format.BoolMark(cr.PathCorrect),
		})
	}
	data["case_results"] = caseRows

	var gapRows []map[string]any
	for _, cr := range r.CaseResults {
		if len(cr.EvidenceGaps) == 0 {
			continue
		}
		cats := make([]string, 0, len(cr.EvidenceGaps))
		for _, g := range cr.EvidenceGaps {
			cats = append(cats, g.Category)
		}
		gapRows = append(gapRows, map[string]any{
			"Case":       cr.CaseID,
			"Verdict":    cr.VerdictConfidence,
			"Gaps":       fmt.Sprintf("%d", len(cr.EvidenceGaps)),
			"Categories": strings.Join(cats, ", "),
		})
	}
	data["evidence_gaps"] = gapRows

	return data
}

// RenderCalibrationReport produces the human-readable calibration report.
func RenderCalibrationReport(r *CalibrationReport, templateData []byte) (string, error) {
	def, err := report.ParseReportDef(templateData)
	if err != nil {
		return "", fmt.Errorf("parse calibration report template: %w", err)
	}
	data := CalibrationReportData(r)
	return report.Render(def, data)
}

// --- Analysis (RCA) report ---

// AnalysisReportData converts an AnalysisReport into the map[string]any
// shape expected by the rca-report.yaml template.
func AnalysisReportData(r *AnalysisReport, timestamp time.Time) map[string]any {
	data := make(map[string]any)
	data["launch_name"] = r.SourceName
	data["total_cases"] = len(r.CaseResults)
	data["transformer"] = r.Transformer

	headerFields := []map[string]any{}
	if r.SourceName != "" {
		headerFields = append(headerFields, map[string]any{
			"Field": "Launch", "Value": r.SourceName,
		})
	}
	headerFields = append(headerFields,
		map[string]any{"Field": "Analyzed", "Value": timestamp.UTC().Format("2006-01-02 15:04 UTC")},
		map[string]any{"Field": "Transformer", "Value": r.Transformer},
		map[string]any{"Field": "Failures", "Value": r.TotalCases},
	)
	data["header_fields"] = headerFields

	var investigated, skipped, recallHits, cascades int
	compCounts := make(map[string]int)
	defectCounts := make(map[string]int)
	for _, cr := range r.CaseResults {
		if cr.RCAID != 0 {
			investigated++
		}
		if cr.Skip {
			skipped++
		}
		if cr.RecallHit {
			recallHits++
		}
		if cr.Cascade {
			cascades++
		}
		comp := cr.Component
		if comp == "" {
			comp = "unknown"
		}
		compCounts[comp]++
		defectCounts[cr.DefectType]++
	}

	var summaryParts []string
	summaryParts = append(summaryParts, fmt.Sprintf("**%d** failures analyzed, **%d** investigated", r.TotalCases, investigated))
	if skipped > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d skipped", skipped))
	}
	if recallHits > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d recall hits", recallHits))
	}
	if cascades > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d cascades", cascades))
	}
	summary := strings.Join(summaryParts, ", ")
	summary += "\n\n**Components:** " + toolkit.FormatDistribution(compCounts, nil)
	summary += "\n\n**Defect types:** " + toolkit.FormatDistribution(defectCounts, vocabNameWithCode)
	data["summary_text"] = summary

	groups := groupAnalysisByComponent(r.CaseResults)
	sortedComps := toolkit.SortedKeys(groups)
	var components []map[string]any
	for _, comp := range sortedComps {
		cases := groups[comp]
		var caseRows []map[string]any
		for _, cr := range cases {
		srcTag := vocabSourceIssueTag(cr.SourceIssueType, cr.SourceAutoAnalyzed)
		if srcTag == "" {
			srcTag = "--"
		}
		caseRows = append(caseRows, map[string]any{
			"Case":       cr.CaseLabel,
			"Test":       format.Truncate(cr.TestName, 60),
			"Verdict":    vocabNameWithCode(cr.DefectType),
			"Confidence": fmt.Sprintf("%.0f%%", math.Round(cr.Convergence*100)),
			"Source":     srcTag,
			})
		}
		evidenceSet := collectAnalysisEvidence(cases)
		evidenceText := ""
		if len(evidenceSet) > 0 {
			evidenceText = "**Evidence:** " + strings.Join(evidenceSet, ", ")
		}
		components = append(components, map[string]any{
			"component_title": fmt.Sprintf("%s (%d %s)", comp, len(cases), toolkit.PluralizeCount(len(cases), "failure", "failures")),
			"cases":           caseRows,
			"evidence_text":   evidenceText,
		})
	}
	data["components"] = components

	var caseDetails []map[string]any
	for _, cr := range r.CaseResults {
		fields := []map[string]any{
			{"Field": "Verdict", "Value": vocabNameWithCode(cr.DefectType)},
			{"Field": "Category", "Value": cr.Category},
		}
		comp := cr.Component
		if comp == "" {
			comp = "unknown"
		}
		fields = append(fields,
			map[string]any{"Field": "Component", "Value": comp},
			map[string]any{"Field": "Confidence", "Value": fmt.Sprintf("%.0f%%", math.Round(cr.Convergence*100))},
			map[string]any{"Field": "Circuit", "Value": vocabStagePath(cr.Path)},
		)
		if cr.SourceIssueType != "" {
			fields = append(fields, map[string]any{"Field": "Source Classification", "Value": vocabSourceIssueTag(cr.SourceIssueType, cr.SourceAutoAnalyzed)})
		}
		if len(cr.SelectedRepos) > 0 {
			fields = append(fields, map[string]any{"Field": "Repos investigated", "Value": strings.Join(cr.SelectedRepos, ", ")})
		}
		if len(cr.EvidenceRefs) > 0 {
			fields = append(fields, map[string]any{"Field": "Evidence", "Value": strings.Join(cr.EvidenceRefs, ", ")})
		}
		var flags []string
		if cr.RecallHit {
			flags = append(flags, "recall-hit")
		}
		if cr.Skip {
			flags = append(flags, "skipped")
		}
		if cr.Cascade {
			flags = append(flags, "cascade")
		}
		if len(flags) > 0 {
			fields = append(fields, map[string]any{"Field": "Flags", "Value": strings.Join(flags, ", ")})
		}
		rcaText := ""
		if cr.RCAMessage != "" {
			rcaText = "**RCA:** " + cr.RCAMessage
		}
		caseDetails = append(caseDetails, map[string]any{
			"case_title": fmt.Sprintf("%s: %s", cr.CaseLabel, cr.TestName),
			"fields":     fields,
			"rca_text":   rcaText,
		})
	}
	data["case_details"] = caseDetails

	return data
}

// RenderAnalysisReport produces a Markdown RCA report.
// When templateData is nil the embedded rca-report.yaml is used.
func RenderAnalysisReport(r *AnalysisReport, timestamp time.Time, templateData []byte) (string, error) {
	if r == nil || len(r.CaseResults) == 0 {
		return "# RCA Report\n\nNo failures analyzed.\n", nil
	}
	def, err := report.ParseReportDef(templateData)
	if err != nil {
		return "", fmt.Errorf("parse RCA report template: %w", err)
	}
	data := AnalysisReportData(r, timestamp)
	return report.Render(def, data)
}

// --- Cost bill (moved from tokimeter.go) ---

var asteriskStepOrder = []string{
	"F0_RECALL", "F1_TRIAGE", "F2_RESOLVE", "F3_INVESTIGATE",
	"F4_CORRELATE", "F5_REVIEW", "F6_REPORT",
}

// BuildCostBill constructs a billing.CostBill from an Asterisk
// CalibrationReport, injecting domain-specific step names and case metadata.
func BuildCostBill(report *CalibrationReport) *billing.CostBill {
	if report.Tokens == nil {
		return nil
	}

	caseMap := make(map[string]CaseResult, len(report.CaseResults))
	for _, cr := range report.CaseResults {
		caseMap[cr.CaseID] = cr
	}

	return billing.BuildCostBill(report.Tokens,
		billing.WithTitle("TokiMeter"),
		billing.WithSubtitle(fmt.Sprintf("**%s** | transformer: `%s`", report.Scenario, report.Transformer)),
		billing.WithStepOrder(asteriskStepOrder),
		billing.WithStepNames(func(step string) string {
			return vocabNameWithCode(step)
		}),
		billing.WithCaseLabels(func(id string) string { return id }),
		billing.WithCaseDetails(func(id string) string {
			cr, ok := caseMap[id]
			if !ok {
				return "-"
			}
			return fmt.Sprintf("%s/%s", cr.Version, cr.Job)
		}),
	)
}

// --- Transcript types and logic (moved from transcript.go) ---

// TranscriptEntry represents one round of the Asterisk-agent dialog.
type TranscriptEntry struct {
	Step        string `json:"step"`
	StepName    string `json:"step_name"`
	Prompt      string `json:"prompt"`
	Response    string `json:"response"`
	HeuristicID string `json:"heuristic_id"`
	Decision    string `json:"decision"`
	Timestamp   string `json:"timestamp"`
}

// CaseTranscript holds the full dialog for one case.
type CaseTranscript struct {
	CaseID   string            `json:"case_id"`
	TestName string            `json:"test_name"`
	Version  string            `json:"version"`
	Job      string            `json:"job"`
	Path     []string          `json:"path"`
	Entries  []TranscriptEntry `json:"entries"`
}

// RCATranscript groups one or more cases that share the same Root Cause.
type RCATranscript struct {
	RCAID      int64            `json:"rca_id"`
	Component  string           `json:"component"`
	DefectType string           `json:"defect_type"`
	RCAMessage string           `json:"rca_message"`
	Primary    *CaseTranscript  `json:"primary"`
	Correlated []CaseTranscript `json:"correlated,omitempty"`
}

// WeaveTranscripts reads calibration artifacts from disk and produces one
// RCATranscript per distinct Root Cause. Returns nil (not an error) when
// weaving is not possible.
func WeaveTranscripts(calReport *CalibrationReport) ([]RCATranscript, error) {
	if calReport == nil || len(calReport.CaseResults) == 0 {
		return nil, nil
	}

	groups := groupByRCA(calReport.CaseResults)
	var transcripts []RCATranscript

	for rcaID, cases := range groups {
		t := RCATranscript{RCAID: rcaID}

		primary := pickPrimary(cases)
		t.Component = primary.ActualComponent
		t.DefectType = primary.ActualDefectType
		t.RCAMessage = primary.ActualRCAMessage

		ct, err := buildCaseTranscript(calReport, primary)
		if err != nil {
			return nil, fmt.Errorf("weave case %s: %w", primary.CaseID, err)
		}
		t.Primary = ct

		for i := range cases {
			if cases[i].CaseID == primary.CaseID {
				continue
			}
			corr, err := buildCaseTranscript(calReport, &cases[i])
			if err != nil {
				return nil, fmt.Errorf("weave correlated case %s: %w", cases[i].CaseID, err)
			}
			t.Correlated = append(t.Correlated, *corr)
		}

		transcripts = append(transcripts, t)
	}

	return transcripts, nil
}

// TranscriptSlug returns a filesystem-safe slug for naming the transcript file.
func TranscriptSlug(t *RCATranscript) string {
	comp := strings.ToLower(strings.ReplaceAll(t.Component, " ", "-"))
	dt := strings.ToLower(t.DefectType)
	if comp == "" {
		comp = "unknown"
	}
	if dt == "" {
		dt = "unknown"
	}
	return fmt.Sprintf("rca-transcript-%s-%s", comp, dt)
}

// TranscriptData converts an RCATranscript into the map[string]any
// shape expected by the transcript-report.yaml template.
func TranscriptData(t *RCATranscript) map[string]any {
	data := make(map[string]any)

	data["transcript_title"] = fmt.Sprintf("RCA Transcript — %s: %s",
		t.Component, vocabNameWithCode(t.DefectType))

	caseIDs := []string{t.Primary.CaseID + " (primary)"}
	for _, c := range t.Correlated {
		caseIDs = append(caseIDs, c.CaseID+" (correlated)")
	}
	data["header_fields"] = []map[string]any{
		{"Field": "RCA ID", "Value": fmt.Sprintf("%d", t.RCAID)},
		{"Field": "Component", "Value": t.Component},
		{"Field": "Defect Type", "Value": vocabNameWithCode(t.DefectType)},
		{"Field": "Cases", "Value": strings.Join(caseIDs, ", ")},
		{"Field": "Generated", "Value": time.Now().UTC().Format(time.RFC3339)},
	}

	data["primary_info"] = fmt.Sprintf("**Test:** %s  \n**Path:** %s",
		t.Primary.TestName, vocabStagePath(t.Primary.Path))
	data["primary_entries"] = transcriptEntryData(t.Primary.Entries)

	var correlated []map[string]any
	for _, c := range t.Correlated {
		correlated = append(correlated, map[string]any{
			"case_title": fmt.Sprintf("Correlated Case: %s", c.CaseID),
			"case_info":  fmt.Sprintf("**Test:** %s  \n**Path:** %s", c.TestName, vocabStagePath(c.Path)),
			"entries":    transcriptEntryData(c.Entries),
		})
	}
	data["correlated_cases"] = correlated

	return data
}

func transcriptEntryData(entries []TranscriptEntry) []map[string]any {
	var result []map[string]any
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		var body strings.Builder
		if e.Prompt != "" {
			body.WriteString("#### Prompt\n\n")
			for _, line := range strings.Split(e.Prompt, "\n") {
				body.WriteString("> " + line + "\n")
			}
			body.WriteString("\n")
		}
		if e.Response != "" {
			body.WriteString("#### Response\n\n```json\n")
			body.WriteString(e.Response)
			body.WriteString("\n```\n\n")
		}
		body.WriteString(fmt.Sprintf("#### Decision: %s — %s", e.HeuristicID, e.Decision))

		result = append(result, map[string]any{
			"entry_title": fmt.Sprintf("%s %s (%s)", e.Step, e.StepName, e.Timestamp),
			"entry_body":  body.String(),
		})
	}
	return result
}

// RenderTranscript produces a Markdown document for one RCA transcript.
func RenderTranscript(t *RCATranscript, templateData []byte) (string, error) {
	def, err := report.ParseReportDef(templateData)
	if err != nil {
		return "", fmt.Errorf("parse transcript template: %w", err)
	}
	data := TranscriptData(t)
	return report.Render(def, data)
}

// --- internal helpers ---

func groupByRCA(results []CaseResult) map[int64][]CaseResult {
	groups := make(map[int64][]CaseResult)
	for _, cr := range results {
		key := cr.ActualRCAID
		if key == 0 {
			key = -cr.StoreCaseID
		}
		groups[key] = append(groups[key], cr)
	}
	return groups
}

func pickPrimary(cases []CaseResult) *CaseResult {
	best := &cases[0]
	for i := 1; i < len(cases); i++ {
		if len(cases[i].ActualPath) > len(best.ActualPath) {
			best = &cases[i]
		}
	}
	return best
}

func buildCaseTranscript(calReport *CalibrationReport, cr *CaseResult) (*CaseTranscript, error) {
	ct := &CaseTranscript{
		CaseID:   cr.CaseID,
		TestName: cr.TestName,
		Version:  cr.Version,
		Job:      cr.Job,
		Path:     cr.ActualPath,
	}

	caseDir := CaseDir(calReport.BasePath, calReport.SuiteID, cr.StoreCaseID)

	stateData, stateErr := os.ReadFile(filepath.Join(caseDir, "state.json"))
	if stateErr != nil {
		if os.IsNotExist(stateErr) {
			return ct, nil
		}
		return ct, fmt.Errorf("load state: %w", stateErr)
	}
	var stateVal CaseState
	if err := json.Unmarshal(stateData, &stateVal); err != nil {
		return ct, fmt.Errorf("parse state: %w", err)
	}
	state := &stateVal

	for _, record := range state.History {
		step := record.Step
		if step == "INIT" || step == "DONE" {
			continue
		}

		entry := TranscriptEntry{
			Step:        step,
			StepName:    vocabName(step),
			HeuristicID: record.HeuristicID,
			Decision:    record.Outcome,
			Timestamp:   record.Timestamp,
		}

		promptFile := NodePromptFilename(step, 0)
		if promptFile != "" {
			if data, err := os.ReadFile(filepath.Join(caseDir, promptFile)); err == nil {
				entry.Prompt = string(data)
			}
		}

		artifactFile := NodeArtifactFilename(step)
		if artifactFile != "" {
			if data, err := os.ReadFile(filepath.Join(caseDir, artifactFile)); err == nil {
				var buf json.RawMessage
				if json.Unmarshal(data, &buf) == nil {
					if pretty, err := json.MarshalIndent(buf, "", "  "); err == nil {
						entry.Response = string(pretty)
					} else {
						entry.Response = string(data)
					}
				} else {
					entry.Response = string(data)
				}
			}
		}

		ct.Entries = append(ct.Entries, entry)
	}

	return ct, nil
}

func groupAnalysisByComponent(cases []AnalysisCaseResult) map[string][]AnalysisCaseResult {
	groups := make(map[string][]AnalysisCaseResult)
	for _, cr := range cases {
		comp := cr.Component
		if comp == "" {
			comp = "unknown"
		}
		groups[comp] = append(groups[comp], cr)
	}
	return groups
}


func collectAnalysisEvidence(cases []AnalysisCaseResult) []string {
	seen := make(map[string]bool)
	var result []string
	for _, cr := range cases {
		for _, ref := range cr.EvidenceRefs {
			if !seen[ref] {
				seen[ref] = true
				result = append(result, ref)
			}
		}
	}
	return result
}

