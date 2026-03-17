package mcpconfig

import (
	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/dispatch"
)

// SessionObserver receives lifecycle and progress events from the MCP
// server. Implementations wire these to visualization layers (e.g.
// Kami SSE + view.CircuitStore) without the MCP server importing them.
type SessionObserver interface {
	OnSessionCreate(def *framework.CircuitDef, bus *dispatch.SignalBus)
	OnStepDispatched(caseID, step string)
	OnStepCompleted(caseID, step string, dispatchID int64)
	OnCircuitDone()
	OnSessionEnd()
	Close()
}
