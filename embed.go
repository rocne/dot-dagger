// Package dotdagger hosts repo-root assets embedded into the binary.
//
// It exists only because //go:embed can reach files at or below the directive
// file's own directory: docs/ lives at the module root, and cmd/dotd (a
// subdirectory) cannot embed ../docs. This root package is the standard Go
// answer for embedding repo-root assets. Inclusion is by explicit pattern —
// docs/superpowers (internal specs/plans) is deliberately never listed, so it
// can never ship in the binary. Adding a new top-level doc section requires
// adding it to the patterns below.
package dotdagger

import "embed"

//go:embed docs/index.md docs/concepts docs/getting-started docs/reference
var DocsFS embed.FS
