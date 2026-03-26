package rca

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/engine"
)

// Hooks returns the SessionHooks that fold-generated code calls.
func Hooks() engine.SessionHooks {
	return engine.SessionHooks{
		CreateSession: createSession,
		StepSchemas:   RCAStepSchemas(),
		FormatReport: func(result any) (string, any, error) {
			report, ok := result.(*CalibrationReport)
			if !ok {
				return "", nil, nil
			}
			formatted, err := RenderCalibrationReport(report, nil)
			return formatted, report, err
		},
	}
}

// createSession wires scenario loading, transformer selection, and
// calibration scoring into a SessionConfig with a custom RunFunc.
func createSession(_ context.Context, params engine.SessionParams) (*engine.SessionConfig, error) {
	// --- Parse domain params from Extra ---
	scenarioName, _ := params.Extra["scenario"].(string)
	if scenarioName == "" {
		scenarioName = "ptp"
	}
	backend, _ := params.Extra["backend"].(string)
	if backend == "" {
		backend = "llm"
	}
	mode, _ := params.Extra["mode"].(string)
	if mode == "" {
		mode = "offline"
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
	if mode == "offline" {
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
	case "stub":
		stubT := NewStubTransformer(scenario)
		idMapper = stubT
		transformerComp = TransformerComponent(stubT, circuitDef)
	case "llm":
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
		return RunCalibration(ctx, RunConfig{
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

			genReport, err := cal.Run(ctx, cal.HarnessConfig{
				Loader:    adapter,
				Collector: adapter,
				CircuitDef: circuitDef,
				ScoreCard:  scoreCard,
				Contract:   cal.ContractFromDef(circuitDef.Calibration),
				Shared: engine.GraphRegistries{
					MediatorEndpoint: mediatorEndpoint,
				},
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
		RunFunc: runFunc,
	}, nil
}
