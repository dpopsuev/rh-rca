package rca

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/dpopsuev/origami-rca/rcatype"

	"gopkg.in/yaml.v3"
)

// LoadScenario reads a scenario by name from the given filesystem.
// Name is derived from the filename; a bare name: field in the YAML is optional.
func LoadScenario(fsys fs.FS, name string) (*Scenario, error) {
	data, err := fs.ReadFile(fsys, name+".yaml")
	if err != nil {
		return nil, fmt.Errorf("scenario %q not found (available: %s): %w",
			name, strings.Join(ListScenarios(fsys), ", "), err)
	}
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse scenario %q: %w", name, err)
	}
	if s.Name == "" {
		s.Name = name
	}
	s.ApplyDefaults()
	return &s, nil
}

// ResolveOfflineRP loads pre-staged RP envelopes from the offline bundle FS
// and populates case ErrorMessage/LogSnippet fields for cases that have a
// SourceLaunchID. This replaces live RP fetching in offline mode.
func ResolveOfflineRP(offlineFS fs.FS, scenario *Scenario) error {
	logger := slog.Default().With("component", "offline-rp")
	cache := make(map[int]*rcatype.Envelope)

	resolve := func(cases []GroundTruthCase) error {
		for i := range cases {
			c := &cases[i]
			if c.SourceLaunchID <= 0 {
				continue
			}
			env, ok := cache[c.SourceLaunchID]
			if !ok {
				path := fmt.Sprintf("rp/%d.json", c.SourceLaunchID)
				data, err := fs.ReadFile(offlineFS, path)
				if err != nil {
					available := listDir(offlineFS, "rp")
					return fmt.Errorf("offline RP launch %d for case %s: %w (available files in rp/: %s)",
						c.SourceLaunchID, c.ID, err, available)
				}
				env = new(rcatype.Envelope)
				if err := json.Unmarshal(data, env); err != nil {
					return fmt.Errorf("parse offline RP launch %d: %w", c.SourceLaunchID, err)
				}
				cache[c.SourceLaunchID] = env
				logger.Info("loaded offline RP launch",
					"launch_id", c.SourceLaunchID, "failures", len(env.FailureList))
			}
			item := matchOfflineItem(env, c)
			if item == nil {
				return fmt.Errorf("case %s: no matching failure item in offline RP launch %d (test_id=%s)",
					c.ID, c.SourceLaunchID, c.TestID)
			}
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

	if err := resolve(scenario.Cases); err != nil {
		return err
	}
	return resolve(scenario.Candidates)
}

func matchOfflineItem(env *rcatype.Envelope, c *GroundTruthCase) *rcatype.FailureItem {
	if c.TestID != "" {
		for i := range env.FailureList {
			if env.FailureList[i].ID == c.TestID {
				return &env.FailureList[i]
			}
			tag := "test_id:" + c.TestID
			if strings.Contains(env.FailureList[i].Name, tag) {
				return &env.FailureList[i]
			}
		}
	}
	if len(env.FailureList) == 1 {
		return &env.FailureList[0]
	}
	return nil
}

// ListScenarios returns the names of all scenarios in the given filesystem, sorted.
func ListScenarios(fsys fs.FS) []string {
	entries, _ := fs.ReadDir(fsys, ".")
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}

// listDir returns a comma-separated list of file names in a directory,
// or "<empty>" / "<unreadable>" for diagnostics.
func listDir(fsys fs.FS, dir string) string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Sprintf("<unreadable: %v>", err)
	}
	if len(entries) == 0 {
		return "<empty>"
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
