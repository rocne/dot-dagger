package predicate

import (
	"os/exec"

	"github.com/rocne/dot-dagger/internal/packages"
)

// emptyRegistry is an empty package registry used as a fallback when no
// packages.yaml is loaded. It has non-nil maps so registry lookups are safe.
// With an empty registry, installed(name) falls back to checking whether
// `name` is directly on PATH; installable(name) returns false (no entry).
var emptyRegistry = &packages.Registry{
	PackageManagers: packages.ManagersSection{
		Defs: map[string]packages.PackageManagerDef{},
	},
	Packages: map[string]packages.PackageEntry{},
}

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
		reg = emptyRegistry
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
