package rca

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/dpopsuev/rh-rca/rcatype"
)

// ResolveRPCases fetches real failure data from ReportPortal for cases that
// have SourceLaunchID set, updating their ErrorMessage and LogSnippet in place.
// Cases without SourceLaunchID are left unchanged. Envelopes are cached by launch
// ID so multiple cases sharing a launch only trigger one API call.
func ResolveRPCases(fetcher rcatype.EnvelopeFetcher, scenario *Scenario) error {
	logger := slog.Default().With("component", "rp-source")
	cache := make(map[int]*rcatype.Envelope)

	for i := range scenario.Cases {
		c := &scenario.Cases[i]
		if c.SourceLaunchID <= 0 {
			continue
		}

		env, ok := cache[c.SourceLaunchID]
		if !ok {
			var err error
			env, err = fetcher.Fetch(fmt.Sprintf("%d", c.SourceLaunchID))
			if err != nil {
				return fmt.Errorf("fetch RP launch %d for case %s: %w", c.SourceLaunchID, c.ID, err)
			}
			cache[c.SourceLaunchID] = env
			logger.Info("fetched RP launch",
				"launch_id", c.SourceLaunchID, "name", env.Name, "failures", len(env.FailureList))
		}

		item := matchFailureItem(env, c)
		if item == nil {
			return fmt.Errorf("case %s: no matching failure item in RP launch %d (test=%q, item_id=%d)",
				c.ID, c.SourceLaunchID, c.TestName, c.SourceItemID)
		}

		logger.Info("matched RP item", "case_id", c.ID, "item_id", item.ID, "item_name", item.Name)

		if item.Description != "" {
			c.ErrorMessage = item.Description
		}
		if c.LogSnippet == "" && item.LogSnippet != "" {
			c.LogSnippet = item.LogSnippet
		}
		if item.Tags != nil {
			c.SourceIssueType = item.Tags["rp.issue_type"]
			c.SourceAutoAnalyzed = item.Tags["rp.auto_analyzed"] == "true"
		}
	}

	return nil
}

func matchFailureItem(env *rcatype.Envelope, c *GroundTruthCase) *rcatype.FailureItem {
	if c.SourceItemID > 0 {
		want := fmt.Sprintf("%d", c.SourceItemID)
		for i := range env.FailureList {
			if env.FailureList[i].ID == want {
				return &env.FailureList[i]
			}
		}
	}

	if c.TestID != "" {
		tag := "test_id:" + c.TestID
		for i := range env.FailureList {
			if strings.Contains(env.FailureList[i].Name, tag) {
				return &env.FailureList[i]
			}
		}
	}

	testLower := strings.ToLower(c.TestName)
	if testLower != "" {
		for i := range env.FailureList {
			nameLower := strings.ToLower(env.FailureList[i].Name)
			if strings.Contains(nameLower, testLower) {
				return &env.FailureList[i]
			}
			if strings.Contains(testLower, nameLower) {
				return &env.FailureList[i]
			}
		}
	}

	return nil
}
