package rcatype

import (
	"fmt"
	"time"
)

// SourceReaderFactory creates a SourceReader from connection parameters.
type SourceReaderFactory func(baseURL, apiKeyPath, project string) (SourceReader, error)

// SourceReader reads test failure data from an external tracker.
type SourceReader interface {
	FetchEnvelope(runID string) (*Envelope, error)
	EnvelopeFetcher() EnvelopeFetcher
	CurrentUser() string
}

// DefectWriterFactory creates a DefectWriter from connection parameters.
type DefectWriterFactory func(baseURL, apiKeyPath, project, submittedBy string) (DefectWriter, error)

// RCAVerdict is the structured input for pushing RCA results.
type RCAVerdict struct {
	RunID            string   `json:"run_id"`
	CaseIDs          []string `json:"case_ids"`
	RCAMessage       string   `json:"rca_message"`
	DefectType       string   `json:"defect_type"`
	Component        string   `json:"component,omitempty"`
	ConvergenceScore float64  `json:"convergence_score"`
	EvidenceRefs     []string `json:"evidence_refs,omitempty"`
	JiraTicketID     string   `json:"jira_ticket_id,omitempty"`
	JiraLink         string   `json:"jira_link,omitempty"`
}

// DefectWriter writes RCA results back to an external system.
type DefectWriter interface {
	Push(verdict RCAVerdict) (*PushedRecord, error)
}

// PushedRecord captures the result of a defect write operation.
type PushedRecord struct {
	RunID      string
	DefectType string
}

// DefaultDefectWriter extracts defect type locally without remote API.
type DefaultDefectWriter struct{}

func (DefaultDefectWriter) Push(verdict RCAVerdict) (*PushedRecord, error) {
	return &PushedRecord{RunID: verdict.RunID, DefectType: verdict.DefectType}, nil
}

// RunInfo summarizes a CI run for the ingestion circuit.
type RunInfo struct {
	ID          int       `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Number      int       `json:"number"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"start_time"`
	FailedCount int       `json:"failed_count"`
}

// FailureInfo represents a parsed test failure from a CI run.
type FailureInfo struct {
	RunID        int    `json:"run_id"`
	RunName      string `json:"run_name"`
	ItemID       int    `json:"item_id"`
	ItemUUID     string `json:"item_uuid"`
	TestName     string `json:"test_name"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	IssueType    string `json:"issue_type,omitempty"`
	AutoAnalyzed bool   `json:"auto_analyzed,omitempty"`
}

// DedupKey generates the deduplication key for a failure.
func (f *FailureInfo) DedupKey(project string) string {
	return fmt.Sprintf("%s:%d:%d", project, f.RunID, f.ItemID)
}

// RunDiscoverer discovers available CI runs and their failures.
type RunDiscoverer interface {
	DiscoverRuns(project string, since time.Time) ([]RunInfo, error)
	FetchFailures(runID int) ([]FailureInfo, error)
}

// RunDiscovererFactory creates a RunDiscoverer from connection parameters.
type RunDiscovererFactory func(baseURL, apiKeyPath, project string) (RunDiscoverer, error)
