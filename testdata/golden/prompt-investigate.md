# F3 — Investigate: Deep Root Cause Analysis

**Case:** #7  
**Launch:** launch-42  
**Step:** investigate

---

## Task

Perform deep root-cause analysis for the failed test by investigating the selected repo(s). Trace the error chain to the actual root cause with evidence.

Timestamps are in UTC. CI cluster uses chrony for NTP sync.

## Failure under investigation

**Test name:** `[T-TSC] PTP Recovery after grandmaster clock switchover`  
**Status:** FAILED

**Error message:**
```
Expected clock class 6 but got 248 after 300s holdover timeout
```


**Log snippet:**
```
level=error msg="holdover timeout exceeded" class=248 expected=6
ts2phc[123]: DPLL not locked
```
**Warning: log was truncated. The actual error may not be visible.** State that the log is incomplete and lower your confidence. Do NOT infer root cause from truncated output alone.


## Triage context (from F1)

- Symptom category: `product`
- Defect type hypothesis: `pb001`



## Repo selection (from F2)

### linuxptp-daemon
- **Path:** /repos/linuxptp-daemon
- **Focus paths:** `pkg/daemon/` `api/v1/` 
- **Branch:** main
- **Reason:** Triage indicates product bug in PTP sync daemon code

**Cross-reference strategy:** Check test assertion in cnf-gotests, then verify SUT behavior in linuxptp-daemon.


## Launch attributes

| Key | Value |
|-----|-------|
| ocp_version | 4.21.3 |
| cluster | lab-sno-01 |



## Linked Jira tickets

| Ticket | URL |
|--------|-----|
| OCPBUGS-12345 | https://issues.redhat.com/browse/OCPBUGS-12345 |



## Git context

**Branch:** release-4.21
**Commit:** abc1234def5678





## Defect type taxonomy

Defect types:
- ab001: Automation Bug
- au001: Automation Bug
- en001: Environment Issue
- fw001: Firmware Issue
- ib003: Infrastructure Bug
- nd001: No Defect
- pb001: Product Bug
- si001: System Issue
- ti001: To Investigate

## Guards

- **G1 (truncated-log):** If the log is incomplete, state it and lower confidence.
- **G2 (missing-logs):** If no error message, state that investigation requires log data.
- **G3 (ansi-noise):** Ignore formatting artifacts (`[31m`, `\x1b[0m`, `<br/>`, etc.).
- **G10 (parallel-interference):** Consider whether sibling tests sharing cluster resources could have caused the state change.
- **G11 (cascade-error-blindness):** Read the log **chronologically**. Identify the **first anomaly** — it's the likely root cause. Subsequent errors may be cascades.
- **G12 (recency-bias):** Recent commits are suspects, not convictions. Verify changed lines are in the failure's execution path. Check if the same test passed after that commit elsewhere.
- **G13 (name-based-guessing):** Do NOT infer root cause from the test name alone.
- **G14 (confirmation-bias):** After forming your hypothesis, actively look for **contradicting evidence**. List at least one reason your hypothesis could be wrong.
- **G15 (single-cause-assumption):** Consider whether the failure requires a **combination** of conditions.
- **G16 (phantom-code-blame):** Before blaming code, check: has this code changed since the last passing run? If not, the cause is likely environmental.
- **G17 (confidence-anchoring):** Calibrate convergence score: **0.9+** = exact line/commit + full causal chain; **0.7–0.9** = strong hypothesis with evidence; **0.5–0.7** = clear direction from error/log but no direct file access; **below 0.5** = speculative or contradictory evidence. When the error message and triage context clearly point to a component and defect type, score **0.6+** even without direct file evidence — the causal direction is established.
- **G19 (backport-lag):** If a related fix exists on main/newer branch, check if it's backported to the branch under test.
- **G21 (cluster-state-leftover):** If the error suggests unexpected initial state, consider dirty state from a previous test/job.
- **G22 (operator-version-tunnel-vision):** Don't blame version changes without connecting them to the failure path.
- **G27 (git-blame-wrong-file):** Verify the file you're blaming is in the failure's execution path.
- **G29 (hallucinated-evidence):** Every evidence ref MUST be real and verifiable. Do not fabricate commit SHAs, file paths, or log excerpts.
- **G30 (red-herring-refactor):** Distinguish behavioral changes from refactoring in recent commits.
- **G31 (missing-git-context):** If no branch/commit from envelope, state the uncertainty.
- **G32 (vague-rca-message):** RCA must be specific and actionable: name exact component/function/config, describe causal mechanism, state what would fix it. Include concrete values (e.g. "timeout changed from 300s to 60s"), function/method names (e.g. "AfterSuite"), and the component name (e.g. "linuxptp-daemon"). Generic phrases like "configuration issue" or "test failure" are insufficient.
- **G33 (wrong-defect-type-code):** Use ONLY codes from the taxonomy above. If none fit, use `ti001`.
- **G34 (evidence-without-reasoning):** For each evidence ref, explain **how** it supports the conclusion.

## Component frequency distribution (PTP Operator CI domain)

Use these base rates when choosing the root cause component. Do not override strong evidence, but when evidence is ambiguous, prefer the higher-frequency component.

| Component | Frequency | Notes |
|-----------|-----------|-------|
| `linuxptp-daemon` | ~78% (14/18 verified) | Dominant root cause. PTP sync logic, holdover state, clock class, DPLL tracking. |
| `cloud-event-proxy` | ~11% (2/18) | GNSS sync state mapping, cloud event publishing. |
| `ptp-operator` | ~6% (1/18) | Operator reconciliation, profile management. |
| Other (cnf-gotests, eco-gotests, WLP) | ~5% (1/18) | Test harness or specialized components. |

## Instructions

1. For each selected repo, investigate the focus paths.
2. Use `git log`, `git blame`, and code reading to trace the error to its root cause.
3. Read the log **chronologically** — identify the **first** anomaly.
4. After forming a hypothesis, actively look for contradicting evidence.
5. Calibrate your convergence score honestly based on evidence strength.
6. Produce the artifact JSON below.

## Output format

Save as `artifact.json`:

```json
{
  "launch_id": "launch-42",
  "case_ids": [7],
  "rca_message": "Specific root cause description: component X fails because Y changed in commit Z, causing W.",
  "defect_type": "pb001",
  "component": "linuxptp-daemon",
  "convergence_score": 0.85,
  "evidence_refs": [
    "linuxptp-daemon-operator:pkg/daemon/config.go:abc1234",
    "ptp-test-framework:test/e2e/ptp_config_test.go:AfterSuite"
  ]
}
```

### Evidence Gap Brief (when confidence < 0.80)

If your convergence_score is below 0.80, add a `gap_brief` field listing what evidence is missing and how it would change your conclusion. Categories: `log_depth`, `source_code`, `ci_context`, `cluster_state`, `version_info`, `historical`, `jira_context`, `human_input`. See `review/gap-analysis.md` for full details.

```json
{
  "gap_brief": {
    "verdict": "low-confidence",
    "gap_items": [
      {
        "category": "log_depth",
        "description": "Only short error message available",
        "would_help": "Full pod logs would reveal the actual error chain",
        "source": "CI job console log"
      }
    ]
  }
}
```

### Evidence ref format

Each evidence ref MUST follow the structured format: `<repo-name>:<file-path>:<identifier>`

- `repo-name`: the repository name from the workspace (e.g. `linuxptp-daemon-operator`)
- `file-path`: path within the repo to the relevant file (e.g. `pkg/daemon/config.go`)
- `identifier`: a commit SHA, function name, or keyword (e.g. `abc1234`, `AfterSuite`)

Good: `"linuxptp-daemon-operator:pkg/daemon/config.go:abc1234"`  
Good: `"ptp-test-framework:test/e2e/ptp_config_test.go:AfterSuite"`  
Bad: `"The holdover timeout was changed from 300s to 60s"` (free-form text, not structured)  
Bad: `"config changes"` (vague, missing repo and path)

**CRITICAL:** Every `evidence_refs` entry MUST start with one of the repo names from the workspace. If you cannot identify a specific file path, reference the most likely directory: `"<repo>:<dir>/<best-guess-file>:<function-or-keyword>"`.
