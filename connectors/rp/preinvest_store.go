package rp

// EnvelopeStore persists and retrieves envelopes by launch ID.
type EnvelopeStore interface {
	Save(launchID int, envelope *Envelope) error
	Get(launchID int) (*Envelope, error)
}

// MemEnvelopeStore is an in-memory envelope store for tests.
type MemEnvelopeStore struct {
	envelopes map[int]*Envelope
}

// NewMemEnvelopeStore returns a new in-memory envelope store.
func NewMemEnvelopeStore() *MemEnvelopeStore {
	return &MemEnvelopeStore{envelopes: make(map[int]*Envelope)}
}

// Save stores the envelope by launch ID.
func (s *MemEnvelopeStore) Save(launchID int, envelope *Envelope) error {
	s.envelopes[launchID] = envelope
	return nil
}

// Get returns the envelope for the launch ID, or nil if not found.
func (s *MemEnvelopeStore) Get(launchID int) (*Envelope, error) {
	return s.envelopes[launchID], nil
}
