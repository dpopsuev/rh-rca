package rca

import (
	"context"
	"encoding/json"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/rh-rca/store"
	"github.com/dpopsuev/origami/schematics/toolkit"
)

// Context keys used by inject hooks to store assembled template data.
const (
	KeyParamsEnvelope = "params.envelope"
	KeyParamsFailure  = "params.failure"
	KeyParamsHistory  = "params.history"
	KeyParamsDigest   = "params.recall_digest"
	KeyParamsSources  = "params.sources"
	KeyParamsPrior    = "params.prior"
	KeyParamsTaxonomy = "params.taxonomy"
	KeyParamsCode     = "params.code"
)

// InjectHookOpts configures the inject hook registry.
type InjectHookOpts struct {
	Store           store.Store
	CaseData        *store.Case
	Envelope        *rcatype.Envelope
	Catalog         toolkit.SourceCatalog
	CaseDir         string
}

// InjectHooks creates a HookRegistry with the inject.* before-hooks
// that populate walker.Context with per-concern template data.
// Each hook uses WalkerStateFromContext to write into walker.Context.
func InjectHooks(st store.Store, caseData *store.Case, env *rcatype.Envelope, catalog toolkit.SourceCatalog, caseDir string) framework.HookRegistry {
	return InjectHooksWithOpts(InjectHookOpts{
		Store:    st,
		CaseData: caseData,
		Envelope: env,
		Catalog:  catalog,
		CaseDir:  caseDir,
	})
}

// InjectHooksWithOpts creates inject hooks using the full options struct.
func InjectHooksWithOpts(opts InjectHookOpts) framework.HookRegistry {
	reg := framework.HookRegistry{}

	reg.Register(newInjectEnvelopeHook(opts.Envelope))
	reg.Register(newInjectFailureHook(opts.CaseData))
	reg.Register(newInjectHistoryHook(opts.Store, opts.CaseData))
	reg.Register(newInjectRecallDigestHook(opts.Store))
	reg.Register(newInjectSourcesHook(opts.Envelope, opts.Catalog))
	reg.Register(newInjectPriorHook(opts.CaseDir))
	reg.Register(newInjectTaxonomyHook())

	// Circuit-composition bridge hooks.
	reg.Register(newInjectCodeKeywordsHook())
	reg.Register(newBridgeCodeContextHook())

	return reg
}

func newInjectEnvelopeHook(env *rcatype.Envelope) framework.Hook {
	return toolkit.NewContextInjector("inject.envelope", func(walkerCtx map[string]any) {
		injectEnvelopeData(env, walkerCtx)
	})
}

func newInjectFailureHook(caseData *store.Case) framework.Hook {
	return toolkit.NewContextInjector("inject.failure", func(walkerCtx map[string]any) {
		injectFailureData(caseData, walkerCtx)
	})
}

func newInjectHistoryHook(st store.Store, caseData *store.Case) framework.Hook {
	return toolkit.NewContextInjector("inject.history", func(walkerCtx map[string]any) {
		injectHistoryData(st, caseData, walkerCtx)
	})
}

func newInjectRecallDigestHook(st store.Store) framework.Hook {
	return toolkit.NewContextInjector("inject.recall-digest", func(walkerCtx map[string]any) {
		injectRecallDigestData(st, walkerCtx)
	})
}

func newInjectSourcesHook(env *rcatype.Envelope, catalog toolkit.SourceCatalog) framework.Hook {
	return toolkit.NewContextInjector("inject.sources", func(walkerCtx map[string]any) {
		injectSourcesData(env, catalog, walkerCtx)
	})
}

func newInjectPriorHook(caseDir string) framework.Hook {
	return toolkit.NewContextInjector("inject.prior", func(walkerCtx map[string]any) {
		injectPriorData(caseDir, walkerCtx)
	})
}

func newInjectTaxonomyHook() framework.Hook {
	return toolkit.NewContextInjector("inject.taxonomy", func(walkerCtx map[string]any) {
		injectTaxonomyData(walkerCtx)
	})
}

// ParamsFromContext assembles a TemplateParams from walker context.
// Before-hooks inject their data into walker.Context with keys like
// "params.envelope", "params.failure", etc. This function collects
// them into the TemplateParams structure that templates expect.
func ParamsFromContext(walkerCtx map[string]any) *TemplateParams {
	params := &TemplateParams{}

	if v, ok := walkerCtx[KeyParamsEnvelope].(*EnvelopeParams); ok {
		params.Envelope = v
		params.SourceID = v.RunID
	}

	if v, ok := walkerCtx[KeyParamsFailure].(*FailureParams); ok {
		params.Failure = v
	}

	if v, ok := walkerCtx[KeyParamsSources].(*SourceParams); ok {
		params.Sources = v
	}

	if v, ok := walkerCtx[KeyParamsHistory].(*HistoryParams); ok {
		params.History = v
	}

	if v, ok := walkerCtx[KeyParamsDigest].([]RecallDigestEntry); ok {
		params.RecallDigest = v
	}

	if v, ok := walkerCtx[KeyParamsPrior].(*PriorParams); ok {
		params.Prior = v
	}

	if v, ok := walkerCtx[KeyParamsTaxonomy].(*TaxonomyParams); ok {
		params.Taxonomy = v
	}

	if v, ok := walkerCtx[KeyParamsCode].(*CodeParams); ok {
		params.Code = v
	}

	if cd, ok := walkerCtx[KeyCaseData].(*store.Case); ok {
		params.CaseID = cd.ID
	}

	if _, ok := walkerCtx[KeyParamsEnvelope].(*EnvelopeParams); ok {
		if env, ok := walkerCtx[KeyEnvelope].(*rcatype.Envelope); ok {
			for _, f := range env.FailureList {
				params.Siblings = append(params.Siblings, SiblingParams{
					ID: f.ID, Name: f.Name, Status: f.Status,
				})
			}
		}
	}

	if params.Timestamps == nil {
		params.Timestamps = &TimestampParams{
			ClockPlaneNote: "Note: Timestamps may originate from different clock planes (executor, test node, SUT). Cross-plane time comparisons may be unreliable.",
		}
	}

	return params
}

// Concrete implementations that actually inject data into walker context.

func injectEnvelopeData(env *rcatype.Envelope, walkerCtx map[string]any) {
	if env == nil {
		return
	}
	walkerCtx[KeyParamsEnvelope] = &EnvelopeParams{
		Name:  env.Name,
		RunID: env.RunID,
	}
}

func injectFailureData(caseData *store.Case, walkerCtx map[string]any) {
	if caseData == nil {
		return
	}
	walkerCtx[KeyParamsFailure] = &FailureParams{
		TestName:     caseData.Name,
		ErrorMessage: caseData.ErrorMessage,
		LogSnippet:   caseData.LogSnippet,
		LogTruncated: caseData.LogTruncated,
		Status:       caseData.Status,
	}
}

func injectHistoryData(st store.Store, caseData *store.Case, walkerCtx map[string]any) {
	if st == nil || caseData == nil {
		return
	}
	if caseData.SymptomID != 0 {
		walkerCtx[KeyParamsHistory] = loadHistory(st, caseData.SymptomID)
	} else {
		walkerCtx[KeyParamsHistory] = findRecallCandidates(st, caseData.Name)
	}
}

func injectRecallDigestData(st store.Store, walkerCtx map[string]any) {
	if st == nil {
		return
	}
	walkerCtx[KeyParamsDigest] = buildRecallDigest(st)
}

func injectSourcesData(env *rcatype.Envelope, catalog toolkit.SourceCatalog, walkerCtx map[string]any) {
	walkerCtx[KeyParamsSources] = buildSourceParams(env, catalog)
}

func injectPriorData(caseDir string, walkerCtx map[string]any) {
	if caseDir == "" {
		return
	}
	walkerCtx[KeyParamsPrior] = loadPriorArtifacts(caseDir)
}

func injectTaxonomyData(walkerCtx map[string]any) {
	walkerCtx[KeyParamsTaxonomy] = DefaultTaxonomy()
}

func ensureCodeParams(walkerCtx map[string]any) *CodeParams {
	if v, ok := walkerCtx[KeyParamsCode].(*CodeParams); ok {
		return v
	}
	code := &CodeParams{}
	walkerCtx[KeyParamsCode] = code
	return code
}

func extractSearchKeywords(walkerCtx map[string]any) []string {
	var keywords []string
	if fp, ok := walkerCtx[KeyParamsFailure].(*FailureParams); ok && fp != nil {
		if fp.TestName != "" {
			keywords = append(keywords, fp.TestName)
		}
	}
	if prior, ok := walkerCtx[KeyParamsPrior].(*PriorParams); ok && prior != nil {
		if triage := (*prior)["Triage"]; triage != nil {
			if repos, ok := triage["candidate_repos"].([]any); ok {
				for _, r := range repos {
					if s, ok := r.(string); ok {
						keywords = append(keywords, s)
					}
				}
			}
		}
		if resolve := (*prior)["Resolve"]; resolve != nil {
			if repos, ok := resolve["selected_repos"].([]any); ok {
				for _, r := range repos {
					if rm, ok := r.(map[string]any); ok {
						if name, ok := rm["name"].(string); ok {
							keywords = append(keywords, name)
						}
					}
				}
			}
		}
	}
	return keywords
}

// Circuit-composition bridge hooks

// newInjectCodeKeywordsHook creates a before-hook that extracts search
// keywords from the walker context and writes them to
// "dsr.search_keywords" so the GND sub-circuit can use them.
func newInjectCodeKeywordsHook() framework.Hook {
	return toolkit.NewContextInjector("inject.code-keywords", func(walkerCtx map[string]any) {
		keywords := extractSearchKeywords(walkerCtx)
		if len(keywords) > 0 {
			walkerCtx["dsr.search_keywords"] = keywords
		}
	})
}

// newBridgeCodeContextHook creates an after-hook that reads the GND
// circuit's output from delegate artifacts and converts it to *CodeParams
// for consumption by downstream RCA nodes.
func newBridgeCodeContextHook() framework.Hook {
	return framework.NewHookFunc("bridge.code-context", func(ctx context.Context, _ string, art framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}

		da, ok := art.(*framework.DelegateArtifact)
		if !ok || da == nil {
			return nil
		}

		// Find the "read" node's artifact in the delegate's inner results.
		readArt, ok := da.InnerArtifacts["read"]
		if !ok || readArt == nil {
			return nil
		}

		// Decode the raw artifact via JSON roundtrip to avoid importing
		// the GND package. The raw value may be a concrete struct (local
		// delegate) or a map[string]any (mediator delegation).
		raw := readArt.Raw()
		if raw == nil {
			return nil
		}
		data, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		var cc codeContextWire
		if err := json.Unmarshal(data, &cc); err != nil {
			return nil
		}

		code := ensureCodeParams(ws.Context)
		for _, tree := range cc.Trees {
			var entries []TreeEntry
			for _, e := range tree.Entries {
				entries = append(entries, TreeEntry{Path: e.Path, IsDir: e.IsDir})
			}
			code.Trees = append(code.Trees, CodeTreeParams{
				Repo:    tree.Repo,
				Branch:  tree.Branch,
				Entries: entries,
			})
		}
		for _, hit := range cc.SearchResults {
			code.SearchResults = append(code.SearchResults, CodeSearchResult{
				Repo:    hit.Repo,
				File:    hit.File,
				Line:    hit.Line,
				Snippet: hit.Snippet,
			})
		}
		for _, f := range cc.Files {
			code.Files = append(code.Files, CodeFileParams{
				Repo:      f.Repo,
				Path:      f.Path,
				Content:   f.Content,
				Truncated: f.Truncated,
			})
		}
		code.Truncated = cc.Truncated
		return nil
	})
}

// codeContextWire mirrors the GND CodeContext fields consumed by the bridge
// hook. Decoupled via JSON roundtrip so rh-rca never imports rh-gnd.
type codeContextWire struct {
	Trees         []struct {
		Repo    string `json:"repo"`
		Branch  string `json:"branch"`
		Entries []struct {
			Path  string `json:"path"`
			IsDir bool   `json:"is_dir"`
		} `json:"entries"`
	} `json:"trees"`
	SearchResults []struct {
		Repo    string `json:"repo"`
		File    string `json:"file"`
		Line    int    `json:"line"`
		Snippet string `json:"snippet"`
	} `json:"search_results"`
	Files []struct {
		Repo      string `json:"repo"`
		Path      string `json:"path"`
		Content   string `json:"content"`
		Truncated bool   `json:"truncated"`
	} `json:"files"`
	Truncated bool `json:"truncated"`
}
