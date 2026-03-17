package rca

import (
	"context"
	"fmt"

	"github.com/dpopsuev/rh-rca/store"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/origami/schematics/toolkit"
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
	Components  []*framework.Component
}

// WalkResult captures the outcome of a walk-based RCA.
type WalkResult struct {
	Path          []string
	StepArtifacts map[string]framework.Artifact
	State         *framework.WalkerState
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

	results := framework.BatchWalk(ctx, framework.BatchWalkConfig{
		Def:    def,
		Shared: framework.GraphRegistries{},
		Cases: []framework.BatchCase{{
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
