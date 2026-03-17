# F1 — Triage: Classify Symptoms

**Case:** #{{.CaseID}}  
{{if .Envelope}}**Launch:** {{.Envelope.Name}} ({{.Envelope.RunID}}){{end}}  
**Step:** {{.StepName}}

---

## Task

Classify the failure symptom from the error output and envelope metadata. No repo access needed — this is a surface-level assessment.

## Failure under investigation

**Test name:** `{{.Failure.TestName}}`  
**Status:** {{.Failure.Status}}

{{if .Failure.ErrorMessage}}**Error message:**
```
{{.Failure.ErrorMessage}}
```
{{else}}**No error message available for this item.** Classify as `unknown`. Do NOT guess or fabricate error text.{{end}}

{{if .Failure.LogSnippet}}**Log snippet:**
```
{{.Failure.LogSnippet}}
```
{{if .Failure.LogTruncated}}**Warning: log was truncated. The actual error may not be visible.** State that the log is incomplete and lower your confidence. Do NOT infer root cause from truncated output alone.{{end}}
{{end}}

{{if .Timestamps}}{{if .Timestamps.ClockPlaneNote}}**{{.Timestamps.ClockPlaneNote}}**{{end}}
{{if .Timestamps.ClockSkewWarning}}**Clock skew warning:** {{.Timestamps.ClockSkewWarning}}{{end}}
{{end}}

{{if .Siblings}}## Sibling failures in this launch

| ID | Name | Status |
|----|------|--------|
{{range .Siblings}}| {{.ID}} | {{.Name}} | {{.Status}} |
{{end}}
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

| Repo | Path | Purpose |
|------|------|---------|
{{range .Sources.Repos}}| {{.Name}} | {{.Path}} | {{.Purpose}} |
{{end}}
{{else}}*No workspace repos configured.*
{{end}}{{end}}

## Symptom categories

Classify by **root cause domain** — where does the bug live?

| Category | Meaning | Signal examples | Likely defect type |
|----------|---------|----------------|-------------------|
| `product` | Bug in the product under test (operator, daemon, proxy). Code logic error, wrong state machine transition, incorrect value mapping. | Assertion failures on SUT behavior ("Expected X got Y" on product state), panic/segfault in product code, incorrect sync state, wrong clock class, holdover re-entry timing | pb001 |
| `automation` | Bug in the test framework or test code itself. The product is correct but the test is wrong. | Test harness misconfiguration, wrong test assertion, test setup error, test timeout due to bad polling interval, test code referencing wrong resource | au001 |
| `infra` | Bug in the infrastructure, cluster, or CI environment. Neither product nor test code is at fault. | Node not ready, DNS failure, connection refused, resource quota exceeded, operator not installed, missing CRD, NTP/chrony unreachable, cluster state leftover from prior test | en001 |
| `flake` | Transient, non-reproducible failure. Product and test are both correct but timing or environment conditions caused a one-off failure. | Intermittent timeout, offset variance spike, Eventually timeout on edge-case timing, known unstable test, non-deterministic ordering | nd001 |
| `firmware` | Bug in firmware or hardware-adjacent code (NIC, FPGA, PHC). Not product-level software. | NIC firmware mismatch, FPGA register misconfiguration, PHC clock source error | fw001 |

**Decision guide:**
1. If the error traces to product source code (operator, daemon, proxy) -> `product`
2. If the error is in test assertions, test setup, or test fixtures -> `automation`
3. If the error is from infrastructure, cluster state, or CI environment -> `infra`
4. If the failure is intermittent and non-reproducible, with no clear code or infra fault -> `flake`
5. When uncertain, prefer `product` — in this domain, ~80% of verified bugs are product bugs.

**Key disambiguation — product vs automation:**
- If the error shows a **product behavior discrepancy** (e.g. timeout value changed from 300s to 60s, wrong state transition, incorrect clock class), classify as `product` even if the failure manifests as a test assertion ("Expected X got Y"). The product is doing the wrong thing; the test is correctly catching it.
- Reserve `automation` only for cases where the **test code itself** is wrong: missing cleanup (stale CRDs), wrong assertion target, test setup error, bad polling interval. The product behavior is correct but the test is broken.
- A holdover/sync timeout discrepancy (e.g. "expected 300s" vs "after 60s") is a product configuration change, not a test bug.

**Key disambiguation — infra vs flake:**
- `infra`: the failure has a clear, persistent infrastructure cause (NTP unreachable, node not ready, missing CRD). Re-running would likely fail again unless the infra is fixed.
- `flake`: the failure is transient and non-reproducible — a timing window was missed, a race condition in the environment, or variance caused a threshold violation. Re-running would likely pass. Use `flake` only when there is no persistent root cause.

{{.Taxonomy.DefectTypes}}

## Guards

- **G6 (beforesuite-cascade):** Check if multiple failures have identical or near-identical error messages, especially setup/teardown errors. If so, this is likely a **cascade from a shared setup failure** — classify the parent, not each child. Set `cascade_suspected: true`.
- **G7 (eventually-vs-timeout):** If the error contains "Timed out" from Gomega `Eventually` or `Consistently`, classify as `assertion` (expected state was never reached), NOT as `timeout`. Look for "Expected ... to ..." or "polling every ..." patterns.
- **G8 (ordered-spec-poison):** If the failure was aborted due to a prior spec failure in the same ordered container, trace back to the **first failure** and classify that one instead.
- **G9 (skip-count-signal):** If skipped > 40% of total, comment on possible causes (feature gate, setup dependency, ordered container abort).
- **G11 (cascade-error-blindness):** Read the log **chronologically from earliest to latest**. Identify the **first anomaly or error** — this is the most likely root cause.
- **G13 (name-based-guessing):** Do NOT infer root cause from the test name alone. Trace from the **actual error**.
- **G26 (partial-step-conflation):** If this is a TEST-level item with STEP children, identify which specific STEPs failed.
- **Clock skew guard:** Before classifying as `timeout`, check for clock skew. A step that appears to take hours likely has timestamp misalignment, not an actual timeout.

## Instructions

1. Read the error message and log snippet.
2. Classify the symptom using the category table above.
3. Hypothesize a defect type from the taxonomy.
4. Rank candidate repos by relevance to the symptom (using repo purposes).
5. Determine whether repo investigation is needed (`skip_investigation`).
6. Check for cascade patterns, clock skew, and data quality issues.

## Output format

Save as `triage-result.json`:

```json
{
  "symptom_category": "product",
  "severity": "high",
  "defect_type_hypothesis": "pb001",
  "candidate_repos": ["ptp-operator", "cnf-gotests"],
  "skip_investigation": false,
  "clock_skew_suspected": false,
  "cascade_suspected": false,
  "data_quality_notes": ""
}
```
