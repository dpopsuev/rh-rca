package rp


// EnvelopeSource provides an envelope by launch ID (e.g. pre-investigation store).
type EnvelopeSource interface {
	Get(launchID int) (*Envelope, error)
}
