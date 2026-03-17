package rca

import (
	"math"
	"os"
	"testing"

	cal "github.com/dpopsuev/origami/calibrate"
)

func testScoreCard(t *testing.T) *cal.ScoreCard {
	t.Helper()
	path := "testdata/scorecard.yaml"
	if _, err := os.Stat(path); err != nil {
		t.Skip("scorecard YAML not found at", path)
	}
	sc, err := cal.LoadScoreCard(path)
	if err != nil {
		t.Fatalf("load scorecard: %v", err)
	}
	return sc
}

func runBatchScorer(t *testing.T, metricID string, results []CaseResult, scenario *Scenario) (float64, string) {
	t.Helper()
	sc := testScoreCard(t)
	reg := cal.DefaultScorerRegistry()
	batch, batchCtx := PrepareBatchInput(results, scenario)
	values, details, err := sc.ScoreCase(batch, batchCtx, reg)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	return values[metricID], details[metricID]
}

func buildFixtureScenario() *Scenario {
	return &Scenario{
		RCAs: []GroundTruthRCA{
			{
				ID: "R1", DefectType: "pb001", Component: "linuxptp-daemon",
				RequiredKeywords: []string{"ptp", "clock", "offset"},
				KeywordThreshold: 2, RelevantRepos: []string{"linuxptp-daemon"},
			},
			{
				ID: "R2", DefectType: "au001", Component: "cnf-gotests",
				RequiredKeywords: []string{"automation", "skip"},
				KeywordThreshold: 1, RelevantRepos: []string{"cnf-gotests"},
			},
		},
		Cases: []GroundTruthCase{
			{
				ID: "C1", RCAID: "R1",
				ExpectedTriage:  &ExpectedTriage{SymptomCategory: "product"},
				ExpectedInvest:  &ExpectedInvest{EvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
				ExpectedResolve: &ExpectedResolve{SelectedRepos: []ExpectedResolveRepo{{Name: "linuxptp-daemon"}}},
				ExpectedPath:    []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
				ExpectedLoops:   0,
			},
			{
				ID: "C2", RCAID: "R1", ExpectRecallHit: true,
				ExpectedTriage: &ExpectedTriage{SymptomCategory: "product"},
				ExpectedPath:   []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
				ExpectedLoops:  0,
			},
			{
				ID: "C3", RCAID: "R2", ExpectSkip: true,
				ExpectedTriage: &ExpectedTriage{SymptomCategory: "automation"},
				ExpectedPath:   []string{"F0", "F1"},
				ExpectedLoops:  0,
			},
			{
				ID: "C4", ExpectCascade: true,
				ExpectedTriage: &ExpectedTriage{SymptomCategory: "product"},
				ExpectedPath:   []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
				ExpectedLoops:  0,
			},
		},
	}
}

// --- Scorer tests via batch patterns ---

func TestScorerM1_DefectTypeAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all correct", []CaseResult{
			{CaseID: "C1", ActualDefectType: "pb001"},
			{CaseID: "C2", ActualDefectType: "pb001"},
			{CaseID: "C3", ActualDefectType: "au001"},
		}, 1.0},
		{"one wrong", []CaseResult{
			{CaseID: "C1", ActualDefectType: "pb001"},
			{CaseID: "C2", ActualDefectType: "wrong"},
			{CaseID: "C3", ActualDefectType: "au001"},
		}, 2.0 / 3.0},
		{"empty results", []CaseResult{}, 1.0},
		{"case without RCA ignored", []CaseResult{
			{CaseID: "C4", ActualDefectType: "pb001"},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M1", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM2_SymptomCategoryAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all correct", []CaseResult{
			{CaseID: "C1", ActualCategory: "product"},
			{CaseID: "C3", ActualCategory: "automation"},
		}, 1.0},
		{"one wrong", []CaseResult{
			{CaseID: "C1", ActualCategory: "wrong"},
			{CaseID: "C3", ActualCategory: "automation"},
		}, 0.5},
		{"no triage expected", []CaseResult{
			{CaseID: "C4", ActualCategory: "product"},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M2", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM3_RecallHitRate(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"hit detected", []CaseResult{{CaseID: "C2", ActualRecallHit: true}}, 1.0},
		{"hit missed", []CaseResult{{CaseID: "C2", ActualRecallHit: false}}, 0.0},
		{"no recall expected", []CaseResult{{CaseID: "C1", ActualRecallHit: true}}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M3", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM4_RecallFalsePositiveRate(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"no false positive", []CaseResult{{CaseID: "C1", ActualRecallHit: false}}, 0.0},
		{"false positive", []CaseResult{{CaseID: "C1", ActualRecallHit: true}}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M4", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM5_SerialKillerDetection(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"linked to same RCA", []CaseResult{
			{CaseID: "C1", ActualRCAID: 100},
			{CaseID: "C2", ActualRCAID: 100},
		}, 1.0},
		{"linked to different RCAs", []CaseResult{
			{CaseID: "C1", ActualRCAID: 100},
			{CaseID: "C2", ActualRCAID: 200},
		}, 0.0},
		{"single case per RCA", []CaseResult{
			{CaseID: "C1", ActualRCAID: 100},
			{CaseID: "C3", ActualRCAID: 200},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M5", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM6_SkipAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"skip detected", []CaseResult{{CaseID: "C3", ActualSkip: true}}, 1.0},
		{"skip missed", []CaseResult{{CaseID: "C3", ActualSkip: false}}, 0.0},
		{"no skip expected", []CaseResult{{CaseID: "C1", ActualSkip: true}}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M6", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM7_CascadeDetection(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"cascade detected", []CaseResult{{CaseID: "C4", ActualCascade: true}}, 1.0},
		{"cascade missed", []CaseResult{{CaseID: "C4", ActualCascade: false}}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M7", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM12_EvidenceRecall(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"evidence found", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
		}, 1.0},
		{"evidence not found", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"unrelated:file.go"}},
		}, 0.0},
		{"no evidence expected", []CaseResult{
			{CaseID: "C2", ActualEvidenceRefs: []string{"anything"}},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M12", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM13_EvidencePrecision(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all relevant", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
		}, 1.0},
		{"half relevant", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c", "irrelevant"}},
		}, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M13", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM14_RCAMessageRelevance(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all keywords", []CaseResult{
			{CaseID: "C1", ActualRCAMessage: "ptp clock offset is wrong"},
		}, 1.0},
		{"one keyword", []CaseResult{
			{CaseID: "C1", ActualRCAMessage: "ptp issue"},
		}, 0.5},
		{"no message", []CaseResult{
			{CaseID: "C1", ActualRCAMessage: ""},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M14", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM15_ComponentIdentification(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"exact match", []CaseResult{
			{CaseID: "C1", ActualComponent: "linuxptp-daemon"},
			{CaseID: "C3", ActualComponent: "cnf-gotests"},
		}, 1.0},
		{"keyword in message", []CaseResult{
			{CaseID: "C1", ActualComponent: "wrong", ActualRCAMessage: "issue in linuxptp-daemon"},
		}, 1.0},
		{"no match", []CaseResult{
			{CaseID: "C1", ActualComponent: "wrong", ActualRCAMessage: "no clue"},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M15", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM16_CircuitPathAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"correct path", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"}},
			{CaseID: "C3", ActualPath: []string{"F0", "F1"}},
		}, 1.0},
		{"wrong path", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1"}},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M16", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM17_LoopEfficiency(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"no loops expected or taken", []CaseResult{
			{CaseID: "C1", ActualLoops: 0},
		}, 1.0},
		{"expected loops matched", []CaseResult{
			{CaseID: "C1", ActualLoops: 0},
			{CaseID: "C2", ActualLoops: 0},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M17", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM18_TotalPromptTokens(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name      string
		results   []CaseResult
		wantValue float64
	}{
		{"stub mode estimate", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1", "F2"}},
		}, 3000},
		{"real tokens measured", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1"}, PromptTokensTotal: 5000},
		}, 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M18", tt.results, scenario)
			if math.Abs(val-tt.wantValue) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.wantValue)
			}
		})
	}
}

func TestScorerM9_RepoSelectionPrecision(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"perfect selection", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
		}, 1.0},
		{"extra irrelevant repo", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon", "red-herring"}},
		}, 0.5},
		{"no repos selected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: nil},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M9", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM10_RepoSelectionRecall(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all relevant selected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
		}, 1.0},
		{"relevant missing", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"wrong-repo"}},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M10", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM11_RedHerringRejection(t *testing.T) {
	scenario := buildFixtureScenario()
	scenario.SourcePack = SourcePackConfig{
		Repos: []RepoConfig{
			{Name: "linuxptp-daemon"},
			{Name: "red-herring", IsRedHerring: true},
		},
	}
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"red herring rejected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
		}, 1.0},
		{"red herring selected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"red-herring"}},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runBatchScorer(t, "M11", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

// --- ScoreCard YAML tests ---

func TestScoreCardEvaluate(t *testing.T) {
	sc := testScoreCard(t)
	tests := []struct {
		name  string
		id    string
		value float64
		want  bool
	}{
		{"M1 pass", "M1", 0.90, true},
		{"M1 fail", "M1", 0.70, false},
		{"M4 lower better pass", "M4", 0.05, true},
		{"M4 lower better fail", "M4", 0.15, false},
		{"M17 in range", "M17", 1.0, true},
		{"M17 too low", "M17", -0.1, false},
		{"M17 too high", "M17", 3.5, false},
		{"M18 budget pass", "M18", 50000, true},
		{"M18 budget fail", "M18", 250000, false},
		{"M20 variance pass", "M20", 0.10, true},
		{"M20 variance fail", "M20", 0.20, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := sc.FindDef(tt.id)
			if def == nil {
				t.Fatalf("metric %s not found in scorecard", tt.id)
			}
			got := def.Evaluate(tt.value)
			if got != tt.want {
				t.Errorf("MetricDef(%s).Evaluate(%f) = %v, want %v (threshold=%f, direction=%s)",
					tt.id, tt.value, got, tt.want, def.Threshold, def.Direction)
			}
		})
	}
}

func TestScoreOverallAccuracy_ViaScoreCard(t *testing.T) {
	sc := testScoreCard(t)
	ms := MetricSet{Metrics: []Metric{
		{ID: "M1", Value: 1.0}, {ID: "M2", Value: 1.0},
		{ID: "M3", Value: 1.0}, {ID: "M4", Value: 0.0},
		{ID: "M5", Value: 1.0}, {ID: "M6", Value: 1.0},
		{ID: "M7", Value: 1.0}, {ID: "M8", Value: 1.0},
		{ID: "M9", Value: 1.0}, {ID: "M10", Value: 1.0}, {ID: "M11", Value: 1.0},
		{ID: "M12", Value: 1.0}, {ID: "M13", Value: 1.0},
		{ID: "M14", Value: 1.0}, {ID: "M15", Value: 1.0},
		{ID: "M16", Value: 1.0}, {ID: "M17", Value: 1.0}, {ID: "M18", Value: 1000},
	}}

	agg, err := sc.ComputeAggregate(ms)
	if err != nil {
		t.Fatalf("ComputeAggregate: %v", err)
	}
	if agg.ID != "M19" {
		t.Errorf("expected ID=M19, got %s", agg.ID)
	}
	if math.Abs(agg.Value-1.0) > 1e-9 {
		t.Errorf("expected overall accuracy 1.0 when all metrics perfect, got %f", agg.Value)
	}
}

// --- aggregateRunMetrics ---

func TestAggregateRunMetrics(t *testing.T) {
	sc := testScoreCard(t)

	t.Run("empty", func(t *testing.T) {
		agg := AggregateRunMetrics(nil, sc)
		if len(agg.AllMetrics()) != 0 {
			t.Error("expected empty MetricSet")
		}
	})

	t.Run("single run passthrough", func(t *testing.T) {
		ms := MetricSet{Metrics: []Metric{
			{ID: "M1", Value: 0.9},
			{ID: "M19", Value: 0.85},
		}}
		agg := AggregateRunMetrics([]MetricSet{ms}, sc)
		if agg.Metrics[0].Value != 0.9 {
			t.Errorf("expected 0.9, got %f", agg.Metrics[0].Value)
		}
	})

	t.Run("two identical runs", func(t *testing.T) {
		ms := MetricSet{Metrics: []Metric{
			{ID: "M1", Value: 0.8, Threshold: 0.85},
			{ID: "M9", Value: 0.7, Threshold: 0.65},
			{ID: "M12", Value: 0.6, Threshold: 0.65},
			{ID: "M14", Value: 0.7, Threshold: 0.60},
			{ID: "M16", Value: 0.5, Threshold: 0.50},
			{ID: "M19", Value: 0.75, Threshold: 0.70},
			{ID: "M20", Value: 0, Threshold: 0.15},
		}}
		agg := AggregateRunMetrics([]MetricSet{ms, ms}, sc)
		if math.Abs(agg.Metrics[0].Value-0.8) > 1e-9 {
			t.Errorf("M1 mean = %f, want 0.8", agg.Metrics[0].Value)
		}
		for _, m := range agg.AllMetrics() {
			if m.ID == "M20" && m.Value != 0 {
				t.Errorf("M20 variance = %f, want 0", m.Value)
			}
		}
	})
}

// --- PrepareBatchInput ---

func TestPrepareBatchInput(t *testing.T) {
	scenario := buildFixtureScenario()
	results := []CaseResult{
		{CaseID: "C1", ActualDefectType: "pb001", ActualCategory: "product",
			ActualSelectedRepos: []string{"linuxptp-daemon"}, ActualConvergence: 0.85},
	}

	batch, batchCtx := PrepareBatchInput(results, scenario)
	if len(batch) != 1 {
		t.Fatalf("expected 1 batch item, got %d", len(batch))
	}

	item := batch[0]
	if item["actual_defect_type"] != "pb001" {
		t.Errorf("actual_defect_type = %v", item["actual_defect_type"])
	}
	if item["rca_defect_type"] != "pb001" {
		t.Errorf("rca_defect_type = %v (should be joined from RCA)", item["rca_defect_type"])
	}
	if item["rca_id"] != "R1" {
		t.Errorf("rca_id = %v", item["rca_id"])
	}
	if item["expected_symptom_category"] != "product" {
		t.Errorf("expected_symptom_category = %v", item["expected_symptom_category"])
	}
	if item["has_convergence"] != true {
		t.Errorf("has_convergence = %v", item["has_convergence"])
	}

	repos, _ := batchCtx["red_herring_repos"].([]string)
	if repos != nil && len(repos) > 0 {
		t.Errorf("expected no red herring repos, got %v", repos)
	}
}

// --- computeMetrics integration ---

func TestComputeMetrics_EmptyResults(t *testing.T) {
	sc := testScoreCard(t)
	scenario := &Scenario{
		RCAs:  []GroundTruthRCA{{ID: "R1", DefectType: "pb001"}},
		Cases: []GroundTruthCase{{ID: "C1", RCAID: "R1"}},
	}
	ms := computeMetrics(scenario, nil, sc)
	all := ms.AllMetrics()
	if len(all) < 21 {
		t.Errorf("expected at least 21 metrics, got %d", len(all))
	}
}

func TestComputeMetrics_IgnoresCandidates(t *testing.T) {
	sc := testScoreCard(t)
	scenario := &Scenario{
		RCAs: []GroundTruthRCA{
			{ID: "R1", DefectType: "pb001", Component: "daemon", Verified: true},
			{ID: "R2", DefectType: "au001", Component: "tests", Verified: false},
		},
		Cases: []GroundTruthCase{
			{ID: "C1", RCAID: "R1", ExpectedTriage: &ExpectedTriage{SymptomCategory: "product"},
				ExpectedPath: []string{"F0", "F1"}},
		},
		Candidates: []GroundTruthCase{
			{ID: "C2", RCAID: "R2", ExpectedTriage: &ExpectedTriage{SymptomCategory: "automation"},
				ExpectedPath: []string{"F0", "F1"}},
		},
	}

	results := []CaseResult{
		{CaseID: "C1", ActualDefectType: "pb001", ActualCategory: "product",
			ActualPath: []string{"F0", "F1"}},
	}

	ms := computeMetrics(scenario, results, sc)
	m1 := ms.ByID()["M1"]
	if m1.Detail != "1/1" {
		t.Errorf("M1 detail = %q; candidate case C2 should not be counted", m1.Detail)
	}
}

// --- ScoreCard YAML parse test ---

func TestLoadScoreCard_AsteriskRCA(t *testing.T) {
	sc := testScoreCard(t)

	if sc.Name != "rca" {
		t.Errorf("scorecard name = %q, want rca", sc.Name)
	}
	if len(sc.MetricDefs) != 22 {
		t.Errorf("expected 22 metric defs (M1-M18,M14b,M20,M21,M22), got %d", len(sc.MetricDefs))
	}
	if sc.Aggregate == nil {
		t.Fatal("expected aggregate config")
	}
	if sc.Aggregate.ID != "M19" {
		t.Errorf("aggregate id = %q, want M19", sc.Aggregate.ID)
	}
	if sc.CostModel == nil {
		t.Fatal("expected cost model")
	}

	for _, id := range []string{"M1", "M4", "M14b", "M17", "M18", "M20", "M21", "M22"} {
		if sc.FindDef(id) == nil {
			t.Errorf("missing metric def for %s", id)
		}
	}

	for _, def := range sc.MetricDefs {
		if def.ID != "M20" && def.Scorer == "" {
			t.Errorf("metric %s missing scorer declaration", def.ID)
		}
	}
}

// --- buildDatasetHealth ---

func TestBuildDatasetHealth(t *testing.T) {
	scenario := &Scenario{
		RCAs: []GroundTruthRCA{
			{ID: "R1", DefectType: "pb001", Verified: true, JiraID: "BUG-1", FixPRs: []string{"repo#1"}},
			{ID: "R2", DefectType: "au001", Verified: false, JiraID: "BUG-2"},
			{ID: "R3", DefectType: "pb001", Verified: false, JiraID: "BUG-3", FixPRs: []string{"repo#3"}},
		},
		Cases: []GroundTruthCase{
			{ID: "C1", RCAID: "R1"},
		},
		Candidates: []GroundTruthCase{
			{ID: "C2", RCAID: "R2"},
			{ID: "C3", RCAID: "R3"},
		},
	}

	dh := buildDatasetHealth(scenario)
	if dh.VerifiedCount != 1 {
		t.Errorf("verified_count = %d, want 1", dh.VerifiedCount)
	}
	if dh.CandidateCount != 2 {
		t.Errorf("candidate_count = %d, want 2", dh.CandidateCount)
	}
	if len(dh.Candidates) != 2 {
		t.Fatalf("candidates length = %d, want 2", len(dh.Candidates))
	}
}
