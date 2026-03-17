package mcpconfig

import (
	"context"

	fwmcp "github.com/dpopsuev/origami/mcp"
)

// WatchStdin monitors for parent process death in a background goroutine.
// Delegates to the framework MCP WatchStdin so internal/mcp wraps framework only.
func WatchStdin(ctx context.Context, _ any, cancelFn context.CancelFunc) {
	fwmcp.WatchStdin(ctx, nil, cancelFn)
}
