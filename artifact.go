package rca

import "github.com/dpopsuev/origami/toolkit"

// DefaultBasePath is the default subdirectory under StateDir for
// investigation data. Consumers resolve this relative to their StateDir.
const DefaultBasePath = "investigations"

// rcaNodeArtifacts maps all known RCA node names to their artifact filenames.
// Unknown nodes return "". Nodes with standard naming use the convention.
var rcaNodeArtifacts = map[string]string{
	"recall":      "recall-result.json",
	"triage":      "triage-result.json",
	"resolve":     "resolve-result.json",
	"investigate": "artifact.json",
	"correlate":   "correlate-result.json",
	"review":      "review-decision.json",
	"report":      "jira-draft.json",
}

func CaseDir(basePath string, suiteID, caseID int64) string {
	return toolkit.CaseDir(basePath, suiteID, caseID)
}

func EnsureCaseDir(basePath string, suiteID, caseID int64) (string, error) {
	return toolkit.EnsureCaseDir(basePath, suiteID, caseID)
}

func ListCaseDirs(basePath string, suiteID int64) ([]string, error) {
	return toolkit.ListCaseDirs(basePath, suiteID)
}

func NodeArtifactFilename(nodeName string) string {
	if _, known := rcaNodeArtifacts[nodeName]; !known {
		return ""
	}
	return toolkit.NodeArtifactFilename(nodeName, rcaNodeArtifacts)
}

func NodePromptFilename(nodeName string, loopIter int) string {
	return toolkit.NodePromptFilename(nodeName, loopIter)
}

func ReadMapArtifact(caseDir, filename string) (map[string]any, error) {
	return toolkit.ReadMapArtifact(caseDir, filename)
}

func WriteArtifact(caseDir, filename string, data any) error {
	return toolkit.WriteArtifact(caseDir, filename, data)
}

func WriteNodePrompt(caseDir string, nodeName string, loopIter int, content string) (string, error) {
	return toolkit.WriteNodePrompt(caseDir, nodeName, loopIter, content)
}
