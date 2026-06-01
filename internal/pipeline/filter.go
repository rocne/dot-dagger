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
	ev := predicate.NewEvaluator(env)
	return ev.Eval(parsed)
}

// CollectMissingKeys returns env keys referenced by predicate expressions across
// all nodes that are absent from env. Uses AST key extraction (no evaluation),
// so AND/OR short-circuiting cannot hide keys. Returns keys in encounter order.
func CollectMissingKeys(nodes []RawNode, env map[string]string) ([]string, error) {
	seen := map[string]bool{}
	var keys []string
	for _, n := range nodes {
		if n.EffectiveWhen == "" {
			continue
		}
		parsed, err := predicate.Parse(n.EffectiveWhen)
		if err != nil {
			return nil, fmt.Errorf("filter: node %q: %w", n.LogicalName, err)
		}
		for _, k := range parsed.Keys() {
			if _, ok := env[k]; !ok && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}
