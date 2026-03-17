package rca

import (
	"context"

	framework "github.com/dpopsuev/origami"

	"github.com/dpopsuev/rh-rca/store"
)

// StoreHooks creates a HookRegistry with per-step store effect hooks
// bound to the given store and case data. Register these on the Runner
// so they fire automatically after each node completes.
func StoreHooks(st store.Store, caseData *store.Case) framework.HookRegistry {
	reg := framework.HookRegistry{}
	reg.Register(framework.NewHookFunc("store.recall", func(_ context.Context, _ string, art framework.Artifact) error {
		return applyRecallEffects(st, caseData, art.Raw())
	}))
	reg.Register(framework.NewHookFunc("store.triage", func(_ context.Context, _ string, art framework.Artifact) error {
		return applyTriageEffects(st, caseData, art.Raw())
	}))
	reg.Register(framework.NewHookFunc("store.investigate", func(_ context.Context, _ string, art framework.Artifact) error {
		return applyInvestigateEffects(st, caseData, art.Raw())
	}))
	reg.Register(framework.NewHookFunc("store.correlate", func(_ context.Context, _ string, art framework.Artifact) error {
		return applyCorrelateEffects(st, caseData, art.Raw())
	}))
	reg.Register(framework.NewHookFunc("store.review", func(_ context.Context, _ string, art framework.Artifact) error {
		return applyReviewEffects(st, caseData, art.Raw())
	}))
	return reg
}
