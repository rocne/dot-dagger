package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rocne/dot-dagger/internal/pipeline"
)

// unmetRequireError builds the hard-error 'dotd apply' returns when one or
// more @require dependencies are neither installed nor installable. Lists
// every unmet node/package pair (not just the first) so a single run surfaces
// the whole remediation list.
func unmetRequireError(unmet []pipeline.UnmetRequire) error {
	lines := make([]string, len(unmet))
	for i, u := range unmet {
		lines[i] = fmt.Sprintf("unmet @require: %s requires %q (not installed; no configured manager can install it)",
			u.Node, u.Package)
	}
	return &hintError{
		err:  errors.New(strings.Join(lines, "\n")),
		hint: "install the package manually, add it (and a manager entry) to packages.yaml, or drop @require from the file",
	}
}

// requireReason formats the --inactive reason shown for a node excluded from
// 'dotd list's active set because of an unmet @require, e.g.
// "require: nonexistent-pkg missing" or, for multiple unmet packages on the
// same node, "require: pkg-a, pkg-b missing".
func requireReason(pkgs []string) string {
	return fmt.Sprintf("require: %s missing", strings.Join(pkgs, ", "))
}
