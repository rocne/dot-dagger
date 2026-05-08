// Package node provides types and utilities for dotfiles nodes.
package node

import (
	"path/filepath"
	"strings"
)

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
		c = strings.TrimPrefix(c, "nosync-")
		c = strings.TrimPrefix(c, "dot-")
		if i == len(components)-1 {
			if ext := filepath.Ext(c); ext != "" {
				c = strings.TrimSuffix(c, ext)
			}
		}
		result[i] = c
	}
	return strings.Join(result, ".")
}
