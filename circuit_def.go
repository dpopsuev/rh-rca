package rca

import (
	_ "embed"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

//go:embed circuit.yaml
var defaultCircuitYAML []byte

// DefaultCircuitYAML returns the embedded base RCA circuit definition.
func DefaultCircuitYAML() []byte { return defaultCircuitYAML }

// SchematicResolver returns an AssetResolver that resolves "rca" to the
// embedded base circuit. Consumer overlays use `import: rca` to merge
// with this base.
func SchematicResolver() framework.AssetResolver {
	return func(name string) ([]byte, error) {
		if name == "rca" {
			return defaultCircuitYAML, nil
		}
		return nil, fmt.Errorf("unknown schematic %q", name)
	}
}

// ThresholdsToVars converts typed Thresholds to a map for circuit vars / expression config.
func ThresholdsToVars(th Thresholds) map[string]any {
	return map[string]any{
		"recall_hit":             th.RecallHit,
		"recall_uncertain":       th.RecallUncertain,
		"convergence_sufficient": th.ConvergenceSufficient,
		"max_investigate_loops":  th.MaxInvestigateLoops,
		"correlate_dup":          th.CorrelateDup,
	}
}

// LoadCircuitDef loads an RCA circuit from the given YAML data and
// overrides vars with the provided thresholds. If the data is a consumer
// overlay (has `import: rca`), it is merged with the embedded base circuit.
// If data is nil, the embedded base circuit is used directly.
func LoadCircuitDef(data []byte, th Thresholds) (*framework.CircuitDef, error) {
	if data == nil {
		data = defaultCircuitYAML
	}
	def, err := framework.LoadCircuitWithOverlay(data, SchematicResolver())
	if err != nil {
		return nil, fmt.Errorf("load circuit YAML: %w", err)
	}
	thVars := ThresholdsToVars(th)
	if def.Vars == nil {
		def.Vars = thVars
	} else {
		for k, v := range thVars {
			def.Vars[k] = v
		}
	}
	return def, nil
}

// BuildRunner constructs a framework.Runner from the RCA circuit
// definition with the given thresholds and components. When circuitData
// is nil the embedded default is used.
func BuildRunner(circuitData []byte, th Thresholds, comps ...*framework.Component) (*framework.Runner, error) {
	def, err := LoadCircuitDef(circuitData, th)
	if err != nil {
		return nil, err
	}
	reg := framework.GraphRegistries{}
	if len(comps) > 0 {
		reg, err = framework.MergeComponents(reg, comps...)
		if err != nil {
			return nil, fmt.Errorf("merge components: %w", err)
		}
	}
	return framework.NewRunnerWith(def, reg)
}

