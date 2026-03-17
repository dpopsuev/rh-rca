# Evidence Gap Analysis Instructions

When your investigation confidence is below 0.80, you MUST include a `gap_brief` in your artifact output that articulates what evidence is missing and how it would change the outcome.

## Verdict classification

| Verdict | Condition | Action |
|---------|-----------|--------|
| `confident` | convergence >= 0.80 | No gap brief needed |
| `low-confidence` | 0.50 <= convergence < 0.80 | Gap brief recommended |
| `inconclusive` | convergence < 0.50 OR defect_type is "unknown" | Gap brief required |

## Gap categories

Use these categories to classify what evidence is missing:

| Category | When to use |
|----------|-------------|
| `log_depth` | Only error message available, no full logs or stack trace |
| `source_code` | Repo in workspace but no local path or no code access |
| `ci_context` | No CI circuit env vars, stage timing, or artifacts |
| `cluster_state` | No must-gather, cluster events, or node health data |
| `version_info` | Operator/OCP version not surfaced in prompts |
| `historical` | No cross-run data available for recurrence patterns |
| `jira_context` | Jira links present but not resolved |
| `human_input` | Multiple equally plausible root causes |

## Output format

Add `gap_brief` to your F3 artifact JSON when confidence < 0.80:

```json
{
  "launch_id": "...",
  "case_ids": [...],
  "rca_message": "...",
  "defect_type": "pb001",
  "component": "linuxptp-daemon",
  "convergence_score": 0.65,
  "evidence_refs": ["..."],
  "gap_brief": {
    "verdict": "low-confidence",
    "gap_items": [
      {
        "category": "log_depth",
        "description": "Only a short error message is available; no full logs or stack trace",
        "would_help": "Full pod logs from the failure window would show the actual error chain",
        "source": "CI job console log"
      },
      {
        "category": "jira_context",
        "description": "OCPBUGS-70233 is referenced but not resolved",
        "would_help": "Linked Jira ticket description would confirm or deny the hypothesis",
        "source": "Jira"
      }
    ]
  }
}
```

Each gap item must be actionable: state what evidence is missing, where to get it, and why it matters for the conclusion.
