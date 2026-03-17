# F0 — Recall: Judge Similarity

**Case:** #{{.CaseID}}  
**Test:** {{.Failure.TestName}}  
**Step:** {{.StepName}}

---

## Task

Determine whether this failure has been seen before by comparing it against prior symptom and RCA data.

## Failure under investigation

**Test name:** `{{.Failure.TestName}}`  
{{if .Failure.ErrorMessage}}**Error message:**
```
{{.Failure.ErrorMessage}}
```
{{else}}**No error message available for this item.** Do NOT guess or fabricate error text.{{end}}
{{if .Failure.LogSnippet}}**Log snippet:**
```
{{.Failure.LogSnippet}}
```
{{if .Failure.LogTruncated}}**Warning: log was truncated. The actual error may not be visible.**{{end}}
{{end}}

{{if .History}}{{if .History.SymptomInfo}}## Known symptom

| Field | Value |
|-------|-------|
| Name | {{.History.SymptomInfo.Name}} |
| Status | {{.History.SymptomInfo.Status}} |
| Occurrences | {{.History.SymptomInfo.OccurrenceCount}} |
| First seen | {{.History.SymptomInfo.FirstSeen}} |
| Last seen | {{.History.SymptomInfo.LastSeen}} |

{{if .History.SymptomInfo.IsDormantReactivation}}**⚠ Dormant symptom reactivated — potential regression.**{{end}}
{{end}}

{{if .History.PriorRCAs}}## Prior RCAs linked to this symptom

{{range .History.PriorRCAs}}| Field | Value |
|-------|-------|
| RCA #{{.ID}} | {{.Title}} |
| Defect type | {{.DefectType}} |
| Status | {{.Status}} |
| Affected versions | {{.AffectedVersions}} |
{{if .JiraLink}}| Jira | {{.JiraLink}} |{{end}}
{{if .ResolvedAt}}| Resolved at | {{.ResolvedAt}} |{{end}}

{{end}}{{end}}{{end}}

{{if .RecallDigest}}## All known RCAs in this run

These RCAs were discovered from other cases in the current calibration run. If the current failure's error pattern matches any of these, set `match: true` with the matching RCA ID and high confidence.

| RCA ID | Component | Defect Type | Summary |
|--------|-----------|-------------|---------|
{{range .RecallDigest}}| #{{.ID}} | {{.Component}} | {{.DefectType}} | {{.Summary}} |
{{end}}
{{end}}

## Guards

{{if .Failure.LogTruncated}}- **G1 (truncated-log):** The log snippet ends abruptly. State that the log is incomplete and lower your confidence. Do NOT infer root cause from truncated output alone.{{end}}
{{if not .Failure.ErrorMessage}}- **G2 (missing-logs):** No error message is available. Classify as low-confidence recall. Do NOT guess or fabricate error text.{{end}}
- **G5 (stale-recall-match):** When judging similarity to a prior RCA, compare not only the error pattern but also the environment context (OCP version, operator version, cluster). A test can fail for different reasons in different versions. If the environment differs significantly, lower your match confidence.

## Instructions

1. Compare the current failure's error pattern against the known symptom and prior RCAs above.
2. Consider whether the **environment context** (versions, cluster) matches — same test can fail differently across versions.
3. If a prior RCA's symptom was marked as `dormant` or `resolved` and this failure matches, flag `is_regression: true`.
4. Produce the output JSON below.

## Output format

Save as `recall-result.json`:

```json
{
  "match": true,
  "prior_rca_id": 42,
  "symptom_id": 7,
  "confidence": 0.85,
  "reasoning": "Same error pattern as RCA #42: ...",
  "is_regression": false
}
```

- `match`: true if a prior RCA likely explains this failure.
- `prior_rca_id`: the RCA ID if matched, 0 otherwise.
- `symptom_id`: the symptom ID if matched, 0 otherwise.
- `confidence`: 0.0–1.0 (>= 0.8 = high-confidence hit; 0.4–0.8 = uncertain; < 0.4 = miss).
- `reasoning`: brief explanation of match or mismatch.
- `is_regression`: true if this is a known-resolved or dormant symptom reappearing.
