package pipeline

import (
	"fmt"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
)

// normalizeActionAnnotations converts legacy action annotation keys to canonical
// {Key:"action", Args:"<type>"} or {Key:"action", Args:"<type>(dest)"} form.
// Non-action annotations pass through unchanged.
func normalizeActionAnnotations(anns []annotation.Annotation) []annotation.Annotation {
	result := make([]annotation.Annotation, 0, len(anns))
	for _, a := range anns {
		switch a.Key {
		case annotation.KeyAction:
			result = append(result, a)
		case ActionSource:
			result = append(result, annotation.Annotation{Key: annotation.KeyAction, Args: ActionSource, Line: a.Line})
		case ActionNoSource:
			result = append(result, annotation.Annotation{Key: annotation.KeyAction, Args: ActionNoSource, Line: a.Line})
		case ActionLink, "symlink":
			result = append(result, annotation.Annotation{Key: annotation.KeyAction, Args: ActionLink + "(" + a.Args + ")", Line: a.Line})
		default:
			result = append(result, a)
		}
	}
	return result
}

// ValidateNodes checks every node for action sequencing errors and cross-node
// link conflicts. All per-node errors are collected and returned together.
// If opts.HomeDir is non-empty, link destinations are resolved (~ expanded)
// before conflict comparison — matching Act's behaviour. Returns nil if all
// nodes are valid.
func ValidateNodes(nodes []RawNode, opts ActOptions) error {
	var errs []string
	for _, n := range nodes {
		if err := validateNode(n); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if err := CheckLinkConflicts(nodes, opts); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("action validation:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// CheckLinkConflicts walks nodes, resolves their link destinations, and returns
// the first cross-node conflict found, or nil. If opts.HomeDir is empty, ~
// expansion is skipped and raw dest strings are compared.
// Compose fragments are skipped; their destinations belong to the parent node.
func CheckLinkConflicts(nodes []RawNode, opts ActOptions) error {
	destSeen := map[string]string{} // resolved dest → logical name
	for _, n := range nodes {
		if n.IsCompose {
			continue
		}
		for _, a := range n.Actions {
			if a.Type != ActionLink {
				continue
			}
			dest := a.Dest
			if opts.HomeDir != "" {
				dest = resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
			} else if dest == "" {
				// Can't resolve without HomeDir; skip empty dests.
				continue
			}
			if prev, ok := destSeen[dest]; ok {
				return fmt.Errorf("link conflict: %s and %s both link to %s", prev, n.LogicalName, dest)
			}
			destSeen[dest] = n.LogicalName
		}
	}
	return nil
}

func validateNode(n RawNode) error {
	// Compose fragments are consumed by the compose mechanism; act.go never
	// processes their actions as standalone nodes.
	if n.IsCompose {
		return nil
	}

	// Directory nodes have ComposeTarget == Path (set by Walk for composition.enabled dirs).
	isDir := n.ComposeTarget != "" && n.ComposeTarget == n.Path

	seenCompose := false
	var linkDests []string

	for _, a := range n.Actions {
		switch a.Type {
		case ActionCompose:
			if !isDir {
				return fmt.Errorf("node %s: compose is only valid on directories", n.LogicalName)
			}
			seenCompose = true
		case ActionLink:
			// Empty dest is valid when link_root is set — destination is derived at act time.
			if a.Dest == "" && n.LinkRoot == "" {
				return fmt.Errorf("node %s: link requires a destination", n.LogicalName)
			}
			if isDir && !seenCompose {
				return fmt.Errorf("node %s: link/source must follow compose", n.LogicalName)
			}
			for _, prev := range linkDests {
				if prev != a.Dest {
					return fmt.Errorf("node %s: conflicting link destinations", n.LogicalName)
				}
			}
			linkDests = append(linkDests, a.Dest)
		case ActionSource:
			if isDir && !seenCompose {
				return fmt.Errorf("node %s: link/source must follow compose", n.LogicalName)
			}
		}
	}
	return nil
}
