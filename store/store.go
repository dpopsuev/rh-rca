package store

import "github.com/dpopsuev/rh-rca/rcatype"

// DefaultDBPath is the default relative path for the SQLite DB (per-workspace).
// Resolve against cwd or workspace root; Open() creates the parent dir (e.g. .asterisk).
const DefaultDBPath = ".asterisk/asterisk.db"

// Store is the persistence facade for the two-tier data model.
// Tier 1: investigation-scoped entities (suite, circuit, launch, job, case, triage).
// Tier 2: global knowledge entities (symptom, rca, symptom_rca).
// Domain and CLI code use only this interface; implementation is SQLite or in-memory.
type Store interface {
	// Close releases underlying resources (e.g. database connections).
	Close() error

	// ---------------------------------------------------------------
	// Investigation-scoped entities
	// ---------------------------------------------------------------

	// Suite operations
	CreateSuite(suite *InvestigationSuite) (int64, error)
	GetSuite(id int64) (*InvestigationSuite, error)
	ListSuites() ([]*InvestigationSuite, error)
	CloseSuite(id int64) error

	// Version operations
	CreateVersion(v *Version) (int64, error)
	GetVersion(id int64) (*Version, error)
	GetVersionByLabel(label string) (*Version, error)
	ListVersions() ([]*Version, error)

	// Circuit operations
	CreateCircuit(p *Circuit) (int64, error)
	GetCircuit(id int64) (*Circuit, error)
	ListCircuitsBySuite(suiteID int64) ([]*Circuit, error)

	// Launch operations
	CreateLaunch(l *Launch) (int64, error)
	GetLaunch(id int64) (*Launch, error)
	GetLaunchBySourceRunID(circuitID int64, sourceRunID string) (*Launch, error)
	ListLaunchesByCircuit(circuitID int64) ([]*Launch, error)
	// SaveEnvelope stores an envelope blob by source run ID.
	SaveEnvelope(runID string, env *rcatype.Envelope) error
	// GetEnvelope returns the envelope for the source run ID.
	GetEnvelope(runID string) (*rcatype.Envelope, error)

	// Job operations
	CreateJob(j *Job) (int64, error)
	GetJob(id int64) (*Job, error)
	ListJobsByLaunch(launchID int64) ([]*Job, error)

	// Case operations
	CreateCase(c *Case) (int64, error)
	GetCase(id int64) (*Case, error)
	ListCasesByJob(jobID int64) ([]*Case, error)
	ListCasesBySymptom(symptomID int64) ([]*Case, error)
	UpdateCaseStatus(caseID int64, status string) error
	// LinkCaseToRCA sets case.rca_id (the verdict shortcut).
	LinkCaseToRCA(caseID, rcaID int64) error
	LinkCaseToSymptom(caseID, symptomID int64) error

	// Triage operations
	CreateTriage(t *Triage) (int64, error)
	GetTriageByCase(caseID int64) (*Triage, error)

	// ---------------------------------------------------------------
	// Global knowledge entities
	// ---------------------------------------------------------------

	// Symptom operations
	CreateSymptom(s *Symptom) (int64, error)
	GetSymptom(id int64) (*Symptom, error)
	GetSymptomByFingerprint(fingerprint string) (*Symptom, error)
	// FindSymptomCandidates returns symptoms whose Name matches testName exactly.
	// Used at F0_RECALL to find prior data before a fingerprint (which requires
	// the triage category) is available. Returns nil slice if no match.
	FindSymptomCandidates(testName string) ([]*Symptom, error)
	// UpdateSymptomSeen increments occurrence_count and updates last_seen_at.
	UpdateSymptomSeen(id int64) error
	ListSymptoms() ([]*Symptom, error)
	// MarkDormantSymptoms transitions active symptoms older than staleDays to dormant.
	// Returns the number of rows affected.
	MarkDormantSymptoms(staleDays int) (int64, error)

	// RCA operations
	SaveRCA(rca *RCA) (int64, error)
	GetRCA(id int64) (*RCA, error)
	// ListRCAs returns all RCAs.
	ListRCAs() ([]*RCA, error)
	ListRCAsByStatus(status string) ([]*RCA, error)
	UpdateRCAStatus(id int64, status string) error

	// SymptomRCA operations (junction table)
	LinkSymptomToRCA(link *SymptomRCA) (int64, error)
	GetRCAsForSymptom(symptomID int64) ([]*SymptomRCA, error)
	GetSymptomsForRCA(rcaID int64) ([]*SymptomRCA, error)
}
