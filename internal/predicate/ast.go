// Package predicate parses and evaluates @when predicate expressions.
//
// Grammar:
//
//	expr       = or_expr
//	or_expr    = and_expr (OR and_expr)*
//	and_expr   = atom (AND atom)*
//	atom       = "(" expr ")" | call | condition
//	call       = IDENT "(" IDENT ")"
//	condition  = KEY "=" VALUE ("," VALUE)*
//
// AND binds tighter than OR. Comma is same-key OR shorthand.
// AND and OR are case-sensitive uppercase keywords.
package predicate

// Expr is a node in the predicate AST.
type Expr interface {
	expr()
	// Keys returns the environment keys referenced by this expression.
	Keys() []string
}

// OrExpr evaluates to true if any of its operands are true.
type OrExpr struct {
	Operands []Expr
}

func (OrExpr) expr() {}
func (e OrExpr) Keys() []string {
	return collectKeys(e.Operands)
}

// AndExpr evaluates to true if all of its operands are true.
type AndExpr struct {
	Operands []Expr
}

func (AndExpr) expr() {}
func (e AndExpr) Keys() []string {
	return collectKeys(e.Operands)
}

// ConditionExpr evaluates to true if the env key equals any of the values.
// Multiple values are an OR shorthand: os=macos,linux means os=macos OR os=linux.
type ConditionExpr struct {
	Key    string
	Values []string
}

func (ConditionExpr) expr() {}
func (e ConditionExpr) Keys() []string {
	return []string{e.Key}
}

// CallExpr evaluates to true if the named predicate function returns true for arg.
type CallExpr struct {
	Name string
	Arg  string
}

func (CallExpr) expr() {}
func (CallExpr) Keys() []string {
	return nil
}

// TrueExpr always evaluates to true. Used as the identity element for And.
type TrueExpr struct{}

func (TrueExpr) expr() {}
func (TrueExpr) Keys() []string { return nil }

func collectKeys(exprs []Expr) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, e := range exprs {
		for _, k := range e.Keys() {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys
}
