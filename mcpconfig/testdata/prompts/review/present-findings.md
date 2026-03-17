# F5 — Review: Present Findings

**Case:** #{{.CaseID}}  
{{if .SourceID}}**Launch:** {{.SourceID}}{{end}}  
**Step:** {{.StepName}}

---

## Human Review Gate

This step presents the investigation findings for your review. **No write to RP happens until you approve.**

## Summary

**Test name:** `{{.Failure.TestName}}`

{{if .Prior}}{{if .Prior.Investigate}}### Investigation result

| Field | Value |
|-------|-------|
| **RCA message** | {{.Prior.Investigate.rca_message}} |
| **Defect type** | `{{.Prior.Investigate.defect_type}}` |
| **Convergence score** | {{.Prior.Investigate.convergence_score}} |

**Evidence:**
{{range .Prior.Investigate.evidence_refs}}- {{.}}
{{end}}
{{end}}

{{if .Prior.Recall}}{{if .Prior.Recall.match}}### Recall match

This case matched a prior RCA (#{{.Prior.Recall.prior_rca_id}}) with confidence {{.Prior.Recall.confidence}}.
{{if .Prior.Recall.is_regression}}**⚠ This appears to be a regression — a previously resolved or dormant symptom has reappeared.**{{end}}
{{end}}{{end}}

{{if .Prior.Triage}}### Triage classification

- Category: `{{.Prior.Triage.symptom_category}}`
- Defect hypothesis: `{{.Prior.Triage.defect_type_hypothesis}}`
{{if .Prior.Triage.clock_skew_suspected}}- **⚠ Clock skew suspected** — timestamps may be unreliable. Verify real vs apparent timing before accepting timeout classification.{{end}}
{{if .Prior.Triage.cascade_suspected}}- **⚠ Cascade suspected** — this may be a downstream effect of a shared setup failure.{{end}}
{{end}}

{{if .Prior.Correlate}}### Correlation result

{{if .Prior.Correlate.is_duplicate}}- **Duplicate** of RCA #{{.Prior.Correlate.linked_rca_id}} (confidence: {{.Prior.Correlate.confidence}})
{{if .Prior.Correlate.cross_version_match}}- Cross-version match across: {{range .Prior.Correlate.affected_versions}}`{{.}}` {{end}}{{end}}
{{else}}- Not a duplicate (confidence: {{.Prior.Correlate.confidence}})
- Reasoning: {{.Prior.Correlate.reasoning}}
{{end}}
{{end}}{{end}}

## Decision

Choose one of the following:

### ✅ Approve
The RCA is correct. Proceed to report generation (F6).

### 🔄 Reassess
The RCA needs rework. Specify where to loop back:
- `F1_TRIAGE` — wrong symptom classification
- `F2_RESOLVE` — wrong repo chosen
- `F3_INVESTIGATE` — missed something in the repo

### ❌ Overturn
The RCA is wrong. Provide the correct answer.

## Output format

Save as `review-decision.json`:

```json
{
  "decision": "approve",
  "human_override": null,
  "loop_target": ""
}
```

For reassess:
```json
{
  "decision": "reassess",
  "human_override": null,
  "loop_target": "F2_RESOLVE"
}
```

For overturn:
```json
{
  "decision": "overturn",
  "human_override": {
    "defect_type": "au001",
    "rca_message": "The actual root cause is..."
  },
  "loop_target": ""
}
```
