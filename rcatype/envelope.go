// Package rcatype defines RCA domain types decoupled from any data source.
// Both schematics/rca and schematics/rca/store import this package to
// avoid circular dependencies. Conversion to/from source-specific types
// happens inside each connector (e.g. connectors/rp/source.go).
package rcatype

// Envelope is the execution envelope (run + failure list).
// Source-agnostic: RP-specific values live in Tags (e.g. Tags["rp.launch_uuid"]).
type Envelope struct {
	RunID       string            `json:"run_id"`
	Name        string            `json:"name"`
	FailureList []FailureItem     `json:"failure_list"`
	Tags        map[string]string `json:"tags,omitempty"`

	LaunchAttributes []Attribute `json:"launch_attributes,omitempty"`
}

// Attribute is a key-value pair from launch or test item attributes.
type Attribute struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	System bool   `json:"system,omitempty"`
}

// FailureItem is one failed test in the envelope.
// Source-agnostic: ID is an opaque string, RP-specific values live in Tags.
type FailureItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Description  string `json:"description,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	LogSnippet   string `json:"log_snippet,omitempty"`

	ExternalIssues []ExternalIssue   `json:"external_issues,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// ExternalIssue links a test failure to an external bug tracker ticket.
type ExternalIssue struct {
	TicketID string `json:"ticket_id"`
	URL      string `json:"url,omitempty"`
}

// EnvelopeFetcher retrieves an envelope by run ID.
type EnvelopeFetcher interface {
	Fetch(runID string) (*Envelope, error)
}
