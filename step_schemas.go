package rca

import "github.com/dpopsuev/origami/toolkit"

// RCAStepSchemas returns the step schemas for the RCA circuit.
// Workers use these to know what fields each step expects.
func RCAStepSchemas() []toolkit.StepSchema {
	return []toolkit.StepSchema{
		{
			Name: "recall",
			Defs: []toolkit.FieldDef{
				{Name: "match", Type: "bool", Required: true, Desc: "true if a prior RCA likely explains this failure"},
				{Name: "prior_rca_id", Type: "int", Required: false, Desc: "matched RCA ID, 0 if no match"},
				{Name: "symptom_id", Type: "int", Required: false, Desc: "matched symptom ID, 0 if no match"},
				{Name: "confidence", Type: "float", Required: true, Desc: "0.0-1.0 (>=0.8 high, 0.4-0.8 uncertain, <0.4 miss)"},
				{Name: "reasoning", Type: "string", Required: true, Desc: "brief explanation of match or mismatch"},
				{Name: "is_regression", Type: "bool", Required: false, Desc: "true if known-resolved symptom reappearing"},
			},
		},
		{
			Name: "triage",
			Defs: []toolkit.FieldDef{
				{Name: "symptom_category", Type: "string", Required: true, Desc: "product, automation, environment, infra, firmware, flake"},
				{Name: "severity", Type: "string", Required: true, Desc: "critical, high, medium, low"},
				{Name: "defect_type_hypothesis", Type: "string", Required: true, Desc: "e.g. pb001, au001, en001"},
				{Name: "candidate_repos", Type: "array", Required: false, Desc: "repos likely relevant to this failure"},
				{Name: "skip_investigation", Type: "bool", Required: false, Desc: "true for infra/flake cases that need no deep-dive"},
				{Name: "cascade_suspected", Type: "bool", Required: false, Desc: "true if failure may be caused by upstream cascade"},
			},
		},
		{
			Name: "resolve",
			Defs: []toolkit.FieldDef{
				{Name: "selected_repos", Type: "array", Required: true, Desc: "array of {name, branch, rationale} objects"},
			},
		},
		{
			Name: "investigate",
			Defs: []toolkit.FieldDef{
				{Name: "rca_message", Type: "string", Required: true, Desc: "root cause description"},
				{Name: "defect_type", Type: "string", Required: true, Desc: "e.g. pb001, au001, en001"},
				{Name: "component", Type: "string", Required: true, Desc: "affected component (e.g. linuxptp-daemon)"},
				{Name: "convergence_score", Type: "float", Required: false, Desc: "0.0-1.0 confidence in root cause"},
				{Name: "evidence_refs", Type: "array", Required: false, Desc: "repo:file:line citations supporting the RCA"},
				{Name: "gap_brief", Type: "object", Required: false, Desc: "evidence gaps when convergence is low"},
			},
		},
		{
			Name: "correlate",
			Defs: []toolkit.FieldDef{
				{Name: "is_duplicate", Type: "bool", Required: true, Desc: "true if same root cause as a prior case"},
				{Name: "prior_rca_id", Type: "int", Required: false, Desc: "matched prior RCA store ID"},
				{Name: "confidence", Type: "float", Required: true, Desc: "0.0-1.0 duplicate match confidence"},
				{Name: "reasoning", Type: "string", Required: false, Desc: "why this is or isn't a duplicate"},
			},
		},
		{
			Name: "review",
			Defs: []toolkit.FieldDef{
				{Name: "decision", Type: "string", Required: true, Desc: "approve, reassess, or overturn"},
				{Name: "reasoning", Type: "string", Required: false, Desc: "rationale for the decision"},
				{Name: "adjustments", Type: "object", Required: false, Desc: "field overrides if overturning"},
			},
		},
		{
			Name: "report",
			Defs: []toolkit.FieldDef{
				{Name: "defect_type", Type: "string", Required: true, Desc: "final defect type classification"},
				{Name: "component", Type: "string", Required: false, Desc: "affected component"},
				{Name: "summary", Type: "string", Required: true, Desc: "one-paragraph RCA summary"},
				{Name: "confidence", Type: "float", Required: false, Desc: "overall confidence in the RCA"},
				{Name: "jira_summary", Type: "string", Required: false, Desc: "suggested Jira ticket title"},
			},
		},
	}
}
