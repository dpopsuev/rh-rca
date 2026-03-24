package rp

import (
	"encoding/json"
	"os"

	"github.com/dpopsuev/origami/toolkit"
)

// Analyzer runs investigation: envelope → cases + artifact. Mock implementation only.
type Analyzer interface {
	Analyze(src EnvelopeSource, launchID int, artifactPath string) error
}

// DefaultAnalyzer is the mock analyzer used in tests and wiring.
type DefaultAnalyzer struct{}

// Analyze implements Analyzer: read envelope from source, one case per failure, write artifact.
// Contract: .cursor/contracts/mock-investigation.md
func (DefaultAnalyzer) Analyze(src EnvelopeSource, launchID int, artifactPath string) error {
	return Analyze(src, launchID, artifactPath)
}

// Analyze reads envelope from source, creates one case per failure, and writes artifact to path.
// Contract: mock-investigation — no real AI; artifact has launch_id, case_ids, placeholder RCA fields.
func Analyze(src EnvelopeSource, launchID int, artifactPath string) error {
	return AnalyzeWithCatalog(src, launchID, artifactPath, nil)
}

// AnalyzeWithCatalog is like Analyze but accepts an optional GND source catalog.
// When non-nil, catalog is available for downstream (e.g. prompts).
func AnalyzeWithCatalog(src EnvelopeSource, launchID int, artifactPath string, cat toolkit.SourceCatalog) error {
	env, err := src.Get(launchID)
	if err != nil {
		return err
	}
	if env == nil {
		return nil
	}
	_ = cat // used by prompts when building context; artifact unchanged for PoC
	artifact := Artifact{
		LaunchID:         env.RunID,
		CaseIDs:          CaseIDsFromEnvelope(env),
		RCAMessage:       "",
		DefectType:       "ti001",
		ConvergenceScore: 0.85,
		EvidenceRefs:     []string{},
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(artifactPath, data, 0644)
}
