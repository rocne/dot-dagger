package pipeline

import (
	"fmt"
	"sort"
	"strings"
)

// Order topologically sorts nodes using Kahn's algorithm with alphabetical
// tie-breaking on logical name. Returns nodes in DAG order.
// Errors on cycles or duplicate logical names.
func Order(nodes []RawNode) ([]RawNode, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	// Duplicate logical name detection.
	seen := make(map[string]string)
	for _, n := range nodes {
		if prev, ok := seen[n.LogicalName]; ok {
			return nil, fmt.Errorf("order: conflict: logical name %q at %s and %s", n.LogicalName, prev, n.Path)
		}
		seen[n.LogicalName] = n.Path
	}

	// Build index: logical name → slice index.
	idx := make(map[string]int, len(nodes))
	for i, n := range nodes {
		idx[n.LogicalName] = i
	}

	// Build adjacency: successors[i] lists indices that must come after i.
	successors := make([][]int, len(nodes))
	inDegree := make([]int, len(nodes))

	for i, n := range nodes {
		for _, dep := range n.After {
			deps := ResolveAfterRef(dep, nodes)
			for _, d := range deps {
				j, ok := idx[d]
				if !ok {
					continue // unknown @after reference: ignore
				}
				if j == i {
					continue // self-reference
				}
				successors[j] = append(successors[j], i)
				inDegree[i]++
			}
		}
	}

	// Kahn's with alphabetical tie-break.
	var frontier []int
	for i := range nodes {
		if inDegree[i] == 0 {
			frontier = append(frontier, i)
		}
	}
	sortByLogicalName(frontier, nodes)

	result := make([]RawNode, 0, len(nodes))
	for len(frontier) > 0 {
		cur := frontier[0]
		frontier = frontier[1:]
		result = append(result, nodes[cur])

		var newly []int
		for _, s := range successors[cur] {
			inDegree[s]--
			if inDegree[s] == 0 {
				newly = append(newly, s)
			}
		}
		if len(newly) > 0 {
			sortByLogicalName(newly, nodes)
			frontier = mergeSortedByName(frontier, newly, nodes)
		}
	}

	if len(result) != len(nodes) {
		return nil, fmt.Errorf("order: cycle detected among nodes")
	}
	return result, nil
}

// ResolveAfterRef expands a single @after reference to a list of logical names.
// A reference ending in "/" is a prefix: it matches all nodes whose logical name
// starts with that prefix (with "." substituted for "/").
func ResolveAfterRef(ref string, nodes []RawNode) []string {
	if strings.HasSuffix(ref, "/") {
		prefix := strings.TrimSuffix(ref, "/")
		dotPrefix := strings.ReplaceAll(prefix, "/", ".")
		var matches []string
		for _, n := range nodes {
			// Match the dir node itself ("foo") and its descendants ("foo.bar"),
			// but not an unrelated sibling that merely shares the prefix string
			// ("foobar"). The boundary is a dot, not a bare string prefix.
			if n.LogicalName == dotPrefix || strings.HasPrefix(n.LogicalName, dotPrefix+".") {
				matches = append(matches, n.LogicalName)
			}
		}
		return matches
	}
	return []string{ref}
}

func sortByLogicalName(indices []int, nodes []RawNode) {
	sort.Slice(indices, func(i, j int) bool {
		return nodes[indices[i]].LogicalName < nodes[indices[j]].LogicalName
	})
}

// mergeSortedByName merges two sorted index slices, maintaining sort order.
func mergeSortedByName(a, b []int, nodes []RawNode) []int {
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
