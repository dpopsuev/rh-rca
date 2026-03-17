package rca

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFillTemplateString(t *testing.T) {
	params := &TemplateParams{
		CaseID:   42,
		StepName: "F1_TRIAGE",
		Failure: &FailureParams{
			TestName:     "[T-TSC] PTP Recovery test",
			ErrorMessage: "Expected 0 to equal 1",
			Status:       "FAILED",
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `# Test: {{.Failure.TestName}}
Case: #{{.CaseID}}
Step: {{.StepName}}
{{if .Failure.ErrorMessage}}Error: {{.Failure.ErrorMessage}}{{end}}
{{if .Failure.LogTruncated}}LOG TRUNCATED{{end}}`

	result, err := FillTemplateString("test", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "[T-TSC] PTP Recovery test") {
		t.Errorf("missing test name in output: %s", result)
	}
	if !strings.Contains(result, "#42") {
		t.Errorf("missing case ID in output: %s", result)
	}
	if !strings.Contains(result, "Expected 0 to equal 1") {
		t.Errorf("missing error message in output: %s", result)
	}
	if strings.Contains(result, "LOG TRUNCATED") {
		t.Errorf("LogTruncated should be false, but output contains marker: %s", result)
	}
}

func TestFillTemplateString_Guards(t *testing.T) {
	params := &TemplateParams{
		CaseID: 1,
		Failure: &FailureParams{
			LogTruncated: true,
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `{{if .Failure.LogTruncated}}TRUNCATED{{end}}
{{if not .Failure.ErrorMessage}}NO_ERROR{{end}}`

	result, err := FillTemplateString("guards", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "TRUNCATED") {
		t.Error("expected TRUNCATED guard to fire")
	}
	if !strings.Contains(result, "NO_ERROR") {
		t.Error("expected NO_ERROR guard to fire")
	}
}

func TestFillTemplateString_Siblings(t *testing.T) {
	params := &TemplateParams{
		CaseID:  1,
		Failure: &FailureParams{TestName: "test1"},
		Siblings: []SiblingParams{
			{ID: "1", Name: "test1", Status: "FAILED"},
			{ID: "2", Name: "test2", Status: "FAILED"},
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `{{range .Siblings}}{{.ID}}: {{.Name}}
{{end}}`

	result, err := FillTemplateString("siblings", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "1: test1") || !strings.Contains(result, "2: test2") {
		t.Errorf("siblings not rendered: %s", result)
	}
}

func TestFillTemplate_File(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.md")

	content := `# {{.StepName}}
Case: #{{.CaseID}}
Test: {{.Failure.TestName}}`

	if err := os.WriteFile(tmplPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	params := &TemplateParams{
		CaseID:   99,
		StepName: "F3_INVESTIGATE",
		Failure:  &FailureParams{TestName: "my test"},
		Taxonomy: DefaultTaxonomy(),
	}

	result, err := FillTemplate(tmplPath, params)
	if err != nil {
		t.Fatalf("FillTemplate: %v", err)
	}
	if !strings.Contains(result, "#99") || !strings.Contains(result, "my test") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestFillTemplateString_PriorArtifacts(t *testing.T) {
	params := &TemplateParams{
		CaseID:  1,
		Failure: &FailureParams{TestName: "test"},
		Prior: &PriorParams{
			"Triage": {
				"symptom_category":       "assertion",
				"defect_type_hypothesis": "pb001",
			},
			"Investigate": {
				"rca_message":       "Root cause found",
				"defect_type":       "pb001",
				"convergence_score": 0.85,
			},
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `{{if .Prior}}{{if .Prior.Triage}}Category: {{.Prior.Triage.symptom_category}}{{end}}
{{if .Prior.Investigate}}RCA: {{.Prior.Investigate.rca_message}} ({{.Prior.Investigate.convergence_score}}){{end}}{{end}}`

	result, err := FillTemplateString("prior", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "Category: assertion") {
		t.Errorf("missing triage category: %s", result)
	}
	if !strings.Contains(result, "Root cause found") {
		t.Errorf("missing RCA message: %s", result)
	}
}

func TestValidateTemplateFields(t *testing.T) {
	paramType := reflect.TypeOf(TemplateParams{})

	tests := []struct {
		name      string
		tmpl      string
		wantErrs  int
		wantField string
	}{
		{
			name:     "valid top-level field",
			tmpl:     `{{.CaseID}}`,
			wantErrs: 0,
		},
		{
			name:     "valid nested field",
			tmpl:     `{{.Failure.ErrorMessage}}`,
			wantErrs: 0,
		},
		{
			name:      "typo in nested field",
			tmpl:      `{{.Failure.ErrorMesage}}`,
			wantErrs:  1,
			wantField: "Failure.ErrorMesage",
		},
		{
			name:     "deeply nested field",
			tmpl:     `{{if .Prior}}{{.Prior.Triage.defect_type_hypothesis}}{{end}}`,
			wantErrs: 0,
		},
		{
			name:      "nonexistent top-level field",
			tmpl:      `{{.Bogus}}`,
			wantErrs:  1,
			wantField: "Bogus",
		},
		{
			name:     "map field access accepted",
			tmpl:     `{{range .Env}}{{.}}{{end}}`,
			wantErrs: 0,
		},
		{
			name:     "range over slice with valid element field",
			tmpl:     `{{range .Siblings}}{{.Name}}{{end}}`,
			wantErrs: 0,
		},
		{
			name:      "range over slice with invalid element field",
			tmpl:      `{{range .Siblings}}{{.BadField}}{{end}}`,
			wantErrs:  1,
			wantField: "BadField",
		},
		{
			name:     "if guard on boolean",
			tmpl:     `{{if .Failure.LogTruncated}}truncated{{end}}`,
			wantErrs: 0,
		},
		{
			name:     "function call with field arg",
			tmpl:     `{{sub .CaseID 1}}`,
			wantErrs: 0,
		},
		{
			name:      "multiple errors",
			tmpl:      `{{.Bad1}} {{.Bad2}}`,
			wantErrs:  2,
			wantField: "Bad1",
		},
		{
			name:     "with narrows dot type",
			tmpl:     `{{with .Failure}}{{.TestName}}{{end}}`,
			wantErrs: 0,
		},
		{
			name:      "with narrows dot — invalid inner field",
			tmpl:      `{{with .Failure}}{{.CaseID}}{{end}}`,
			wantErrs:  1,
			wantField: "CaseID",
		},
		{
			name:     "plain text — no errors",
			tmpl:     `No template directives here.`,
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateTemplateFields(tt.tmpl, paramType, PromptFuncMap)
			if len(errs) != tt.wantErrs {
				t.Errorf("got %d errors, want %d: %+v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantField != "" && len(errs) > 0 && errs[0].Field != tt.wantField {
				t.Errorf("first error field = %q, want %q", errs[0].Field, tt.wantField)
			}
		})
	}
}

func TestValidateTemplateFields_ParseError(t *testing.T) {
	errs := ValidateTemplateFields(`{{.Broken`, reflect.TypeOf(TemplateParams{}), PromptFuncMap)
	if len(errs) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Message, "parse error") {
		t.Errorf("expected parse error message, got %q", errs[0].Message)
	}
}

func TestTemplateParams_AllFieldsUsed(t *testing.T) {
	paramType := reflect.TypeOf(TemplateParams{})
	allPaths := AllFieldPaths(paramType)

	// Fields intentionally not used in prompt templates.
	// These exist for Go-level consumption (linking, routing, scoring, etc.).
	excluded := map[string]string{
		"URLs":                 "navigable links used in Go output, not in LLM prompts",
		"URLs.SourceDashboard": "navigable links used in Go output, not in LLM prompts",
		"URLs.SourceItem":      "navigable links used in Go output, not in LLM prompts",
		"Env":                  "env vars injected into context map, not referenced in templates",
		"Failure.Path":         "file path used for Go-level routing, not in prompts",
		"Envelope.Status":      "launch status used for Go-level filtering, not in prompts",

		"History.PriorRCAs.DaysSinceResolved": "available but not surfaced in current prompt templates",

		"Code.Trees":               "populated by Harvester subgraph but not yet rendered in prompts",
		"Code.Trees.Repo":          "sub-field of Code.Trees",
		"Code.Trees.Branch":        "sub-field of Code.Trees",
		"Code.Trees.Entries":       "sub-field of Code.Trees",
		"Code.Trees.Entries.Path":  "sub-field of Code.Trees",
		"Code.Trees.Entries.IsDir": "sub-field of Code.Trees",
		"Code.SearchResults.Score": "used for ranking in Go, not rendered in prompts",
	}

	// Collect all field references across all embedded prompt templates.
	refs := make(map[string]bool)
	promptFS := testdataPromptFS()
	err := fs.WalkDir(promptFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, readErr := fs.ReadFile(promptFS, path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "{{") {
			return nil
		}
		fields, extractErr := ExtractTemplateFields(content, paramType, PromptFuncMap)
		if extractErr != nil {
			t.Logf("skipping %s: %v", path, extractErr)
			return nil
		}
		for _, f := range fields {
			refs[f] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk prompts: %v", err)
	}

	var uncovered []string
	for _, path := range allPaths {
		if _, ok := excluded[path]; ok {
			continue
		}
		if !refs[path] {
			uncovered = append(uncovered, path)
		}
	}

	if len(uncovered) > 0 {
		t.Errorf("TemplateParams fields not referenced by any prompt template (%d):\n", len(uncovered))
		for _, p := range uncovered {
			t.Errorf("  - %s", p)
		}
		t.Errorf("\nAdd the field to a prompt template, or add it to the excluded map with a reason.")
	}
}
