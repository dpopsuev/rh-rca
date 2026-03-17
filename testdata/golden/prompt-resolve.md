# F2 — Resolve: Select Repos and Scope

**Case:** #7  
**Launch:** ptp-ci-nightly (run-123)  
**Step:** resolve

---

## Task

Given the triage classification and the available repos, select which repo(s) to investigate and narrow the focus to specific paths/modules.

## Triage result (from F1)

| Field | Value |
|-------|-------|
| Symptom category | product |
| Severity | high |
| Defect type hypothesis | pb001 |
| Candidate repos | `ptp-operator` `linuxptp-daemon` |
| Skip investigation | false |

| Clock skew suspected | true |


## Domain knowledge

### PTP Domain Knowledge — PTP protocol reference

PTP uses Best Master Clock Algorithm (BMCA) to select the grandmaster.


## Prior investigation (loop retry)

Previous investigation converged at **0.85** with defect type `pb001`:

> Holdover timeout changed from 300s to 60s in commit abc1234, causing premature clock class transition to 248.

The convergence was too low. Select a different repo or broader scope for the retry.


## Failure context

**Test name:** `[T-TSC] PTP Recovery after grandmaster clock switchover`  
**Error message:**
```
Expected clock class 6 but got 248 after 300s holdover timeout
```


## Git context

| Field | Value |
|-------|-------|
| Branch | release-4.21 |
| Commit | abc1234def5678 |


## Launch attributes

| Key | Value |
|-----|-------|
| ocp_version | 4.21.3 |
| cluster | lab-sno-01 |



## Linked Jira tickets

| Ticket | URL |
|--------|-----|
| OCPBUGS-12345 | https://issues.redhat.com/browse/OCPBUGS-12345 |



## Available repos

| Repo | Path | Purpose | Branch |
|------|------|---------|--------|
| ptp-operator | /repos/ptp-operator | PTP operator reconciliation | release-4.21 |
| linuxptp-daemon | /repos/linuxptp-daemon | PTP sync logic | main |
| cnf-gotests | /repos/cnf-gotests | test framework | release-4.21 |



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
