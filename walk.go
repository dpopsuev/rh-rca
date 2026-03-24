package rca

import (
	"context"
	"fmt"

	"github.com/dpopsuev/rh-rca/store"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/origami/toolkit"
)

// WalkConfig holds configuration for a walk-based RCA run.
type WalkConfig struct {
	Store       store.Store
	CaseData    *store.Case
	Envelope    *rcatype.Envelope
	Catalog     toolkit.SourceCatalog
	CaseDir     string
	CaseLabel   string
	Thresholds  Thresholds
	CircuitData []byte
	Components  []*engine.Component
}

// WalkResult captures the outcome of a walk-based RCA.
type WalkResult struct {
	Path          []string
	StepArtifacts map[string]circuit.Artifact
	State         *circuit.WalkerState
}

// WalkCase runs a single case through the RCA circuit using BatchWalk.
func WalkCase(ctx context.Context, cfg WalkConfig) (*WalkResult, error) {
	th := cfg.Thresholds
	if th == (Thresholds{}) {
		th = DefaultThresholds()
	}

	def, err := LoadCircuitDef(cfg.CircuitData, th)
	if err != nil {
		return nil, fmt.Errorf("load circuit def: %w", err)
	}

	results := engine.BatchWalk(ctx, engine.BatchWalkConfig{
		Def:    def,
		Shared: engine.GraphRegistries{},
		Cases: []engine.BatchCase{{
			ID: cfg.CaseLabel,
			Context: map[string]any{
				KeyCaseData:  cfg.CaseData,
				KeyEnvelope:  cfg.Envelope,
				KeyCaseDir:   cfg.CaseDir,
				KeyCaseLabel: cfg.CaseLabel,
			},
			Components: cfg.Components,
		}},
	})

	r := results[0]
	if r.Error != nil {
		return nil, r.Error
	}

	return &WalkResult{
		Path:          r.Path,
		StepArtifacts: r.StepArtifacts,
		State:         r.State,
	}, nil
}
