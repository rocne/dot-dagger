// Package node provides types and utilities for dotfiles nodes.
package node

import (
	"path/filepath"
	"strings"
)

// StripRepoPrefix strips the "nosync-" then "dot-" leading prefixes from a single
// path component — the naming convention for repo entries that should not sync or
// that map to dotfiles (hidden files).
func StripRepoPrefix(s string) string {
	s = strings.TrimPrefix(s, "nosync-")
	s = strings.TrimPrefix(s, "dot-")
	return s
}

// DeriveName computes the dot-separated logical name from a path
// relative to the dotfiles repo root.
//
// Per path component:
//  1. Strip leading "nosync-"
//  2. Strip leading "dot-" (after nosync-)
//  3. Strip file extension from the final component only
func DeriveName(relPath string) string {
	components := strings.Split(filepath.ToSlash(relPath), "/")
	result := make([]string, len(components))
	for i, c := range components {
		// Preserve the component when stripping the prefix would empty it (e.g.
		// a file named exactly "dot-" or "nosync-"). An empty logical name
		// collides with every other empty name and aborts apply with a spurious
		// duplicate-name error — the same failure mode as the ".gitignore" → ""
		// extension case handled below.
		if stripped := StripRepoPrefix(c); stripped != "" {
			c = stripped
		}
		if i == len(components)-1 {
			// Strip the extension only when a non-empty base remains. For a
			// leading-dot name like ".gitignore", filepath.Ext returns the whole
			// string, so trimming would yield "" — two such files would then
			// collide as duplicate empty logical names. Keep the name intact.
			if ext := filepath.Ext(c); ext != "" {
				if base := strings.TrimSuffix(c, ext); base != "" {
					c = base
				}
			}
		}
		result[i] = c
	}
	return strings.Join(result, ".")
}
