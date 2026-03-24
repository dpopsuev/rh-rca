# F4 — Correlate: Match Cases

**Case:** #{{.CaseID}}  
{{if .SourceID}}**Launch:** {{.SourceID}}{{end}}  
**Step:** {{.StepName}}

---

## Task

Determine whether this case's root cause matches another case in the same launch, circuit, or suite. Detect "serial killers" (same root cause spanning multiple cases or versions).

{{if .Prior}}{{if .Prior.Investigate}}## Investigation result (from F3)

| Field | Value |
|-------|-------|
| RCA message | {{.Prior.Investigate.rca_message}} |
| Defect type | {{.Prior.Investigate.defect_type}} |
| Convergence | {{.Prior.Investigate.convergence_score}} |
| Evidence | {{range .Prior.Investigate.evidence_refs}}`{{.}}` {{end}} |
{{end}}{{end}}

{{if .History}}{{if .History.SymptomInfo}}## Symptom context

| Field | Value |
|-------|-------|
| Symptom | {{.History.SymptomInfo.Name}} |
| Status | {{.History.SymptomInfo.Status}} |
| Occurrences | {{.History.SymptomInfo.OccurrenceCount}} |
| First seen | {{.History.SymptomInfo.FirstSeen}} |
| Last seen | {{.History.SymptomInfo.LastSeen}} |
{{end}}

{{if .History.PriorRCAs}}## Prior RCAs for this symptom

{{range .History.PriorRCAs}}| RCA #{{.ID}} | {{.Title}} | `{{.DefectType}}` | {{.Status}} | Versions: {{.AffectedVersions}} |
{{end}}{{end}}{{end}}

{{if .Siblings}}## Sibling failures in this launch

| ID | Name | Status |
|----|------|--------|
{{range .Siblings}}| {{.ID}} | {{.Name}} | {{.Status}} |
{{end}}
{{end}}

## Guards

- **G23 (false-dedup):** Name similarity is not cause similarity. Before linking two cases to the same RCA, verify: (1) actual error messages match, (2) failure code path is the same, (3) environment context is comparable.
- **G24 (version-crossing-false-equiv):** Same test failing in different versions may have different root causes. Compare actual error details and environment.
- **G25 (shared-setup-misattribution):** If multiple cases share identical error messages pointing to setup, link them to **one RCA for the setup failure**.

## Instructions

1. Compare the current case's RCA against sibling failures and prior RCAs for this symptom.
2. Check if the **actual error messages** match (not just test names).
3. Check for cross-version patterns: same symptom across 4.20, 4.21, 4.22 with the same RCA = "serial killer".
4. If duplicate, specify the linked RCA ID.

## Output format

Save as `correlate-result.json`:

```json
{
  "is_duplicate": false,
  "linked_rca_id": 0,
  "confidence": 0.3,
  "reasoning": "Different error patterns despite similar test names.",
  "cross_version_match": false,
  "affected_versions": []
}
```
