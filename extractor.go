package rca

import (
	"context"
	"encoding/json"
	"fmt"
)

// MapExtractor is a framework.Extractor that unmarshals JSON into map[string]any.
// All circuit steps use the same extractor — no typed generics needed.
type MapExtractor struct {
	name string
}

func NewMapExtractor(name string) *MapExtractor {
	return &MapExtractor{name: name}
}

func (e *MapExtractor) Name() string { return e.name }

func (e *MapExtractor) Extract(_ context.Context, input any) (any, error) {
	var data []byte
	switch v := input.(type) {
	case json.RawMessage:
		data = v
	case []byte:
		data = v
	default:
		return nil, fmt.Errorf("MapExtractor %q: expected json.RawMessage or []byte, got %T", e.name, input)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("MapExtractor %q: %w", e.name, err)
	}
	return result, nil
}
