package predicate

import "fmt"

// Parse parses a predicate expression string and returns the root AST node.
// Returns an error if the input is syntactically invalid.
// An empty string parses to TrueExpr.
func Parse(input string) (Expr, error) {
	if input == "" {
		return TrueExpr{}, nil
	}
	p := &parser{lex: newLexer(input), input: input}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if tok := p.lex.peek(); tok.typ != tEOF {
		return nil, fmt.Errorf("predicate: unexpected token %q in %q", tok.val, input)
	}
	return expr, nil
}

// maxParseDepth bounds parenthesis nesting. The parser is recursive-descent, so
// each "(" recurses; without a bound, pathological input (thousands of nested
// parens) would grow the goroutine stack until it overflows — an unrecoverable
// fatal crash. No legitimate predicate nests anywhere near this deep.
const maxParseDepth = 256

type parser struct {
	lex   *lexer
	input string
	depth int // current parenthesis nesting depth
}

func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	operands := []Expr{left}
	for p.lex.peek().typ == tOR {
		p.lex.next() // consume OR
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		operands = append(operands, right)
	}
	if len(operands) == 1 {
		return operands[0], nil
	}
	return OrExpr{Operands: operands}, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	operands := []Expr{left}
	for p.lex.peek().typ == tAND {
		p.lex.next() // consume AND
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		operands = append(operands, right)
	}
	if len(operands) == 1 {
		return operands[0], nil
	}
	return AndExpr{Operands: operands}, nil
}

func (p *parser) parseAtom() (Expr, error) {
	tok := p.lex.peek()

	// parenthesised expression
	if tok.typ == tLParen {
		if p.depth >= maxParseDepth {
			return nil, fmt.Errorf("predicate: nesting too deep (limit %d) in %q", maxParseDepth, p.input)
		}
		p.depth++
		p.lex.next() // consume (
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		p.depth--
		if close := p.lex.next(); close.typ != tRParen {
			return nil, fmt.Errorf("predicate: expected ) in %q, got %q", p.input, close.val)
		}
		return expr, nil
	}

	// must be IDENT — either a call or a condition
	if tok.typ != tIDENT {
		return nil, fmt.Errorf("predicate: unexpected token %q in %q", tok.val, p.input)
	}
	p.lex.next() // consume IDENT
	name := tok.val

	// call: IDENT ( IDENT )
	if p.lex.peek().typ == tLParen {
		p.lex.next() // consume (
		arg := p.lex.next()
		if arg.typ != tIDENT {
			return nil, fmt.Errorf("predicate: expected argument in call %s(), got %q", name, arg.val)
		}
		if close := p.lex.next(); close.typ != tRParen {
			return nil, fmt.Errorf("predicate: expected ) after %s(%s, got %q", name, arg.val, close.val)
		}
		return CallExpr{Name: name, Arg: arg.val}, nil
	}

	// condition: KEY = VALUE (, VALUE)*
	if p.lex.peek().typ != tEquals {
		return nil, fmt.Errorf("predicate: expected = after %q in %q", name, p.input)
	}
	p.lex.next() // consume =

	firstVal := p.lex.next()
	if firstVal.typ != tIDENT {
		return nil, fmt.Errorf("predicate: expected value after %s= in %q", name, p.input)
	}
	values := []string{firstVal.val}

	for p.lex.peek().typ == tComma {
		p.lex.next() // consume ,
		val := p.lex.next()
		if val.typ != tIDENT {
			return nil, fmt.Errorf("predicate: expected value after comma in %q", p.input)
		}
		values = append(values, val.val)
	}

	return ConditionExpr{Key: name, Values: values}, nil
}
