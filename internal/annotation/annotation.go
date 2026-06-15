// Package annotation scans files for @key and @key(args) annotations
// in comment lines at the top of a file.
package annotation

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Key constants for the supported .dagger annotation vocabulary.
// These are the canonical names for annotation keys used in file headers and
// .dagger YAML files. Code that matches or compares annotation keys must use
// these constants; the yaml struct tags in internal/dagger must match them.
const (
	KeyAction  = "action"
	KeyAfter   = "after"
	KeyRequire = "require"
	KeyRequest = "request"
	KeyDisable = "disable"
	KeyName    = "name"
	KeyWhen    = "when"
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

		content, isComment := commentContent(line)
		if !isComment {
			// Non-comment, non-blank line: stop scanning.
			break
		}

		key, args, ok := annotationFromContent(content)
		if !ok {
			continue // non-@ comment or empty key: ignore, keep scanning
		}
		anns = append(anns, Annotation{Key: key, Args: args, Line: lineNum})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("annotation: scan: %w", err)
	}
	return anns, nil
}

// SplitParen parses "head(body)" syntax.
// Returns (head, body, true) for "head(body)" — both trimmed.
// Returns (s, "", false) for "head" (no parens) — s is trimmed.
// Returns (s, "", false) for malformed input (e.g. missing closing paren) — s is the trimmed original.
func SplitParen(s string) (head, body string, ok bool) {
	s = strings.TrimSpace(s)
	i := strings.IndexByte(s, '(')
	if i < 0 {
		return s, "", false
	}
	head = strings.TrimSpace(s[:i])
	rest := s[i+1:]
	j := strings.LastIndexByte(rest, ')')
	if j < 0 {
		return s, "", false
	}
	return head, strings.TrimSpace(rest[:j]), true
}

// parseKeyArgs splits "key(args)" into key and args.
// For "key" with no parens, args is "".
//
// An unterminated paren (a typo: "key(args" with no ")") is recovered to the
// intended (key, args) rather than collapsing into a junk key that still
// contains "(". A junk key never matches the annotation vocabulary, so the
// annotation would be silently dropped — for @when, that flips a conditional
// file to unconditionally active. Recovering the key fails loud (the key is
// recognized) instead.
func parseKeyArgs(s string) (key, args string) {
	head, body, ok := SplitParen(s)
	if ok {
		return head, body
	}
	// SplitParen returns the whole trimmed string as head on failure. If it
	// holds an opening paren, the closing one is missing: split at the first
	// "(" so the key is the token before it and args is the remainder.
	if i := strings.IndexByte(head, '('); i >= 0 {
		return strings.TrimSpace(head[:i]), strings.TrimSpace(head[i+1:])
	}
	return head, body
}

// commentContent returns the content of a "#" or "//" comment line — the text
// after the comment marker, trimmed — and isComment=true. For a non-comment,
// non-blank line it returns isComment=false, which signals Scan to stop.
func commentContent(line string) (content string, isComment bool) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "//") {
		return strings.TrimSpace(line[2:]), true
	}
	if strings.HasPrefix(line, "#") {
		return strings.TrimSpace(line[1:]), true
	}
	return "", false
}

// annotationFromContent parses a comment's content (the text after the comment
// marker) into (key, args). ok is false when the content is not a recognized
// @-annotation: empty, not starting with "@", or yielding an empty key.
func annotationFromContent(content string) (key, args string, ok bool) {
	if content == "" || !strings.HasPrefix(content, "@") {
		return "", "", false
	}
	key, args = parseKeyArgs(content[1:]) // strip leading @
	if key == "" {
		return "", "", false
	}
	return key, args, true
}

// IsAnnotationLine reports whether a header line is an @-annotation that Scan
// would record. Write uses this to strip exactly the lines Scan recognizes, so
// the scanner and the rewriter can never disagree about what an annotation is.
func IsAnnotationLine(line string) bool {
	content, isComment := commentContent(line)
	if !isComment {
		return false
	}
	_, _, ok := annotationFromContent(content)
	return ok
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
		if a.Key == KeyWhen && a.Args != "" {
			parts = append(parts, "("+a.Args+")")
		}
	}
	return strings.Join(parts, " AND ")
}
