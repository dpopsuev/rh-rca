package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami-rca/rcatype"
	"github.com/dpopsuev/origami-rca/store"
	"github.com/dpopsuev/origami/toolkit"
)

// HITLConfig holds configuration for the interactive HITL circuit mode.
type HITLConfig struct {
	Store     store.Store
	CaseData  *store.Case
	Envelope  *rcatype.Envelope
	Catalog   toolkit.SourceCatalog
	PromptFS  fs.FS
	CaseDir   string
}

// HITLResult is the RCA-specific alias for engine.HITLResult.
type HITLResult = engine.HITLResult

// RunHITLStep runs (or resumes) the circuit until it either pauses for
// human input (Interrupt) or completes. If a checkpoint exists, the walk
// resumes from the last interrupted node.
func RunHITLStep(ctx context.Context, cfg HITLConfig) (*HITLResult, error) {
	th := DefaultThresholds()
	walkerID := fmt.Sprintf("case-%d", cfg.CaseData.ID)

	cp, err := engine.NewJSONCheckpointer(cfg.CaseDir)
	if err != nil {
		return nil, fmt.Errorf("create checkpointer: %w", err)
	}

	hitlComp := HITLComponent()
	storeComp := &engine.Component{
		Namespace: "store",
		Name:      "rca-store-hooks",
		Hooks:     StoreHooks(cfg.Store, cfg.CaseData),
	}
	runner, err := BuildRunner(nil, th, hitlComp, storeComp)
	if err != nil {
		return nil, fmt.Errorf("build runner: %w", err)
	}

	walker, startNode, err := prepareWalker(cp, walkerID, cfg)
	if err != nil {
		return nil, err
	}

	wrapped := engine.WrapWithCheckpointer(walker, cp)
	walkErr := runner.Walk(ctx, wrapped, startNode)
	return buildResult(walker, walkErr)
}

// ResumeHITLStep reads a saved artifact and resumes the walk from the
// last checkpointed node.
func ResumeHITLStep(ctx context.Context, cfg HITLConfig, artifactData []byte) (*HITLResult, error) {
	th := DefaultThresholds()
	walkerID := fmt.Sprintf("case-%d", cfg.CaseData.ID)

	cp, err := engine.NewJSONCheckpointer(cfg.CaseDir)
	if err != nil {
		return nil, fmt.Errorf("create checkpointer: %w", err)
	}

	hitlComp := HITLComponent()
	storeComp := &engine.Component{
		Namespace: "store",
		Name:      "rca-store-hooks",
		Hooks:     StoreHooks(cfg.Store, cfg.CaseData),
	}
	runner, err := BuildRunner(nil, th, hitlComp, storeComp)
	if err != nil {
		return nil, fmt.Errorf("build runner: %w", err)
	}

	walker, startNode, err := prepareWalker(cp, walkerID, cfg)
	if err != nil {
		return nil, err
	}

	var artifact any
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}
	walker.State().Context["resume_input"] = artifact

	wrapped := engine.WrapWithCheckpointer(walker, cp)
	walkErr := runner.Walk(ctx, wrapped, startNode)
	return buildResult(walker, walkErr)
}

// LoadCheckpointState loads the WalkerState from the checkpoint directory.
// Returns nil, nil if no checkpoint exists.
func LoadCheckpointState(caseDir string, caseID int64) (*circuit.WalkerState, error) {
	return engine.LoadCheckpointState(caseDir, fmt.Sprintf("case-%d", caseID))
}

func prepareWalker(cp circuit.Checkpointer, walkerID string, cfg HITLConfig) (circuit.Walker, string, error) {
	loaded, _ := cp.Load(walkerID)

	walker := circuit.NewProcessWalker(walkerID)
	injectHITLContext(walker.State(), cfg)

	startNode := "recall"
	if resumed := engine.RestoreWalkerState(walker, loaded); resumed != "" {
		startNode = resumed
	}

	return walker, startNode, nil
}

func injectHITLContext(state *circuit.WalkerState, cfg HITLConfig) {
	state.Context[KeyStore] = cfg.Store
	state.Context[KeyCaseData] = cfg.CaseData
	state.Context[KeyEnvelope] = cfg.Envelope
	state.Context[KeyCatalog] = cfg.Catalog
	state.Context[KeyCaseDir] = cfg.CaseDir
	if cfg.PromptFS != nil {
		state.Context[KeyPromptFS] = cfg.PromptFS
	}
}

func buildResult(walker circuit.Walker, walkErr error) (*HITLResult, error) {
	return engine.BuildHITLResult(walker, walkErr)
}
