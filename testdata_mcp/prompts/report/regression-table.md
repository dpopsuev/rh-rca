# F6 — Report: Generate Outputs

**Case:** #{{.CaseID}}  
{{if .SourceID}}**Launch:** {{.SourceID}}{{end}}  
**Step:** {{.StepName}}

---

## Task

Generate the final report artifacts: a Jira ticket draft and a regression summary table.

{{if .Prior}}{{if .Prior.Investigate}}## Approved RCA

| Field | Value |
|-------|-------|
| **RCA message** | {{.Prior.Investigate.rca_message}} |
| **Defect type** | `{{.Prior.Investigate.defect_type}}` |
| **Convergence** | {{.Prior.Investigate.convergence_score}} |

**Evidence:**
{{range .Prior.Investigate.evidence_refs}}- {{.}}
{{end}}
{{end}}

{{if .Prior.Correlate}}{{if .Prior.Correlate.cross_version_match}}### Cross-version impact

Affected versions: {{range .Prior.Correlate.affected_versions}}`{{.}}` {{end}}
{{end}}{{end}}{{end}}

## Failure context

**Test name:** `{{.Failure.TestName}}`  
{{if .Failure.ErrorMessage}}**Error:**
```
{{.Failure.ErrorMessage}}
```
{{end}}

{{.Taxonomy.DefectTypes}}

## Instructions

### 1. Jira ticket draft

Generate a Jira-ready ticket based on the defect type:

**For product bugs (`pb001`):**
- Summary: Clear, searchable title
- Description: Root cause, affected component, reproduction path
- Components: Affected components
- Priority: Based on severity
- Evidence: Links to logs, commits, code

**For other defect types:**
- Adjust the template (automation bugs target test repos, system issues target infra, etc.)

### 2. Regression summary table

Generate a markdown table summarizing all investigated cases:

```markdown
| Case | Test | Defect Type | RCA | Jira | Confidence |
|------|------|-------------|-----|------|------------|
| #N   | name | pb001       | ... | TBD  | 0.85       |
```

## Output format

Save as `jira-draft.json`:

```json
{
  "summary": "Brief Jira title",
  "description": "Full Jira description with root cause, evidence, and fix suggestion",
  "defect_type": "pb001",
  "priority": "High",
  "components": ["component-name"],
  "evidence_refs": ["path/to/evidence"],
  "affected_versions": ["4.21"]
}
```

Also save `regression-report.md` with the regression summary table.
