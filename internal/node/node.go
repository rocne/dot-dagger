// Package node provides types and utilities for dotfiles nodes.
package node

import (
	"path/filepath"
	"strings"
)

// Action type constants — the canonical vocabulary for node actions, shared by
// the pipeline (execution) and the annotation wizard (user-facing options).
// This leaf package is the single owner; annotation cannot import pipeline
// (import cycle), so both sides reference these values instead of re-declaring
// them.
const (
	ActionCompose  = "compose"
	ActionSource   = "source"
	ActionNoSource = "no-source"
	ActionLink     = "link"
)

// Repo naming-convention prefixes for path components. PrefixNoSync marks
// entries sync tools should ignore; PrefixDot maps a repo-visible name to a
// hidden dotfile ("dot-bashrc" ↔ ".bashrc"). Single owner — all prefix
// stripping and construction routes through these.
const (
	PrefixNoSync = "nosync-"
	PrefixDot    = "dot-"
)

// StripRepoPrefix strips the PrefixNoSync then PrefixDot leading prefixes from
// a single path component — the naming convention for repo entries that should
// not sync or that map to dotfiles (hidden files).
func StripRepoPrefix(s string) string {
	s = strings.TrimPrefix(s, PrefixNoSync)
	s = strings.TrimPrefix(s, PrefixDot)
	return s
}

// LinkName converts one repo path component to its on-disk name for link
// destinations: strips PrefixNoSync, then rewrites a PrefixDot prefix to a
// leading dot ("dot-tmux.conf" → ".tmux.conf").
func LinkName(s string) string {
	s = strings.TrimPrefix(s, PrefixNoSync)
	if rest, ok := strings.CutPrefix(s, PrefixDot); ok {
		return "." + rest
	}
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
