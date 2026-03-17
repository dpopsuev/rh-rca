package rca

import (
	"math"
	"strings"
	"testing"

	framework "github.com/dpopsuev/origami"

	"github.com/dpopsuev/rh-rca/store"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func newTestHeuristic(t *testing.T, repos []string) (*heuristicTransformer, store.Store) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return NewHeuristicTransformer(st, repos, readInternalTestdata(t, "heuristics.yaml")), st
}

// ---------------------------------------------------------------------------
// classifyDefect
// ---------------------------------------------------------------------------

func TestClassifyDefect(t *testing.T) {
	ht, _ := newTestHeuristic(t, nil)

	tests := []struct {
		name     string
		text     string
		wantCat  string
		wantHypo string
		wantSkip bool
	}{
		{
			name: "automation skip — automation: keyword",
			text: "automation: add version conditions for tests",
			wantCat: "automation", wantHypo: "au001", wantSkip: true,
		},
		{
			name: "automation skip — flip-flop keyword",
			text: "flip-flop between 6 and 248 values",
			wantCat: "automation", wantHypo: "au001", wantSkip: true,
		},
		{
			name: "environment skip — ordinary clock 2 port failure",
			text: "ordinary clock 2 port failure detected",
			wantCat: "environment", wantHypo: "en001", wantSkip: true,
		},
		{
			name: "environment — http events env skip",
			text: "http events using consumer /var/lib/jenkins some path",
			wantCat: "environment", wantHypo: "en001", wantSkip: true,
		},
		{
			name: "automation generic — test setup failed",
			text: "test setup failed during initialization",
			wantCat: "automation", wantHypo: "au001", wantSkip: true,
		},
		{
			name: "automation generic — ginkgo internal",
			text: "ginkgo internal error occurred",
			wantCat: "automation", wantHypo: "au001", wantSkip: true,
		},
		{
			name: "automation — bare events metrics path",
			text: "ptp_events_and_metrics.go",
			wantCat: "automation", wantHypo: "au001", wantSkip: true,
		},
		{
			name: "product — ptp keyword",
			text: "ptp daemon restarted unexpectedly",
			wantCat: "product", wantHypo: "pb001", wantSkip: false,
		},
		{
			name: "product — phc2sys keyword",
			text: "phc2sys process crashed with segfault",
			wantCat: "product", wantHypo: "pb001", wantSkip: false,
		},
		{
			name: "firmware — firmware keyword",
			text: "firmware update failed during flash",
			wantCat: "firmware", wantHypo: "fw001", wantSkip: false,
		},
		{
			name: "firmware — bios keyword",
			text: "bios settings misconfigured",
			wantCat: "firmware", wantHypo: "fw001", wantSkip: false,
		},
		{
			name: "infra — timeout keyword",
			text: "timeout waiting for node readiness",
			wantCat: "infra", wantHypo: "ti001", wantSkip: true,
		},
		{
			name: "infra — connection refused",
			text: "connection refused to api server",
			wantCat: "infra", wantHypo: "ti001", wantSkip: true,
		},
		{
			name: "product — /var/lib/jenkins fallback",
			text: "/var/lib/jenkins build log content",
			wantCat: "product", wantHypo: "pb001", wantSkip: false,
		},
		{
			name: "product — ocpbugs- fallback",
			text: "ocpbugs-12345 regression",
			wantCat: "product", wantHypo: "pb001", wantSkip: false,
		},
		{
			name: "product — cnf- fallback",
			text: "cnf-500 test failure",
			wantCat: "product", wantHypo: "pb001", wantSkip: false,
		},
		{
			name: "default — unrecognized text",
			text: "some completely unrelated error message",
			wantCat: "product", wantHypo: "pb001", wantSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, hypo, skip := ht.classifyDefect(tt.text)
			if cat != tt.wantCat {
				t.Errorf("category = %q, want %q", cat, tt.wantCat)
			}
			if hypo != tt.wantHypo {
				t.Errorf("hypothesis = %q, want %q", hypo, tt.wantHypo)
			}
			if skip != tt.wantSkip {
				t.Errorf("skip = %v, want %v", skip, tt.wantSkip)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// identifyComponent
// ---------------------------------------------------------------------------

func TestIdentifyComponent(t *testing.T) {
	ht, _ := newTestHeuristic(t, nil)

	tests := []struct {
		name string
		text string
		want string
	}{
		{"cnf-features-deploy — losing subscription", "losing subscription to events in namespace", "cnf-features-deploy"},
		{"cnf-features-deploy — remove phc2sys + option", "remove phc2sys option from config", "cnf-features-deploy"},
		{"cnf-features-deploy — ocpbugs-49372", "ocpbugs-49372 causes regression", "cnf-features-deploy"},
		{"cnf-features-deploy — ocpbugs-49373", "fix for ocpbugs-49373", "cnf-features-deploy"},

		{"cnf-gotests — ntpfailover-specific tests", "ntpfailover-specific tests failing", "cnf-gotests"},
		{"cnf-gotests — tracking issue for failures", "tracking issue for failures list", "cnf-gotests"},

		{"cloud-event-proxy — cloud event", "cloud event delivery failed", "cloud-event-proxy"},
		{"cloud-event-proxy — cloud-event-proxy", "cloud-event-proxy pod restart", "cloud-event-proxy"},
		{"cloud-event-proxy — gnss sync state", "gnss sync state not available", "cloud-event-proxy"},
		{"cloud-event-proxy — configmap update", "configmap update triggered reconcile", "cloud-event-proxy"},
		{"cloud-event-proxy — sidecar container", "sidecar container not ready", "cloud-event-proxy"},

		{"cloud-event-proxy — interface down (no ordinary clock)", "interface down on ens3", "cloud-event-proxy"},
		{"linuxptp-daemon — interface down + ptp_interfaces.go", "interface down in ptp_interfaces.go handler", "linuxptp-daemon"},
		{"ordinary clock — skips interface down, hits PTP generic", "interface down ordinary clock 2 port", "linuxptp-daemon"},

		{"linuxptp-daemon — http events no /var/lib no ptp_events", "http events using consumer on node", "linuxptp-daemon"},
		{"cloud-event-proxy — http events no phc2sys no ptp4l", "http events using consumer /var/lib output", "cloud-event-proxy"},

		{"linuxptp-daemon — phc2sys", "phc2sys offset too large", "linuxptp-daemon"},
		{"linuxptp-daemon — ptp4l", "ptp4l config error", "linuxptp-daemon"},
		{"linuxptp-daemon — clock state locked", "clock state not locked", "linuxptp-daemon"},
		{"linuxptp-daemon — offset threshold", "offset threshold exceeded", "linuxptp-daemon"},
		{"linuxptp-daemon — ptp_recovery.go", "error in ptp_recovery.go:42", "linuxptp-daemon"},
		{"linuxptp-daemon — ptp_events_and_metrics.go", "failure in ptp_events_and_metrics.go handler", "linuxptp-daemon"},
		{"linuxptp-daemon — ptp_interfaces.go standalone", "error in ptp_interfaces.go", "linuxptp-daemon"},

		{"linuxptp-daemon — workload partitioning", "workload partitioning error", "linuxptp-daemon"},
		{"cloud-event-proxy — workload partitioning + workload_partitioning.go", "workload partitioning workload_partitioning.go fail", "cloud-event-proxy"},

		{"linuxptp-daemon — ocpbugs-54967", "ocpbugs-54967 fix needed", "linuxptp-daemon"},

		{"linuxptp-daemon — ptp keyword", "ptp daemon not running", "linuxptp-daemon"},
		{"linuxptp-daemon — gnss keyword", "gnss receiver disconnected", "linuxptp-daemon"},
		{"linuxptp-daemon — offset keyword", "offset drift detected", "linuxptp-daemon"},

		{"unknown — no matching keywords", "random unrelated failure", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ht.identifyComponent(tt.text)
			if got != tt.want {
				t.Errorf("identifyComponent(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// computeConvergence
// ---------------------------------------------------------------------------

func TestComputeConvergence(t *testing.T) {
	ht, _ := newTestHeuristic(t, nil)

	tests := []struct {
		name      string
		text      string
		component string
		want      float64
	}{
		{"unknown component — base 0.70", "any text", "unknown", 0.70},
		{"known component no keywords — 0.70", "clean text with no heuristic keywords", "linuxptp-daemon", 0.70},
		{"jira keyword — +0.10", "ocpbugs- reference found", "linuxptp-daemon", 0.80},
		{"one descriptive keyword — +0.05", "phc2sys mentioned once", "linuxptp-daemon", 0.75},
		{"two descriptive keywords — +0.10", "phc2sys and ptp4l both present", "linuxptp-daemon", 0.80},
		{"jira + two descriptive — capped at 0.90", "ocpbugs- reference with phc2sys and ptp4l", "linuxptp-daemon", 0.90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ht.computeConvergence(tt.text, tt.component)
			if !approxEqual(got, tt.want) {
				t.Errorf("computeConvergence = %.4f, want %.4f", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildGapBrief
// ---------------------------------------------------------------------------

func TestBuildGapBrief(t *testing.T) {
	ht, _ := newTestHeuristic(t, nil)

	t.Run("short message produces log_depth gap", func(t *testing.T) {
		fp := failureInfo{errorMessage: "short"}
		brief := ht.buildGapBrief(fp, "short", "unknown", "pb001", 0.70)
		if brief == nil {
			t.Fatal("expected non-nil gap brief")
		}
		hasLogDepth := false
		for _, g := range brief.GapItems {
			if g.Category == GapLogDepth {
				hasLogDepth = true
			}
		}
		if !hasLogDepth {
			t.Error("expected log_depth gap for short message")
		}
	})

	t.Run("no jira reference produces jira_context gap", func(t *testing.T) {
		fp := failureInfo{errorMessage: strings.Repeat("x", 200)}
		brief := ht.buildGapBrief(fp, strings.Repeat("x", 200), "linuxptp-daemon", "pb001", 0.70)
		if brief == nil {
			t.Fatal("expected non-nil gap brief")
		}
		hasJira := false
		for _, g := range brief.GapItems {
			if g.Category == GapJiraContext {
				hasJira = true
			}
		}
		if !hasJira {
			t.Error("expected jira_context gap when no Jira ID present")
		}
	})

	t.Run("unknown component produces source_code gap", func(t *testing.T) {
		fp := failureInfo{errorMessage: strings.Repeat("x", 200)}
		brief := ht.buildGapBrief(fp, strings.Repeat("x", 200), "unknown", "pb001", 0.70)
		if brief == nil {
			t.Fatal("expected non-nil gap brief")
		}
		hasSrc := false
		for _, g := range brief.GapItems {
			if g.Category == GapSourceCode {
				hasSrc = true
			}
		}
		if !hasSrc {
			t.Error("expected source_code gap for unknown component")
		}
	})

	t.Run("no version info produces version_info gap", func(t *testing.T) {
		fp := failureInfo{errorMessage: strings.Repeat("x", 200)}
		brief := ht.buildGapBrief(fp, strings.Repeat("x", 200), "linuxptp-daemon", "pb001", 0.70)
		if brief == nil {
			t.Fatal("expected non-nil gap brief")
		}
		hasVer := false
		for _, g := range brief.GapItems {
			if g.Category == GapVersionInfo {
				hasVer = true
			}
		}
		if !hasVer {
			t.Error("expected version_info gap when no version keywords present")
		}
	})

	t.Run("with jira reference — no jira_context gap", func(t *testing.T) {
		text := strings.Repeat("x", 200) + " ocpbugs-12345"
		fp := failureInfo{errorMessage: text}
		brief := ht.buildGapBrief(fp, text, "linuxptp-daemon", "pb001", 0.70)
		if brief == nil {
			t.Fatal("expected non-nil gap brief")
		}
		for _, g := range brief.GapItems {
			if g.Category == GapJiraContext {
				t.Error("should not have jira_context gap when Jira ID present")
			}
		}
	})
}

// ---------------------------------------------------------------------------
// extractEvidenceRefs
// ---------------------------------------------------------------------------

func TestExtractEvidenceRefs(t *testing.T) {
	t.Run("component ref and jira IDs", func(t *testing.T) {
		refs := extractEvidenceRefs("OCPBUGS-12345 regression in CNF-500", "linuxptp-daemon")
		if len(refs) != 3 {
			t.Fatalf("expected 3 refs, got %d: %v", len(refs), refs)
		}
		if refs[0] != "linuxptp-daemon:relevant_source_file" {
			t.Errorf("first ref = %q, want component ref", refs[0])
		}
	})

	t.Run("unknown component — no component ref", func(t *testing.T) {
		refs := extractEvidenceRefs("OCPBUGS-999", "unknown")
		if len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %d: %v", len(refs), refs)
		}
		if refs[0] != "OCPBUGS-999" {
			t.Errorf("ref = %q, want OCPBUGS-999", refs[0])
		}
	})

	t.Run("deduplicates jira IDs", func(t *testing.T) {
		refs := extractEvidenceRefs("OCPBUGS-123 and OCPBUGS-123 again", "")
		if len(refs) != 1 {
			t.Errorf("expected dedup to 1 ref, got %d", len(refs))
		}
	})

	t.Run("no refs", func(t *testing.T) {
		refs := extractEvidenceRefs("no jira ids here", "unknown")
		if len(refs) != 0 {
			t.Errorf("expected 0 refs, got %d", len(refs))
		}
	})
}

// ---------------------------------------------------------------------------
// matchCount
// ---------------------------------------------------------------------------

func TestMatchCount(t *testing.T) {
	if got := matchCount("phc2sys and ptp4l present", []string{"phc2sys", "ptp4l", "gnss"}); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
	if got := matchCount("nothing here", []string{"a", "b"}); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// buildRecall — store interaction
// ---------------------------------------------------------------------------

func TestBuildRecall_NoMatch(t *testing.T) {
	ht, _ := newTestHeuristic(t, nil)
	fp := failureInfo{name: "test", errorMessage: "error"}
	result := ht.buildRecall(fp)
	if mapBool(result, "match") {
		t.Error("expected no match for empty store")
	}
	if mapFloat(result, "confidence") != 0.0 {
		t.Errorf("confidence = %f, want 0.0", mapFloat(result, "confidence"))
	}
}

func TestBuildRecall_SymptomHit_NoRCA(t *testing.T) {
	ht, st := newTestHeuristic(t, nil)
	fp := failureInfo{name: "test", errorMessage: "clock drift"}

	fingerprint := ComputeFingerprint(fp.name, fp.errorMessage, "")
	sym := &store.Symptom{Name: "test symptom", Fingerprint: fingerprint, OccurrenceCount: 3}
	if _, err := st.CreateSymptom(sym); err != nil {
		t.Fatal(err)
	}

	result := ht.buildRecall(fp)
	if !mapBool(result, "match") {
		t.Error("expected match")
	}
	if mapFloat(result, "confidence") != 0.60 {
		t.Errorf("confidence = %f, want 0.60", mapFloat(result, "confidence"))
	}
	if mapFloat(result, "prior_rca_id") != 0 {
		t.Errorf("expected no prior RCA, got %v", result["prior_rca_id"])
	}
}

func TestBuildRecall_SymptomHit_WithRCA(t *testing.T) {
	ht, st := newTestHeuristic(t, nil)
	fp := failureInfo{name: "test", errorMessage: "clock drift"}

	fingerprint := ComputeFingerprint(fp.name, fp.errorMessage, "")
	sym := &store.Symptom{Name: "test symptom", Fingerprint: fingerprint, OccurrenceCount: 5}
	symID, err := st.CreateSymptom(sym)
	if err != nil {
		t.Fatal(err)
	}

	rcaRec := &store.RCA{Title: "root cause", DefectType: "pb001", Description: "desc"}
	rcaID, err := st.SaveRCA(rcaRec)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := st.LinkSymptomToRCA(&store.SymptomRCA{SymptomID: symID, RCAID: rcaID}); err != nil {
		t.Fatal(err)
	}

	result := ht.buildRecall(fp)
	if !mapBool(result, "match") {
		t.Error("expected match")
	}
	if mapFloat(result, "confidence") != 0.85 {
		t.Errorf("confidence = %f, want 0.85", mapFloat(result, "confidence"))
	}
	if mapInt64(result, "prior_rca_id") != rcaID {
		t.Errorf("prior_rca_id = %d, want %d", mapInt64(result, "prior_rca_id"), rcaID)
	}
}

// ---------------------------------------------------------------------------
// buildTriage
// ---------------------------------------------------------------------------

func TestBuildTriage(t *testing.T) {
	repos := []string{"linuxptp-daemon", "cloud-event-proxy"}
	ht, _ := newTestHeuristic(t, repos)

	t.Run("product with known component", func(t *testing.T) {
		fp := failureInfo{name: "phc2sys offset test", errorMessage: "phc2sys offset exceeded"}
		result := ht.buildTriage(fp)
		if mapStr(result, "symptom_category") != "product" {
			t.Errorf("category = %q, want product", mapStr(result, "symptom_category"))
		}
		if mapBool(result, "skip_investigation") {
			t.Error("product should not skip investigation")
		}
		repos := mapStrSlice(result, "candidate_repos")
		if len(repos) != 1 || repos[0] != "linuxptp-daemon" {
			t.Errorf("repos = %v, want [linuxptp-daemon]", repos)
		}
	})

	t.Run("unknown component uses all repos", func(t *testing.T) {
		fp := failureInfo{name: "random test", errorMessage: "random error"}
		result := ht.buildTriage(fp)
		repos := mapStrSlice(result, "candidate_repos")
		if len(repos) != 2 {
			t.Errorf("expected 2 repos, got %d", len(repos))
		}
	})

	t.Run("cascade suspected", func(t *testing.T) {
		fp := failureInfo{name: "test", errorMessage: "aftereach cleanup failed"}
		result := ht.buildTriage(fp)
		if !mapBool(result, "cascade_suspected") {
			t.Error("expected cascade for aftereach keyword")
		}
	})

	t.Run("severity always medium", func(t *testing.T) {
		fp := failureInfo{name: "test", errorMessage: "error"}
		result := ht.buildTriage(fp)
		if mapStr(result, "severity") != "medium" {
			t.Errorf("severity = %q, want medium", mapStr(result, "severity"))
		}
	})
}

// ---------------------------------------------------------------------------
// buildResolve
// ---------------------------------------------------------------------------

func TestBuildResolve(t *testing.T) {
	repos := []string{"repo-a", "repo-b"}
	ht, _ := newTestHeuristic(t, repos)

	t.Run("known component — single repo", func(t *testing.T) {
		fp := failureInfo{name: "phc2sys test", errorMessage: "phc2sys error"}
		result := ht.buildResolve(fp)
		repos := mapSlice(result, "selected_repos")
		if len(repos) != 1 {
			t.Fatalf("expected 1 repo, got %d", len(repos))
		}
		rm := repos[0].(map[string]any)
		if mapStr(rm, "name") != "linuxptp-daemon" {
			t.Errorf("repo = %q, want linuxptp-daemon", mapStr(rm, "name"))
		}
	})

	t.Run("unknown component — all repos", func(t *testing.T) {
		fp := failureInfo{name: "random", errorMessage: "nothing"}
		result := ht.buildResolve(fp)
		repos := mapSlice(result, "selected_repos")
		if len(repos) != 2 {
			t.Errorf("expected 2 repos, got %d", len(repos))
		}
	})
}

// ---------------------------------------------------------------------------
// buildInvestigate
// ---------------------------------------------------------------------------

func TestBuildInvestigate(t *testing.T) {
	ht, _ := newTestHeuristic(t, nil)

	t.Run("constructs rca message from parts", func(t *testing.T) {
		fp := failureInfo{name: "TestClock", errorMessage: "phc2sys offset exceeded"}
		result := ht.buildInvestigate(fp)
		if mapStr(result, "component") != "linuxptp-daemon" {
			t.Errorf("component = %q, want linuxptp-daemon", mapStr(result, "component"))
		}
		msg := mapStr(result, "rca_message")
		if !strings.Contains(msg, "phc2sys offset exceeded") {
			t.Error("rca message should contain error message")
		}
		if !strings.Contains(msg, "TestClock") {
			t.Error("rca message should contain test name")
		}
		if !strings.Contains(msg, "Suspected component") {
			t.Error("rca message should mention suspected component")
		}
	})

	t.Run("empty case — fallback message", func(t *testing.T) {
		fp := failureInfo{}
		result := ht.buildInvestigate(fp)
		if mapStr(result, "rca_message") != "investigation pending (no error message available)" {
			t.Errorf("unexpected fallback message: %q", mapStr(result, "rca_message"))
		}
	})

	t.Run("convergence score populated", func(t *testing.T) {
		fp := failureInfo{name: "test", errorMessage: "phc2sys ptp4l ocpbugs-123"}
		result := ht.buildInvestigate(fp)
		if mapFloat(result, "convergence_score") < 0.70 {
			t.Errorf("convergence = %f, expected >= 0.70", mapFloat(result, "convergence_score"))
		}
	})

	t.Run("evidence refs populated", func(t *testing.T) {
		fp := failureInfo{name: "test", errorMessage: "OCPBUGS-555 in phc2sys"}
		result := ht.buildInvestigate(fp)
		if len(mapStrSlice(result, "evidence_refs")) == 0 {
			t.Error("expected evidence refs")
		}
	})
}

// ---------------------------------------------------------------------------
// buildCorrelate — store interaction
// ---------------------------------------------------------------------------

func TestBuildCorrelate_EmptyStore(t *testing.T) {
	ht, _ := newTestHeuristic(t, nil)
	fp := failureInfo{errorMessage: "some error"}
	result := ht.buildCorrelate(fp)
	if mapBool(result, "is_duplicate") {
		t.Error("expected no duplicate for empty store")
	}
}

func TestBuildCorrelate_EmptyErrorMessage(t *testing.T) {
	ht, st := newTestHeuristic(t, nil)
	rca := &store.RCA{Title: "existing", Description: "existing rca desc"}
	if _, err := st.SaveRCA(rca); err != nil {
		t.Fatal(err)
	}
	fp := failureInfo{errorMessage: ""}
	result := ht.buildCorrelate(fp)
	if mapBool(result, "is_duplicate") {
		t.Error("expected no duplicate for empty error message")
	}
}

func TestBuildCorrelate_Match(t *testing.T) {
	ht, st := newTestHeuristic(t, nil)
	rca := &store.RCA{Title: "clock drift", Description: "phc2sys offset too large"}
	rcaID, err := st.SaveRCA(rca)
	if err != nil {
		t.Fatal(err)
	}
	fp := failureInfo{errorMessage: "phc2sys offset too large"}
	result := ht.buildCorrelate(fp)
	if !mapBool(result, "is_duplicate") {
		t.Error("expected duplicate match")
	}
	if mapInt64(result, "linked_rca_id") != rcaID {
		t.Errorf("linked_rca_id = %d, want %d", mapInt64(result, "linked_rca_id"), rcaID)
	}
	if mapFloat(result, "confidence") != 0.75 {
		t.Errorf("confidence = %f, want 0.75", mapFloat(result, "confidence"))
	}
}

func TestBuildCorrelate_NoMatch(t *testing.T) {
	ht, st := newTestHeuristic(t, nil)
	rca := &store.RCA{Title: "unrelated", Description: "something completely different"}
	if _, err := st.SaveRCA(rca); err != nil {
		t.Fatal(err)
	}
	fp := failureInfo{errorMessage: "phc2sys offset exceeded"}
	result := ht.buildCorrelate(fp)
	if mapBool(result, "is_duplicate") {
		t.Error("expected no match for unrelated RCA")
	}
}

// ---------------------------------------------------------------------------
// failureFromContext
// ---------------------------------------------------------------------------

func TestFailureFromContext_Nil(t *testing.T) {
	fp := failureFromContext(nil)
	if fp.name != "" || fp.errorMessage != "" {
		t.Error("expected empty failureInfo for nil walker state")
	}
}

func TestFailureFromContext_FailureParams(t *testing.T) {
	ws := &framework.WalkerState{Context: map[string]any{}}
	ws.Context[KeyParamsFailure] = &FailureParams{
		TestName:     "T1",
		ErrorMessage: "err",
		LogSnippet:   "log",
	}
	fp := failureFromContext(ws)
	if fp.name != "T1" || fp.errorMessage != "err" || fp.logSnippet != "log" {
		t.Errorf("got %+v", fp)
	}
}

func TestFailureFromContext_CaseData(t *testing.T) {
	ws := &framework.WalkerState{Context: map[string]any{}}
	ws.Context[KeyCaseData] = &store.Case{
		Name:         "T2",
		ErrorMessage: "err2",
		LogSnippet:   "log2",
	}
	fp := failureFromContext(ws)
	if fp.name != "T2" || fp.errorMessage != "err2" || fp.logSnippet != "log2" {
		t.Errorf("got %+v", fp)
	}
}
