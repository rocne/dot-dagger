package predicate

import (
	"errors"
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
		reg := NewFuncRegistry(Strict)
		reg.Register("installable", func(arg string) (bool, error) { return true, nil })
		ev := &Evaluator{Env: map[string]string{}, Funcs: reg}
		got, err := ev.Eval(expr)
		if err != nil || !got {
			t.Errorf("registered func: got=%v err=%v, want true nil", got, err)
		}
	})

	t.Run("strict mode: unknown function returns error", func(t *testing.T) {
		reg := NewFuncRegistry(Strict)
		ev := &Evaluator{Env: map[string]string{}, Funcs: reg}
		_, err := ev.Eval(expr)
		if err == nil {
			t.Error("expected error for unknown function in strict mode")
		}
	})

	t.Run("warn mode: unknown function returns false with warning", func(t *testing.T) {
		reg := NewFuncRegistry(Warn)
		var buf strings.Builder
		reg.SetWarnOutput(&buf)
		ev := &Evaluator{Env: map[string]string{}, Funcs: reg}
		got, err := ev.Eval(expr)
		if err != nil || got {
			t.Errorf("warn mode: got=%v err=%v, want false nil", got, err)
		}
		if !strings.Contains(buf.String(), "installable") {
			t.Errorf("warning %q does not mention function name", buf.String())
		}
	})
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
