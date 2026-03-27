package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dpopsuev/origami/connectors/sqlite"
	"github.com/dpopsuev/origami-rca/rcatype"
)

// MemStore is an in-memory Store for tests. Implements Store.
type MemStore struct {
	mu        sync.Mutex
	envelopes map[string]*rcatype.Envelope
	mes       *sqlite.MemEntityStore
}

// NewMemStore returns a new in-memory Store using the embedded reference schema.
// Prefer NewMemStoreWithSchema when the consumer provides its own schema.
func NewMemStore() *MemStore {
	schema, _ := LoadSchema()
	return &MemStore{
		envelopes: make(map[string]*rcatype.Envelope),
		mes:       sqlite.NewMemEntityStore(schema),
	}
}

// NewMemStoreWithSchema returns an in-memory Store using consumer-provided schema data.
func NewMemStoreWithSchema(schemaData []byte) (*MemStore, error) {
	schema, err := sqlite.ParseSchema(schemaData)
	if err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	return &MemStore{
		envelopes: make(map[string]*rcatype.Envelope),
		mes:       sqlite.NewMemEntityStore(schema),
	}, nil
}

// Close is a no-op for in-memory stores.
func (s *MemStore) Close() error { return nil }

// SaveEnvelope implements Store.
func (s *MemStore) SaveEnvelope(runID string, env *rcatype.Envelope) error {
	if env == nil {
		return errors.New("envelope is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envelopes[runID] = env
	return nil
}

// GetEnvelope implements Store.
func (s *MemStore) GetEnvelope(runID string) (*rcatype.Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.envelopes[runID], nil
}
