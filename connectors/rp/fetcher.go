package rp

import "context"

// Fetcher implements EnvelopeFetcher by calling the RP API via the
// scope-based client.
type Fetcher struct {
	client  *Client
	project string
}

// NewFetcher returns a Fetcher that uses the given client and project.
func NewFetcher(client *Client, project string) *Fetcher {
	return &Fetcher{client: client, project: project}
}

// Fetch implements EnvelopeFetcher.
func (f *Fetcher) Fetch(launchID int) (*Envelope, error) {
	return f.client.Project(f.project).FetchEnvelope(context.Background(), launchID)
}
