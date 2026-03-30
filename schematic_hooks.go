package rca

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/engine"
)

// rcaSessionFactory implements engine.SessionFactory (and the optional
// ReportFormatter / StepSchemaProvider interfaces) for the RCA domain.
type rcaSessionFactory struct{}

// CreateSession implements engine.SessionFactory.
func (f *rcaSessionFactory) CreateSession(ctx context.Context, params *engine.SessionParams) (*engine.SessionConfig, error) {
	return createSession(ctx, params)
}

// FormatReport implements engine.ReportFormatter.
func (f *rcaSessionFactory) FormatReport(result any) (string, any, error) {
	report, ok := result.(*CalibrationReport)
	if !ok {
		return "", nil, nil
	}
	formatted, err := RenderCalibrationReport(report, nil)
	return formatted, report, err
}

// StepSchemas implements engine.StepSchemaProvider.
func (f *rcaSessionFactory) StepSchemas() []engine.StepSchema {
	return RCAStepSchemas()
}

// Factory returns the SessionFactory that fold-generated code calls.
func Factory() engine.SessionFactory {
	return &rcaSessionFactory{}
}

// createSession wires scenario loading, transformer selection, and
// calibration scoring into a SessionConfig with a custom RunFunc.
func createSession(_ context.Context, params *engine.SessionParams) (*engine.SessionConfig, error) {
	// --- Parse domain params from Extra ---
	scenarioName, _ := params.Extra["scenario"].(string)
	if scenarioName == "" {
		scenarioName = "ptp"
	}
	backend, _ := params.Extra["backend"].(string)
	if backend == "" {
		backend = backendLLM
	}
	mode, _ := params.Extra["mode"].(string)
	if mode == "" {
		mode = string(ModeOffline)
	}

	// --- Load scenario from domain FS ---
	if params.DomainFS == nil {
		return nil, fmt.Errorf("DomainFS is nil — fold-generated code must set CircuitConfig.DomainFS")
	}

	scenarioFS, err := fs.Sub(params.DomainFS, "scenarios")
	if err != nil {
		return nil, fmt.Errorf("sub-fs scenarios/: %w", err)
	}
	scenario, err := LoadScenario(scenarioFS, scenarioName)
	if err != nil {
		return nil, fmt.Errorf("load scenario %q: %w", scenarioName, err)
	}

	// --- Resolve RP failure data ---
	if mode == string(ModeOffline) {
		offlineFS, err := fs.Sub(params.DomainFS, "offline")
		if err == nil {
			if resolveErr := ResolveOfflineRP(offlineFS, scenario); resolveErr != nil {
				return nil, fmt.Errorf("resolve offline RP: %w", resolveErr)
			}
		}
	}

	// --- Load circuit definition ---
	circuitData, err := fs.ReadFile(params.DomainFS, "circuits/rca.yaml")
	if err != nil {
		return nil, fmt.Errorf("read circuit YAML: %w", err)
	}
	thresholds := DefaultThresholds()
	circuitDef, err := LoadCircuitDef(circuitData, thresholds)
	if err != nil {
		return nil, fmt.Errorf("load circuit def: %w", err)
	}

	// --- Load scorecard ---
	scorecardData, err := fs.ReadFile(params.DomainFS, "scorecards/rca.yaml")
	if err != nil {
		return nil, fmt.Errorf("read scorecard: %w", err)
	}
	scoreCard, err := cal.ParseScoreCard(scorecardData)
	if err != nil {
		return nil, fmt.Errorf("parse scorecard: %w", err)
	}

	// --- Select transformer based on backend ---
	var transformerComp *engine.Component
	var idMapper IDMappable
	transformerName := backend

	switch backend {
	case backendStub:
		stubT := NewStubTransformer(scenario)
		idMapper = stubT
		transformerComp = TransformerComponent(stubT, circuitDef)
	case backendLLM:
		if params.Dispatcher == nil {
			return nil, fmt.Errorf("backend %q requires a dispatcher (framework must provide SessionParams.Dispatcher)", backend)
		}
		rcaT := NewRCATransformer(params.Dispatcher, params.DomainFS)
		transformerComp = TransformerComponent(rcaT, circuitDef)
	default:
		return nil, fmt.Errorf("unknown backend %q (supported: stub, llm)", backend)
	}

	// --- Mediator endpoint for sub-circuit delegation (gather-code → gnd) ---
	mediatorEndpoint := os.Getenv("ORIGAMI_MEDIATOR_ENDPOINT")

	// --- Build RunFunc that runs the full calibration pipeline ---
	runFunc := func(ctx context.Context) (any, error) {
		return RunCalibration(ctx, &RunConfig{
			Scenario:        scenario,
			Components:      []*engine.Component{transformerComp},
			IDMapper:        idMapper,
			TransformerName: transformerName,
			Thresholds:      thresholds,
			ScoreCard:       scoreCard,
			CircuitData:     circuitData,
			Parallel:        params.Parallel,
		})
	}

	// When mediator endpoint is set, use calibrate.Run directly with
	// PromptRelayer + MediatorEndpoint for sub-circuit delegation.
	if mediatorEndpoint != "" {
		runFunc = func(ctx context.Context) (any, error) {
			adapter := &RCACalibrationAdapter{
				Scenario:   scenario,
				Components: []*engine.Component{transformerComp},
				IDMapper:   idMapper,
				Thresholds: thresholds,
				ScoreCard:  scoreCard,
			}

			genReport, err := cal.Run(ctx, &cal.HarnessConfig{
				Loader:    adapter,
				Collector: adapter,
				CircuitDef: circuitDef,
				ScoreCard:  scoreCard,
				Contract:   cal.ContractFromDef(circuitDef.Calibration),
				Shared: &engine.GraphRegistries{
					MediatorEndpoint: mediatorEndpoint,
				},
				Components:     []*engine.Component{transformerComp},
				PromptRelayer:  params.Relayer,
				Scenario:       scenario.Name,
				Transformer:    transformerName,
				Runs:           1,
				Parallel:       params.Parallel,
				OnCaseComplete: adapter.OnCaseComplete(),
				Observer:       params.Observer,
			})
			if err != nil {
				return nil, err
			}

			report := adapter.RCAReport(genReport)
			ApplyDryCaps(&report.Metrics, scenario.DryCappedMetrics)
			return report, nil
		}
	}

	return &engine.SessionConfig{
		CircuitDef: circuitDef,
		Meta: engine.SessionMeta{
			TotalCases: len(scenario.Cases),
			Scenario:   scenario.Name,
		},
		RunFunc:   runFunc,
		Preflight: buildPreflight(backend, mode, mediatorEndpoint),
	}, nil
}

// buildPreflight returns a fail-fast validation function that checks
// runtime prerequisites before the calibration run starts.
func buildPreflight(backend, mode, mediatorEndpoint string) func(context.Context) error {
	return func(ctx context.Context) error {
		// LLM backend requires a dispatcher (already checked in createSession,
		// but preflight is the canonical enforcement point going forward).
		if backend == backendLLM && mode != string(ModeOffline) {
			// Online mode: mediator must be reachable for sub-circuit delegation.
			if mediatorEndpoint == "" {
				return fmt.Errorf("ORIGAMI_MEDIATOR_ENDPOINT is required for online calibration (backend=%s, mode=%s)", backend, mode)
			}
			httpClient := &http.Client{Timeout: 5 * time.Second}
			healthURL := mediatorEndpoint[:len(mediatorEndpoint)-len("/mcp")] + "/healthz"
			resp, err := httpClient.Get(healthURL)
			if err != nil {
				return fmt.Errorf("mediator unreachable at %s: %w", healthURL, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("mediator unhealthy at %s: status %d", healthURL, resp.StatusCode)
			}
		}
		return nil
	}
}
