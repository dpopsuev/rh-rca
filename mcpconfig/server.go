package mcpconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/bugle/signal"
	"github.com/dpopsuev/bugle/billing"
	fwmcp "github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/rh-rca"
	"github.com/dpopsuev/rh-rca/rcatype"
	"github.com/dpopsuev/rh-rca/scenarios"
	"github.com/dpopsuev/rh-rca/store"
	"gopkg.in/yaml.v3"
)

var (
	DefaultGetNextStepTimeout = 10 * time.Second
	DefaultSessionTTL         = 5 * time.Minute
)

// Server wraps the generic CircuitServer with RCA-specific domain hooks.
type Server struct {
	*fwmcp.CircuitServer
	ProductName     string
	ProjectRoot     string // source tree root for reading domain data (scorecard, datasets)
	StateDir        string // writable root for runtime artifacts (calibrate, investigations)
	ReaderFactory   rca.SourceReaderFactory
	StoreFactory    rca.StoreFactory
	SubCircuitResolvers  map[string]circuit.AssetResolver
	MediatorEndpoint     string
	StepSchemas          []fwmcp.StepSchema
	DomainFS             fs.FS

	Observer SessionObserver
}

// ServerOption configures an RCA MCP server.
type ServerOption func(*Server)

// WithSourceReader injects a factory for creating SourceReaders from
// connection parameters provided in MCP start_circuit requests.
func WithSourceReader(f rca.SourceReaderFactory) ServerOption {
	return func(s *Server) { s.ReaderFactory = f }
}

// WithSubCircuitResolvers injects asset resolvers for sub-circuit
// overlay resolution. Keyed by circuit name (e.g., "gnd", "dsr").
// The consumer (via fold) provides these — the schematic never imports
// another schematic directly (SOLID: Dependency Inversion).
func WithSubCircuitResolvers(r map[string]circuit.AssetResolver) ServerOption {
	return func(s *Server) { s.SubCircuitResolvers = r }
}

// WithMediatorEndpoint sets the Origami Mediator MCP endpoint for runtime
// sub-circuit delegation. When set, handler_type: circuit nodes that can't
// resolve locally will delegate to the mediator via MCP.
func WithMediatorEndpoint(ep string) ServerOption {
	return func(s *Server) { s.MediatorEndpoint = ep }
}

// WithStepSchemas overrides the default RCA step schemas.
func WithStepSchemas(schemas []fwmcp.StepSchema) ServerOption {
	return func(s *Server) { s.StepSchemas = schemas }
}

// WithDomainFS provides an fs.FS for all domain data (scenarios,
// prompts, heuristics, etc.). Sub-paths like "scenarios/", "prompts/"
// are resolved relative to this FS. When nil, embedded defaults are used.
func WithDomainFS(fsys fs.FS) ServerOption {
	return func(s *Server) { s.DomainFS = fsys }
}

// WithStateDir overrides the runtime state directory used for calibration
// sessions, investigation artifacts, and other writable output.
func WithStateDir(dir string) ServerOption {
	return func(s *Server) { s.StateDir = dir }
}

// defaultStateDir returns $XDG_STATE_HOME/<product>, falling back to
// ~/.local/state/<product> per the XDG Base Directory Specification.
func defaultStateDir(product string) string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, product)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("."+product, "state")
	}
	return filepath.Join(home, ".local", "state", product)
}

// NewServer creates an RCA MCP server. The productName identifies the consumer
// (e.g. "asterisk"). Pass it from the consumer's manifest or CLI config.
//
// ProjectRoot defaults to cwd (for reading domain data like scorecards).
// StateDir defaults to $XDG_STATE_HOME/<product> (for writing runtime
// artifacts like calibration sessions and investigations).
func NewServer(productName string, opts ...ServerOption) *Server {
	cwd, _ := os.Getwd()
	s := &Server{
		ProductName: productName,
		ProjectRoot: cwd,
		StateDir:    defaultStateDir(productName),
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.DomainFS != nil {
		if vocabData, err := fs.ReadFile(s.DomainFS, "vocabulary.yaml"); err == nil {
			rca.InitVocab(vocabData)
		}
	}

	s.CircuitServer = fwmcp.NewCircuitServer(s.buildConfig())
	return s
}

// WithSessionObserver injects a SessionObserver for live visualization
// (e.g. Kami SSE + view.CircuitStore). When nil, no UI events are emitted.
func WithSessionObserver(o SessionObserver) ServerOption {
	return func(s *Server) { s.Observer = o }
}

// Shutdown closes any active observer resources, then shuts down
// the underlying CircuitServer.
func (s *Server) Shutdown() {
	if s.Observer != nil {
		s.Observer.Close()
	}
	s.CircuitServer.Shutdown()
}

func (s *Server) loadScorecard(path, projectRoot string) (*cal.ScoreCard, error) {
	if s.DomainFS != nil {
		data, err := fs.ReadFile(s.DomainFS, path)
		if err == nil {
			return cal.ParseScoreCard(data)
		}
	}
	return cal.LoadScoreCard(filepath.Join(projectRoot, path))
}

func (s *Server) readDomainCircuit() []byte {
	if s.DomainFS == nil {
		return nil
	}
	data, _ := fs.ReadFile(s.DomainFS, "circuits/rca.yaml")
	return data
}

// loadSubCircuits loads sub-circuit definitions from domain FS using the
// framework's LoadSubCircuitsFromFS utility with consumer-injected resolvers.
func (s *Server) loadSubCircuits() map[string]*circuit.CircuitDef {
	return circuit.LoadSubCircuitsFromFS(s.DomainFS, s.SubCircuitResolvers)
}

func (s *Server) buildConfig() fwmcp.CircuitConfig {
	schemas := s.StepSchemas
	if len(schemas) == 0 && s.DomainFS != nil {
		if data, err := fs.ReadFile(s.DomainFS, "llm-output-schemas/rca.yaml"); err == nil {
			schemas, _ = LoadCollapsedSchemas(data)
		}
	}
	if len(schemas) == 0 && s.DomainFS != nil {
		if sub, err := fs.Sub(s.DomainFS, "schemas/rca"); err == nil {
			schemas, _ = LoadStepSchemas(sub)
		}
	}
	if len(schemas) == 0 {
		schemas = rcaStepSchemas()
	}
	cfg := fwmcp.CircuitConfig{
		Name:        s.ProductName,
		Version:     "dev",
		StateDir:    s.StateDir,
		StepSchemas: schemas,
		WorkerPreamble: fmt.Sprintf("You are a %s calibration worker.", s.ProductName),
		DefaultGetNextStepTimeout: int(DefaultGetNextStepTimeout / time.Millisecond),
		DefaultSessionTTL:         int(DefaultSessionTTL / time.Millisecond),
		ExtraParamDefs: []fwmcp.ExtraParamDef{
			{Name: "scenario", Type: "string", Description: "Calibration scenario name (e.g. 'ptp')", Required: true},
			{Name: "mode", Type: "string", Description: "Execution mode", Enum: []string{"offline", "online"}},
			{Name: "backend", Type: "string", Description: "Transformer backend for LLM processing", Enum: []string{"stub", "basic", "llm"}},
			{Name: "resolution", Type: "string", Description: "Calibration resolution: unit (single circuit, stubs at ports), pairwise, integrated, or empty for full", Enum: []string{"unit", "pairwise", "integrated"}},
			{Name: "rp_base_url", Type: "string", Description: "ReportPortal base URL for online mode (e.g. 'https://rp.example.com')"},
			{Name: "rp_project", Type: "string", Description: "ReportPortal project name (defaults to $ASTERISK_RP_PROJECT)"},
			{Name: "rp_api_key_path", Type: "string", Description: "Path to RP API key file (defaults to $ASTERISK_RP_API_KEY_PATH or '.rp-api-key')"},
		},
		CreateSession: func(ctx context.Context, params fwmcp.StartParams, disp *dispatch.MuxDispatcher, bus signal.Bus) (fwmcp.RunFunc, fwmcp.SessionMeta, error) {
			return s.createSession(ctx, params, disp, bus)
		},
		FormatReport: func(result any) (string, any, error) {
			report, ok := result.(*rca.CalibrationReport)
			if !ok {
				return "", nil, fmt.Errorf("unexpected result type: %T", result)
			}
			var reportTemplate []byte
			if s.DomainFS != nil {
				var err error
				reportTemplate, err = fs.ReadFile(s.DomainFS, "reports/rca-report.yaml")
				if err != nil {
					slog.Warn("report template not found, using default",
						"path", "reports/rca-report.yaml",
						"error", err,
					)
					reportTemplate = nil
				}
			}
			formatted, err := rca.RenderCalibrationReport(report, reportTemplate)
			if err != nil {
				return "", nil, fmt.Errorf("render calibration report: %w", err)
			}
			return formatted, report, nil
		},
	}

	cfg.Observer = s.Observer // framework auto-wires On* callbacks

	return cfg
}

func (s *Server) createSession(ctx context.Context, params fwmcp.StartParams, disp *dispatch.MuxDispatcher, bus signal.Bus) (fwmcp.RunFunc, fwmcp.SessionMeta, error) {
	if s.Observer != nil {
		def, err := rca.LoadCircuitDef(s.readDomainCircuit(), rca.DefaultThresholds())
		if err == nil {
			s.Observer.OnSessionCreate(def, bus)
		}
	}

	extra := params.Extra

	scenarioName, _ := extra["scenario"].(string)
	if scenarioName == "" {
		return nil, fwmcp.SessionMeta{}, fmt.Errorf("extra.scenario is required; pass it in start_circuit extra, e.g. extra:{\"scenario\":\"ptp\"}")
	}
	transformerName, _ := extra["backend"].(string)
	resolutionStr, _ := extra["resolution"].(string)
	rpBaseURL, _ := extra["rp_base_url"].(string)
	rpProject, _ := extra["rp_project"].(string)
	modeStr, _ := extra["mode"].(string)
	mode := rca.ParseCalibrationMode(modeStr)

	var resolution cal.Resolution
	var portStubs cal.PortStubs
	if resolutionStr != "" {
		var err error
		resolution, err = cal.ParseResolution(resolutionStr)
		if err != nil {
			return nil, fwmcp.SessionMeta{}, fmt.Errorf("invalid resolution: %w", err)
		}
		if s.DomainFS != nil {
			stubsDir := fmt.Sprintf("stubs/%s", resolutionStr)
			if stubFS, fsErr := fs.Sub(s.DomainFS, stubsDir); fsErr == nil {
				ps := cal.PortStubs{}
				entries, _ := fs.ReadDir(stubFS, ".")
				for _, e := range entries {
					if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
						continue
					}
					data, readErr := fs.ReadFile(stubFS, e.Name())
					if readErr != nil {
						continue
					}
					var v any
					if jsonErr := json.Unmarshal(data, &v); jsonErr != nil {
						continue
					}
					portName := e.Name()[:len(e.Name())-len(".json")]
					ps[portName] = v
				}
				if len(ps) > 0 {
					portStubs = ps
				}
			}
		}
	}

	var scenarioFS fs.FS
	if s.DomainFS != nil {
		scenarioFS, _ = fs.Sub(s.DomainFS, "scenarios")
	}
	scenario, err := scenarios.LoadScenario(scenarioFS, scenarioName)
	if err != nil {
		return nil, fwmcp.SessionMeta{}, err
	}

	switch mode {
	case rca.ModeOffline:
		if s.DomainFS != nil {
			offlineFS, fsErr := fs.Sub(s.DomainFS, "offline")
			if fsErr != nil {
				return nil, fwmcp.SessionMeta{}, fmt.Errorf("offline bundle not found in domain FS (expected 'offline/' directory with rp/ sub-dir): %w", fsErr)
			}
			if err := scenarios.ResolveOfflineRP(offlineFS, scenario); err != nil {
				return nil, fwmcp.SessionMeta{}, fmt.Errorf("resolve offline RP data (scenario=%s, mode=offline): %w", scenarioName, err)
			}
		}
	default:
		var rpFetcher rcatype.EnvelopeFetcher
		if rpBaseURL != "" {
			if rpProject == "" {
				rpProject = os.Getenv("ASTERISK_RP_PROJECT")
			}
			if rpProject == "" {
				return nil, fwmcp.SessionMeta{}, fmt.Errorf("rp_project is required when rp_base_url is set")
			}
			rpAPIKeyPath, _ := extra["rp_api_key_path"].(string)
			if rpAPIKeyPath == "" {
				rpAPIKeyPath = os.Getenv("ASTERISK_RP_API_KEY_PATH")
			}
			if rpAPIKeyPath == "" {
				rpAPIKeyPath = ".rp-api-key"
			}
			if s.ReaderFactory == nil {
				return nil, fwmcp.SessionMeta{}, fmt.Errorf("no source connector configured (ReaderFactory not set)")
			}
			source, err := s.ReaderFactory(rpBaseURL, rpAPIKeyPath, rpProject)
			if err != nil {
				return nil, fwmcp.SessionMeta{}, fmt.Errorf("create source reader: %w", err)
			}
			rpFetcher = source.EnvelopeFetcher()
			if err := rca.ResolveRPCases(rpFetcher, scenario); err != nil {
				return nil, fwmcp.SessionMeta{}, fmt.Errorf("resolve RP-sourced cases: %w", err)
			}
		}
		_ = rpFetcher
	}

	root := s.ProjectRoot
	promptFS := s.DomainFS
	basePath := filepath.Join(s.StateDir, "calibrate")

	tokenTracker := billing.NewTracker()
	tracked := dispatch.NewTokenTrackingDispatcher(disp, tokenTracker)

	var comps []*engine.Component
	var transformerLabel string
	var idMapper rca.IDMappable
	switch transformerName {
	case "stub":
		stub := rca.NewStubTransformer(scenario)
		comps = []*engine.Component{rca.TransformerComponent(stub)}
		transformerLabel = "stub"
		idMapper = stub
	case "basic":
		var basicSt store.Store
		var stErr error
		if s.StoreFactory != nil {
			basicSt, stErr = s.StoreFactory(":memory:")
		} else {
			basicSt, stErr = store.Open(":memory:")
		}
		if stErr != nil {
			return nil, fwmcp.SessionMeta{}, fmt.Errorf("basic transformer: open store: %w", stErr)
		}
		var repoNames []string
		for _, r := range scenario.SourcePack.Repos {
			repoNames = append(repoNames, r.Name)
		}
		var heuristicsData []byte
		if s.DomainFS != nil {
			heuristicsData, _ = fs.ReadFile(s.DomainFS, "heuristics.yaml")
		}
		comps = []*engine.Component{rca.HeuristicComponent(basicSt, repoNames, heuristicsData)}
		transformerLabel = "basic"
	default:
		t := rca.NewRCATransformer(
			tracked,
			promptFS,
			rca.WithRCABasePath(basePath),
		)
		comps = []*engine.Component{rca.TransformerComponent(t)}
		transformerLabel = "rca"
	}

	parallel := params.Parallel
	if parallel < 1 {
		parallel = 1
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fwmcp.SessionMeta{}, fmt.Errorf("create calibrate dir: %w", err)
	}

	circuitDef, err := rca.LoadCircuitDef(s.readDomainCircuit(), rca.DefaultThresholds())
	if err != nil {
		return nil, fwmcp.SessionMeta{}, fmt.Errorf("load circuit def for scorecard: %w", err)
	}
	scorecardPath := circuitDef.Scorecard
	if scorecardPath == "" {
		scorecardPath = "scorecards/rca.yaml"
	}
	sc, err := s.loadScorecard(scorecardPath, root)
	if err != nil {
		return nil, fwmcp.SessionMeta{}, fmt.Errorf("load scorecard: %w", err)
	}

	var calReportTemplate []byte
	if s.DomainFS != nil {
		var readErr error
		calReportTemplate, readErr = fs.ReadFile(s.DomainFS, "reports/rca-report.yaml")
		if readErr != nil {
			slog.Warn("report template not found, using default",
				"path", "reports/rca-report.yaml",
				"error", readErr,
			)
		}
	}
	adapter := &rca.RCACalibrationAdapter{
		Scenario:        scenario,
		Components:      comps,
		IDMapper:        idMapper,
		BasePath:        basePath,
		Thresholds:      rca.DefaultThresholds(),
		ScoreCard:       sc,
		TokenTracker:    tokenTracker,
		ReportTemplate:  calReportTemplate,
	}

	// Load sub-circuit definitions (e.g., gnd/dsr) from domain FS.
	subCircuits := s.loadSubCircuits()

	runFn := func(ctx context.Context) (any, error) {
		genReport, err := cal.Run(ctx, cal.HarnessConfig{
			Loader:         adapter,
			Collector:      adapter,
			Renderer:       adapter,
			CircuitDef:     circuitDef,
			ScoreCard:      sc,
			Shared: engine.GraphRegistries{
				Circuits:         subCircuits,
				MediatorEndpoint: s.MediatorEndpoint,
			},
			PromptRelayer: &dispatch.MuxRelayer{Disp: disp},
			Contract:       cal.ContractFromDef(circuitDef.Calibration),
			Resolution:     resolution,
			PortStubs:      portStubs,
			Scenario:       scenario.Name,
			Transformer:    transformerLabel,
			Runs:           1,
			Parallel:       parallel,
			OnCaseComplete: adapter.OnCaseComplete(),
			Observer:       params.Observer,
		})
		if err != nil {
			return nil, err
		}

		report := adapter.RCAReport(genReport)
		rca.ApplyDryCaps(&report.Metrics, scenario.DryCappedMetrics)
		if m20def := sc.FindDef("M20"); m20def != nil {
			report.Metrics.Metrics = append(report.Metrics.Metrics,
				m20def.ToMetric(0, "single run"))
		}
		return report, nil
	}

	meta := fwmcp.SessionMeta{
		TotalCases: len(scenario.Cases),
		Scenario:   scenario.Name,
	}

	return runFn, meta, nil
}

// stepSchemaFile is the YAML representation of a StepSchema.
// Supports two formats:
//   - Legacy: flat fields (map[string]string) + separate defs list
//   - Unified: structured fields (map[string]{type,required}) with no defs
//
// Envelope fields (kind, metadata) are accepted and used to resolve the name.
type stepSchemaFile struct {
	Kind     string            `yaml:"kind,omitempty"`
	Metadata stepMetadata      `yaml:"metadata,omitempty"`
	Name     string            `yaml:"name,omitempty"`
	Fields   map[string]any    `yaml:"fields"`
	Defs     []fieldDefFile    `yaml:"defs,omitempty"`
}

type stepMetadata struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type fieldDefFile struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Desc     string `yaml:"desc,omitempty"`
}

func (f *stepSchemaFile) resolveName() string {
	if f.Metadata.Name != "" {
		return f.Metadata.Name
	}
	return f.Name
}

// resolveSchema converts the parsed stepSchemaFile into a StepSchema.
// It detects whether fields use flat (string) or structured (map) format
// and derives Defs from structured fields when no explicit Defs are present.
func (f *stepSchemaFile) resolveSchema() fwmcp.StepSchema {
	s := fwmcp.StepSchema{Name: f.resolveName()}

	isStructured := false
	for _, v := range f.Fields {
		if _, ok := v.(string); !ok {
			isStructured = true
			break
		}
	}

	if isStructured {
		for name, raw := range f.Fields {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if len(f.Defs) == 0 {
				typ, _ := m["type"].(string)
				req, _ := m["required"].(bool)
				desc, _ := m["desc"].(string)
				s.Defs = append(s.Defs, fwmcp.FieldDef{
					Name:     name,
					Type:     typ,
					Required: req,
					Desc:     desc,
				})
			}
		}
	} else if len(f.Defs) == 0 {
		// Legacy flat fields: convert to Defs
		for name, v := range f.Fields {
			typ, _ := v.(string)
			s.Defs = append(s.Defs, fwmcp.FieldDef{
				Name: name,
				Type: typ,
			})
		}
	}

	if len(f.Defs) > 0 {
		for _, d := range f.Defs {
			s.Defs = append(s.Defs, fwmcp.FieldDef{
				Name:     d.Name,
				Type:     d.Type,
				Required: d.Required,
				Desc:     d.Desc,
			})
		}
	}

	return s
}

// collapsedSchemaFile represents a single YAML file with stages keyed by name.
type collapsedSchemaFile struct {
	Stages map[string]struct {
		Fields map[string]any `yaml:"fields"`
	} `yaml:"stages"`
}

// LoadCollapsedSchemas parses a multi-stage schema file (kind: artifact-schema)
// where all stages are in a single file under the stages: key.
func LoadCollapsedSchemas(data []byte) ([]fwmcp.StepSchema, error) {
	var f collapsedSchemaFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse collapsed schema: %w", err)
	}
	if len(f.Stages) == 0 {
		return nil, fmt.Errorf("collapsed schema: no stages found")
	}
	schemas := make([]fwmcp.StepSchema, 0, len(f.Stages))
	for name, stage := range f.Stages {
		sf := stepSchemaFile{Name: name, Fields: stage.Fields}
		schemas = append(schemas, sf.resolveSchema())
	}
	return schemas, nil
}

// LoadStepSchemas reads step schema YAML files from fsys (e.g. "schemas/rca/")
// and returns them as []fwmcp.StepSchema. Each .yaml file in the directory
// defines one step. Supports both legacy (flat fields + defs) and unified
// (structured fields) formats.
func LoadStepSchemas(fsys fs.FS) ([]fwmcp.StepSchema, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read step schema dir: %w", err)
	}

	var schemas []fwmcp.StepSchema
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read step schema %q: %w", e.Name(), err)
		}
		var f stepSchemaFile
		if err := yaml.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse step schema %q: %w", e.Name(), err)
		}
		schemas = append(schemas, f.resolveSchema())
	}

	if len(schemas) == 0 {
		return nil, fmt.Errorf("no step schemas found")
	}
	return schemas, nil
}

// rcaStepSchemas returns the F0-F6 step schemas for RCA calibration.

func rcaStepSchemas() []fwmcp.StepSchema {
	return []fwmcp.StepSchema{
		{
			Name: "recall",
			Defs: []fwmcp.FieldDef{
				{Name: "match", Type: "bool", Required: true},
				{Name: "confidence", Type: "float", Required: true},
				{Name: "reasoning", Type: "string", Required: true},
			},
		},
		{
			Name: "triage",
			Defs: []fwmcp.FieldDef{
				{Name: "symptom_category", Type: "string", Required: true},
				{Name: "severity", Type: "string", Required: true},
				{Name: "defect_type_hypothesis", Type: "string", Required: true},
				{Name: "candidate_repos", Type: "array", Required: false},
				{Name: "skip_investigation", Type: "bool", Required: false},
				{Name: "cascade_suspected", Type: "bool", Required: false},
			},
		},
		{
			Name: "resolve",
			Defs: []fwmcp.FieldDef{
				{Name: "selected_repos", Type: "array", Required: true},
			},
		},
		{
			Name: "investigate",
			Defs: []fwmcp.FieldDef{
				{Name: "rca_message", Type: "string", Required: true},
				{Name: "defect_type", Type: "string", Required: true},
				{Name: "component", Type: "string", Required: true},
				{Name: "convergence_score", Type: "float", Required: false},
				{Name: "evidence_refs", Type: "array", Required: false},
			},
		},
		{
			Name: "correlate",
			Defs: []fwmcp.FieldDef{
				{Name: "is_duplicate", Type: "bool", Required: true},
				{Name: "confidence", Type: "float", Required: true},
			},
		},
		{
			Name: "review",
			Defs: []fwmcp.FieldDef{
				{Name: "decision", Type: "string", Required: true},
			},
		},
		{
			Name: "report",
			Defs: []fwmcp.FieldDef{
				{Name: "defect_type", Type: "string", Required: true},
				{Name: "case_id", Type: "string", Required: true},
				{Name: "summary", Type: "string", Required: true},
			},
		},
	}
}
