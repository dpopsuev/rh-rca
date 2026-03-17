# F0 — Recall: Judge Similarity

**Case:** #7  
**Test:** [T-TSC] PTP Recovery after grandmaster clock switchover  
**Step:** recall

---

## Task

Determine whether this failure has been seen before by comparing it against prior symptom and RCA data.

## Failure under investigation

**Test name:** `[T-TSC] PTP Recovery after grandmaster clock switchover`  
**Error message:**
```
Expected clock class 6 but got 248 after 300s holdover timeout
```

**Log snippet:**
```
level=error msg="holdover timeout exceeded" class=248 expected=6
ts2phc[123]: DPLL not locked
```
**Warning: log was truncated. The actual error may not be visible.**


## Known symptom

| Field | Value |
|-------|-------|
| Name | PTP holdover timeout |
| Status | active |
| Occurrences | 5 |
| First seen | 2025-11-01 |
| Last seen | 2026-02-28 |

**⚠ Dormant symptom reactivated — potential regression.**


## Prior RCAs linked to this symptom

| Field | Value |
|-------|-------|
| RCA #42 | Holdover timeout regression |
| Defect type | pb001 |
| Status | resolved |
| Affected versions | 4.20, 4.21 |
| Jira | OCPBUGS-12345 |
| Resolved at | 2026-01-15 |



## All known RCAs in this run

These RCAs were discovered from other cases in the current calibration run. If the current failure's error pattern matches any of these, set `match: true` with the matching RCA ID and high confidence.

| RCA ID | Component | Defect Type | Summary |
|--------|-----------|-------------|---------|
| #42 | linuxptp-daemon | pb001 | Holdover timeout regression |
| #38 | cloud-event-proxy | pb001 | GNSS sync state mapping error |



## Guards

- **G1 (truncated-log):** The log snippet ends abruptly. State that the log is incomplete and lower your confidence. Do NOT infer root cause from truncated output alone.

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
