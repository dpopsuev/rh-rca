package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/dispatch"
)

const calibrationPreamble = `> **CALIBRATION MODE — BLIND EVALUATION**
>
> You are participating in a calibration run. Your responses at each circuit
> step will be **scored against known ground truth** using 20 metrics including
> defect type accuracy, component identification, evidence quality, circuit
> path efficiency, and semantic relevance.
>
> **Rules:**
> 1. Respond ONLY based on the information provided in this prompt.
> 2. Do NOT read scenario definition files, ground truth files, expected
>    results, or any calibration/test code in the repository. This includes
>    any file under ` + "`internal/calibrate/scenarios/`" + `, any ` + "`*_test.go`" + ` file,
>    and the ` + "`.cursor/contracts/`" + ` directory.
> 3. Do NOT look at previous artifact files for other cases unless
>    explicitly referenced in the prompt context.
> 4. Treat each step independently — base your output solely on the
>    provided context for THIS step.
>
> Violating these rules contaminates the calibration signal.

`

type rcaTransformer struct {
	dispatcher dispatch.Dispatcher
	promptFS   fs.FS
	basePath   string
}

type RCATransformerOption func(*rcaTransformer)

func WithRCABasePath(p string) RCATransformerOption {
	return func(t *rcaTransformer) { t.basePath = p }
}

// NewRCATransformer creates an RCA transformer that reads prompt templates
// from promptFS. Pass DefaultPromptFS for embedded prompts, or os.DirFS(dir)
// to override with a custom prompt directory.
func NewRCATransformer(d dispatch.Dispatcher, promptFS fs.FS, opts ...RCATransformerOption) framework.Transformer {
	t := &rcaTransformer{
		dispatcher: d,
		promptFS:   promptFS,
		basePath:   DefaultBasePath,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *rcaTransformer) Name() string        { return "rca" }
func (t *rcaTransformer) Deterministic() bool { return false }

func (t *rcaTransformer) Transform(ctx context.Context, tc *framework.TransformerContext) (any, error) {
	nodeName := tc.NodeName
	if nodeName == "" {
		return nil, fmt.Errorf("rca transformer: empty node name")
	}

	params := ParamsFromContext(tc.WalkerState.Context)
	params.StepName = nodeName

	templatePath := tc.Prompt
	if templatePath == "" {
		return nil, fmt.Errorf("rca transformer: node %q has no prompt: field", nodeName)
	}

	prompt, err := FillTemplateFS(t.promptFS, templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("rca transformer: fill template for %s: %w", nodeName, err)
	}
	prompt = calibrationPreamble + prompt

	caseDir, _ := tc.WalkerState.Context[KeyCaseDir].(string)
	if caseDir == "" {
		caseDir = os.TempDir()
	}

	promptFile := filepath.Join(caseDir, NodePromptFilename(nodeName, 0))
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return nil, fmt.Errorf("rca transformer: write prompt: %w", err)
	}

	artifactFile := filepath.Join(caseDir, NodeArtifactFilename(nodeName))

	caseLabel, _ := tc.WalkerState.Context[KeyCaseLabel].(string)
	if caseLabel == "" {
		caseLabel = tc.WalkerState.ID
	}

	data, err := t.dispatcher.Dispatch(ctx, dispatch.DispatchContext{
		CaseID: caseLabel, Step: nodeName,
		PromptPath: promptFile, ArtifactPath: artifactFile,
	})
	if err != nil {
		return nil, fmt.Errorf("rca transformer: dispatch %s/%s: %w", caseLabel, nodeName, err)
	}

	if f := dispatch.UnwrapFinalizer(t.dispatcher); f != nil {
		f.MarkDone(artifactFile)
	}

	return parseArtifact(json.RawMessage(data))
}
