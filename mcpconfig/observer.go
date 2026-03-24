package mcpconfig

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/signal"
	fwmcp "github.com/dpopsuev/origami/mcp"
)

// SessionObserver receives lifecycle and progress events from the MCP
// server. Implementations wire these to visualization layers (e.g.
// Kami SSE + view.CircuitStore) without the MCP server importing them.
//
// Embeds mcp.SessionObserver for the four auto-wired callbacks;
// adds OnSessionCreate and Close for consumer-specific lifecycle.
type SessionObserver interface {
	fwmcp.SessionObserver
	OnSessionCreate(def *circuit.CircuitDef, bus signal.Bus)
	Close()
}
