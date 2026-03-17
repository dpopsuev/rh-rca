package rca

import (
	"context"
	"log/slog"
	"sync"
	"time"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/schematics/toolkit"
)

// RoutingEntry is the RCA-specific alias for toolkit.RoutingEntry.
type RoutingEntry = toolkit.RoutingEntry

// RoutingLog is the RCA-specific alias for toolkit.RoutingLog.
type RoutingLog = toolkit.RoutingLog

// RoutingDiff is the RCA-specific alias for toolkit.RoutingDiff.
type RoutingDiff = toolkit.RoutingDiff

// SaveRoutingLog delegates to toolkit with logging.
func SaveRoutingLog(path string, log RoutingLog) error {
	err := toolkit.SaveRoutingLog(path, log)
	if err == nil {
		slog.Info("routing log saved", "component", "routing", "path", path, "entries", len(log))
	}
	return err
}

// LoadRoutingLog delegates to toolkit.
func LoadRoutingLog(path string) (RoutingLog, error) {
	return toolkit.LoadRoutingLog(path)
}

// CompareRoutingLogs delegates to toolkit.
func CompareRoutingLogs(expected, actual RoutingLog) []RoutingDiff {
	return toolkit.CompareRoutingLogs(expected, actual)
}

// RoutingRecorder wraps a framework.Transformer, recording every Transform call.
// Thread-safe for parallel calibration.
type RoutingRecorder struct {
	inner framework.Transformer
	color string
	mu    sync.Mutex
	log   RoutingLog
	seq   int64
}

func NewRoutingRecorder(inner framework.Transformer, color string) *RoutingRecorder {
	return &RoutingRecorder{inner: inner, color: color}
}

func (r *RoutingRecorder) Name() string { return r.inner.Name() }
func (r *RoutingRecorder) Deterministic() bool {
	return framework.IsDeterministic(r.inner)
}

func (r *RoutingRecorder) Transform(ctx context.Context, tc *framework.TransformerContext) (any, error) {
	caseLabel, _ := tc.WalkerState.Context[KeyCaseLabel].(string)
	if caseLabel == "" {
		caseLabel = tc.WalkerState.ID
	}

	r.mu.Lock()
	r.seq++
	entry := RoutingEntry{
		CaseID: caseLabel, Step: tc.NodeName, Color: r.color,
		Timestamp: time.Now(), DispatchID: r.seq,
	}
	r.log = append(r.log, entry)
	r.mu.Unlock()

	slog.Info("dispatch", "component", "routing", "color", r.color, "case_id", caseLabel, "step", tc.NodeName)
	return r.inner.Transform(ctx, tc)
}

func (r *RoutingRecorder) Log() RoutingLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(RoutingLog, len(r.log))
	copy(out, r.log)
	return out
}

// IDMappable delegation — stubTransformer implements SetRCAID/SetSymptomID.
func (r *RoutingRecorder) SetRCAID(gtID string, storeID int64) {
	if im, ok := r.inner.(IDMappable); ok {
		im.SetRCAID(gtID, storeID)
	}
}

func (r *RoutingRecorder) SetSymptomID(gtID string, storeID int64) {
	if im, ok := r.inner.(IDMappable); ok {
		im.SetSymptomID(gtID, storeID)
	}
}
