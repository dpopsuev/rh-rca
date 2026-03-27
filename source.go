package rca

import (
	"github.com/dpopsuev/origami-rca/rcatype"
	"github.com/dpopsuev/origami-rca/store"
)

// Re-export domain types from rcatype for backwards compatibility.
// These will be removed once all consumers import rcatype directly.
type (
	SourceReader        = rcatype.SourceReader
	SourceReaderFactory = rcatype.SourceReaderFactory
	DefectWriter        = rcatype.DefectWriter
	DefectWriterFactory = rcatype.DefectWriterFactory
	RCAVerdict          = rcatype.RCAVerdict
	PushedRecord        = rcatype.PushedRecord
	RunDiscoverer       = rcatype.RunDiscoverer
	RunDiscovererFactory = rcatype.RunDiscovererFactory
	RunInfo             = rcatype.RunInfo
	FailureInfo         = rcatype.FailureInfo
	DefaultDefectWriter = rcatype.DefaultDefectWriter
)

// StoreFactory creates a Store from a database path.
type StoreFactory func(path string) (store.Store, error)

// TokenChecker validates the presence and permissions of a token file.
type TokenChecker func(path string) error
