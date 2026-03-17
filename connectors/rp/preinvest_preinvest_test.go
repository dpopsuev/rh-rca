package rp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixtureEnvelope(t *testing.T) *Envelope {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "pre-investigation-33195-4.21", "envelope_33195_4.21.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture not found (run from repo root): %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &env
}

func TestFetchAndSave_StoresEnvelopeByLaunchID(t *testing.T) {
	launchID := 33195
	env := fixtureEnvelope(t)
	fetcher := NewStubFetcher(env)
	store := NewMemEnvelopeStore()

	err := FetchAndSave(fetcher, store, launchID)
	if err != nil {
		t.Fatalf("FetchAndSave: %v", err)
	}

	got, err := store.Get(launchID)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if got == nil {
		t.Fatal("envelope not stored (Get returned nil)")
	}
	if got.RunID != env.RunID {
		t.Errorf("RunID: got %q want %q", got.RunID, env.RunID)
	}
	if len(got.FailureList) != len(env.FailureList) {
		t.Errorf("FailureList len: got %d want %d", len(got.FailureList), len(env.FailureList))
	}
}

func TestMemEnvelopeStore_GetUnknownLaunchIDReturnsNil(t *testing.T) {
	store := NewMemEnvelopeStore()
	got, err := store.Get(33199)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("Get(unknown): got %v want nil", got)
	}
}
