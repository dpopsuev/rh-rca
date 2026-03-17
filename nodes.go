package rca

import (
	"encoding/json"
)

// WalkerContextKeys used by RCA transformers and hooks to read runtime
// dependencies from the walker's context map.
const (
	KeyCaseLabel = "rca.case_label"
	KeyStore     = "rca.store"
	KeyCaseData  = "rca.case_data"
	KeyEnvelope  = "rca.envelope"
	KeyCatalog   = "rca.catalog"
	KeyCaseDir   = "rca.case_dir"
	KeyPromptFS  = "rca.prompt_fs"
)

// parseArtifact parses a JSON response into map[string]any.
// Uses cleanJSON (from cal_runner.go) to strip markdown fences, etc.
func parseArtifact(data json.RawMessage) (map[string]any, error) {
	result, err := parseJSON[map[string]any](data)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return *result, nil
}
