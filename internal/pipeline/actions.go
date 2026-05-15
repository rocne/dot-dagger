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
		case "action":
			result = append(result, a)
		case "source":
			result = append(result, annotation.Annotation{Key: "action", Args: "source", Line: a.Line})
		case "no-source":
			result = append(result, annotation.Annotation{Key: "action", Args: "no-source", Line: a.Line})
		case "link":
			result = append(result, annotation.Annotation{Key: "action", Args: "link(" + a.Args + ")", Line: a.Line})
		case "symlink":
			result = append(result, annotation.Annotation{Key: "action", Args: "link(" + a.Args + ")", Line: a.Line})
		default:
			result = append(result, a)
		}
	}
	return result
}

// ValidateNodes checks every node for action sequencing errors. All errors are
// collected and returned together. Returns nil if all nodes are valid.
func ValidateNodes(nodes []RawNode) error {
	var errs []string
	for _, n := range nodes {
		if err := validateNode(n); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("action validation:\n  %s", strings.Join(errs, "\n  "))
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
		case "compose":
			if !isDir {
				return fmt.Errorf("node %s: compose is only valid on directories", n.LogicalName)
			}
			seenCompose = true
		case "link":
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
		case "source":
			if isDir && !seenCompose {
				return fmt.Errorf("node %s: link/source must follow compose", n.LogicalName)
			}
		}
	}
	return nil
}
