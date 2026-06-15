package predicate

import (
	"errors"
	"fmt"
	"os/exec"
)

// MissingKeyError is returned when a predicate references an env key that is not set.
type MissingKeyError struct {
	Key string
}

func (e *MissingKeyError) Error() string {
	return fmt.Sprintf("predicate: env key %q not set", e.Key)
}

// isMissingKey reports whether err is (or wraps) a *MissingKeyError.
func isMissingKey(err error) bool {
	var mke *MissingKeyError
	return errors.As(err, &mke)
}

// Func is a predicate function registered with a FuncRegistry.
// It receives the argument string from the call expression and returns
// whether the predicate holds.
type Func func(arg string) (bool, error)

// FuncRegistry holds registered predicate functions. Unknown calls are
// errors. The zero value is not usable; use NewFuncRegistry.
type FuncRegistry struct {
	funcs map[string]Func
}

// NewFuncRegistry returns an empty FuncRegistry.
func NewFuncRegistry() *FuncRegistry {
	return &FuncRegistry{funcs: make(map[string]Func)}
}

// Register registers f under the given name.
// Panics if name is already registered.
func (r *FuncRegistry) Register(name string, f Func) {
	if _, exists := r.funcs[name]; exists {
		panic(fmt.Sprintf("predicate: function %q already registered", name))
	}
	r.funcs[name] = f
}

// Override registers or replaces f under the given name without panicking.
// Use this when upgrading a default registration (e.g. wiring a real package
// registry on top of the PATH-only default set by NewEvaluator).
func (r *FuncRegistry) Override(name string, f Func) {
	r.funcs[name] = f
}

// Call invokes the function registered under name with arg.
// An unregistered name is an error.
func (r *FuncRegistry) Call(name, arg string) (bool, error) {
	f, ok := r.funcs[name]
	if !ok {
		return false, fmt.Errorf("predicate: unknown function %q", name)
	}
	return f(arg)
}

// NewEvaluator returns an Evaluator with env set and a FuncRegistry
// pre-populated with the default built-in functions. Use this constructor
// rather than an inline struct literal so that both filter and manifest
// evaluation share identical capabilities.
//
// The built-in functions registered are:
//   - installed(name) — true if `name` is present on PATH (uses exec.LookPath)
//   - installable(name) — true if a known package manager for `name` is on PATH
//
// Both functions operate without a packages.Registry; pass a registry via
// RegisterPackageRegistry if package-level binary aliases are needed.
func NewEvaluator(env map[string]string) *Evaluator {
	funcs := NewFuncRegistry()
	registerBuiltins(funcs, nil, nil)
	return &Evaluator{
		Env:   env,
		Funcs: funcs,
	}
}

// Evaluator evaluates a parsed predicate Expr against an environment.
type Evaluator struct {
	// Env is the resolved environment map used for condition evaluation.
	Env map[string]string

	// LookPath checks whether a command is available on PATH.
	// Defaults to exec.LookPath if nil.
	LookPath func(file string) (string, error)

	// Funcs is the registry of callable predicate functions.
	// If nil, any CallExpr other than the built-in exists() returns an error.
	Funcs *FuncRegistry
}

// Eval evaluates expr and returns whether it holds.
func (e *Evaluator) Eval(expr Expr) (bool, error) {
	switch x := expr.(type) {
	case TrueExpr:
		return true, nil

	case ConditionExpr:
		actual, ok := e.Env[x.Key]
		if !ok {
			return false, &MissingKeyError{Key: x.Key}
		}
		for _, v := range x.Values {
			if actual == v {
				return true, nil
			}
		}
		return false, nil

	case AndExpr:
		// A definitely-false operand decides the AND, so a MissingKeyError from
		// another operand is deferred: it only surfaces if no operand is false.
		// This keeps AND commutative under a missing key (see MissingKeyError).
		var missErr error
		for _, operand := range x.Operands {
			ok, err := e.Eval(operand)
			if err != nil {
				if isMissingKey(err) {
					if missErr == nil {
						missErr = err
					}
					continue
				}
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		if missErr != nil {
			return false, missErr
		}
		return true, nil

	case OrExpr:
		// A true operand decides the OR, so a MissingKeyError from another
		// operand is deferred: it only surfaces if no operand is true. This
		// keeps OR commutative under a missing key (see MissingKeyError).
		var missErr error
		for _, operand := range x.Operands {
			ok, err := e.Eval(operand)
			if err != nil {
				if isMissingKey(err) {
					if missErr == nil {
						missErr = err
					}
					continue
				}
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		if missErr != nil {
			return false, missErr
		}
		return false, nil

	case CallExpr:
		return e.evalCall(x)

	default:
		return false, fmt.Errorf("predicate: unknown expr type %T", expr)
	}
}

func (e *Evaluator) evalCall(x CallExpr) (bool, error) {
	// built-in: exists(cmd)
	if x.Name == "exists" {
		lookPath := e.LookPath
		if lookPath == nil {
			lookPath = exec.LookPath
		}
		_, err := lookPath(x.Arg)
		return err == nil, nil
	}

	// delegate to registry
	if e.Funcs != nil {
		return e.Funcs.Call(x.Name, x.Arg)
	}
	return false, fmt.Errorf("predicate: unknown function %q (no registry)", x.Name)
}
