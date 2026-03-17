package store

import (
	"fmt"
	"time"

	"github.com/dpopsuev/origami/connectors/sqlite"
)

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// --- Suite ---

func (s *MemStore) CreateSuite(suite *InvestigationSuite) (int64, error) {
	if suite == nil {
		return 0, fmt.Errorf("suite is nil")
	}
	if suite.Status == "" {
		suite.Status = "open"
	}
	if suite.CreatedAt == "" {
		suite.CreatedAt = now()
	}
	return s.mes.Create("investigation_suites", sqlite.Row{
		"name": suite.Name, "description": suite.Description,
		"status": suite.Status, "created_at": suite.CreatedAt, "closed_at": suite.ClosedAt,
	})
}

func (s *MemStore) GetSuite(id int64) (*InvestigationSuite, error) {
	row, err := s.mes.Get("investigation_suites", id)
	if err != nil || row == nil {
		return nil, err
	}
	return suiteFromRow(row), nil
}

func (s *MemStore) ListSuites() ([]*InvestigationSuite, error) {
	rows, err := s.mes.List("investigation_suites", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, suiteFromRow), nil
}

func (s *MemStore) CloseSuite(id int64) error {
	return s.mes.Update("investigation_suites", id, sqlite.Row{
		"status": "closed", "closed_at": now(),
	})
}

// --- Version ---

func (s *MemStore) CreateVersion(ver *Version) (int64, error) {
	if ver == nil {
		return 0, fmt.Errorf("version is nil")
	}
	existing, _ := s.mes.GetBy("versions", sqlite.Row{"label": ver.Label})
	if existing != nil {
		return 0, fmt.Errorf("version label %q already exists", ver.Label)
	}
	return s.mes.Create("versions", sqlite.Row{
		"label": ver.Label, "build_id": ver.BuildID,
	})
}

func (s *MemStore) GetVersion(id int64) (*Version, error) {
	row, err := s.mes.Get("versions", id)
	if err != nil || row == nil {
		return nil, err
	}
	return versionFromRow(row), nil
}

func (s *MemStore) GetVersionByLabel(label string) (*Version, error) {
	row, err := s.mes.GetBy("versions", sqlite.Row{"label": label})
	if err != nil || row == nil {
		return nil, err
	}
	return versionFromRow(row), nil
}

func (s *MemStore) ListVersions() ([]*Version, error) {
	rows, err := s.mes.List("versions", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, versionFromRow), nil
}

// --- Circuit ---

func (s *MemStore) CreateCircuit(p *Circuit) (int64, error) {
	if p == nil {
		return 0, fmt.Errorf("circuit is nil")
	}
	return s.mes.Create("circuits", sqlite.Row{
		"suite_id": p.SuiteID, "version_id": p.VersionID, "name": p.Name,
		"source_run_id": p.SourceRunID, "status": p.Status,
		"started_at": p.StartedAt, "ended_at": p.EndedAt,
	})
}

func (s *MemStore) GetCircuit(id int64) (*Circuit, error) {
	row, err := s.mes.Get("circuits", id)
	if err != nil || row == nil {
		return nil, err
	}
	return circuitFromRow(row), nil
}

func (s *MemStore) ListCircuitsBySuite(suiteID int64) ([]*Circuit, error) {
	rows, err := s.mes.List("circuits", sqlite.Row{"suite_id": suiteID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, circuitFromRow), nil
}

// --- Launch ---

func (s *MemStore) CreateLaunch(l *Launch) (int64, error) {
	if l == nil {
		return 0, fmt.Errorf("launch is nil")
	}
	return s.mes.Create("launches", sqlite.Row{
		"circuit_id": l.CircuitID, "source_run_id": l.SourceRunID,
		"source_run_uuid": l.SourceRunUUID, "name": l.Name,
		"status": l.Status, "started_at": l.StartedAt, "ended_at": l.EndedAt,
		"env_attributes": l.EnvAttributes, "git_branch": l.GitBranch,
		"git_commit": l.GitCommit, "envelope_payload": l.EnvelopePayload,
	})
}

func (s *MemStore) GetLaunch(id int64) (*Launch, error) {
	row, err := s.mes.Get("launches", id)
	if err != nil || row == nil {
		return nil, err
	}
	return launchFromRow(row), nil
}

func (s *MemStore) GetLaunchBySourceRunID(circuitID int64, sourceRunID string) (*Launch, error) {
	row, err := s.mes.GetBy("launches", sqlite.Row{
		"circuit_id": circuitID, "source_run_id": sourceRunID,
	})
	if err != nil || row == nil {
		return nil, err
	}
	return launchFromRow(row), nil
}

func (s *MemStore) ListLaunchesByCircuit(circuitID int64) ([]*Launch, error) {
	rows, err := s.mes.List("launches", sqlite.Row{"circuit_id": circuitID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, launchFromRow), nil
}

// --- Job ---

func (s *MemStore) CreateJob(j *Job) (int64, error) {
	if j == nil {
		return 0, fmt.Errorf("job is nil")
	}
	return s.mes.Create("jobs", sqlite.Row{
		"launch_id": j.LaunchID, "source_item_id": j.SourceItemID,
		"name": j.Name, "clock_type": j.ClockType, "status": j.Status,
		"stats_total": j.StatsTotal, "stats_failed": j.StatsFailed,
		"stats_passed": j.StatsPassed, "stats_skipped": j.StatsSkipped,
		"started_at": j.StartedAt, "ended_at": j.EndedAt,
	})
}

func (s *MemStore) GetJob(id int64) (*Job, error) {
	row, err := s.mes.Get("jobs", id)
	if err != nil || row == nil {
		return nil, err
	}
	return jobFromRow(row), nil
}

func (s *MemStore) ListJobsByLaunch(launchID int64) ([]*Job, error) {
	rows, err := s.mes.List("jobs", sqlite.Row{"launch_id": launchID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, jobFromRow), nil
}

// --- Case ---

func (s *MemStore) CreateCase(c *Case) (int64, error) {
	if c == nil {
		return 0, fmt.Errorf("case is nil")
	}
	if c.Status == "" {
		c.Status = "open"
	}
	if c.CreatedAt == "" {
		c.CreatedAt = now()
	}
	c.UpdatedAt = now()
	return s.mes.Create("cases", sqlite.Row{
		"job_id": c.JobID, "launch_id": c.LaunchID,
		"source_item_id": c.SourceItemID, "name": c.Name,
		"external_ref": c.ExternalRef, "status": c.Status,
		"symptom_id": c.SymptomID, "rca_id": c.RCAID,
		"error_message": c.ErrorMessage, "log_snippet": c.LogSnippet,
		"log_truncated": boolToInt(c.LogTruncated),
		"started_at": c.StartedAt, "ended_at": c.EndedAt,
		"created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	})
}

func (s *MemStore) GetCase(id int64) (*Case, error) {
	row, err := s.mes.Get("cases", id)
	if err != nil || row == nil {
		return nil, err
	}
	return caseFromRow(row), nil
}

func (s *MemStore) ListCasesByJob(jobID int64) ([]*Case, error) {
	rows, err := s.mes.List("cases", sqlite.Row{"job_id": jobID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, caseFromRow), nil
}

func (s *MemStore) ListCasesBySymptom(symptomID int64) ([]*Case, error) {
	rows, err := s.mes.List("cases", sqlite.Row{"symptom_id": symptomID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, caseFromRow), nil
}

func (s *MemStore) UpdateCaseStatus(caseID int64, status string) error {
	return s.mes.Update("cases", caseID, sqlite.Row{
		"status": status, "updated_at": now(),
	})
}

func (s *MemStore) LinkCaseToRCA(caseID, rcaID int64) error {
	return s.mes.Update("cases", caseID, sqlite.Row{"rca_id": rcaID})
}

func (s *MemStore) LinkCaseToSymptom(caseID, symptomID int64) error {
	return s.mes.Update("cases", caseID, sqlite.Row{
		"symptom_id": symptomID, "updated_at": now(),
	})
}

// --- Triage ---

func (s *MemStore) CreateTriage(t *Triage) (int64, error) {
	if t == nil {
		return 0, fmt.Errorf("triage is nil")
	}
	if t.CreatedAt == "" {
		t.CreatedAt = now()
	}
	return s.mes.Create("triages", sqlite.Row{
		"case_id": t.CaseID, "symptom_category": t.SymptomCategory,
		"severity": t.Severity, "defect_type_hypothesis": t.DefectTypeHypothesis,
		"skip_investigation": boolToInt(t.SkipInvestigation),
		"clock_skew_suspected": boolToInt(t.ClockSkewSuspected),
		"cascade_suspected": boolToInt(t.CascadeSuspected),
		"candidate_repos": t.CandidateRepos, "data_quality_notes": t.DataQualityNotes,
		"created_at": t.CreatedAt,
	})
}

func (s *MemStore) GetTriageByCase(caseID int64) (*Triage, error) {
	row, err := s.mes.GetBy("triages", sqlite.Row{"case_id": caseID})
	if err != nil || row == nil {
		return nil, err
	}
	return triageFromRow(row), nil
}

// --- Symptom ---

func (s *MemStore) CreateSymptom(sym *Symptom) (int64, error) {
	if sym == nil {
		return 0, fmt.Errorf("symptom is nil")
	}
	existing, _ := s.mes.GetBy("symptoms", sqlite.Row{"fingerprint": sym.Fingerprint})
	if existing != nil {
		return 0, fmt.Errorf("symptom with fingerprint %q already exists", sym.Fingerprint)
	}
	if sym.Status == "" {
		sym.Status = "active"
	}
	if sym.OccurrenceCount == 0 {
		sym.OccurrenceCount = 1
	}
	if sym.FirstSeenAt == "" {
		sym.FirstSeenAt = now()
	}
	if sym.LastSeenAt == "" {
		sym.LastSeenAt = sym.FirstSeenAt
	}
	return s.mes.Create("symptoms", sqlite.Row{
		"fingerprint": sym.Fingerprint, "name": sym.Name,
		"description": sym.Description, "error_pattern": sym.ErrorPattern,
		"test_name_pattern": sym.TestNamePattern, "component": sym.Component,
		"severity": sym.Severity, "first_seen_at": sym.FirstSeenAt,
		"last_seen_at": sym.LastSeenAt, "occurrence_count": sym.OccurrenceCount,
		"status": sym.Status,
	})
}

func (s *MemStore) GetSymptom(id int64) (*Symptom, error) {
	row, err := s.mes.Get("symptoms", id)
	if err != nil || row == nil {
		return nil, err
	}
	return symptomFromRow(row), nil
}

func (s *MemStore) GetSymptomByFingerprint(fingerprint string) (*Symptom, error) {
	row, err := s.mes.GetBy("symptoms", sqlite.Row{"fingerprint": fingerprint})
	if err != nil || row == nil {
		return nil, err
	}
	return symptomFromRow(row), nil
}

func (s *MemStore) FindSymptomCandidates(testName string) ([]*Symptom, error) {
	if testName == "" {
		return nil, nil
	}
	rows, err := s.mes.List("symptoms", sqlite.Row{"name": testName}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomFromRow), nil
}

func (s *MemStore) UpdateSymptomSeen(id int64) error {
	return s.mes.Mutate("symptoms", id, func(r sqlite.Row) {
		r["occurrence_count"] = r.Int64("occurrence_count") + 1
		r["last_seen_at"] = now()
		if r.String("status") == "dormant" {
			r["status"] = "active"
		}
	})
}

func (s *MemStore) ListSymptoms() ([]*Symptom, error) {
	rows, err := s.mes.List("symptoms", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomFromRow), nil
}

// SnapshotSymptoms returns a copy of all current symptoms.
func (s *MemStore) SnapshotSymptoms() []*Symptom {
	rows, _ := s.mes.List("symptoms", nil, "")
	return rowsTo(rows, symptomFromRow)
}

func (s *MemStore) MarkDormantSymptoms(staleDays int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -staleDays).Format(time.RFC3339)
	return s.mes.MutateAll("symptoms", func(r sqlite.Row) bool {
		if r.String("status") == "active" && r.String("last_seen_at") < cutoff {
			r["status"] = "dormant"
			return true
		}
		return false
	})
}

// --- RCA ---

func (s *MemStore) SaveRCA(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, fmt.Errorf("rca is nil")
	}
	if rca.ID != 0 {
		existing, _ := s.mes.Get("rcas", rca.ID)
		if existing != nil {
			return rca.ID, s.mes.Update("rcas", rca.ID, sqlite.Row{
				"title": rca.Title, "description": rca.Description,
				"defect_type": rca.DefectType, "category": rca.Category,
				"component": rca.Component, "affected_versions": rca.AffectedVersions,
				"evidence_refs": rca.EvidenceRefs, "convergence_score": rca.ConvergenceScore,
				"jira_ticket_id": rca.JiraTicketID, "jira_link": rca.JiraLink,
				"status": rca.Status, "resolved_at": rca.ResolvedAt,
				"verified_at": rca.VerifiedAt, "archived_at": rca.ArchivedAt,
			})
		}
	}
	if rca.Status == "" {
		rca.Status = "open"
	}
	if rca.CreatedAt == "" {
		rca.CreatedAt = now()
	}
	return s.mes.Create("rcas", sqlite.Row{
		"title": rca.Title, "description": rca.Description,
		"defect_type": rca.DefectType, "category": rca.Category,
		"component": rca.Component, "affected_versions": rca.AffectedVersions,
		"evidence_refs": rca.EvidenceRefs, "convergence_score": rca.ConvergenceScore,
		"jira_ticket_id": rca.JiraTicketID, "jira_link": rca.JiraLink,
		"status": rca.Status, "created_at": rca.CreatedAt,
		"resolved_at": rca.ResolvedAt, "verified_at": rca.VerifiedAt,
		"archived_at": rca.ArchivedAt,
	})
}

func (s *MemStore) GetRCA(id int64) (*RCA, error) {
	row, err := s.mes.Get("rcas", id)
	if err != nil || row == nil {
		return nil, err
	}
	return rcaFromRow(row), nil
}

func (s *MemStore) ListRCAs() ([]*RCA, error) {
	rows, err := s.mes.List("rcas", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, rcaFromRow), nil
}

func (s *MemStore) ListRCAsByStatus(status string) ([]*RCA, error) {
	rows, err := s.mes.List("rcas", sqlite.Row{"status": status}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, rcaFromRow), nil
}

func (s *MemStore) UpdateRCAStatus(id int64, status string) error {
	set := sqlite.Row{"status": status}
	switch status {
	case "resolved":
		set["resolved_at"] = now()
	case "verified":
		set["verified_at"] = now()
	case "archived":
		set["archived_at"] = now()
	case "open":
		set["resolved_at"] = ""
		set["verified_at"] = ""
	}
	return s.mes.Update("rcas", id, set)
}

// --- SymptomRCA ---

func (s *MemStore) LinkSymptomToRCA(link *SymptomRCA) (int64, error) {
	if link == nil {
		return 0, fmt.Errorf("link is nil")
	}
	// Check for duplicate
	rows, _ := s.mes.List("symptom_rca", sqlite.Row{
		"symptom_id": link.SymptomID, "rca_id": link.RCAID,
	}, "")
	if len(rows) > 0 {
		return 0, fmt.Errorf("symptom-rca link already exists")
	}
	if link.LinkedAt == "" {
		link.LinkedAt = now()
	}
	return s.mes.Create("symptom_rca", sqlite.Row{
		"symptom_id": link.SymptomID, "rca_id": link.RCAID,
		"confidence": link.Confidence, "notes": link.Notes,
		"linked_at": link.LinkedAt,
	})
}

func (s *MemStore) GetRCAsForSymptom(symptomID int64) ([]*SymptomRCA, error) {
	rows, err := s.mes.List("symptom_rca", sqlite.Row{"symptom_id": symptomID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomRCAFromRow), nil
}

func (s *MemStore) GetSymptomsForRCA(rcaID int64) ([]*SymptomRCA, error) {
	rows, err := s.mes.List("symptom_rca", sqlite.Row{"rca_id": rcaID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomRCAFromRow), nil
}
