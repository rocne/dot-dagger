// Package fileset filters a walked directory tree by predicate evaluation,
// producing the shared active-file context consumed by all downstream stages.
package fileset

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/predicate"
	"github.com/rocne/dot-dagger/internal/walk"
)

// Kind mirrors walk.Kind and classifies active nodes by directory type.
type Kind = walk.Kind

const (
	KindOther  = walk.KindOther
	KindScript = walk.KindScript
	KindConf   = walk.KindConf
	KindBin    = walk.KindBin
)

// Node is an active file in the FileSet.
type Node struct {
	// Path is the absolute filesystem path.
	Path string

	// Kind identifies the special directory type.
	Kind Kind

	// LogicalName is the DAG identity of the file.
	LogicalName string

	// Annotations are the resolved annotations for this file.
	Annotations []annotation.Annotation
}

// Set is the shared in-memory context passed to all downstream stages.
// It contains only active (predicate-passing) nodes.
type Set struct {
	// Nodes are the active files after predicate evaluation.
	Nodes []Node

	// Env is the fully resolved environment used during filtering.
	Env map[string]string
}

// Scripts returns all nodes with KindScript.
func (s *Set) Scripts() []Node { return s.byKind(KindScript) }

// Conf returns all nodes with KindConf.
func (s *Set) Conf() []Node { return s.byKind(KindConf) }

// Bin returns all nodes with KindBin.
func (s *Set) Bin() []Node { return s.byKind(KindBin) }

func (s *Set) byKind(k Kind) []Node {
	var result []Node
	for _, n := range s.Nodes {
		if n.Kind == k {
			result = append(result, n)
		}
	}
	return result
}

// Options configures predicate evaluation during Build.
type Options struct {
	// Funcs is an optional registry of custom predicate functions.
	// If nil, only built-in functions (exists) are available.
	Funcs *predicate.FuncRegistry
}

// Build evaluates predicates on raw walk nodes against env and returns
// a Set containing only the active nodes.
// Nodes with an empty EffectiveWhen are unconditionally included.
func Build(nodes []walk.Node, env map[string]string, opts *Options) (*Set, error) {
	if opts == nil {
		opts = &Options{}
	}

	ev := &predicate.Evaluator{
		Env:   env,
		Funcs: opts.Funcs,
	}

	var active []Node
	for _, n := range nodes {
		ok, err := evaluate(ev, n.EffectiveWhen)
		if err != nil {
			return nil, fmt.Errorf("fileset: evaluate %s: %w", n.Path, err)
		}
		if ok {
			active = append(active, Node{
				Path:        n.Path,
				Kind:        n.Kind,
				LogicalName: n.LogicalName,
				Annotations: n.Annotations,
			})
		}
	}

	return &Set{Nodes: active, Env: env}, nil
}

// evaluate parses and evaluates a when expression.
// Empty expression returns true (unconditionally active).
func evaluate(ev *predicate.Evaluator, when string) (bool, error) {
	expr, err := predicate.Parse(when)
	if err != nil {
		return false, fmt.Errorf("parse predicate %q: %w", when, err)
	}
	return ev.Eval(expr)
}
