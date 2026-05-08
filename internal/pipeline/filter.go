package pipeline

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/predicate"
)

// Filter returns nodes whose EffectiveWhen predicate matches env.
// Nodes with an empty EffectiveWhen are always included.
func Filter(nodes []RawNode, env map[string]string) ([]RawNode, error) {
	var active []RawNode
	for _, n := range nodes {
		ok, err := evalWhen(n.EffectiveWhen, env)
		if err != nil {
			return nil, fmt.Errorf("filter: node %q: %w", n.LogicalName, err)
		}
		if ok {
			active = append(active, n)
		}
	}
	return active, nil
}

func evalWhen(expr string, env map[string]string) (bool, error) {
	if expr == "" {
		return true, nil
	}
	parsed, err := predicate.Parse(expr)
	if err != nil {
		return false, err
	}
	ev := &predicate.Evaluator{Env: env, Funcs: predicate.NewFuncRegistry(predicate.Strict)}
	return ev.Eval(parsed)
}
