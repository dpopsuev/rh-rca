package rp

// EnvelopeFetcher returns an envelope for a launch ID (e.g. from RP API or stub).
type EnvelopeFetcher interface {
	Fetch(launchID int) (*Envelope, error)
}

// StubFetcher returns a fixed envelope for any launch ID (mock; no HTTP).
type StubFetcher struct {
	Env *Envelope
}

// Fetch implements EnvelopeFetcher by returning the fixed envelope.
func (f *StubFetcher) Fetch(launchID int) (*Envelope, error) {
	return f.Env, nil
}

// NewStubFetcher returns an EnvelopeFetcher that always returns env.
func NewStubFetcher(env *Envelope) *StubFetcher {
	return &StubFetcher{Env: env}
}
