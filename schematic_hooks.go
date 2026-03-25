package rca

import (
	fwmcp "github.com/dpopsuev/origami/mcp"
)

// Hooks returns the SchematicHooks that fold-generated code calls.
// This wraps the existing Server to provide CreateSession and FormatReport
// callbacks without exposing the full server API.
func Hooks() fwmcp.SchematicHooks {
	// Fold-generated code injects DomainFS, connector factories, etc.
	// via WithX options before calling Hooks(). For now, create a minimal
	// server that the hooks close over.
	s := &Server{}
	return fwmcp.SchematicHooks{
		CreateSession: s.createSession,
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
