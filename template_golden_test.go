package rca

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "overwrite golden files with current output")

// goldenFixtureParams returns a TemplateParams with every field populated.
// Used by golden render tests and coverage tests.
func goldenFixtureParams() *TemplateParams {
	return &TemplateParams{
		SourceID: "launch-42",
		CaseID:   7,
		StepName: "", // set per-test
		Envelope: &EnvelopeParams{
			Name:   "ptp-ci-nightly",
			RunID:  "run-123",
			Status: "FAILED",
		},
		Env: map[string]string{
			"OCP_VERSION":      "4.21",
			"OPERATOR_VERSION": "4.21.0-rc1",
		},
		Git: &GitParams{
			Branch: "release-4.21",
			Commit: "abc1234def5678",
		},
		Failure: &FailureParams{
			TestName:     "[T-TSC] PTP Recovery after grandmaster clock switchover",
			ErrorMessage: "Expected clock class 6 but got 248 after 300s holdover timeout",
			LogSnippet:   "level=error msg=\"holdover timeout exceeded\" class=248 expected=6\nts2phc[123]: DPLL not locked",
			LogTruncated: true,
			Status:       "FAILED",
			Path:         "test/e2e/ptp_recovery_test.go",
		},
		Siblings: []SiblingParams{
			{ID: "1", Name: "[T-TSC] PTP Sync test", Status: "FAILED"},
			{ID: "2", Name: "[T-TSC] PTP Clock accuracy", Status: "PASSED"},
			{ID: "3", Name: "[T-TSC] PTP DPLL tracking", Status: "FAILED"},
		},
		Sources: &SourceParams{
			Repos: []RepoParams{
				{Name: "ptp-operator", Path: "/repos/ptp-operator", Purpose: "PTP operator reconciliation", Branch: "release-4.21"},
				{Name: "linuxptp-daemon", Path: "/repos/linuxptp-daemon", Purpose: "PTP sync logic", Branch: "main"},
				{Name: "cnf-gotests", Path: "/repos/cnf-gotests", Purpose: "test framework", Branch: "release-4.21"},
			},
			LaunchAttributes: []AttributeParams{
				{Key: "ocp_version", Value: "4.21.3", System: false},
				{Key: "cluster", Value: "lab-sno-01", System: false},
				{Key: "agent", Value: "internal", System: true},
			},
			JiraLinks: []JiraLinkParams{
				{TicketID: "OCPBUGS-12345", URL: "https://issues.redhat.com/browse/OCPBUGS-12345"},
			},
			AttrsStatus: Resolved,
			JiraStatus:  Resolved,
			ReposStatus: Resolved,
			AlwaysRead: []AlwaysReadSource{
				{Name: "PTP Domain Knowledge", Purpose: "PTP protocol reference", Content: "PTP uses Best Master Clock Algorithm (BMCA) to select the grandmaster."},
			},
		},
		URLs: &URLParams{
			SourceDashboard: "https://rp.example.com/launches/42",
			SourceItem:      "https://rp.example.com/items/7",
		},
		Prior: &PriorParams{
			"Recall": {
				"match":         true,
				"prior_rca_id":  float64(42),
				"symptom_id":    float64(7),
				"confidence":    0.85,
				"reasoning":     "Same holdover timeout pattern as RCA #42",
				"is_regression": true,
			},
			"Triage": {
				"symptom_category":       "product",
				"severity":               "high",
				"defect_type_hypothesis": "pb001",
				"candidate_repos":        []any{"ptp-operator", "linuxptp-daemon"},
				"skip_investigation":     false,
				"clock_skew_suspected":   true,
				"cascade_suspected":      false,
				"data_quality_notes":     "Log truncated at 4KB",
			},
			"Resolve": {
				"selected_repos": []any{
					map[string]any{
						"name":        "linuxptp-daemon",
						"path":        "/repos/linuxptp-daemon",
						"focus_paths": []any{"pkg/daemon/", "api/v1/"},
						"branch":      "main",
						"reason":      "Triage indicates product bug in PTP sync daemon code",
					},
				},
				"cross_ref_strategy": "Check test assertion in cnf-gotests, then verify SUT behavior in linuxptp-daemon.",
			},
			"Investigate": {
				"run_id":            "launch-42",
				"case_ids":          []any{"7"},
				"rca_message":       "Holdover timeout changed from 300s to 60s in commit abc1234, causing premature clock class transition to 248.",
				"defect_type":       "pb001",
				"component":         "linuxptp-daemon",
				"convergence_score": 0.85,
				"evidence_refs":     []any{"linuxptp-daemon:pkg/daemon/config.go:abc1234", "cnf-gotests:test/e2e/ptp_recovery_test.go:TestRecovery"},
				"gap_brief": map[string]any{
					"verdict": "low-confidence",
					"gap_items": []any{
						map[string]any{
							"category":    "log-depth",
							"description": "Log truncated at 4KB",
							"would_help":  "Full log would show complete error chain",
							"source":      "CI console",
						},
					},
				},
			},
			"Correlate": {
				"is_duplicate":        false,
				"linked_rca_id":       float64(0),
				"confidence":          0.3,
				"reasoning":           "Different error patterns despite similar test names",
				"cross_version_match": true,
				"affected_versions":   []any{"4.20", "4.21"},
			},
		},
		History: &HistoryParams{
			SymptomInfo: &SymptomInfoParams{
				Name:                  "PTP holdover timeout",
				OccurrenceCount:       5,
				FirstSeen:             "2025-11-01",
				LastSeen:              "2026-02-28",
				Status:                "active",
				IsDormantReactivation: true,
			},
			PriorRCAs: []PriorRCAParams{
				{
					ID:                42,
					Title:             "Holdover timeout regression",
					DefectType:        "pb001",
					Status:            "resolved",
					AffectedVersions:  "4.20, 4.21",
					JiraLink:          "OCPBUGS-12345",
					ResolvedAt:        "2026-01-15",
					DaysSinceResolved: 47,
				},
			},
		},
		RecallDigest: []RecallDigestEntry{
			{ID: 42, Component: "linuxptp-daemon", DefectType: "pb001", Summary: "Holdover timeout regression"},
			{ID: 38, Component: "cloud-event-proxy", DefectType: "pb001", Summary: "GNSS sync state mapping error"},
		},
		Taxonomy: DefaultTaxonomy(),
		Timestamps: &TimestampParams{
			ClockPlaneNote:   "Timestamps are in UTC. CI cluster uses chrony for NTP sync.",
			ClockSkewWarning: "Detected 2.3s clock skew between worker nodes.",
		},
	}
}

func TestPromptGolden(t *testing.T) {
	steps := []struct {
		family string
		prompt string
	}{
		{"recall", "prompts/recall/judge-similarity.md"},
		{"triage", "prompts/triage/classify-symptoms.md"},
		{"resolve", "prompts/resolve/select-repo.md"},
		{"investigate", "prompts/investigate/deep-rca.md"},
		{"correlate", "prompts/correlate/match-cases.md"},
		{"review", "prompts/review/present-findings.md"},
		{"report", "prompts/report/regression-table.md"},
	}

	for _, tt := range steps {
		t.Run(tt.family, func(t *testing.T) {
			params := goldenFixtureParams()
			params.StepName = tt.family

			templatePath := tt.prompt

			got, err := FillTemplateFS(testdataPromptFS(), templatePath, params)
			if err != nil {
				t.Fatalf("FillTemplateFS(%s): %v", templatePath, err)
			}

			goldenFile := filepath.Join("testdata", "golden", "prompt-"+tt.family+".md")

			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenFile), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenFile, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
				t.Logf("updated %s", goldenFile)
				return
			}

			want, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("read golden file (run with -update-golden to create): %v", err)
			}
			if got != string(want) {
				t.Errorf("output differs from golden file %s.\nRun with -update-golden to update.\n\nGot (first 500 chars):\n%s", goldenFile, truncate(got, 500))
			}
		})
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
