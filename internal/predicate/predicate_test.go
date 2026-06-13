package predicate

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// --- Parser tests ---

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Expr
		wantErr bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  TrueExpr{},
		},
		{
			name:  "simple condition",
			input: "os=macos",
			want:  ConditionExpr{Key: "os", Values: []string{"macos"}},
		},
		{
			name:  "multi-value condition",
			input: "os=macos,linux",
			want:  ConditionExpr{Key: "os", Values: []string{"macos", "linux"}},
		},
		{
			name:  "call expression",
			input: "exists(nvim)",
			want:  CallExpr{Name: "exists", Arg: "nvim"},
		},
		{
			name:  "AND of two conditions",
			input: "os=macos AND context=work",
			want: AndExpr{Operands: []Expr{
				ConditionExpr{Key: "os", Values: []string{"macos"}},
				ConditionExpr{Key: "context", Values: []string{"work"}},
			}},
		},
		{
			name:  "OR of two conditions",
			input: "os=macos OR os=linux",
			want: OrExpr{Operands: []Expr{
				ConditionExpr{Key: "os", Values: []string{"macos"}},
				ConditionExpr{Key: "os", Values: []string{"linux"}},
			}},
		},
		{
			name:  "AND binds tighter than OR",
			input: "os=macos AND context=work OR os=linux",
			want: OrExpr{Operands: []Expr{
				AndExpr{Operands: []Expr{
					ConditionExpr{Key: "os", Values: []string{"macos"}},
					ConditionExpr{Key: "context", Values: []string{"work"}},
				}},
				ConditionExpr{Key: "os", Values: []string{"linux"}},
			}},
		},
		{
			name:  "parentheses override precedence",
			input: "(os=macos OR os=linux) AND context=work",
			want: AndExpr{Operands: []Expr{
				OrExpr{Operands: []Expr{
					ConditionExpr{Key: "os", Values: []string{"macos"}},
					ConditionExpr{Key: "os", Values: []string{"linux"}},
				}},
				ConditionExpr{Key: "context", Values: []string{"work"}},
			}},
		},
		{
			name:    "missing value after equals",
			input:   "os=",
			wantErr: true,
		},
		{
			name:    "missing equals",
			input:   "os",
			wantErr: true,
		},
		{
			name:    "unclosed paren",
			input:   "(os=macos",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.input, err)
			}
			if !exprEqual(got, tt.want) {
				t.Errorf("Parse(%q)\n  got  = %#v\n  want = %#v", tt.input, got, tt.want)
			}
		})
	}
}

// --- Evaluator tests ---

func TestEval(t *testing.T) {
	env := map[string]string{
		"os":      "macos",
		"context": "work",
	}

	tests := []struct {
		name    string
		input   string
		env     map[string]string
		want    bool
		wantErr bool
	}{
		{
			name:  "true expr",
			input: "",
			want:  true,
		},
		{
			name:  "condition match",
			input: "os=macos",
			want:  true,
		},
		{
			name:  "condition no match",
			input: "os=linux",
			want:  false,
		},
		{
			name:  "multi-value OR match",
			input: "os=linux,macos",
			want:  true,
		},
		{
			name:  "multi-value OR no match",
			input: "os=linux,windows",
			want:  false,
		},
		{
			name:  "AND both true",
			input: "os=macos AND context=work",
			want:  true,
		},
		{
			name:  "AND one false",
			input: "os=macos AND context=personal",
			want:  false,
		},
		{
			name:  "OR first true",
			input: "os=macos OR os=linux",
			want:  true,
		},
		{
			name:  "OR both false",
			input: "os=windows OR os=linux",
			want:  false,
		},
		{
			name:    "missing env key",
			input:   "distro=ubuntu",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := env
			if tt.env != nil {
				e = tt.env
			}
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.input, err)
			}
			ev := &Evaluator{Env: e}
			got, err := ev.Eval(expr)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Eval(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Eval(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestEvalExists(t *testing.T) {
	expr, _ := Parse("exists(mytool)")
	ev := &Evaluator{
		Env:      map[string]string{},
		LookPath: func(s string) (string, error) { return "/usr/bin/" + s, nil },
	}
	got, err := ev.Eval(expr)
	if err != nil || !got {
		t.Errorf("exists() with found tool: got=%v err=%v, want true nil", got, err)
	}

	ev.LookPath = func(s string) (string, error) { return "", errors.New("not found") }
	got, err = ev.Eval(expr)
	if err != nil || got {
		t.Errorf("exists() with missing tool: got=%v err=%v, want false nil", got, err)
	}
}

func TestEvalCustomFunc(t *testing.T) {
	expr, _ := Parse("installable(nvim)")

	t.Run("registered function called", func(t *testing.T) {
		reg := NewFuncRegistry()
		reg.Register("installable", func(arg string) (bool, error) { return true, nil })
		ev := &Evaluator{Env: map[string]string{}, Funcs: reg}
		got, err := ev.Eval(expr)
		if err != nil || !got {
			t.Errorf("registered func: got=%v err=%v, want true nil", got, err)
		}
	})

	t.Run("unknown function returns error", func(t *testing.T) {
		reg := NewFuncRegistry()
		ev := &Evaluator{Env: map[string]string{}, Funcs: reg}
		_, err := ev.Eval(expr)
		if err == nil {
			t.Error("expected error for unknown function")
		}
	})
}

// TestNewEvaluatorBuiltins verifies that NewEvaluator pre-registers installed()
// and installable() so they resolve without "unknown function" errors.
func TestNewEvaluatorBuiltins(t *testing.T) {
	// A LookPath that finds "sh" but not "nonexistent-binary-xyz".
	fakeLookPath := func(name string) (string, error) {
		switch name {
		case "sh":
			return "/bin/sh", nil
		default:
			return "", errors.New("not found")
		}
	}

	t.Run("installed(sh) returns true when binary on PATH", func(t *testing.T) {
		expr, err := Parse("installed(sh)")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		ev := NewEvaluator(map[string]string{})
		ev.LookPath = fakeLookPath // reuse LookPath field for exists(); builtins use registry
		// Override builtins with the controlled lookPath.
		ev.RegisterPackageRegistry(nil, fakeLookPath)
		got, err := ev.Eval(expr)
		if err != nil {
			t.Fatalf("Eval error: %v", err)
		}
		if !got {
			t.Error("installed(sh): got false, want true")
		}
	})

	t.Run("installed(nonexistent-binary-xyz) returns false when binary not on PATH", func(t *testing.T) {
		expr, err := Parse("installed(nonexistent-binary-xyz)")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		ev := NewEvaluator(map[string]string{})
		ev.RegisterPackageRegistry(nil, fakeLookPath)
		got, err := ev.Eval(expr)
		if err != nil {
			t.Fatalf("Eval error: %v", err)
		}
		if got {
			t.Error("installed(nonexistent-binary-xyz): got true, want false")
		}
	})

	t.Run("installed() no longer returns unknown function error", func(t *testing.T) {
		expr, err := Parse("installed(sh)")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		ev := NewEvaluator(map[string]string{})
		ev.RegisterPackageRegistry(nil, fakeLookPath)
		_, err = ev.Eval(expr)
		if err != nil {
			t.Errorf("NewEvaluator should not return unknown function error; got: %v", err)
		}
	})

	t.Run("installable() is registered (no error)", func(t *testing.T) {
		expr, err := Parse("installable(sh)")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		ev := NewEvaluator(map[string]string{})
		ev.RegisterPackageRegistry(nil, fakeLookPath)
		// No entry in empty registry => installable returns false, no error.
		got, err := ev.Eval(expr)
		if err != nil {
			t.Errorf("installable() should not error with empty registry; got: %v", err)
		}
		if got {
			t.Error("installable(sh): got true, want false (no registry entry)")
		}
	})
}

// --- NewEvaluator + nil-registry tests (AUDIT-052) ---

// TestNewEvaluator_ConstructorSetsEnv verifies that NewEvaluator (the
// constructor, not a struct literal) populates the Env field correctly.
func TestNewEvaluator_ConstructorSetsEnv(t *testing.T) {
	env := map[string]string{"os": "linux", "context": "work"}
	ev := NewEvaluator(env)
	if ev.Env == nil {
		t.Fatal("NewEvaluator: Env is nil, want populated map")
	}
	if ev.Env["os"] != "linux" {
		t.Errorf("NewEvaluator: Env[os] = %q, want linux", ev.Env["os"])
	}
}

// TestNewEvaluator_HasFuncRegistry verifies that NewEvaluator initialises a
// non-nil Funcs registry so callers don't have to guard against nil Funcs.
func TestNewEvaluator_HasFuncRegistry(t *testing.T) {
	ev := NewEvaluator(map[string]string{})
	if ev.Funcs == nil {
		t.Fatal("NewEvaluator: Funcs is nil, want non-nil registry")
	}
}

// TestNewEvaluator_BuiltinsRegistered verifies that installed() and
// installable() are pre-registered by NewEvaluator so that calling them does
// not return an "unknown function" error (AUDIT-052, cross-ref AUDIT-026).
func TestNewEvaluator_BuiltinsRegistered(t *testing.T) {
	fakeLookPath := func(name string) (string, error) {
		if name == "sh" {
			return "/bin/sh", nil
		}
		return "", errors.New("not found")
	}

	t.Run("installed() is registered", func(t *testing.T) {
		expr, _ := Parse("installed(sh)")
		ev := NewEvaluator(map[string]string{})
		ev.RegisterPackageRegistry(nil, fakeLookPath)
		_, err := ev.Eval(expr)
		if err != nil {
			t.Errorf("installed() call via NewEvaluator returned error: %v", err)
		}
	})

	t.Run("installable() is registered", func(t *testing.T) {
		expr, _ := Parse("installable(sh)")
		ev := NewEvaluator(map[string]string{})
		ev.RegisterPackageRegistry(nil, fakeLookPath)
		_, err := ev.Eval(expr)
		if err != nil {
			t.Errorf("installable() call via NewEvaluator returned error: %v", err)
		}
	})
}

// TestEvalCall_NilRegistry verifies that when Evaluator.Funcs is nil, calling
// any non-builtin function (i.e. not "exists") returns a graceful error rather
// than panicking. The "exists" built-in is handled before the registry check,
// so it is unaffected.
func TestEvalCall_NilRegistry(t *testing.T) {
	// Funcs deliberately left nil — struct literal, not NewEvaluator.
	ev := &Evaluator{
		Env:  map[string]string{},
		// Funcs: nil  (zero value)
	}

	t.Run("unknown function with nil Funcs returns error, no panic", func(t *testing.T) {
		expr, err := Parse("installable(nvim)")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		_, err = ev.Eval(expr)
		if err == nil {
			t.Error("expected error for unknown function with nil registry, got nil")
		}
		if !strings.Contains(err.Error(), "unknown function") {
			t.Errorf("error message %q does not mention 'unknown function'", err.Error())
		}
	})

	t.Run("exists() still works with nil Funcs (built-in path)", func(t *testing.T) {
		expr, err := Parse("exists(sh)")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		ev2 := &Evaluator{
			Env:      map[string]string{},
			LookPath: func(name string) (string, error) { return "/bin/" + name, nil },
			// Funcs: nil
		}
		got, err := ev2.Eval(expr)
		if err != nil {
			t.Fatalf("exists() with nil Funcs returned error: %v", err)
		}
		if !got {
			t.Error("exists() with nil Funcs and found tool: got false, want true")
		}
	})
}

// --- Keys() tests (AUDIT-045) ---

// TestKeys_NodeTypes verifies that every AST node type returns the expected set
// of environment keys from its Keys() method.
func TestKeys_NodeTypes(t *testing.T) {
	t.Run("TrueExpr returns nil", func(t *testing.T) {
		if k := (TrueExpr{}).Keys(); k != nil {
			t.Errorf("TrueExpr.Keys() = %v, want nil", k)
		}
	})

	t.Run("ConditionExpr returns its key", func(t *testing.T) {
		got := ConditionExpr{Key: "os", Values: []string{"linux"}}.Keys()
		if len(got) != 1 || got[0] != "os" {
			t.Errorf("ConditionExpr.Keys() = %v, want [os]", got)
		}
	})

	t.Run("CallExpr returns nil (no env key)", func(t *testing.T) {
		got := CallExpr{Name: "exists", Arg: "nvim"}.Keys()
		if got != nil {
			t.Errorf("CallExpr.Keys() = %v, want nil", got)
		}
	})

	t.Run("AndExpr deduplicates keys across operands", func(t *testing.T) {
		e := AndExpr{Operands: []Expr{
			ConditionExpr{Key: "os", Values: []string{"linux"}},
			ConditionExpr{Key: "os", Values: []string{"macos"}}, // same key — dedup
			ConditionExpr{Key: "context", Values: []string{"work"}},
		}}
		got := e.Keys()
		if len(got) != 2 {
			t.Errorf("AndExpr.Keys() = %v, want [os context]", got)
		}
		keyset := map[string]bool{}
		for _, k := range got {
			keyset[k] = true
		}
		if !keyset["os"] || !keyset["context"] {
			t.Errorf("AndExpr.Keys() = %v missing expected keys", got)
		}
	})

	t.Run("OrExpr deduplicates keys across operands", func(t *testing.T) {
		e := OrExpr{Operands: []Expr{
			ConditionExpr{Key: "os", Values: []string{"linux"}},
			ConditionExpr{Key: "distro", Values: []string{"fedora"}},
			ConditionExpr{Key: "os", Values: []string{"macos"}}, // duplicate key — dedup
		}}
		got := e.Keys()
		if len(got) != 2 {
			t.Errorf("OrExpr.Keys() = %v, want 2 unique keys", got)
		}
		keyset := map[string]bool{}
		for _, k := range got {
			keyset[k] = true
		}
		if !keyset["os"] || !keyset["distro"] {
			t.Errorf("OrExpr.Keys() = %v missing expected keys", got)
		}
	})
}

// TestCollectKeys_Dedup verifies the unexported collectKeys dedup logic by
// exercising it through OrExpr/AndExpr.Keys() with repeated keys.
func TestCollectKeys_Dedup(t *testing.T) {
	// Three operands sharing two keys — collectKeys must return each once.
	e := AndExpr{Operands: []Expr{
		ConditionExpr{Key: "os", Values: []string{"linux"}},
		ConditionExpr{Key: "context", Values: []string{"work"}},
		ConditionExpr{Key: "os", Values: []string{"macos"}},
	}}
	got := e.Keys()
	seen := map[string]int{}
	for _, k := range got {
		seen[k]++
	}
	for key, count := range seen {
		if count > 1 {
			t.Errorf("key %q appears %d times in Keys() output — expected dedup", key, count)
		}
	}
}

// --- Helpers ---

// exprEqual does a structural comparison of two Expr values.
func exprEqual(a, b Expr) bool {
	switch x := a.(type) {
	case TrueExpr:
		_, ok := b.(TrueExpr)
		return ok
	case ConditionExpr:
		y, ok := b.(ConditionExpr)
		if !ok || x.Key != y.Key || len(x.Values) != len(y.Values) {
			return false
		}
		for i := range x.Values {
			if x.Values[i] != y.Values[i] {
				return false
			}
		}
		return true
	case CallExpr:
		y, ok := b.(CallExpr)
		return ok && x.Name == y.Name && x.Arg == y.Arg
	case AndExpr:
		y, ok := b.(AndExpr)
		if !ok || len(x.Operands) != len(y.Operands) {
			return false
		}
		for i := range x.Operands {
			if !exprEqual(x.Operands[i], y.Operands[i]) {
				return false
			}
		}
		return true
	case OrExpr:
		y, ok := b.(OrExpr)
		if !ok || len(x.Operands) != len(y.Operands) {
			return false
		}
		for i := range x.Operands {
			if !exprEqual(x.Operands[i], y.Operands[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// TestFuncRegistry_RegisterPanicsOnDuplicate verifies that Register panics if
// the same name is registered twice. Override is the documented escape hatch.
func TestFuncRegistry_RegisterPanicsOnDuplicate(t *testing.T) {
	reg := NewFuncRegistry()
	reg.Register("foo", func(arg string) (bool, error) { return true, nil })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate Register")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T %v, want string", r, r)
		}
		if !strings.Contains(msg, "foo") || !strings.Contains(msg, "already registered") {
			t.Errorf("panic message %q missing expected substrings", msg)
		}
	}()
	reg.Register("foo", func(arg string) (bool, error) { return false, nil })
}

// TestMissingKeyError verifies error message format and errors.As extraction.
func TestMissingKeyError(t *testing.T) {
	err := &MissingKeyError{Key: "context"}
	wantMsg := `predicate: env key "context" not set`
	if got := err.Error(); got != wantMsg {
		t.Errorf("Error() = %q, want %q", got, wantMsg)
	}

	// errors.As round-trip — wrapping should preserve the type.
	wrapped := fmt.Errorf("outer: %w", err)
	var mke *MissingKeyError
	if !errors.As(wrapped, &mke) {
		t.Fatal("errors.As failed to unwrap MissingKeyError")
	}
	if mke.Key != "context" {
		t.Errorf("unwrapped Key = %q, want %q", mke.Key, "context")
	}
}
