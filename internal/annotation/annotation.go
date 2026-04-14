// Package annotation scans files for @key value annotations in comment lines
// and provides a handler registry for dispatching tool-owned annotation keys.
package annotation

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Core annotation keys handled directly by suite internals.
// These are read from the annotations slice by callers and are not dispatched
// through a Registry.
const (
	KeyWhen         = "when"
	KeyName         = "name"
	KeyAfter        = "after"
	KeySymlink      = "symlink"
	KeyRetainPrefix = "retain-prefix"
)

// IsCoreKey reports whether key is a built-in annotation key handled directly
// by the suite rather than through a handler Registry.
func IsCoreKey(key string) bool {
	switch key {
	case KeyWhen, KeyName, KeyAfter, KeySymlink, KeyRetainPrefix:
		return true
	}
	return false
}

// Annotation is a single @key value pair found in a comment line.
type Annotation struct {
	Key   string
	Value string
	Line  int
}

// Scan reads r and returns all @key value annotations found in comment lines.
// Lines beginning with # or // (after optional leading whitespace) are treated
// as comments. Each annotation has the form @key or @key value within a comment.
func Scan(r io.Reader) ([]Annotation, error) {
	var anns []Annotation
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		var content string
		switch {
		case strings.HasPrefix(line, "//"):
			content = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "#"):
			content = strings.TrimSpace(line[1:])
		default:
			continue
		}

		if !strings.HasPrefix(content, "@") {
			continue
		}

		rest := content[1:] // strip leading @
		parts := strings.SplitN(rest, " ", 2)
		key := parts[0]
		if key == "" {
			continue
		}
		value := ""
		if len(parts) == 2 {
			value = strings.TrimSpace(parts[1])
		}

		anns = append(anns, Annotation{Key: key, Value: value, Line: lineNum})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("annotation: scan: %w", err)
	}
	return anns, nil
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
// joining them with AND. Each individual expression is wrapped in parentheses.
// Returns an empty string if no @when annotations are present.
func CombineWhen(anns []Annotation) string {
	var parts []string
	for _, a := range anns {
		if a.Key == KeyWhen && a.Value != "" {
			parts = append(parts, "("+a.Value+")")
		}
	}
	return strings.Join(parts, " AND ")
}
