package rca

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/origami"
)

func TestMapExtractor_ImplementsExtractor(t *testing.T) {
	var ext framework.Extractor = NewMapExtractor("test-step")
	if ext.Name() != "test-step" {
		t.Errorf("Name() = %q, want %q", ext.Name(), "test-step")
	}
}

func TestMapExtractor_RawMessage(t *testing.T) {
	ext := NewMapExtractor("step-raw")
	input := json.RawMessage(`{"status":"ok","score":99}`)
	result, err := ext.Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if m["status"] != "ok" {
		t.Errorf("status = %v, want ok", m["status"])
	}
	if m["score"] != float64(99) {
		t.Errorf("score = %v, want 99", m["score"])
	}
}

func TestMapExtractor_Bytes(t *testing.T) {
	ext := NewMapExtractor("step-bytes")
	result, err := ext.Extract(context.Background(), []byte(`{"status":"done","score":1}`))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m := result.(map[string]any)
	if m["status"] != "done" {
		t.Errorf("status = %v, want done", m["status"])
	}
}

func TestMapExtractor_MalformedJSON(t *testing.T) {
	ext := NewMapExtractor("step-bad")
	_, err := ext.Extract(context.Background(), json.RawMessage(`{broken`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestMapExtractor_WrongType(t *testing.T) {
	ext := NewMapExtractor("step-type")
	_, err := ext.Extract(context.Background(), "not bytes")
	if err == nil {
		t.Fatal("expected error for string input")
	}
}

func TestMapExtractor_MatchesParseJSON(t *testing.T) {
	data := json.RawMessage(`{"status":"match","score":42}`)

	oldResult, err := parseJSON[map[string]any](data)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}

	ext := NewMapExtractor("match-test")
	newResult, err := ext.Extract(context.Background(), data)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m := newResult.(map[string]any)

	if m["status"] != (*oldResult)["status"] || m["score"] != (*oldResult)["score"] {
		t.Errorf("mismatch: extractor=%v vs parseJSON=%v", m, *oldResult)
	}
}
