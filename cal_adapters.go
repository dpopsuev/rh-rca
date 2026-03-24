package rca

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/dpopsuev/origami/engine"
	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/bugle/billing"
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/rh-rca/store"
)

// Compile-time checks.
var (
	_ cal.ScenarioLoader          = (*RCACalibrationAdapter)(nil)
	_ cal.CaseCollector           = (*RCACalibrationAdapter)(nil)
	_ cal.ContractFieldsReceiver  = (*RCACalibrationAdapter)(nil)
	_ cal.ReportRenderer          = (*RCACalibrationAdapter)(nil)
)

// caseEntry pairs a ground truth case with its store entity and artifact directory.
type caseEntry struct {
	gtCase   GroundTruthCase
	caseData *store.Case
	caseDir  string
}

// RCACalibrationAdapter implements calibrate.ScenarioLoader,
// calibrate.CaseCollector, and calibrate.ReportRenderer for the RCA
// schematic. It bridges the generic calibration harness with RCA-specific
// store bootstrap, result collection, and report rendering.
//
// A single instance is used per calibration run. Load() populates internal
// state (store, entries), Collect() consumes BatchWalkResults and produces
// metric values, and Render() formats the final report.
type RCACalibrationAdapter struct {
	// Input configuration — set by caller before passing to calibrate.Run().
	Scenario        *Scenario
	Components      []*engine.Component
	IDMapper        IDMappable
	BasePath        string
	Thresholds      Thresholds
	ScoreCard       *cal.ScoreCard
	TokenTracker    billing.Tracker
	ReportTemplate  []byte // calibration-report.yaml data

	// Internal state — populated by Load(), consumed by Collect().
	entries []caseEntry
	st      store.Store
	suiteID int64

	// Contract-extracted fields — populated by harness via SetContractFields.
	contractFields []map[string]any

	// Output — populated by Collect(), readable by caller after Run().
	caseResults []CaseResult
	dataset     *DatasetHealth
}

// SetContractFields receives contract-extracted field maps from the harness.
func (a *RCACalibrationAdapter) SetContractFields(fields []map[string]any) {
	a.contractFields = fields
}

// CaseResults returns the per-case results collected during the last Collect() call.
func (a *RCACalibrationAdapter) CaseResults() []CaseResult { return a.caseResults }

// Dataset returns the dataset health summary.
func (a *RCACalibrationAdapter) Dataset() *DatasetHealth { return a.dataset }

// SuiteID returns the store suite ID from the last Load() call.
func (a *RCACalibrationAdapter) SuiteID() int64 { return a.suiteID }

// --- ScenarioLoader ---

// Load bootstraps an in-memory store, creates suite/version/circuit/launch/job/case
// entities, and returns BatchCases ready for engine.BatchWalk.
func (a *RCACalibrationAdapter) Load(_ context.Context) ([]engine.BatchCase, error) {
	if a.BasePath == "" {
		a.BasePath = DefaultBasePath
	}
	a.dataset = buildDatasetHealth(a.Scenario)

	st, err := store.OpenMemory()
	if err != nil {
		return nil, fmt.Errorf("open memory store: %w", err)
	}
	a.st = st

	suite := &store.InvestigationSuite{Name: a.Scenario.Name, Status: "active"}
	suiteID, err := st.CreateSuite(suite)
	if err != nil {
		return nil, fmt.Errorf("create suite: %w", err)
	}
	a.suiteID = suiteID

	versionMap := make(map[string]int64)
	for _, c := range a.Scenario.Cases {
		if _, exists := versionMap[c.Version]; !exists {
			v := &store.Version{Label: c.Version}
			vid, err := st.CreateVersion(v)
			if err != nil {
				return nil, fmt.Errorf("create version %s: %w", c.Version, err)
			}
			versionMap[c.Version] = vid
		}
	}

	circuitMap := make(map[pipeKey]int64)
	jobMap := make(map[pipeKey]int64)
	launchMap := make(map[pipeKey]int64)

	for _, c := range a.Scenario.Cases {
		pk := pipeKey{c.Version, c.Job}
		if _, exists := circuitMap[pk]; !exists {
			pipe := &store.Circuit{
				SuiteID: suiteID, VersionID: versionMap[c.Version],
				Name: fmt.Sprintf("CI %s %s", c.Version, c.Job), Status: "complete",
			}
			pipeID, err := st.CreateCircuit(pipe)
			if err != nil {
				return nil, fmt.Errorf("create circuit: %w", err)
			}
			circuitMap[pk] = pipeID

			launch := &store.Launch{
				CircuitID: pipeID, SourceRunID: "",
				Name: fmt.Sprintf("Launch %s %s", c.Version, c.Job), Status: "complete",
			}
			launchID, err := st.CreateLaunch(launch)
			if err != nil {
				return nil, fmt.Errorf("create launch: %w", err)
			}
			launchMap[pk] = launchID

			job := &store.Job{
				LaunchID: launchID,
				Name:     c.Job, Status: "complete",
			}
			jobID, err := st.CreateJob(job)
			if err != nil {
				return nil, fmt.Errorf("create job: %w", err)
			}
			jobMap[pk] = jobID
		}
	}

	catalog := ScenarioToCatalog(a.Scenario.SourcePack)
	a.entries = make([]caseEntry, len(a.Scenario.Cases))
	batchCases := make([]engine.BatchCase, len(a.Scenario.Cases))

	for i, gtCase := range a.Scenario.Cases {
		pk := pipeKey{gtCase.Version, gtCase.Job}
		caseData := &store.Case{
			JobID:        jobMap[pk],
			LaunchID:     launchMap[pk],
			SourceItemID: strconv.Itoa(i + 1),
			Name:         gtCase.TestName,
			Status:       "open",
			ErrorMessage: gtCase.ErrorMessage,
			LogSnippet:   gtCase.LogSnippet,
		}
		caseID, err := st.CreateCase(caseData)
		if err != nil {
			return nil, fmt.Errorf("create case %s: %w", gtCase.ID, err)
		}
		caseData.ID = caseID

		env := &rcatype.Envelope{
			Name:        caseData.Name,
			FailureList: []rcatype.FailureItem{{Name: caseData.Name}},
		}
		caseDir, _ := EnsureCaseDir(a.BasePath, suiteID, caseData.ID)

		storeComp := &engine.Component{
			Namespace: "store",
			Name:      "rca-store-hooks",
			Hooks:     StoreHooks(st, caseData),
		}
		injectComp := &engine.Component{
			Namespace: "inject",
			Name:      "rca-inject-hooks",
			Hooks: InjectHooksWithOpts(InjectHookOpts{
				Store:           st,
				CaseData:        caseData,
				Envelope:        env,
				Catalog:         catalog,
				CaseDir:         caseDir,
			}),
		}

		a.entries[i] = caseEntry{gtCase: gtCase, caseData: caseData, caseDir: caseDir}

		adapters := make([]*engine.Component, len(a.Components), len(a.Components)+2)
		copy(adapters, a.Components)
		adapters = append(adapters, storeComp, injectComp)

		batchCases[i] = engine.BatchCase{
			ID: gtCase.ID,
			Context: map[string]any{
				KeyCaseData:  caseData,
				KeyEnvelope:  env,
				KeyCaseDir:   caseDir,
				KeyCaseLabel: gtCase.ID,
			},
			Components: adapters,
		}
	}

	return batchCases, nil
}

// OnCaseComplete returns a callback for HarnessConfig.OnCaseComplete that
// updates ID maps for cross-case references (stub transformer).
func (a *RCACalibrationAdapter) OnCaseComplete() func(int, engine.BatchWalkResult) {
	if a.IDMapper == nil {
		return nil
	}
	var mu sync.Mutex
	return func(i int, _ engine.BatchWalkResult) {
		mu.Lock()
		defer mu.Unlock()
		updateIDMaps(a.IDMapper, a.st, a.entries[i].caseData, a.entries[i].gtCase, a.Scenario)
	}
}

// --- CaseCollector ---

// Collect processes BatchWalkResults into CaseResults, scores them against
// ground truth, enriches with token data, and returns metric values/details
// ready for ScoreCard.Evaluate().
func (a *RCACalibrationAdapter) Collect(_ context.Context, results []engine.BatchWalkResult) (map[string]float64, map[string]string, error) {
	logger := slog.Default().With("component", "calibrate")

	// Build a RunConfig for collectCaseResult compatibility.
	cfg := RunConfig{
		Scenario:  a.Scenario,
		BasePath:  a.BasePath,
		ScoreCard: a.ScoreCard,
	}

	caseResults := make([]CaseResult, len(a.entries))
	for i, br := range results {
		entry := a.entries[i]
		logger.Info("processed case",
			"case_id", entry.gtCase.ID, "index", i+1, "total", len(a.entries), "test", entry.gtCase.TestName)

		caseResults[i] = collectCaseResult(br, entry.gtCase, entry.caseData, entry.caseDir, a.suiteID, a.st, cfg)

		// Overlay contract-extracted fields when available. The contract
		// provides a generic extraction path for standard fields; the
		// store-based extraction in collectCaseResult handles
		// RCA-specific state (RCAID, store-persisted defect type, etc.).
		if i < len(a.contractFields) && a.contractFields[i] != nil {
			applyContractFields(&caseResults[i], a.contractFields[i])
		}
	}

	for i := range caseResults {
		scoreCaseResult(&caseResults[i], a.Scenario)
	}

	if a.TokenTracker != nil {
		ts := a.TokenTracker.Summary()
		for i := range caseResults {
			cid := caseResults[i].CaseID
			if cs, ok := ts.PerCase[cid]; ok {
				caseResults[i].PromptTokensTotal = cs.PromptTokens
				caseResults[i].ArtifactTokensTotal = cs.ArtifactTokens
				caseResults[i].StepCount = cs.Steps
				caseResults[i].WallClockMs = cs.WallClockMs
			}
		}
	}

	a.caseResults = caseResults

	reg := cal.DefaultScorerRegistry()
	batchItems, batchCtx := PrepareBatchInput(caseResults, a.Scenario)
	values, details, err := a.ScoreCard.ScoreCase(batchItems, batchCtx, reg)
	if err != nil {
		values = make(map[string]float64)
		details = make(map[string]string)
	}

	return values, details, nil
}

// --- ReportRenderer ---

// Render produces the human-readable RCA calibration report.
func (a *RCACalibrationAdapter) Render(report *cal.CalibrationReport) (string, error) {
	rcaReport := &CalibrationReport{
		CalibrationReport: *report,
		SuiteID:           a.suiteID,
		BasePath:          a.BasePath,
		CaseResults:       a.caseResults,
		Dataset:           a.dataset,
	}
	return RenderCalibrationReport(rcaReport, a.ReportTemplate)
}

// RCAReport builds the full RCA-specific CalibrationReport from the generic
// report and adapter state. Callers use this for domain-specific post-processing
// (transcripts, cost bills, routing logs).
func (a *RCACalibrationAdapter) RCAReport(report *cal.CalibrationReport) *CalibrationReport {
	rcaReport := &CalibrationReport{
		CalibrationReport: *report,
		SuiteID:           a.suiteID,
		BasePath:          a.BasePath,
		CaseResults:       a.caseResults,
		Dataset:           a.dataset,
	}
	if a.TokenTracker != nil {
		ts := a.TokenTracker.Summary()
		rcaReport.Tokens = &ts
	}
	return rcaReport
}
