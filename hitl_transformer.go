package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	framework "github.com/dpopsuev/origami"
)

// hitlTransformerNode implements framework.Transformer for HITL mode.
// It fills a prompt template and returns framework.Interrupt to pause
// the walk for human input. On resume, the caller injects the artifact
// via the "resume_input" context key, and this transformer parses and
// returns it.
type hitlTransformerNode struct {
	nodeName string
}

func (t *hitlTransformerNode) Name() string        { return "hitl-" + t.nodeName }
func (t *hitlTransformerNode) Deterministic() bool { return false }

func (t *hitlTransformerNode) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	if input, ok := tc.WalkerState.Context["resume_input"]; ok {
		delete(tc.WalkerState.Context, "resume_input")
		data, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("hitl %s: marshal resume_input: %w", t.nodeName, err)
		}
		return parseArtifact(json.RawMessage(data))
	}

	caseDir, _ := tc.WalkerState.Context[KeyCaseDir].(string)

	promptFS, _ := tc.WalkerState.Context[KeyPromptFS].(fs.FS)

	params := ParamsFromContext(tc.WalkerState.Context)
	params.StepName = t.nodeName

	templatePath := tc.Prompt
	if templatePath == "" {
		return nil, fmt.Errorf("hitl %s: node %q has no prompt: field", t.nodeName, tc.NodeName)
	}

	prompt, err := FillTemplateFS(promptFS, templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("hitl %s: fill template: %w", t.nodeName, err)
	}

	loopIter := tc.WalkerState.LoopCounts[tc.NodeName]
	promptPath, err := WriteNodePrompt(caseDir, t.nodeName, loopIter, prompt)
	if err != nil {
		return nil, fmt.Errorf("hitl %s: write prompt: %w", t.nodeName, err)
	}

	return nil, framework.Interrupt{
		Reason: fmt.Sprintf("awaiting human input for %s", t.nodeName),
		Data:   map[string]any{"prompt_path": promptPath, "step": t.nodeName},
	}
}
