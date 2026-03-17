package rca

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"
)

// PromptFuncMap is the shared FuncMap for all prompt templates.
var PromptFuncMap = template.FuncMap{
	"sub": func(a, b int) int { return a - b },
	"add": func(a, b int) int { return a + b },
}

// FillTemplateFS loads a Go text/template from an fs.FS, executes it with
// the given params, and returns the rendered string. Template guards (G1–G34)
// are embedded in the templates via conditional blocks ({{if .Field}}).
func FillTemplateFS(fsys fs.FS, templatePath string, params *TemplateParams) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf("read template %s: prompt filesystem is nil", templatePath)
	}
	data, err := fs.ReadFile(fsys, templatePath)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", templatePath, err)
	}
	return FillTemplateString(templatePath, string(data), params)
}

// FillTemplate loads a Go text/template file from disk, executes it with the
// given params, and returns the rendered string. Prefer FillTemplateFS for new
// code; this function exists for callers that work with absolute disk paths.
func FillTemplate(templatePath string, params *TemplateParams) (string, error) {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", templatePath, err)
	}
	return FillTemplateString(templatePath, string(data), params)
}

// FillTemplateString executes a Go text/template from a raw string with the
// given params. Useful for embedded/inline templates and testing.
func FillTemplateString(name, tmplStr string, params *TemplateParams) (string, error) {
	tmpl, err := template.New(name).Funcs(PromptFuncMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}


// FieldError reports a template field reference that cannot be resolved
// against the expected parameter type.
type FieldError struct {
	Field   string
	Message string
}

// ValidateTemplateFields parses a Go text/template and checks that every
// {{.Field}} chain resolves against paramType via reflection.  It handles
// struct fields, pointer dereferencing, range (slice/map element types),
// and with (narrowed dot type).  Map key accesses and interface values
// are silently accepted because their fields cannot be statically checked.
func ValidateTemplateFields(content string, paramType reflect.Type, funcMap template.FuncMap) []FieldError {
	if funcMap == nil {
		funcMap = template.FuncMap{}
	}
	tmpl, err := template.New("validate").Funcs(funcMap).Parse(content)
	if err != nil {
		return []FieldError{{Message: fmt.Sprintf("template parse error: %v", err)}}
	}
	var errs []FieldError
	walkValidateNode(tmpl.Tree.Root, derefType(paramType), &errs)
	return errs
}

func walkValidateNode(node parse.Node, dotType reflect.Type, errs *[]FieldError) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			walkValidateNode(child, dotType, errs)
		}
	case *parse.ActionNode:
		walkValidatePipe(n.Pipe, dotType, errs)
	case *parse.IfNode:
		walkValidatePipe(n.Pipe, dotType, errs)
		walkValidateNode(n.List, dotType, errs)
		walkValidateNode(n.ElseList, dotType, errs)
	case *parse.RangeNode:
		walkValidatePipe(n.Pipe, dotType, errs)
		elemType := resolveRangeType(n.Pipe, dotType)
		walkValidateNode(n.List, elemType, errs)
		walkValidateNode(n.ElseList, dotType, errs)
	case *parse.WithNode:
		walkValidatePipe(n.Pipe, dotType, errs)
		if withType := resolvePipeType(n.Pipe, dotType); withType != nil {
			walkValidateNode(n.List, withType, errs)
		} else {
			walkValidateNode(n.List, dotType, errs)
		}
		walkValidateNode(n.ElseList, dotType, errs)
	}
}

func walkValidatePipe(pipe *parse.PipeNode, dotType reflect.Type, errs *[]FieldError) {
	if pipe == nil {
		return
	}
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			if fn, ok := arg.(*parse.FieldNode); ok {
				if err := checkFieldChain(fn.Ident, dotType); err != nil {
					*errs = append(*errs, FieldError{
						Field:   strings.Join(fn.Ident, "."),
						Message: err.Error(),
					})
				}
			}
		}
	}
}

func checkFieldChain(chain []string, typ reflect.Type) error {
	current := derefType(typ)
	for _, field := range chain {
		switch current.Kind() {
		case reflect.Struct:
			f, ok := current.FieldByName(field)
			if !ok {
				return fmt.Errorf("type %s has no field %q", current.Name(), field)
			}
			current = derefType(f.Type)
		case reflect.Map, reflect.Interface:
			return nil
		default:
			return fmt.Errorf("cannot access field %q on %v", field, current)
		}
	}
	return nil
}

// resolvePipeType returns the reflected type of the first field reference in
// a pipe, or nil if the pipe contains no field node.
func resolvePipeType(pipe *parse.PipeNode, dotType reflect.Type) reflect.Type {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return nil
	}
	for _, arg := range pipe.Cmds[0].Args {
		if fn, ok := arg.(*parse.FieldNode); ok {
			return resolveChainType(fn.Ident, dotType)
		}
	}
	return nil
}

// resolveRangeType determines the dot type inside a range body by unwrapping
// one level of slice/array/map from the pipe's field type.
func resolveRangeType(pipe *parse.PipeNode, dotType reflect.Type) reflect.Type {
	t := resolvePipeType(pipe, dotType)
	if t == nil {
		return dotType
	}
	t = derefType(t)
	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return derefType(t.Elem())
	default:
		return dotType
	}
}

func resolveChainType(chain []string, typ reflect.Type) reflect.Type {
	current := derefType(typ)
	for _, field := range chain {
		if current.Kind() != reflect.Struct {
			return nil
		}
		f, ok := current.FieldByName(field)
		if !ok {
			return nil
		}
		current = derefType(f.Type)
	}
	return current
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

// ExtractTemplateFields parses a Go text/template and returns all root-relative
// field paths referenced via {{.X.Y}} syntax.  Range and with blocks produce
// prefixed paths (e.g. "Siblings.Name" for {{range .Siblings}}{{.Name}}{{end}}).
// Intermediate paths are included (e.g. "Prior" from {{.Prior.Triage}}).
func ExtractTemplateFields(content string, rootType reflect.Type, funcMap template.FuncMap) ([]string, error) {
	if funcMap == nil {
		funcMap = template.FuncMap{}
	}
	tmpl, err := template.New("extract").Funcs(funcMap).Parse(content)
	if err != nil {
		return nil, err
	}
	refs := make(map[string]bool)
	walkExtractNode(tmpl.Tree.Root, derefType(rootType), "", refs)
	result := make([]string, 0, len(refs))
	for ref := range refs {
		result = append(result, ref)
	}
	sort.Strings(result)
	return result, nil
}

func walkExtractNode(node parse.Node, dotType reflect.Type, prefix string, refs map[string]bool) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			walkExtractNode(child, dotType, prefix, refs)
		}
	case *parse.ActionNode:
		extractPipeFields(n.Pipe, prefix, refs)
	case *parse.IfNode:
		extractPipeFields(n.Pipe, prefix, refs)
		walkExtractNode(n.List, dotType, prefix, refs)
		walkExtractNode(n.ElseList, dotType, prefix, refs)
	case *parse.RangeNode:
		extractPipeFields(n.Pipe, prefix, refs)
		newPrefix := pipeFieldPrefix(n.Pipe, prefix)
		elemType := resolveRangeType(n.Pipe, dotType)
		walkExtractNode(n.List, elemType, newPrefix, refs)
		walkExtractNode(n.ElseList, dotType, prefix, refs)
	case *parse.WithNode:
		extractPipeFields(n.Pipe, prefix, refs)
		newPrefix := pipeFieldPrefix(n.Pipe, prefix)
		if withType := resolvePipeType(n.Pipe, dotType); withType != nil {
			walkExtractNode(n.List, withType, newPrefix, refs)
		} else {
			walkExtractNode(n.List, dotType, prefix, refs)
		}
		walkExtractNode(n.ElseList, dotType, prefix, refs)
	}
}

func extractPipeFields(pipe *parse.PipeNode, prefix string, refs map[string]bool) {
	if pipe == nil {
		return
	}
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			fn, ok := arg.(*parse.FieldNode)
			if !ok {
				continue
			}
			full := joinFieldPath(prefix, fn.Ident)
			refs[full] = true
			for i := 1; i < len(fn.Ident); i++ {
				refs[joinFieldPath(prefix, fn.Ident[:i])] = true
			}
		}
	}
}

func pipeFieldPrefix(pipe *parse.PipeNode, currentPrefix string) string {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return currentPrefix
	}
	for _, arg := range pipe.Cmds[0].Args {
		if fn, ok := arg.(*parse.FieldNode); ok {
			return joinFieldPath(currentPrefix, fn.Ident)
		}
	}
	return currentPrefix
}

func joinFieldPath(prefix string, ident []string) string {
	chain := strings.Join(ident, ".")
	if prefix == "" {
		return chain
	}
	return prefix + "." + chain
}

// AllFieldPaths returns all exported field paths of a struct type, recursively
// walking nested structs and slice element types.  Paths use dot notation
// (e.g. "Failure.ErrorMessage", "Siblings.Name").
func AllFieldPaths(t reflect.Type) []string {
	var paths []string
	collectFieldPaths(derefType(t), "", &paths)
	return paths
}

func collectFieldPaths(t reflect.Type, prefix string, paths *[]string) {
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		path := f.Name
		if prefix != "" {
			path = prefix + "." + f.Name
		}
		*paths = append(*paths, path)
		ft := derefType(f.Type)
		switch ft.Kind() {
		case reflect.Struct:
			collectFieldPaths(ft, path, paths)
		case reflect.Slice, reflect.Array:
			elem := derefType(ft.Elem())
			if elem.Kind() == reflect.Struct {
				collectFieldPaths(elem, path, paths)
			}
		}
	}
}
