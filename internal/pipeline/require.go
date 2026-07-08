package pipeline

import "github.com/rocne/dot-dagger/internal/packages"

// UnmetRequire records a node whose @require dependency is neither installed
// nor installable — the gate documented in docs/reference/annotations.md
// ("the file is only active if the package is installed or can be
// installed").
type UnmetRequire struct {
	Node    string // node's LogicalName
	Package string // the unmet package name
}

// CheckRequirements evaluates every node's Require list and returns one
// UnmetRequire per (node, package) pair that is neither installed nor
// installable — i.e. there is no way for the requirement to end up satisfied,
// even after 'dotd package generate' runs. Installed-or-installable pairs are
// not reported; apply's package step is responsible for actually installing
// installable-but-missing packages.
//
// reg == nil is treated as an empty registry (PATH-only lookups), consistent
// with how Filter treats a nil registry. lookPath is injected so the pipeline
// stays pure — pass exec.LookPath from the cmd layer.
func CheckRequirements(nodes []RawNode, reg *packages.Registry, lookPath func(string) (string, error)) ([]UnmetRequire, error) {
	if reg == nil {
		reg = packages.EmptyRegistry()
	}

	var unmet []UnmetRequire
	for _, n := range nodes {
		for _, pkg := range n.Require {
			installed, err := packages.Installed(pkg, reg, lookPath)
			if err != nil {
				return nil, err
			}
			if installed {
				continue
			}
			installable, err := packages.Installable(pkg, reg, lookPath)
			if err != nil {
				return nil, err
			}
			if installable {
				continue
			}
			unmet = append(unmet, UnmetRequire{Node: n.LogicalName, Package: pkg})
		}
	}
	return unmet, nil
}
