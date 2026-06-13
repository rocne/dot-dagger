package predicate

import (
	"os/exec"

	"github.com/rocne/dot-dagger/internal/packages"
)

// RegisterPackageRegistry re-registers installed() and installable() with the
// given package registry, overriding the PATH-only defaults set by
// NewEvaluator. Call this when a packages.yaml registry has been loaded and
// should be used for binary-alias resolution.
//
// lookPath may be nil, in which case exec.LookPath is used.
func (e *Evaluator) RegisterPackageRegistry(reg *packages.Registry, lookPath func(string) (string, error)) {
	registerBuiltins(e.Funcs, reg, lookPath)
}

// registerBuiltins populates r with the built-in predicate functions
// installed() and installable(). The optional reg argument is a package
// registry; pass nil to use PATH-only fallback semantics (installed() checks
// PATH directly; installable() always returns false with no registry entry).
//
// LookPath defaults to exec.LookPath when nil.
func registerBuiltins(r *FuncRegistry, reg *packages.Registry, lookPath func(string) (string, error)) {
	if reg == nil {
		// With an empty registry, installed(name) falls back to PATH lookup
		// and installable(name) is always false (no entry).
		reg = packages.EmptyRegistry()
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	r.Override("installed", func(arg string) (bool, error) {
		return packages.Installed(arg, reg, lookPath)
	})
	r.Override("installable", func(arg string) (bool, error) {
		return packages.Installable(arg, reg, lookPath)
	})
}
