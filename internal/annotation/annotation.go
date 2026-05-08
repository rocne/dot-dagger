// Package annotation scans files for @key and @key(args) annotations
// in comment lines at the top of a file.
package annotation

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Annotation is a single @key or @key(args) found in a comment line.
type Annotation struct {
	Key  string
	Args string // content inside parens; empty for zero-arg annotations
	Line int
}

// Scan reads r and returns all @key/(args) annotations found in the header block.
//
// Scanner rules:
//  1. If the first line is a shebang (#!), skip it.
//  2. Read comment lines (# or //).
//  3. The first non-comment, non-blank line stops the scan.
//  4. Non-@ comment lines are ignored without stopping the scan.
//  5. Blank lines do not stop the scan.
func Scan(r io.Reader) ([]Annotation, error) {
	var anns []Annotation
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		// Skip shebang on line 1.
		if lineNum == 1 && strings.HasPrefix(line, "#!") {
			continue
		}

		// Blank lines are allowed in the annotation block.
		if line == "" {
			continue
		}

		var content string
		if strings.HasPrefix(line, "//") {
			content = strings.TrimSpace(line[2:])
		} else if strings.HasPrefix(line, "#") {
			content = strings.TrimSpace(line[1:])
		} else {
			// Non-comment, non-blank line: stop scanning.
			break
		}

		if content == "" || !strings.HasPrefix(content, "@") {
			continue // non-@ comment: ignore, keep scanning
		}

		rest := content[1:] // strip leading @

		key, args := parseKeyArgs(rest)
		if key == "" {
			continue
		}
		anns = append(anns, Annotation{Key: key, Args: args, Line: lineNum})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("annotation: scan: %w", err)
	}
	return anns, nil
}

// parseKeyArgs splits "key(args)" into key and args.
// For "key" with no parens, args is "".
func parseKeyArgs(s string) (key, args string) {
	i := strings.IndexByte(s, '(')
	if i < 0 {
		return strings.TrimSpace(s), ""
	}
	key = strings.TrimSpace(s[:i])
	rest := s[i+1:]
	j := strings.LastIndexByte(rest, ')')
	if j < 0 {
		// Malformed — treat whole thing as key, no args.
		return strings.TrimSpace(s), ""
	}
	args = strings.TrimSpace(rest[:j])
	return key, args
}

// Get returns all annotations with the given key.
func Get(anns []Annotation, key string) []Annotation {
	var result []Annotation
	for _, a := range anns {
		if a.Key == key {
			result = append(result, a)
		}
	}
	return result
}

// First returns the first annotation with the given key and true,
// or the zero value and false if none is found.
func First(anns []Annotation, key string) (Annotation, bool) {
	for _, a := range anns {
		if a.Key == key {
			return a, true
		}
	}
	return Annotation{}, false
}

// CombineWhen returns a combined @when expression from all @when annotations,
// joining them with AND. Each expression is wrapped in parentheses.
// Returns an empty string if no @when annotations are present.
func CombineWhen(anns []Annotation) string {
	var parts []string
	for _, a := range anns {
		if a.Key == "when" && a.Args != "" {
			parts = append(parts, "("+a.Args+")")
		}
	}
	return strings.Join(parts, " AND ")
}
