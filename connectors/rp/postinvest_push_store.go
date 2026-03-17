package rp

// PushedRecord is what the mock store records after a push.
type PushedRecord struct {
	RunID          string
	CaseIDs        []string
	DefectType     string
	JiraTicketID   string
	JiraLink       string
}

// PushStore records pushed defect type and RCA fields (mock; no HTTP).
type PushStore interface {
	RecordPushed(record PushedRecord) error
	LastPushed() *PushedRecord
}

// MemPushStore is an in-memory push store for tests.
type MemPushStore struct {
	last *PushedRecord
}

// NewMemPushStore returns a new in-memory push store.
func NewMemPushStore() *MemPushStore {
	return &MemPushStore{}
}

// RecordPushed implements PushStore.
func (s *MemPushStore) RecordPushed(record PushedRecord) error {
	s.last = &record
	return nil
}

// LastPushed implements PushStore.
func (s *MemPushStore) LastPushed() *PushedRecord {
	return s.last
}
