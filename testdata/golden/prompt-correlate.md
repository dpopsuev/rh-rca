# F4 — Correlate: Match Cases

**Case:** #7  
**Launch:** launch-42  
**Step:** correlate

---

## Task

Determine whether this case's root cause matches another case in the same launch, circuit, or suite. Detect "serial killers" (same root cause spanning multiple cases or versions).

## Investigation result (from F3)

| Field | Value |
|-------|-------|
| RCA message | Holdover timeout changed from 300s to 60s in commit abc1234, causing premature clock class transition to 248. |
| Defect type | pb001 |
| Convergence | 0.85 |
| Evidence | `linuxptp-daemon:pkg/daemon/config.go:abc1234` `cnf-gotests:test/e2e/ptp_recovery_test.go:TestRecovery`  |


## Symptom context

| Field | Value |
|-------|-------|
| Symptom | PTP holdover timeout |
| Status | active |
| Occurrences | 5 |
| First seen | 2025-11-01 |
| Last seen | 2026-02-28 |


## Prior RCAs for this symptom

| RCA #42 | Holdover timeout regression | `pb001` | resolved | Versions: 4.20, 4.21 |


## Sibling failures in this launch

| ID | Name | Status |
|----|------|--------|
| 1 | [T-TSC] PTP Sync test | FAILED |
| 2 | [T-TSC] PTP Clock accuracy | PASSED |
| 3 | [T-TSC] PTP DPLL tracking | FAILED |



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
