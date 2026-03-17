package rca

import (
	"fmt"

	framework "github.com/dpopsuev/origami"
)

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
// overrides vars with the provided thresholds.
func LoadCircuitDef(data []byte, th Thresholds) (*framework.CircuitDef, error) {
	if data == nil {
		return nil, fmt.Errorf("circuit definition data is required")
	}
	def, err := framework.LoadCircuit(data)
	if err != nil {
		return nil, fmt.Errorf("load circuit YAML: %w", err)
	}
	def.Vars = ThresholdsToVars(th)
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

