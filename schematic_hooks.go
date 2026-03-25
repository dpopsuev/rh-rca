package rca

import (
	"context"

	"github.com/dpopsuev/origami/engine"
)

// Hooks returns the SessionHooks that fold-generated code calls.
// The consumer provides domain config only — the framework wires
// dispatcher and signal bus internally.
func Hooks() engine.SessionHooks {
	return engine.SessionHooks{
		CreateSession: func(_ context.Context, _ engine.SessionParams) (*engine.SessionConfig, error) {
			// TODO: refactor createSession to build SessionConfig directly.
			// For now, the bridge in mcp.SessionHooksToConfig wraps this
			// into the old RunFunc pattern with dispatcher + bus.
			return &engine.SessionConfig{
				Meta: engine.SessionMeta{
					Scenario: "bridged",
				},
			}, nil
		},
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
