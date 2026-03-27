package rca

import (
	"os"
	"strings"

	"github.com/dpopsuev/origami-rca/rcatype"
	"github.com/dpopsuev/origami-rca/store"
	"github.com/dpopsuev/origami/toolkit"
)

func buildSourceParams(env *rcatype.Envelope, catalog toolkit.SourceCatalog) *SourceParams {
	wsp := &SourceParams{}

	if catalog != nil && len(catalog.Sources()) > 0 {
		wsp.ReposStatus = Resolved
		for _, s := range catalog.Sources() {
			wsp.Repos = append(wsp.Repos, RepoParams{
				Name:    s.Name,
				Path:    s.URI,
				Purpose: s.Purpose,
				Branch:  s.Branch,
			})
		}
	} else {
		wsp.ReposStatus = Unavailable
	}

	if env != nil && len(env.LaunchAttributes) > 0 {
		wsp.AttrsStatus = Resolved
		for _, a := range env.LaunchAttributes {
			wsp.LaunchAttributes = append(wsp.LaunchAttributes, AttributeParams{
				Key:    a.Key,
				Value:  a.Value,
				System: a.System,
			})
		}
	} else {
		wsp.AttrsStatus = Unavailable
	}

	if env != nil {
		seen := map[string]bool{}
		for _, f := range env.FailureList {
			for _, ext := range f.ExternalIssues {
				if ext.TicketID != "" && !seen[ext.TicketID] {
					seen[ext.TicketID] = true
					wsp.JiraLinks = append(wsp.JiraLinks, JiraLinkParams{
						TicketID: ext.TicketID,
						URL:      ext.URL,
					})
				}
			}
		}
	}
	if len(wsp.JiraLinks) > 0 {
		wsp.JiraStatus = Resolved
	} else {
		wsp.JiraStatus = Unavailable
	}

	wsp.AlwaysRead = loadAlwaysReadSources(catalog)

	return wsp
}

func loadPriorArtifacts(caseDir string) *PriorParams {
	priorNodes := []string{"recall", "triage", "resolve", "investigate", "correlate"}
	loaded := toolkit.LoadPriorArtifacts(caseDir, priorNodes, func(name string) string {
		return NodeArtifactFilename(name)
	})
	if loaded == nil {
		return nil
	}
	prior := make(PriorParams, len(loaded))
	for k, v := range loaded {
		prior[strings.ToUpper(k[:1])+k[1:]] = v
	}
	return &prior
}

func loadAlwaysReadSources(catalog toolkit.SourceCatalog) []AlwaysReadSource {
	if catalog == nil {
		return nil
	}
	alwaysSources := catalog.AlwaysReadSources()
	if len(alwaysSources) == 0 {
		return nil
	}
	var result []AlwaysReadSource
	for _, s := range alwaysSources {
		if s.LocalPath == "" {
			continue
		}
		content, err := os.ReadFile(s.LocalPath)
		if err != nil || len(content) == 0 {
			continue
		}
		result = append(result, AlwaysReadSource{
			Name:    s.Name,
			Purpose: s.Purpose,
			Content: string(content),
		})
	}
	return result
}

// findRecallCandidates searches the store for symptoms matching the test name.
// At F0_RECALL the case hasn't been triaged yet (SymptomID == 0), so we match
// on test name — the most reliable attribute available before triage.
func findRecallCandidates(st store.Store, testName string) *HistoryParams {
	if testName == "" {
		return nil
	}
	candidates, err := st.FindSymptomCandidates(testName)
	if err != nil || len(candidates) == 0 {
		return nil
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.OccurrenceCount > best.OccurrenceCount {
			best = c
		} else if c.OccurrenceCount == best.OccurrenceCount && c.LastSeenAt > best.LastSeenAt {
			best = c
		}
	}

	history := loadHistory(st, best.ID)
	if history == nil {
		return nil
	}

	if best.Status == "dormant" && history.SymptomInfo != nil {
		history.SymptomInfo.IsDormantReactivation = true
	}

	return history
}

func buildRecallDigest(st store.Store) []RecallDigestEntry {
	rcas, err := st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return nil
	}
	digest := make([]RecallDigestEntry, 0, len(rcas))
	for _, rca := range rcas {
		summary := rca.Description
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		digest = append(digest, RecallDigestEntry{
			ID:         rca.ID,
			Component:  rca.Component,
			DefectType: rca.DefectType,
			Summary:    summary,
		})
	}
	return digest
}

func loadHistory(st store.Store, symptomID int64) *HistoryParams {
	history := &HistoryParams{}

	sym, err := st.GetSymptom(symptomID)
	if err == nil && sym != nil {
		history.SymptomInfo = &SymptomInfoParams{
			Name:            sym.Name,
			OccurrenceCount: sym.OccurrenceCount,
			FirstSeen:       sym.FirstSeenAt,
			LastSeen:        sym.LastSeenAt,
			Status:          sym.Status,
		}
	}

	links, err := st.GetRCAsForSymptom(symptomID)
	if err == nil {
		for _, link := range links {
			rca, err := st.GetRCA(link.RCAID)
			if err != nil || rca == nil {
				continue
			}
			history.PriorRCAs = append(history.PriorRCAs, PriorRCAParams{
				ID:               rca.ID,
				Title:            rca.Title,
				DefectType:       rca.DefectType,
				Status:           rca.Status,
				AffectedVersions: rca.AffectedVersions,
				JiraLink:         rca.JiraLink,
				ResolvedAt:       rca.ResolvedAt,
			})
		}
	}

	return history
}
