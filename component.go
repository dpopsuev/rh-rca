// Package rca provides an Origami component that bundles the RCA circuit's
// hooks, transformers, and extractors under the "rca" namespace.
package rca

import (
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/rh-rca/store"
	"github.com/dpopsuev/origami/schematics/toolkit"

	framework "github.com/dpopsuev/origami"
)

// ComponentConfig holds runtime dependencies injected into the RCA component.
type ComponentConfig struct {
	Store           store.Store
	CaseData        *store.Case
	Envelope        *rcatype.Envelope
	Catalog         toolkit.SourceCatalog
	CaseDir         string
	DSRReader toolkit.SourceReader
	CircuitDef      *framework.CircuitDef
}

// Component returns an Origami Component bundling all RCA circuit plumbing
// (store hooks, context-builder transformer, prompt-filler transformer,
// and per-step extractors) under the "rca" namespace.
func Component(cfg ComponentConfig) *framework.Component {
	return &framework.Component{
		Namespace:    "rca",
		Name:         "origami-rca",
		Version:      "1.0.0",
		Description:  "RCA circuit plumbing for CI root-cause analysis",
		Transformers: buildTransformers(cfg),
		Extractors:   buildExtractors(cfg.CircuitDef),
		Hooks:        buildHooks(cfg),
	}
}

// allNodeNames lists every RCA circuit node name. Used as fallback when
// no CircuitDef is available.
var allNodeNames = []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}

// nodeNames returns node names from the CircuitDef when available,
// falling back to the hardcoded allNodeNames list.
func nodeNames(cd *framework.CircuitDef) []string {
	if cd != nil {
		if names := toolkit.NodeNamesFromCircuit(cd); len(names) > 0 {
			return names
		}
	}
	return allNodeNames
}

// HeuristicComponent returns a Component with per-node heuristic transformers
// that implement deterministic, keyword-based RCA logic.
func HeuristicComponent(st store.Store, repos []string, heuristicsData []byte) *framework.Component {
	ht := NewHeuristicTransformer(st, repos, heuristicsData)
	return &framework.Component{
		Namespace: "rca",
		Name:      "rca-heuristic",
		Transformers: framework.TransformerRegistry{
			"recall":      &recallHeuristic{ht: ht},
			"triage":      &triageHeuristic{ht: ht},
			"resolve":     &resolveHeuristic{ht: ht},
			"investigate": &investigateHeuristic{ht: ht},
			"correlate":   &correlateHeuristic{ht: ht},
			"review":      &reviewHeuristic{},
			"report":      &reportHeuristic{},
		},
	}
}

// TransformerComponent wraps a monolithic framework.Transformer (e.g. stub, rca)
// and registers it under every node name so that DSL transformer: resolution
// can find it. The transformer's Transform() dispatches on tc.NodeName.
// An optional CircuitDef derives node names dynamically; without it,
// the hardcoded allNodeNames list is used.
func TransformerComponent(t framework.Transformer, cd ...*framework.CircuitDef) *framework.Component {
	var def *framework.CircuitDef
	if len(cd) > 0 {
		def = cd[0]
	}
	return &framework.Component{
		Namespace:    "rca",
		Name:         "rca-transformer",
		Transformers: toolkit.TransformerForAllNodes(t, nodeNames(def)),
	}
}

// HITLComponent returns a Component with per-node HITL transformers that
// fill prompt templates and return framework.Interrupt for human input.
// An optional CircuitDef derives node names dynamically; without it,
// the hardcoded allNodeNames list is used.
func HITLComponent(cd ...*framework.CircuitDef) *framework.Component {
	var def *framework.CircuitDef
	if len(cd) > 0 {
		def = cd[0]
	}
	reg := framework.TransformerRegistry{}
	for _, name := range nodeNames(def) {
		reg[name] = &hitlTransformerNode{nodeName: name}
	}
	return &framework.Component{
		Namespace:    "rca",
		Name:         "rca-hitl",
		Transformers: reg,
	}
}

func buildTransformers(_ ComponentConfig) framework.TransformerRegistry {
	return framework.TransformerRegistry{}
}

func buildExtractors(cd *framework.CircuitDef) framework.ExtractorRegistry {
	return toolkit.ExtractorForAllNodes(func(name string) framework.Extractor {
		return NewMapExtractor(name)
	}, nodeNames(cd))
}

func buildHooks(cfg ComponentConfig) framework.HookRegistry {
	reg := framework.HookRegistry{}

	inject := InjectHooksWithOpts(InjectHookOpts{
		Store:           cfg.Store,
		CaseData:        cfg.CaseData,
		Envelope:        cfg.Envelope,
		Catalog:         cfg.Catalog,
		CaseDir:         cfg.CaseDir,
		DSRReader: cfg.DSRReader,
	})
	for name, h := range inject {
		reg[name] = h
	}

	if cfg.Store != nil && cfg.CaseData != nil {
		hooks := StoreHooks(cfg.Store, cfg.CaseData)
		for name, h := range hooks {
			reg[name] = h
		}
	}
	return reg
}
