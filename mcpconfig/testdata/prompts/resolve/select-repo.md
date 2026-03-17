# F2 — Resolve: Select Repos and Scope

**Case:** #{{.CaseID}}  
{{if .Envelope}}**Launch:** {{.Envelope.Name}} ({{.Envelope.RunID}}){{end}}  
**Step:** {{.StepName}}

---

## Task

Given the triage classification and the available repos, select which repo(s) to investigate and narrow the focus to specific paths/modules.

{{if .Prior}}{{if .Prior.Triage}}## Triage result (from F1)

| Field | Value |
|-------|-------|
| Symptom category | {{.Prior.Triage.symptom_category}} |
| Severity | {{.Prior.Triage.severity}} |
| Defect type hypothesis | {{.Prior.Triage.defect_type_hypothesis}} |
| Candidate repos | {{range .Prior.Triage.candidate_repos}}`{{.}}` {{end}}|
| Skip investigation | {{.Prior.Triage.skip_investigation}} |
{{if .Prior.Triage.cascade_suspected}}| Cascade suspected | true |{{end}}
{{if .Prior.Triage.clock_skew_suspected}}| Clock skew suspected | true |{{end}}
{{end}}

{{if .Sources}}{{if .Sources.AlwaysRead}}## Domain knowledge
{{range .Sources.AlwaysRead}}
### {{.Name}}{{if .Purpose}} — {{.Purpose}}{{end}}

{{.Content}}
{{end}}{{end}}{{end}}

{{if .Prior.Investigate}}## Prior investigation (loop retry)

Previous investigation converged at **{{.Prior.Investigate.convergence_score}}** with defect type `{{.Prior.Investigate.defect_type}}`:

> {{.Prior.Investigate.rca_message}}

The convergence was too low. Select a different repo or broader scope for the retry.
{{end}}{{end}}

## Failure context

**Test name:** `{{.Failure.TestName}}`  
{{if .Failure.ErrorMessage}}**Error message:**
```
{{.Failure.ErrorMessage}}
```
{{end}}

{{if .Git}}## Git context

| Field | Value |
|-------|-------|
{{if .Git.Branch}}| Branch | {{.Git.Branch}} |{{end}}
{{if .Git.Commit}}| Commit | {{.Git.Commit}} |{{end}}
{{end}}

{{if .Sources}}{{if eq .Sources.AttrsStatus "resolved"}}## Launch attributes

| Key | Value |
|-----|-------|
{{range .Sources.LaunchAttributes}}{{if not .System}}| {{.Key}} | {{.Value}} |
{{end}}{{end}}
{{else}}*No launch attributes available.*
{{end}}

{{if eq .Sources.JiraStatus "resolved"}}## Linked Jira tickets

| Ticket | URL |
|--------|-----|
{{range .Sources.JiraLinks}}| {{.TicketID}} | {{.URL}} |
{{end}}
{{else}}*No linked Jira tickets.*
{{end}}

{{if eq .Sources.ReposStatus "resolved"}}## Available repos

| Repo | Path | Purpose | Branch |
|------|------|---------|--------|
{{range .Sources.Repos}}| {{.Name}} | {{.Path}} | {{.Purpose}} | {{.Branch}} |
{{end}}
{{else}}*No workspace repos configured.*
{{end}}{{end}}

## Guards

- **G4 (empty-envelope-fields):** If a field is unavailable or empty, do not assume a value. State what is missing and how it limits the analysis.
- **G18 (env-only-failure):** Consider whether the failure could be **environment-only** — code is correct but the runtime environment differs. If `Env.*` attributes show an unexpected version, include the CI config repo.
- **G28 (config-vs-code):** If the triage symptom is `config` or `infra`, prioritize the CI config repo over code repos.

## Instructions

1. Using the triage result and repo purposes, select the **single most relevant repo** for the root cause.
2. Only add a second repo if the error **clearly spans two components** (e.g. test code calls product API incorrectly — need both). In most cases, one repo is sufficient.
3. For each repo, specify focus paths (directories/files to look at) and why.
4. If multiple repos are needed, describe a cross-reference strategy.
5. If this is a loop retry, select a **different** repo or broader scope than the previous attempt.

**Repo selection by defect type:**

| Triage hypothesis | Preferred repo type | Reasoning |
|---|---|---|
| Product bug | Product / operator repo | The root cause lives in the product code, not in the test that revealed it. |
| Automation bug | Test / framework repo | The root cause is in test logic, assertions, or setup code. |
| Environment issue | CI config / infra repo | The root cause is in environment configuration. |

**CRITICAL:** Test frameworks contain assertions that **reveal** symptoms. When the hypothesis is a product bug, the test framework shows **what failed** but not **why** — the root cause is in the product repo where the buggy code lives. Use the `Purpose` column in the Available repos table to identify which repos contain product code vs test code.

**Precision over breadth:** Selecting too many repos dilutes investigation focus. A wrong repo wastes an investigation step. When in doubt, pick the single repo whose purpose most closely matches the triage hypothesis and defect type.

## Output format

Save as `resolve-result.json`:

```json
{
  "selected_repos": [
    {
      "name": "ptp-operator",
      "path": "/path/to/ptp-operator",
      "focus_paths": ["pkg/daemon/", "api/v1/"],
      "branch": "release-4.21",
      "reason": "Triage indicates product bug in PTP sync; daemon code is the likely location."
    }
  ],
  "cross_ref_strategy": "Check test assertion in cnf-gotests, then verify SUT behavior in ptp-operator."
}
```
