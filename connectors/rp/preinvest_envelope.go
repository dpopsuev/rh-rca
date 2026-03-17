package rp

// Envelope is the execution envelope (launch + failure list).
// For mock skeleton we keep minimal fields; full shape matches examples/pre-investigation-33195-4.21.
type Envelope struct {
	RunID       string `json:"run_id"`
	LaunchUUID  string `json:"launch_uuid"`
	Name        string `json:"name"`
	FailureList []FailureItem `json:"failure_list"`

	// LaunchAttributes from RP launch (key-value pairs like OCP version, cluster name).
	LaunchAttributes []Attribute `json:"launch_attributes,omitempty"`
}

// Attribute is a key-value pair from RP launch or test item attributes.
type Attribute struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	System bool   `json:"system,omitempty"`
}

// FailureItem is one failed step (leaf) in the envelope.
type FailureItem struct {
	ID     int    `json:"id"`
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Path   string `json:"path"`

	// Enriched fields (populated by rp.FetchEnvelope from TestItemResource).
	CodeRef      string `json:"code_ref,omitempty"`
	Description  string `json:"description,omitempty"`
	ParentID     int    `json:"parent_id,omitempty"`
	IssueType      string `json:"issue_type,omitempty"`
	IssueComment   string `json:"issue_comment,omitempty"`
	AutoAnalyzed   bool   `json:"auto_analyzed,omitempty"`

	// ExternalIssues are Jira/BTS ticket links from RP test item issues.
	ExternalIssues []ExternalIssue `json:"external_issues,omitempty"`
}

// ExternalIssue links a test failure to an external bug tracker ticket.
type ExternalIssue struct {
	TicketID string `json:"ticket_id"`
	URL      string `json:"url,omitempty"`
}
