package pipeline

import (
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
