package rca

import (
	"sort"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/toolkit"
)

// ResolutionStatus is the RCA-specific alias for toolkit.ResolutionStatus.
type ResolutionStatus = toolkit.ResolutionStatus

const (
	Resolved    = toolkit.Resolved
	Unavailable = toolkit.Unavailable
)

// TemplateParams holds all parameter groups injected into prompt templates.
// Templates use {{.Group.Field}} to access values.
type TemplateParams struct {
	SourceID string
	CaseID   int64
	StepName string

	Envelope *EnvelopeParams

	Env map[string]string

	Git *GitParams

	Failure *FailureParams

	Siblings []SiblingParams

	Sources *SourceParams

	URLs *URLParams

	Prior *PriorParams

	History *HistoryParams

	// Recall digest: all RCAs discovered so far in the current run.
	// Populated at F0_RECALL to enable cross-case recall in parallel mode.
	RecallDigest []RecallDigestEntry

	Taxonomy *TaxonomyParams

	Timestamps *TimestampParams

	Code *CodeParams
}

// EnvelopeParams holds envelope-level context.
type EnvelopeParams struct {
	Name   string
	RunID  string
	Status string
}

// GitParams holds git metadata from the envelope.
type GitParams struct {
	Branch string
	Commit string
}

// FailureParams holds the failure under investigation.
type FailureParams struct {
	TestName     string
	ErrorMessage string
	LogSnippet   string
	LogTruncated bool
	Status       string
	Path         string
}

// SiblingParams holds a sibling failure for context.
type SiblingParams struct {
	ID     string
	Name   string
	Status string
}

// ResolutionStatus, Resolved, and Unavailable are defined above as
// aliases to toolkit.ResolutionStatus for backward compatibility.

// SourceParams holds repo list, launch attributes, Jira links, and always-read sources.
type SourceParams struct {
	Repos            []RepoParams
	LaunchAttributes []AttributeParams
	JiraLinks        []JiraLinkParams
	AttrsStatus      ResolutionStatus
	JiraStatus       ResolutionStatus
	ReposStatus      ResolutionStatus
	AlwaysRead       []AlwaysReadSource
}

// RepoParams holds one repo's metadata.
type RepoParams struct {
	Name    string
	Path    string
	Purpose string
	Branch  string
}

// AttributeParams holds a key-value launch attribute from the data source.
type AttributeParams struct {
	Key    string
	Value  string
	System bool
}

// JiraLinkParams holds an external issue link from test items.
type JiraLinkParams struct {
	TicketID string
	URL      string
}

// URLParams holds pre-built navigable links.
type URLParams struct {
	SourceDashboard string
	SourceItem      string
}

// AlwaysReadSource holds the content of a GND source that is always
// loaded regardless of routing rules (ReadPolicy == ReadAlways).
type AlwaysReadSource struct {
	Name    string
	Purpose string
	Content string
}

// PriorParams holds prior stage artifacts for context injection.
// Keys are step names (e.g. "Recall", "Triage"), values are the
// deserialized JSON artifact for that step. Templates access fields
// via {{.Prior.Triage.symptom_category}} — Go's template engine
// resolves map keys the same way it resolves struct fields.
type PriorParams map[string]map[string]any

// HistoryParams holds historical data from the Store.
type HistoryParams struct {
	SymptomInfo *SymptomInfoParams
	PriorRCAs   []PriorRCAParams
}

// SymptomInfoParams holds cross-version symptom knowledge.
type SymptomInfoParams struct {
	Name                  string
	OccurrenceCount       int
	FirstSeen             string
	LastSeen              string
	Status                string
	IsDormantReactivation bool
}

// PriorRCAParams holds a prior RCA for history injection.
type PriorRCAParams struct {
	ID                int64
	Title             string
	DefectType        string
	Status            string
	AffectedVersions  string
	JiraLink          string
	ResolvedAt        string
	DaysSinceResolved int
}

// RecallDigestEntry summarizes one RCA for the recall digest.
type RecallDigestEntry struct {
	ID         int64
	Component  string
	DefectType string
	Summary    string
}

// TaxonomyParams holds defect type vocabulary.
type TaxonomyParams struct {
	DefectTypes string
}

// TimestampParams holds clock plane warnings.
type TimestampParams struct {
	ClockPlaneNote   string
	ClockSkewWarning string
}

// CodeParams holds injected code context from source repositories.
type CodeParams struct {
	Trees         []CodeTreeParams   `json:"trees,omitempty"`
	SearchResults []CodeSearchResult `json:"search_results,omitempty"`
	Files         []CodeFileParams   `json:"files,omitempty"`
	Truncated     bool               `json:"truncated,omitempty"`
}

// CodeTreeParams holds a repository's directory tree.
type CodeTreeParams struct {
	Repo    string      `json:"repo"`
	Branch  string      `json:"branch"`
	Entries []TreeEntry `json:"entries"`
}

// CodeSearchResult holds a code search match.
type CodeSearchResult struct {
	Repo    string  `json:"repo"`
	File    string  `json:"file"`
	Line    int     `json:"line"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// TreeEntry represents a file or directory in a repository tree.
type TreeEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// CodeFileParams holds the content of a single source file.
type CodeFileParams struct {
	Repo      string `json:"repo"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// DefaultTaxonomy builds the defect type taxonomy from the loaded vocabulary.
// Falls back to an empty taxonomy if no vocabulary has been initialized.
func DefaultTaxonomy() *TaxonomyParams {
	return TaxonomyFromDefectTypes(defaultDefectTypes)
}

// TaxonomyFromDefectTypes builds a TaxonomyParams from a set of defect type
// entries. Each entry is rendered as "- code: Long Name" (with optional
// description appended as " — description"). Used for prompt injection.
func TaxonomyFromDefectTypes(types map[string]circuit.VocabEntry) *TaxonomyParams {
	if len(types) == 0 {
		return &TaxonomyParams{}
	}

	codes := make([]string, 0, len(types))
	for code := range types {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	var b strings.Builder
	b.WriteString("Defect types:")
	for _, code := range codes {
		e := types[code]
		name := e.Long
		if name == "" {
			name = e.Short
		}
		if name == "" {
			name = code
		}
		b.WriteString("\n- ")
		b.WriteString(code)
		b.WriteString(": ")
		b.WriteString(name)
		if e.Description != "" {
			b.WriteString(" — ")
			b.WriteString(e.Description)
		}
	}
	return &TaxonomyParams{DefectTypes: b.String()}
}
