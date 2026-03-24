// Command rca-serve runs the RCA circuit as a standalone MCP server.
// It connects to a domain data server via MCPRemoteFS for scenario,
// prompt, and offline bundle access.
//
// Usage: rca-serve [--port=9200] --domain-endpoint http://domain:9300/mcp
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/domainfs"
	"github.com/dpopsuev/origami/subprocess"
	rp "github.com/dpopsuev/rh-rca/connectors/rp"
	rca "github.com/dpopsuev/rh-rca"
)

type sessionToolCaller struct {
	session *sdkmcp.ClientSession
}

func (s *sessionToolCaller) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	return s.session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
}

var _ subprocess.ToolCaller = (*sessionToolCaller)(nil)

func connectMCP(ctx context.Context, endpoint, label string) *sdkmcp.ClientSession {
	transport := &sdkmcp.StreamableClientTransport{Endpoint: endpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "origami-rca-engine", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("connect to %s at %s: %v", label, endpoint, err)
	}
	return session
}

func main() {
	port := flag.Int("port", 9200, "HTTP port for the RCA MCP server")
	healthz := flag.Bool("healthz", false, "probe /healthz and exit")
	domainEndpoint := flag.String("domain-endpoint", envOr("DOMAIN_ENDPOINT", ""), "Domain data MCP endpoint (required)")
	mediatorEndpoint := flag.String("mediator-endpoint", envOr("MEDIATOR_ENDPOINT", ""), "Mediator MCP endpoint for sub-circuit delegation")
	productName := flag.String("product", envOr("PRODUCT_NAME", "asterisk"), "Product name for state directory")
	flag.Parse()

	if *healthz {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", *port))
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *domainEndpoint == "" {
		log.Fatal("--domain-endpoint is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	domainSession := connectMCP(ctx, *domainEndpoint, "domain")
	defer domainSession.Close()
	remoteFS := domainfs.New(&sessionToolCaller{session: domainSession}).
		WithTimeout(10 * time.Second)
	log.Printf("connected to domain server at %s", *domainEndpoint)

	opts := []rca.ServerOption{
		rca.WithDomainFS(remoteFS),
		rca.WithSourceReader(rp.NewSourceReader),
	}
	if *mediatorEndpoint != "" {
		opts = append(opts, rca.WithMediatorEndpoint(*mediatorEndpoint))
		log.Printf("mediator endpoint: %s", *mediatorEndpoint)
	}

	srv := rca.NewServer(*productName, opts...)
	defer srv.Shutdown()

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return srv.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if srv.MCPServer != nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		httpServer.Shutdown(context.Background())
	}()

	log.Printf("rca engine listening on %s (domain: %s)", addr, *domainEndpoint)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
