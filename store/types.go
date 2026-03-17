package store

// --- Tier 1: Investigation-scoped entities (the execution tree) ---

// InvestigationSuite is the top-level grouping for a regression analysis.
// An analyst opens a suite when starting an analysis (e.g. "PTP Feb 2026 regression").
type InvestigationSuite struct {
	ID          int64
	Name        string
	Description string
	Status      string // open / closed
	CreatedAt   string // ISO 8601
	ClosedAt    string // ISO 8601; empty if open
}

// Version represents a product version. Global reference table.
type Version struct {
	ID      int64
	Label   string // e.g. "4.21"
	BuildID string // e.g. "4.21.2"
}

// Circuit represents one CI circuit run for one version, bound to a suite.
type Circuit struct {
	ID          int64
	SuiteID     int64
	VersionID   int64
	Name        string
	SourceRunID string // denormalized from Launch for quick reference
	Status      string // FAILED, PASSED, etc.
	StartedAt   string // ISO 8601
	EndedAt     string // ISO 8601
}

// Launch represents one source launch/run. Evolves from the flat envelopes blob.
// 1:1 with Circuit for now; schema allows N launches per circuit for future.
type Launch struct {
	ID              int64
	CircuitID       int64
	SourceRunID     string // source run ID (e.g. "33195")
	SourceRunUUID   string
	Name            string
	Status          string // FAILED, PASSED, etc.
	StartedAt       string // ISO 8601
	EndedAt         string // ISO 8601
	EnvAttributes   string // JSON blob of all environment attributes
	GitBranch       string // from envelope git metadata; may be empty
	GitCommit       string // from envelope git metadata; may be empty
	EnvelopePayload []byte // full envelope JSON for backward compat
}

// Job represents a test execution group within a launch (TEST-level item).
type Job struct {
	ID           int64
	LaunchID     int64  // FK → launches.id
	SourceItemID string // source item ID for this TEST-level item
	Name         string // e.g. "[T-TSC] RAN PTP tests"
	ClockType    string // extracted from name/attributes, e.g. "T-TSC"
	Status       string // FAILED, PASSED, etc.
	StatsTotal   int
	StatsFailed  int
	StatsPassed  int
	StatsSkipped int
	StartedAt    string // ISO 8601
	EndedAt      string // ISO 8601
}

// Case is one failure — the leaf of the execution tree. The unit of agent work.
// A Case reports a Symptom (the story it observed). The canonical path to Root Cause
// is always through the Symptom: Case → Symptom → SymptomRCA → RCA.
type Case struct {
	ID           int64
	JobID        int64  // v2: FK → jobs.id; 0 for v1-migrated cases
	LaunchID     int64  // FK → launches.id (v2) or source run ID (v1 before migration)
	SourceItemID string // source test item ID (STEP-level item)
	Name         string // full test name from source
	ExternalRef  string // optional external test case ID (e.g. Polarion)
	Status       string // open / triaged / investigated / reviewed / closed
	SymptomID    int64  // FK → symptoms.id; 0 = not yet matched
	RCAID        int64  // FK → rcas.id; 0 = not yet resolved (denormalized verdict)
	ErrorMessage string // error message from item logs
	LogSnippet   string // truncated log excerpt
	LogTruncated bool   // true if log was truncated
	StartedAt    string // ISO 8601
	EndedAt      string // ISO 8601
	CreatedAt    string // when the case was created in the DB
	UpdatedAt    string // last update
}

// Triage captures F1 output per case. One triage per case.
type Triage struct {
	ID                   int64
	CaseID               int64  // FK → cases.id (one triage per case)
	SymptomCategory      string // timeout, assertion, crash, infra, config, flake, unknown
	Severity             string
	DefectTypeHypothesis string // initial defect type guess (e.g. "pb001")
	SkipInvestigation    bool
	ClockSkewSuspected   bool
	CascadeSuspected     bool
	CandidateRepos       string // JSON array of repo names ranked by relevance
	DataQualityNotes     string
	CreatedAt            string // ISO 8601
}

// --- Tier 2: Global knowledge entities (institutional memory) ---

// Symptom is a recognized failure pattern — the "story" that test cases report.
// Identified by a deterministic fingerprint. Cross-version: the same symptom can
// appear in 4.20, 4.21, 4.22.
type Symptom struct {
	ID              int64
	Fingerprint     string // deterministic hash: normalize(test_name_pattern + error_pattern + component)
	Name            string
	Description     string
	ErrorPattern    string // normalized error snippet or regex
	TestNamePattern string // test name pattern (may include wildcards)
	Component       string // associated component (e.g. "ptp-operator")
	Severity        string
	FirstSeenAt     string // ISO 8601
	LastSeenAt      string // ISO 8601
	OccurrenceCount int    // incremented on each new match; default 1
	Status          string // active / dormant / resolved
}

// RCA is a root-cause analysis record — the "criminal" in the witness/story/criminal model.
// Cross-version, cross-suite. One RCA can cause many symptoms; one symptom can
// (in different contexts) be caused by different RCAs.
type RCA struct {
	ID               int64
	Title            string
	Description      string
	DefectType       string  // e.g. "pb001", "au001", "ti001"
	Category         string  // product / automation / system / infra / config
	Component        string  // primary component (e.g. "linuxptp-daemon")
	AffectedVersions string  // JSON array (e.g. `["4.20","4.21"]`)
	EvidenceRefs     string  // JSON array of evidence references
	ConvergenceScore float64 // confidence 0–1 from last investigation
	JiraTicketID     string
	JiraLink         string
	Status           string // open / resolved / verified / archived
	CreatedAt        string // ISO 8601
	ResolvedAt       string // when fix was identified/merged
	VerifiedAt       string // when fix was confirmed in CI
	ArchivedAt       string // when RCA became irrelevant
}

// SymptomRCA links symptoms to RCAs (many-to-many junction table).
// One story can point to different criminals in different contexts;
// one criminal can produce different stories.
type SymptomRCA struct {
	ID         int64
	SymptomID  int64
	RCAID      int64
	Confidence float64 // confidence that this RCA explains this symptom (0–1)
	Notes      string
	LinkedAt   string // ISO 8601
}
