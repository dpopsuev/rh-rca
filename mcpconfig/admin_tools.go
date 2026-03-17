package mcpconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	fwmcp "github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/rh-rca"
	"github.com/dpopsuev/rh-rca/scenarios"
)

// RegisterAdminTools registers data management and investigation tools
// on the MCP server. These replace the former CLI commands (consume,
// dataset, push, status, gt).
func RegisterAdminTools(srv *fwmcp.CircuitServer, opts AdminToolOpts) {
	m := srv.MCPServer
	tool := func(name, desc string) *sdkmcp.Tool {
		return &sdkmcp.Tool{Name: name, Description: desc}
	}

	sdkmcp.AddTool(m, tool("investigation_status", "Show investigation state for a case (current node, status, history)."), fwmcp.NoOutputSchema(handleStatus(opts)))
	sdkmcp.AddTool(m, tool("dataset_status", "Show dataset and ingestion counts (verified vs candidate)."), fwmcp.NoOutputSchema(handleDatasetStatus(opts)))
	sdkmcp.AddTool(m, tool("dataset_review", "List candidate cases pending review."), fwmcp.NoOutputSchema(handleDatasetReview(opts)))
	sdkmcp.AddTool(m, tool("dataset_promote", "Promote a candidate case to verified status."), fwmcp.NoOutputSchema(handleDatasetPromote(opts)))
	sdkmcp.AddTool(m, tool("gt_status", "Show ground truth dataset completeness."), fwmcp.NoOutputSchema(handleGTStatus(opts)))
	sdkmcp.AddTool(m, tool("gt_import", "Import a scenario to JSON in the datasets directory."), fwmcp.NoOutputSchema(handleGTImport(opts)))
	sdkmcp.AddTool(m, tool("gt_export", "Load a JSON dataset and return metadata."), fwmcp.NoOutputSchema(handleGTExport(opts)))
	sdkmcp.AddTool(m, tool("push_artifact", "Push an RCA artifact to ReportPortal."), fwmcp.NoOutputSchema(handlePush(opts)))
	sdkmcp.AddTool(m, tool("consume", "Discover new CI failures and create candidate cases."), fwmcp.NoOutputSchema(handleConsume(opts)))
}

// ConsumeSummary holds the result counts from a consume run.
type ConsumeSummary struct {
	Discovered int
	Matched    int
	Written    int
	Duplicates int
}

// ConsumeFunc discovers new CI failures and creates candidate cases.
// Implementations wire the ingest pipeline; when nil, the consume tool
// returns an error telling the caller to configure one.
type ConsumeFunc func(ctx context.Context, lookbackDays int, datasetDir, candidateDir string) (ConsumeSummary, error)

// AdminToolOpts configures the admin MCP tools.
type AdminToolOpts struct {
	ProjectRoot  string // source tree root for curated data (datasets, candidates)
	StateDir     string // writable root for runtime artifacts (investigations)
	DatasetDir   string
	CandidateDir string
	BasePath     string
	DefectWriter rca.DefectWriter
	Consume      ConsumeFunc
}

func (o AdminToolOpts) datasetDir() string {
	if o.DatasetDir != "" {
		return filepath.Join(o.ProjectRoot, o.DatasetDir)
	}
	return filepath.Join(o.ProjectRoot, "datasets")
}

func (o AdminToolOpts) candidateDir() string {
	if o.CandidateDir != "" {
		return filepath.Join(o.ProjectRoot, o.CandidateDir)
	}
	return filepath.Join(o.ProjectRoot, "candidates")
}

func (o AdminToolOpts) basePath() string {
	if o.BasePath != "" {
		return filepath.Join(o.StateDir, o.BasePath)
	}
	return filepath.Join(o.StateDir, "investigations")
}

// --- investigation_status ---

type statusInput struct {
	CaseID  int64 `json:"case_id"`
	SuiteID int64 `json:"suite_id"`
}

type statusOutput struct {
	Found       bool              `json:"found"`
	CurrentNode string            `json:"current_node,omitempty"`
	Status      string            `json:"status,omitempty"`
	LoopCounts  map[string]int    `json:"loop_counts,omitempty"`
	History     []statusHistEntry `json:"history,omitempty"`
}

type statusHistEntry struct {
	Node    string `json:"node"`
	Outcome string `json:"outcome"`
	EdgeID  string `json:"edge_id"`
}

func handleStatus(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, statusInput) (*sdkmcp.CallToolResult, statusOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input statusInput) (*sdkmcp.CallToolResult, statusOutput, error) {
		caseDir := rca.CaseDir(opts.basePath(), input.SuiteID, input.CaseID)
		state, err := rca.LoadCheckpointState(caseDir, input.CaseID)
		if err != nil {
			return nil, statusOutput{}, fmt.Errorf("load state: %w", err)
		}
		if state == nil {
			return nil, statusOutput{Found: false}, nil
		}
		out := statusOutput{
			Found:       true,
			CurrentNode: state.CurrentNode,
			Status:      state.Status,
			LoopCounts:  state.LoopCounts,
		}
		for _, h := range state.History {
			out.History = append(out.History, statusHistEntry{
				Node:    h.Node,
				Outcome: h.Outcome,
				EdgeID:  h.EdgeID,
			})
		}
		return nil, out, nil
	}
}

// --- dataset_status ---

type datasetStatusInput struct{}

type datasetStatusOutput struct {
	Verified   int    `json:"verified"`
	Candidates int    `json:"candidates"`
	DatasetDir string `json:"dataset_dir"`
}

func handleDatasetStatus(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, datasetStatusInput) (*sdkmcp.CallToolResult, datasetStatusOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ datasetStatusInput) (*sdkmcp.CallToolResult, datasetStatusOutput, error) {
		return nil, datasetStatusOutput{
			Verified:   countJSONFiles(opts.datasetDir()),
			Candidates: countJSONFiles(opts.candidateDir()),
			DatasetDir: opts.datasetDir(),
		}, nil
	}
}

// --- dataset_review ---

type datasetReviewInput struct{}

type datasetReviewOutput struct {
	Candidates []candidateEntry `json:"candidates"`
}

type candidateEntry struct {
	ID          string `json:"id"`
	TestName    string `json:"test_name"`
	SymptomName string `json:"symptom_name"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

func handleDatasetReview(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, datasetReviewInput) (*sdkmcp.CallToolResult, datasetReviewOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ datasetReviewInput) (*sdkmcp.CallToolResult, datasetReviewOutput, error) {
		dir := opts.candidateDir()
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, datasetReviewOutput{}, nil
			}
			return nil, datasetReviewOutput{}, fmt.Errorf("read candidates: %w", err)
		}

		var out datasetReviewOutput
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var c struct {
				ID          string `json:"id"`
				TestName    string `json:"test_name"`
				SymptomName string `json:"symptom_name"`
				Status      string `json:"status"`
				CreatedAt   string `json:"created_at"`
			}
			if json.Unmarshal(data, &c) != nil {
				continue
			}
			out.Candidates = append(out.Candidates, candidateEntry{
				ID:          c.ID,
				TestName:    c.TestName,
				SymptomName: c.SymptomName,
				Status:      c.Status,
				CreatedAt:   c.CreatedAt,
			})
		}
		return nil, out, nil
	}
}

// --- dataset_promote ---

type datasetPromoteInput struct {
	CandidateID string `json:"candidate_id"`
}

type datasetPromoteOutput struct {
	VerifiedID string `json:"verified_id"`
	Message    string `json:"message"`
}

func handleDatasetPromote(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, datasetPromoteInput) (*sdkmcp.CallToolResult, datasetPromoteOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input datasetPromoteInput) (*sdkmcp.CallToolResult, datasetPromoteOutput, error) {
		candDir := opts.candidateDir()
		dsDir := opts.datasetDir()

		srcPath := filepath.Join(candDir, input.CandidateID+".json")
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, datasetPromoteOutput{}, fmt.Errorf("candidate %q not found: %w", input.CandidateID, err)
		}

		var c map[string]any
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, datasetPromoteOutput{}, fmt.Errorf("parse candidate: %w", err)
		}

		c["status"] = "verified"

		verifiedID, err := nextVerifiedID(dsDir)
		if err != nil {
			return nil, datasetPromoteOutput{}, fmt.Errorf("assign ID: %w", err)
		}
		c["id"] = verifiedID

		if err := os.MkdirAll(dsDir, 0o755); err != nil {
			return nil, datasetPromoteOutput{}, fmt.Errorf("create dataset dir: %w", err)
		}

		out, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return nil, datasetPromoteOutput{}, fmt.Errorf("marshal: %w", err)
		}

		dstPath := filepath.Join(dsDir, verifiedID+".json")
		if err := os.WriteFile(dstPath, out, 0o644); err != nil {
			return nil, datasetPromoteOutput{}, fmt.Errorf("write: %w", err)
		}

		if err := os.Remove(srcPath); err != nil {
			return nil, datasetPromoteOutput{}, fmt.Errorf("remove candidate: %w", err)
		}

		return nil, datasetPromoteOutput{
			VerifiedID: verifiedID,
			Message:    fmt.Sprintf("Promoted %s → %s", input.CandidateID, verifiedID),
		}, nil
	}
}

// --- gt_status ---

type gtStatusInput struct {
	Scenario string `json:"scenario,omitempty"`
}

type gtStatusOutput struct {
	Datasets []string           `json:"datasets,omitempty"`
	Results  []completenessItem `json:"results,omitempty"`
	Total    int                `json:"total,omitempty"`
	Ready    int                `json:"ready,omitempty"`
}

type completenessItem struct {
	CaseID     string   `json:"case_id"`
	RCAID      string   `json:"rca_id"`
	Score      float64  `json:"score"`
	Promotable bool     `json:"promotable"`
	Missing    []string `json:"missing,omitempty"`
}

func handleGTStatus(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, gtStatusInput) (*sdkmcp.CallToolResult, gtStatusOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input gtStatusInput) (*sdkmcp.CallToolResult, gtStatusOutput, error) {
		dsDir := opts.datasetDir()
		store := newFileStore(dsDir)
		ctx := context.Background()

		if input.Scenario == "" {
			names, err := store.list(ctx)
			if err != nil {
				return nil, gtStatusOutput{}, err
			}
			return nil, gtStatusOutput{Datasets: names}, nil
		}

		scenario, err := store.load(ctx, input.Scenario)
		if err != nil {
			return nil, gtStatusOutput{}, err
		}

		var out gtStatusOutput
		out.Total = len(scenario.Cases)
		for _, c := range scenario.Cases {
			var rcaRec *rca.GroundTruthRCA
			for i := range scenario.RCAs {
				if scenario.RCAs[i].ID == c.RCAID {
					rcaRec = &scenario.RCAs[i]
					break
				}
			}
			item := completenessItem{
				CaseID:     c.ID,
				RCAID:      c.RCAID,
				Promotable: true,
			}
			var missing []string
			if c.TestName == "" {
				missing = append(missing, "test_name")
			}
			if c.ErrorMessage == "" {
				missing = append(missing, "error_message")
			}
			if c.RCAID == "" {
				missing = append(missing, "rca_id")
			}
			if rcaRec == nil && c.RCAID != "" {
				missing = append(missing, "rca_record")
			} else if rcaRec != nil {
				if rcaRec.DefectType == "" {
					missing = append(missing, "defect_type")
				}
				if rcaRec.Category == "" {
					missing = append(missing, "category")
				}
			}
			if len(missing) > 0 {
				item.Promotable = false
				item.Missing = missing
			}
			total := 6
			item.Score = float64(total-len(missing)) / float64(total)
			if item.Promotable {
				out.Ready++
			}
			out.Results = append(out.Results, item)
		}

		return nil, out, nil
	}
}

// --- gt_import ---

type gtImportInput struct {
	Scenario string `json:"scenario"`
}

type gtImportOutput struct {
	Cases      int    `json:"cases"`
	Candidates int    `json:"candidates"`
	Path       string `json:"path"`
}

func handleGTImport(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, gtImportInput) (*sdkmcp.CallToolResult, gtImportOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input gtImportInput) (*sdkmcp.CallToolResult, gtImportOutput, error) {
		scenario, err := scenarios.LoadScenario(nil, input.Scenario)
		if err != nil {
			return nil, gtImportOutput{}, fmt.Errorf("load scenario %q: %w", input.Scenario, err)
		}

		dsDir := opts.datasetDir()
		store := newFileStore(dsDir)
		if err := store.save(context.Background(), scenario); err != nil {
			return nil, gtImportOutput{}, fmt.Errorf("save: %w", err)
		}

		return nil, gtImportOutput{
			Cases:      len(scenario.Cases),
			Candidates: len(scenario.Candidates),
			Path:       filepath.Join(dsDir, scenario.Name+".json"),
		}, nil
	}
}

// --- gt_export ---

type gtExportInput struct {
	Scenario string `json:"scenario"`
}

type gtExportOutput struct {
	Name       string `json:"name"`
	Cases      int    `json:"cases"`
	Candidates int    `json:"candidates"`
	RCAs       int    `json:"rcas"`
	Symptoms   int    `json:"symptoms"`
}

func handleGTExport(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, gtExportInput) (*sdkmcp.CallToolResult, gtExportOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input gtExportInput) (*sdkmcp.CallToolResult, gtExportOutput, error) {
		store := newFileStore(opts.datasetDir())
		scenario, err := store.load(context.Background(), input.Scenario)
		if err != nil {
			return nil, gtExportOutput{}, err
		}
		return nil, gtExportOutput{
			Name:       scenario.Name,
			Cases:      len(scenario.Cases),
			Candidates: len(scenario.Candidates),
			RCAs:       len(scenario.RCAs),
			Symptoms:   len(scenario.Symptoms),
		}, nil
	}
}

// --- push_artifact ---

type pushInput struct {
	ArtifactJSON string `json:"artifact_json"`
}

type pushOutput struct {
	RunID      string `json:"run_id,omitempty"`
	DefectType string `json:"defect_type,omitempty"`
	Message    string `json:"message"`
}

func handlePush(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, pushInput) (*sdkmcp.CallToolResult, pushOutput, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input pushInput) (*sdkmcp.CallToolResult, pushOutput, error) {
		var verdict rca.RCAVerdict
		if err := json.Unmarshal([]byte(input.ArtifactJSON), &verdict); err != nil {
			return nil, pushOutput{}, fmt.Errorf("parse artifact: %w", err)
		}

		var writer rca.DefectWriter = rca.DefaultDefectWriter{}
		if opts.DefectWriter != nil {
			writer = opts.DefectWriter
		}

		rec, err := writer.Push(verdict)
		if err != nil {
			return nil, pushOutput{}, fmt.Errorf("push: %w", err)
		}
		if rec != nil {
			return nil, pushOutput{
				RunID:      rec.RunID,
				DefectType: rec.DefectType,
				Message:    fmt.Sprintf("Pushed: run=%s defect_type=%s", rec.RunID, rec.DefectType),
			}, nil
		}
		return nil, pushOutput{Message: "Push completed (no-op writer)"}, nil
	}
}

// --- consume ---

type consumeInput struct {
	DryRun       bool `json:"dry_run"`
	LookbackDays int  `json:"lookback_days,omitempty"`
}

type consumeOutput struct {
	Discovered int    `json:"discovered"`
	Matched    int    `json:"matched"`
	Written    int    `json:"written"`
	Duplicates int    `json:"duplicates"`
	Message    string `json:"message"`
}

func handleConsume(opts AdminToolOpts) func(context.Context, *sdkmcp.CallToolRequest, consumeInput) (*sdkmcp.CallToolResult, consumeOutput, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input consumeInput) (*sdkmcp.CallToolResult, consumeOutput, error) {
		if opts.Consume == nil {
			return nil, consumeOutput{}, fmt.Errorf("consume not configured (set AdminToolOpts.Consume)")
		}

		lookback := input.LookbackDays
		if lookback == 0 {
			lookback = 7
		}

		summary, err := opts.Consume(ctx, lookback, opts.datasetDir(), opts.candidateDir())
		if err != nil {
			return nil, consumeOutput{}, fmt.Errorf("consume: %w", err)
		}

		return nil, consumeOutput{
			Discovered: summary.Discovered,
			Matched:    summary.Matched,
			Written:    summary.Written,
			Duplicates: summary.Duplicates,
			Message:    fmt.Sprintf("Discovered %d, matched %d, created %d (%d dedup)", summary.Discovered, summary.Matched, summary.Written, summary.Duplicates),
		}, nil
	}
}

// --- helpers ---

func countJSONFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			n++
		}
	}
	return n
}

func nextVerifiedID(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	maxNum := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		var num int
		if _, err := fmt.Sscanf(name, "V%d", &num); err == nil && num > maxNum {
			maxNum = num
		}
	}
	return fmt.Sprintf("V%03d", maxNum+1), nil
}

type fileStore struct {
	dir string
}

func newFileStore(dir string) *fileStore { return &fileStore{dir: dir} }

func (fs *fileStore) list(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	return names, nil
}

func (fs *fileStore) load(_ context.Context, name string) (*rca.Scenario, error) {
	data, err := os.ReadFile(filepath.Join(fs.dir, name+".json"))
	if err != nil {
		return nil, fmt.Errorf("load dataset %q: %w", name, err)
	}
	var s rca.Scenario
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse dataset %q: %w", name, err)
	}
	return &s, nil
}

func (fs *fileStore) save(_ context.Context, s *rca.Scenario) error {
	if err := os.MkdirAll(fs.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(fs.dir, s.Name+".json"), data, 0o644)
}

