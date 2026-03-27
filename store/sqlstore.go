package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/connectors/sqlite"
	"github.com/dpopsuev/origami-rca/rcatype"
)

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// SqlStore implements Store with SQLite via the Origami sqlite component.
type SqlStore struct {
	db *sqlite.DB
	es *sqlite.EntityStore
}

// Open opens or creates a SQLite DB at path with the embedded reference schema.
// Prefer OpenWithSchema when the consumer provides its own schema via fold.
func Open(path string) (*SqlStore, error) {
	schema, err := LoadSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	return openDB(path, schema)
}

// OpenWithSchema opens or creates a SQLite DB using consumer-provided schema data.
func OpenWithSchema(path string, schemaData []byte) (*SqlStore, error) {
	schema, err := sqlite.ParseSchema(schemaData)
	if err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	return openDB(path, schema)
}

// OpenMemory opens an in-memory SQLite DB for testing.
func OpenMemory() (*SqlStore, error) {
	schema, err := LoadSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	return openMemDB(schema)
}

// OpenMemoryWithSchema opens an in-memory SQLite DB using consumer-provided schema data.
func OpenMemoryWithSchema(schemaData []byte) (*SqlStore, error) {
	schema, err := sqlite.ParseSchema(schemaData)
	if err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	return openMemDB(schema)
}

func openDB(path string, schema *sqlite.Schema) (*SqlStore, error) {
	db, err := sqlite.Open(path, schema)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return &SqlStore{db: db, es: sqlite.NewEntityStore(db)}, nil
}

func openMemDB(schema *sqlite.Schema) (*SqlStore, error) {
	db, err := sqlite.OpenMemory(schema)
	if err != nil {
		return nil, fmt.Errorf("open memory sqlite: %w", err)
	}
	return &SqlStore{db: db, es: sqlite.NewEntityStore(db)}, nil
}

func (s *SqlStore) Close() error {
	return s.db.Close()
}

// RawDB returns the underlying *sqlite.DB for direct access.
func (s *SqlStore) RawDB() *sqlite.DB {
	return s.db
}

func (s *SqlStore) SaveEnvelope(runID string, env *rcatype.Envelope) error {
	if env == nil {
		return errors.New("envelope is nil")
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	var existingID int64
	err = s.db.QueryRow(
		"SELECT id FROM launches WHERE source_run_id = ? LIMIT 1", runID,
	).Scan(&existingID)
	if err == nil {
		_, err = s.db.Exec(
			"UPDATE launches SET envelope_payload = ? WHERE id = ?",
			payload, existingID,
		)
		if err != nil {
			return fmt.Errorf("update envelope: %w", err)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check existing launch: %w", err)
	}

	now := nowUTC()

	var suiteID int64
	err = s.db.QueryRow(
		"SELECT id FROM investigation_suites WHERE name = 'Default Suite' LIMIT 1",
	).Scan(&suiteID)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.Exec(
			"INSERT INTO investigation_suites(name, description, status, created_at) VALUES(?, ?, 'open', ?)",
			"Default Suite", "Auto-created for v1-style envelope save", now,
		)
		if err != nil {
			return fmt.Errorf("create default suite: %w", err)
		}
		suiteID, _ = res.LastInsertId()
	} else if err != nil {
		return fmt.Errorf("check default suite: %w", err)
	}

	var versionID int64
	err = s.db.QueryRow("SELECT id FROM versions WHERE label = 'unknown' LIMIT 1").Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.Exec("INSERT INTO versions(label) VALUES('unknown')")
		if err != nil {
			return fmt.Errorf("create unknown version: %w", err)
		}
		versionID, _ = res.LastInsertId()
	} else if err != nil {
		return fmt.Errorf("check unknown version: %w", err)
	}

	res, err := s.db.Exec(
		"INSERT INTO circuits(suite_id, version_id, name, source_run_id, status) VALUES(?, ?, ?, ?, 'UNKNOWN')",
		suiteID, versionID, fmt.Sprintf("auto-circuit-%s", runID), runID,
	)
	if err != nil {
		return fmt.Errorf("create circuit: %w", err)
	}
	circuitID, _ := res.LastInsertId()

	res, err = s.db.Exec(
		`INSERT INTO launches(circuit_id, source_run_id, name, envelope_payload)
		 VALUES(?, ?, ?, ?)`,
		circuitID, runID, env.Name, payload,
	)
	if err != nil {
		return fmt.Errorf("create launch: %w", err)
	}
	dbLaunchID, _ := res.LastInsertId()

	_, err = s.db.Exec(
		"INSERT INTO jobs(launch_id, source_item_id, name) VALUES(?, '', 'default-job')",
		dbLaunchID,
	)
	if err != nil {
		return fmt.Errorf("create default job: %w", err)
	}

	return nil
}

func (s *SqlStore) GetEnvelope(runID string) (*rcatype.Envelope, error) {
	var payload []byte
	err := s.db.QueryRow(
		"SELECT envelope_payload FROM launches WHERE source_run_id = ? LIMIT 1",
		runID,
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get envelope: %w", err)
	}
	if payload == nil {
		return nil, nil
	}
	var env rcatype.Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &env, nil
}
