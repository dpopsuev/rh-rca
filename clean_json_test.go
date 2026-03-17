package rca

import (
	"encoding/json"
	"testing"
)

func TestCleanJSON_BareJSON(t *testing.T) {
	input := []byte(`{"match":false,"confidence":0.1}`)
	got := cleanJSON(input)
	if !json.Valid(got) {
		t.Errorf("cleanJSON returned invalid JSON: %s", got)
	}
}

func TestCleanJSON_MarkdownCodeFence(t *testing.T) {
	input := []byte("```json\n{\"match\":false,\"confidence\":0.1}\n```")
	got := cleanJSON(input)
	if !json.Valid(got) {
		t.Errorf("cleanJSON returned invalid JSON: %s", got)
	}
	if string(got) != `{"match":false,"confidence":0.1}` {
		t.Errorf("cleanJSON = %s, want bare JSON", got)
	}
}

func TestCleanJSON_MarkdownNoLang(t *testing.T) {
	input := []byte("```\n{\"key\":\"value\"}\n```")
	got := cleanJSON(input)
	if !json.Valid(got) {
		t.Errorf("cleanJSON returned invalid JSON: %s", got)
	}
}

func TestCleanJSON_WhitespaceWrapped(t *testing.T) {
	input := []byte("  \n  {\"key\":\"value\"}  \n  ")
	got := cleanJSON(input)
	if !json.Valid(got) {
		t.Errorf("cleanJSON returned invalid JSON: %s", got)
	}
}

func TestCleanJSON_EmptyInput(t *testing.T) {
	got := cleanJSON([]byte(""))
	if len(got) != 0 {
		t.Errorf("cleanJSON on empty input returned: %s", got)
	}
}

func TestParseJSON_WithCodeFence(t *testing.T) {
	type simple struct {
		Key   string  `json:"key"`
		Value float64 `json:"value"`
	}
	input := json.RawMessage("```json\n{\"key\":\"test\",\"value\":0.5}\n```")
	result, err := parseJSON[simple](input)
	if err != nil {
		t.Fatalf("parseJSON with code fence failed: %v", err)
	}
	if result.Value != 0.5 {
		t.Errorf("Value = %v, want 0.5", result.Value)
	}
	if result.Key != "test" {
		t.Errorf("Key = %v, want test", result.Key)
	}
}

func TestParseJSON_BareJSON(t *testing.T) {
	type simple struct {
		Key string `json:"key"`
	}
	input := json.RawMessage(`{"key":"test"}`)
	result, err := parseJSON[simple](input)
	if err != nil {
		t.Fatalf("parseJSON with bare JSON failed: %v", err)
	}
	if result.Key != "test" {
		t.Errorf("Key = %v, want test", result.Key)
	}
}
