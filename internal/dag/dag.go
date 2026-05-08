// Package dag builds a topologically sorted DAG from script nodes,
// using @after annotations for ordering and alphabetical tie-breaking.
package dag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
)

// Build validates and topologically sorts nodes.
// Returns nodes in DAG order (alphabetical within each frontier tier).
// Errors on duplicate logical names or cycles.
func Build(nodes []fileset.Node) ([]fileset.Node, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	// Conflict detection: duplicate logical names.
	seen := make(map[string]string) // logical name → path
	for _, n := range nodes {
		if prev, ok := seen[n.LogicalName]; ok {
			return nil, fmt.Errorf("dag: conflict: logical name %q used by %s and %s",
				n.LogicalName, prev, n.Path)
		}
		seen[n.LogicalName] = n.Path
	}

	// Build index: logical name → slice index.
	idx := make(map[string]int, len(nodes))
	for i, n := range nodes {
		idx[n.LogicalName] = i
	}

	// Build adjacency: successors[i] = nodes that come after nodes[i].
	// inDegree[i] = number of nodes that must precede nodes[i].
	successors := make([][]int, len(nodes))
	inDegree := make([]int, len(nodes))

	for i, n := range nodes {
		for _, a := range annotation.Get(n.Annotations, "after") {
			if a.Args == "" {
				continue
			}
			deps := resolveAfter(a.Args, nodes)
			for _, dep := range deps {
				j, ok := idx[dep]
				if !ok {
					continue // missing @after target is a no-op
				}
				if j == i {
					continue // self-reference
				}
				successors[j] = append(successors[j], i)
				inDegree[i]++
			}
		}
	}

	// Kahn's algorithm with alphabetical tie-breaking.
	// frontier holds indices of nodes with inDegree == 0, kept sorted by logical name.
	var frontier []int
	for i := range nodes {
		if inDegree[i] == 0 {
			frontier = append(frontier, i)
		}
	}
	sortByName(frontier, nodes)

	result := make([]fileset.Node, 0, len(nodes))
	for len(frontier) > 0 {
		// Pop the first (alphabetically smallest) node.
		cur := frontier[0]
		frontier = frontier[1:]
		result = append(result, nodes[cur])

		// Reduce in-degree of successors; add newly-free nodes to frontier.
		newFree := frontier[:0:0] // empty slice to collect new additions
		for _, s := range successors[cur] {
			inDegree[s]--
			if inDegree[s] == 0 {
				newFree = append(newFree, s)
			}
		}
		if len(newFree) > 0 {
			sortByName(newFree, nodes)
			frontier = mergeSorted(frontier, newFree, nodes)
		}
	}

	if len(result) != len(nodes) {
		return nil, fmt.Errorf("dag: cycle detected among nodes")
	}
	return result, nil
}

// resolveAfter returns the logical names matched by an @after reference.
// If ref ends with "/", it is a path-prefix: convert to logical name prefix.
// Otherwise it is an exact logical name match.
func resolveAfter(ref string, nodes []fileset.Node) []string {
	if strings.HasSuffix(ref, "/") {
		// Path prefix: tmux/shellrc/ → logical name prefix tmux.shellrc.
		prefix := strings.ReplaceAll(strings.TrimSuffix(ref, "/"), "/", ".") + "."
		var names []string
		for _, n := range nodes {
			if strings.HasPrefix(n.LogicalName, prefix) {
				names = append(names, n.LogicalName)
			}
		}
		return names
	}
	// Exact logical name.
	return []string{ref}
}

func sortByName(indices []int, nodes []fileset.Node) {
	sort.Slice(indices, func(i, j int) bool {
		return nodes[indices[i]].LogicalName < nodes[indices[j]].LogicalName
	})
}

// mergeSorted merges two sorted index slices into one sorted slice.
func mergeSorted(a, b []int, nodes []fileset.Node) []int {
	result := make([]int, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if nodes[a[i]].LogicalName <= nodes[b[j]].LogicalName {
			result = append(result, a[i])
			i++
		} else {
			result = append(result, b[j])
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}
