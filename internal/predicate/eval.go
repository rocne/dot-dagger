package predicate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Mode controls how unknown predicate function calls are handled.
type Mode int

const (
	// Strict causes an unknown function call to return an error. This is the default.
	Strict Mode = iota
	// Warn causes an unknown function call to log a warning and return false.
	Warn
)

// Func is a predicate function registered with a FuncRegistry.
// It receives the argument string from the call expression and returns
// whether the predicate holds.
type Func func(arg string) (bool, error)

// FuncRegistry holds registered predicate functions and controls behaviour
// for unknown calls. The zero value is not usable; use NewFuncRegistry.
type FuncRegistry struct {
	mode    Mode
	funcs   map[string]Func
	warnOut io.Writer
}

// NewFuncRegistry returns a FuncRegistry with the given mode.
func NewFuncRegistry(mode Mode) *FuncRegistry {
	return &FuncRegistry{
		mode:    mode,
		funcs:   make(map[string]Func),
		warnOut: os.Stderr,
	}
}

// SetWarnOutput sets the writer used for Warn mode messages.
func (r *FuncRegistry) SetWarnOutput(w io.Writer) {
	r.warnOut = w
}

// Register registers f under the given name.
// Panics if name is already registered.
func (r *FuncRegistry) Register(name string, f Func) {
	if _, exists := r.funcs[name]; exists {
		panic(fmt.Sprintf("predicate: function %q already registered", name))
	}
	r.funcs[name] = f
}

// Call invokes the function registered under name with arg.
// If name is not registered, behaviour depends on Mode:
//   - Strict: returns an error
//   - Warn: writes a warning and returns false
func (r *FuncRegistry) Call(name, arg string) (bool, error) {
	f, ok := r.funcs[name]
	if !ok {
		switch r.mode {
		case Strict:
			return false, fmt.Errorf("predicate: unknown function %q", name)
		case Warn:
			fmt.Fprintf(r.warnOut, "warning: unknown predicate function %q\n", name)
			return false, nil
		}
	}
	return f(arg)
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
			return false, fmt.Errorf("predicate: env key %q not set", x.Key)
		}
		for _, v := range x.Values {
			if actual == v {
				return true, nil
			}
		}
		return false, nil

	case AndExpr:
		for _, operand := range x.Operands {
			ok, err := e.Eval(operand)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil

	case OrExpr:
		for _, operand := range x.Operands {
			ok, err := e.Eval(operand)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
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
