package rca

import (
	"context"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"

	"github.com/dpopsuev/origami-rca/store"
)

// StoreHooks creates a HookRegistry with per-step store effect hooks
// bound to the given store and case data. Register these on the Runner
// so they fire automatically after each node completes.
func StoreHooks(st store.Store, caseData *store.Case) engine.HookRegistry {
	reg := engine.HookRegistry{}
	reg.Register(engine.NewHookFunc("store.recall", func(_ context.Context, _ string, art circuit.Artifact) error {
		return applyRecallEffects(st, caseData, art.Raw())
	}))
	reg.Register(engine.NewHookFunc("store.triage", func(_ context.Context, _ string, art circuit.Artifact) error {
		return applyTriageEffects(st, caseData, art.Raw())
	}))
	reg.Register(engine.NewHookFunc("store.investigate", func(_ context.Context, _ string, art circuit.Artifact) error {
		return applyInvestigateEffects(st, caseData, art.Raw())
	}))
	reg.Register(engine.NewHookFunc("store.correlate", func(_ context.Context, _ string, art circuit.Artifact) error {
		return applyCorrelateEffects(st, caseData, art.Raw())
	}))
	reg.Register(engine.NewHookFunc("store.review", func(_ context.Context, _ string, art circuit.Artifact) error {
		return applyReviewEffects(st, caseData, art.Raw())
	}))
	return reg
}
