package store

import (
	"fmt"

	"github.com/dpopsuev/origami/connectors/sqlite"
)

// --- Row ↔ struct conversion helpers ---

func suiteFromRow(r sqlite.Row) *InvestigationSuite {
	return &InvestigationSuite{
		ID: r.Int64("id"), Name: r.String("name"), Description: r.String("description"),
		Status: r.String("status"), CreatedAt: r.String("created_at"), ClosedAt: r.String("closed_at"),
	}
}

func versionFromRow(r sqlite.Row) *Version {
	return &Version{ID: r.Int64("id"), Label: r.String("label"), BuildID: r.String("build_id")}
}

func circuitFromRow(r sqlite.Row) *Circuit {
	return &Circuit{
		ID: r.Int64("id"), SuiteID: r.Int64("suite_id"), VersionID: r.Int64("version_id"),
		Name: r.String("name"), SourceRunID: r.String("source_run_id"), Status: r.String("status"),
		StartedAt: r.String("started_at"), EndedAt: r.String("ended_at"),
	}
}

func launchFromRow(r sqlite.Row) *Launch {
	return &Launch{
		ID: r.Int64("id"), CircuitID: r.Int64("circuit_id"),
		SourceRunID: r.String("source_run_id"), SourceRunUUID: r.String("source_run_uuid"),
		Name: r.String("name"), Status: r.String("status"),
		StartedAt: r.String("started_at"), EndedAt: r.String("ended_at"),
		EnvAttributes: r.String("env_attributes"), GitBranch: r.String("git_branch"),
		GitCommit: r.String("git_commit"), EnvelopePayload: r.Bytes("envelope_payload"),
	}
}

func jobFromRow(r sqlite.Row) *Job {
	return &Job{
		ID: r.Int64("id"), LaunchID: r.Int64("launch_id"),
		SourceItemID: r.String("source_item_id"), Name: r.String("name"),
		ClockType: r.String("clock_type"), Status: r.String("status"),
		StatsTotal: r.Int("stats_total"), StatsFailed: r.Int("stats_failed"),
		StatsPassed: r.Int("stats_passed"), StatsSkipped: r.Int("stats_skipped"),
		StartedAt: r.String("started_at"), EndedAt: r.String("ended_at"),
	}
}

func caseFromRow(r sqlite.Row) *Case {
	return &Case{
		ID: r.Int64("id"), JobID: r.Int64("job_id"), LaunchID: r.Int64("launch_id"),
		SourceItemID: r.String("source_item_id"), Name: r.String("name"),
		ExternalRef: r.String("external_ref"), Status: r.String("status"),
		SymptomID: r.Int64("symptom_id"), RCAID: r.Int64("rca_id"),
		ErrorMessage: r.String("error_message"), LogSnippet: r.String("log_snippet"),
		LogTruncated: r.Bool("log_truncated"),
		StartedAt: r.String("started_at"), EndedAt: r.String("ended_at"),
		CreatedAt: r.String("created_at"), UpdatedAt: r.String("updated_at"),
	}
}

func triageFromRow(r sqlite.Row) *Triage {
	return &Triage{
		ID: r.Int64("id"), CaseID: r.Int64("case_id"),
		SymptomCategory: r.String("symptom_category"), Severity: r.String("severity"),
		DefectTypeHypothesis: r.String("defect_type_hypothesis"),
		SkipInvestigation: r.Bool("skip_investigation"),
		ClockSkewSuspected: r.Bool("clock_skew_suspected"),
		CascadeSuspected: r.Bool("cascade_suspected"),
		CandidateRepos: r.String("candidate_repos"), DataQualityNotes: r.String("data_quality_notes"),
		CreatedAt: r.String("created_at"),
	}
}

func symptomFromRow(r sqlite.Row) *Symptom {
	return &Symptom{
		ID: r.Int64("id"), Fingerprint: r.String("fingerprint"), Name: r.String("name"),
		Description: r.String("description"), ErrorPattern: r.String("error_pattern"),
		TestNamePattern: r.String("test_name_pattern"), Component: r.String("component"),
		Severity: r.String("severity"), FirstSeenAt: r.String("first_seen_at"),
		LastSeenAt: r.String("last_seen_at"), OccurrenceCount: r.Int("occurrence_count"),
		Status: r.String("status"),
	}
}

func rcaFromRow(r sqlite.Row) *RCA {
	return &RCA{
		ID: r.Int64("id"), Title: r.String("title"), Description: r.String("description"),
		DefectType: r.String("defect_type"), Category: r.String("category"),
		Component: r.String("component"), AffectedVersions: r.String("affected_versions"),
		EvidenceRefs: r.String("evidence_refs"), ConvergenceScore: r.Float64("convergence_score"),
		JiraTicketID: r.String("jira_ticket_id"), JiraLink: r.String("jira_link"),
		Status: r.String("status"), CreatedAt: r.String("created_at"),
		ResolvedAt: r.String("resolved_at"), VerifiedAt: r.String("verified_at"),
		ArchivedAt: r.String("archived_at"),
	}
}

func symptomRCAFromRow(r sqlite.Row) *SymptomRCA {
	return &SymptomRCA{
		ID: r.Int64("id"), SymptomID: r.Int64("symptom_id"), RCAID: r.Int64("rca_id"),
		Confidence: r.Float64("confidence"), Notes: r.String("notes"),
		LinkedAt: r.String("linked_at"),
	}
}

func rowsTo[T any](rows []sqlite.Row, fn func(sqlite.Row) *T) []*T {
	out := make([]*T, len(rows))
	for i, r := range rows {
		out[i] = fn(r)
	}
	return out
}

// --- nil helpers for optional SQL params ---

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nilIfZero64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- Suite ---

func (s *SqlStore) CreateSuite(suite *InvestigationSuite) (int64, error) {
	if suite == nil {
		return 0, fmt.Errorf("suite is nil")
	}
	if suite.Status == "" {
		suite.Status = "open"
	}
	if suite.CreatedAt == "" {
		suite.CreatedAt = nowUTC()
	}
	return s.es.Create("investigation_suites", sqlite.Row{
		"name": suite.Name, "description": nilIfEmpty(suite.Description),
		"status": suite.Status, "created_at": suite.CreatedAt, "closed_at": nilIfEmpty(suite.ClosedAt),
	})
}

func (s *SqlStore) GetSuite(id int64) (*InvestigationSuite, error) {
	row, err := s.es.Get("investigation_suites", id)
	if err != nil || row == nil {
		return nil, err
	}
	return suiteFromRow(row), nil
}

func (s *SqlStore) ListSuites() ([]*InvestigationSuite, error) {
	rows, err := s.es.List("investigation_suites", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, suiteFromRow), nil
}

func (s *SqlStore) CloseSuite(id int64) error {
	return s.es.Update("investigation_suites", id, sqlite.Row{
		"status": "closed", "closed_at": nowUTC(),
	})
}

// --- Version ---

func (s *SqlStore) CreateVersion(v *Version) (int64, error) {
	if v == nil {
		return 0, fmt.Errorf("version is nil")
	}
	return s.es.Create("versions", sqlite.Row{
		"label": v.Label, "build_id": nilIfEmpty(v.BuildID),
	})
}

func (s *SqlStore) GetVersion(id int64) (*Version, error) {
	row, err := s.es.Get("versions", id)
	if err != nil || row == nil {
		return nil, err
	}
	return versionFromRow(row), nil
}

func (s *SqlStore) GetVersionByLabel(label string) (*Version, error) {
	row, err := s.es.GetBy("versions", sqlite.Row{"label": label})
	if err != nil || row == nil {
		return nil, err
	}
	return versionFromRow(row), nil
}

func (s *SqlStore) ListVersions() ([]*Version, error) {
	rows, err := s.es.List("versions", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, versionFromRow), nil
}

// --- Circuit ---

func (s *SqlStore) CreateCircuit(p *Circuit) (int64, error) {
	if p == nil {
		return 0, fmt.Errorf("circuit is nil")
	}
	return s.es.Create("circuits", sqlite.Row{
		"suite_id": p.SuiteID, "version_id": p.VersionID, "name": p.Name,
		"source_run_id": nilIfEmpty(p.SourceRunID), "status": p.Status,
		"started_at": nilIfEmpty(p.StartedAt), "ended_at": nilIfEmpty(p.EndedAt),
	})
}

func (s *SqlStore) GetCircuit(id int64) (*Circuit, error) {
	row, err := s.es.Get("circuits", id)
	if err != nil || row == nil {
		return nil, err
	}
	return circuitFromRow(row), nil
}

func (s *SqlStore) ListCircuitsBySuite(suiteID int64) ([]*Circuit, error) {
	rows, err := s.es.List("circuits", sqlite.Row{"suite_id": suiteID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, circuitFromRow), nil
}

// --- Launch ---

func (s *SqlStore) CreateLaunch(l *Launch) (int64, error) {
	if l == nil {
		return 0, fmt.Errorf("launch is nil")
	}
	return s.es.Create("launches", sqlite.Row{
		"circuit_id": l.CircuitID, "source_run_id": nilIfEmpty(l.SourceRunID),
		"source_run_uuid": nilIfEmpty(l.SourceRunUUID), "name": nilIfEmpty(l.Name),
		"status": nilIfEmpty(l.Status), "started_at": nilIfEmpty(l.StartedAt),
		"ended_at": nilIfEmpty(l.EndedAt), "env_attributes": nilIfEmpty(l.EnvAttributes),
		"git_branch": nilIfEmpty(l.GitBranch), "git_commit": nilIfEmpty(l.GitCommit),
		"envelope_payload": l.EnvelopePayload,
	})
}

func (s *SqlStore) GetLaunch(id int64) (*Launch, error) {
	row, err := s.es.Get("launches", id)
	if err != nil || row == nil {
		return nil, err
	}
	return launchFromRow(row), nil
}

func (s *SqlStore) GetLaunchBySourceRunID(circuitID int64, sourceRunID string) (*Launch, error) {
	row, err := s.es.GetBy("launches", sqlite.Row{
		"circuit_id": circuitID, "source_run_id": sourceRunID,
	})
	if err != nil || row == nil {
		return nil, err
	}
	return launchFromRow(row), nil
}

func (s *SqlStore) ListLaunchesByCircuit(circuitID int64) ([]*Launch, error) {
	rows, err := s.es.List("launches", sqlite.Row{"circuit_id": circuitID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, launchFromRow), nil
}

// --- Job ---

func (s *SqlStore) CreateJob(j *Job) (int64, error) {
	if j == nil {
		return 0, fmt.Errorf("job is nil")
	}
	return s.es.Create("jobs", sqlite.Row{
		"launch_id": j.LaunchID, "source_item_id": nilIfEmpty(j.SourceItemID),
		"name": j.Name, "clock_type": nilIfEmpty(j.ClockType), "status": nilIfEmpty(j.Status),
		"stats_total": nilIfZero(j.StatsTotal), "stats_failed": nilIfZero(j.StatsFailed),
		"stats_passed": nilIfZero(j.StatsPassed), "stats_skipped": nilIfZero(j.StatsSkipped),
		"started_at": nilIfEmpty(j.StartedAt), "ended_at": nilIfEmpty(j.EndedAt),
	})
}

func (s *SqlStore) GetJob(id int64) (*Job, error) {
	row, err := s.es.Get("jobs", id)
	if err != nil || row == nil {
		return nil, err
	}
	return jobFromRow(row), nil
}

func (s *SqlStore) ListJobsByLaunch(launchID int64) ([]*Job, error) {
	rows, err := s.es.List("jobs", sqlite.Row{"launch_id": launchID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, jobFromRow), nil
}

// --- Case ---

func (s *SqlStore) CreateCase(c *Case) (int64, error) {
	if c == nil {
		return 0, fmt.Errorf("case is nil")
	}
	now := nowUTC()
	if c.Status == "" {
		c.Status = "open"
	}
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	return s.es.Create("cases", sqlite.Row{
		"job_id": c.JobID, "launch_id": c.LaunchID,
		"source_item_id": nilIfEmpty(c.SourceItemID), "name": c.Name,
		"external_ref": nilIfEmpty(c.ExternalRef), "status": c.Status,
		"symptom_id": nilIfZero64(c.SymptomID), "rca_id": nilIfZero64(c.RCAID),
		"error_message": nilIfEmpty(c.ErrorMessage), "log_snippet": nilIfEmpty(c.LogSnippet),
		"log_truncated": boolToInt(c.LogTruncated),
		"started_at": nilIfEmpty(c.StartedAt), "ended_at": nilIfEmpty(c.EndedAt),
		"created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	})
}

func (s *SqlStore) GetCase(id int64) (*Case, error) {
	row, err := s.es.Get("cases", id)
	if err != nil || row == nil {
		return nil, err
	}
	return caseFromRow(row), nil
}

func (s *SqlStore) ListCasesByJob(jobID int64) ([]*Case, error) {
	rows, err := s.es.List("cases", sqlite.Row{"job_id": jobID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, caseFromRow), nil
}

func (s *SqlStore) ListCasesBySymptom(symptomID int64) ([]*Case, error) {
	rows, err := s.es.List("cases", sqlite.Row{"symptom_id": symptomID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, caseFromRow), nil
}

func (s *SqlStore) UpdateCaseStatus(caseID int64, status string) error {
	return s.es.Update("cases", caseID, sqlite.Row{
		"status": status, "updated_at": nowUTC(),
	})
}

func (s *SqlStore) LinkCaseToRCA(caseID, rcaID int64) error {
	return s.es.Update("cases", caseID, sqlite.Row{"rca_id": rcaID})
}

func (s *SqlStore) LinkCaseToSymptom(caseID, symptomID int64) error {
	return s.es.Update("cases", caseID, sqlite.Row{
		"symptom_id": symptomID, "updated_at": nowUTC(),
	})
}

// --- Triage ---

func (s *SqlStore) CreateTriage(t *Triage) (int64, error) {
	if t == nil {
		return 0, fmt.Errorf("triage is nil")
	}
	if t.CreatedAt == "" {
		t.CreatedAt = nowUTC()
	}
	return s.es.Create("triages", sqlite.Row{
		"case_id": t.CaseID, "symptom_category": t.SymptomCategory,
		"severity": nilIfEmpty(t.Severity), "defect_type_hypothesis": nilIfEmpty(t.DefectTypeHypothesis),
		"skip_investigation": boolToInt(t.SkipInvestigation),
		"clock_skew_suspected": boolToInt(t.ClockSkewSuspected),
		"cascade_suspected": boolToInt(t.CascadeSuspected),
		"candidate_repos": nilIfEmpty(t.CandidateRepos),
		"data_quality_notes": nilIfEmpty(t.DataQualityNotes),
		"created_at": t.CreatedAt,
	})
}

func (s *SqlStore) GetTriageByCase(caseID int64) (*Triage, error) {
	row, err := s.es.GetBy("triages", sqlite.Row{"case_id": caseID})
	if err != nil || row == nil {
		return nil, err
	}
	return triageFromRow(row), nil
}

// --- Symptom ---

func (s *SqlStore) CreateSymptom(sym *Symptom) (int64, error) {
	if sym == nil {
		return 0, fmt.Errorf("symptom is nil")
	}
	now := nowUTC()
	if sym.Status == "" {
		sym.Status = "active"
	}
	if sym.OccurrenceCount == 0 {
		sym.OccurrenceCount = 1
	}
	if sym.FirstSeenAt == "" {
		sym.FirstSeenAt = now
	}
	if sym.LastSeenAt == "" {
		sym.LastSeenAt = sym.FirstSeenAt
	}
	return s.es.Create("symptoms", sqlite.Row{
		"fingerprint": sym.Fingerprint, "name": sym.Name,
		"description": nilIfEmpty(sym.Description), "error_pattern": nilIfEmpty(sym.ErrorPattern),
		"test_name_pattern": nilIfEmpty(sym.TestNamePattern), "component": nilIfEmpty(sym.Component),
		"severity": nilIfEmpty(sym.Severity), "first_seen_at": sym.FirstSeenAt,
		"last_seen_at": sym.LastSeenAt, "occurrence_count": sym.OccurrenceCount,
		"status": sym.Status,
	})
}

func (s *SqlStore) GetSymptom(id int64) (*Symptom, error) {
	row, err := s.es.Get("symptoms", id)
	if err != nil || row == nil {
		return nil, err
	}
	return symptomFromRow(row), nil
}

func (s *SqlStore) GetSymptomByFingerprint(fingerprint string) (*Symptom, error) {
	row, err := s.es.GetBy("symptoms", sqlite.Row{"fingerprint": fingerprint})
	if err != nil || row == nil {
		return nil, err
	}
	return symptomFromRow(row), nil
}

func (s *SqlStore) FindSymptomCandidates(testName string) ([]*Symptom, error) {
	if testName == "" {
		return nil, nil
	}
	rows, err := s.es.List("symptoms", sqlite.Row{"name": testName}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomFromRow), nil
}

func (s *SqlStore) UpdateSymptomSeen(id int64) error {
	now := nowUTC()
	res, err := s.es.DB().ExecSQL(
		`UPDATE symptoms SET occurrence_count = occurrence_count + 1, last_seen_at = ?,
		        status = CASE WHEN status = 'dormant' THEN 'active' ELSE status END
		 WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("update symptom seen: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("symptom %d not found", id)
	}
	return nil
}

func (s *SqlStore) ListSymptoms() ([]*Symptom, error) {
	rows, err := s.es.List("symptoms", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomFromRow), nil
}

func (s *SqlStore) MarkDormantSymptoms(staleDays int) (int64, error) {
	res, err := s.es.DB().ExecSQL(
		`UPDATE symptoms SET status = 'dormant'
		 WHERE status = 'active'
		   AND last_seen_at < datetime('now', '-' || ? || ' days')`,
		staleDays,
	)
	if err != nil {
		return 0, fmt.Errorf("mark dormant symptoms: %w", err)
	}
	return res.RowsAffected()
}

// --- RCA ---

func (s *SqlStore) SaveRCA(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, fmt.Errorf("rca is nil")
	}
	if rca.Status == "" {
		rca.Status = "open"
	}
	if rca.CreatedAt == "" {
		rca.CreatedAt = nowUTC()
	}
	row := sqlite.Row{
		"title": rca.Title, "description": rca.Description, "defect_type": rca.DefectType,
		"category": nilIfEmpty(rca.Category), "component": nilIfEmpty(rca.Component),
		"affected_versions": nilIfEmpty(rca.AffectedVersions),
		"evidence_refs": nilIfEmpty(rca.EvidenceRefs), "convergence_score": rca.ConvergenceScore,
		"jira_ticket_id": nilIfEmpty(rca.JiraTicketID), "jira_link": nilIfEmpty(rca.JiraLink),
		"status": rca.Status, "resolved_at": nilIfEmpty(rca.ResolvedAt),
		"verified_at": nilIfEmpty(rca.VerifiedAt), "archived_at": nilIfEmpty(rca.ArchivedAt),
	}
	if rca.ID != 0 {
		return rca.ID, s.es.Update("rcas", rca.ID, row)
	}
	row["created_at"] = rca.CreatedAt
	return s.es.Create("rcas", row)
}

func (s *SqlStore) GetRCA(id int64) (*RCA, error) {
	row, err := s.es.Get("rcas", id)
	if err != nil || row == nil {
		return nil, err
	}
	return rcaFromRow(row), nil
}

func (s *SqlStore) ListRCAs() ([]*RCA, error) {
	rows, err := s.es.List("rcas", nil, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, rcaFromRow), nil
}

func (s *SqlStore) ListRCAsByStatus(status string) ([]*RCA, error) {
	rows, err := s.es.List("rcas", sqlite.Row{"status": status}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, rcaFromRow), nil
}

func (s *SqlStore) UpdateRCAStatus(id int64, status string) error {
	now := nowUTC()
	set := sqlite.Row{"status": status}
	switch status {
	case "resolved":
		set["resolved_at"] = now
	case "verified":
		set["verified_at"] = now
	case "archived":
		set["archived_at"] = now
	case "open":
		set["resolved_at"] = nil
		set["verified_at"] = nil
	}
	return s.es.Update("rcas", id, set)
}

// --- SymptomRCA ---

func (s *SqlStore) LinkSymptomToRCA(link *SymptomRCA) (int64, error) {
	if link == nil {
		return 0, fmt.Errorf("link is nil")
	}
	if link.LinkedAt == "" {
		link.LinkedAt = nowUTC()
	}
	return s.es.Create("symptom_rca", sqlite.Row{
		"symptom_id": link.SymptomID, "rca_id": link.RCAID,
		"confidence": link.Confidence, "notes": nilIfEmpty(link.Notes),
		"linked_at": link.LinkedAt,
	})
}

func (s *SqlStore) GetRCAsForSymptom(symptomID int64) ([]*SymptomRCA, error) {
	rows, err := s.es.List("symptom_rca", sqlite.Row{"symptom_id": symptomID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomRCAFromRow), nil
}

func (s *SqlStore) GetSymptomsForRCA(rcaID int64) ([]*SymptomRCA, error) {
	rows, err := s.es.List("symptom_rca", sqlite.Row{"rca_id": rcaID}, "")
	if err != nil {
		return nil, err
	}
	return rowsTo(rows, symptomRCAFromRow), nil
}
