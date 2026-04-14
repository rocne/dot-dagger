package predicate

import (
	"strings"
	"unicode"
)

// TokenType identifies the kind of a lexed token.
type TokenType int

const (
	tEOF    TokenType = iota
	tIDENT            // identifier or value: [a-zA-Z0-9._-]+
	tAND              // AND keyword
	tOR               // OR keyword
	tLParen           // (
	tRParen           // )
	tEquals           // =
	tComma            // ,
)

// token is a single lexed token.
type token struct {
	typ TokenType
	val string
}

// lexer tokenizes a predicate expression string.
type lexer struct {
	input  []rune
	pos    int
	peeked *token
}

func newLexer(input string) *lexer {
	return &lexer{input: []rune(input)}
}

// next returns the next token, consuming it.
func (l *lexer) next() token {
	if l.peeked != nil {
		t := *l.peeked
		l.peeked = nil
		return t
	}
	return l.scan()
}

// peek returns the next token without consuming it.
func (l *lexer) peek() token {
	if l.peeked == nil {
		t := l.scan()
		l.peeked = &t
	}
	return *l.peeked
}

func (l *lexer) scan() token {
	// skip whitespace
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
	if l.pos >= len(l.input) {
		return token{typ: tEOF}
	}

	ch := l.input[l.pos]

	switch ch {
	case '(':
		l.pos++
		return token{typ: tLParen, val: "("}
	case ')':
		l.pos++
		return token{typ: tRParen, val: ")"}
	case '=':
		l.pos++
		return token{typ: tEquals, val: "="}
	case ',':
		l.pos++
		return token{typ: tComma, val: ","}
	}

	// identifier or keyword
	if isIdentRune(ch) {
		start := l.pos
		for l.pos < len(l.input) && isIdentRune(l.input[l.pos]) {
			l.pos++
		}
		val := string(l.input[start:l.pos])
		switch strings.ToUpper(val) {
		case "AND":
			if val == "AND" {
				return token{typ: tAND, val: val}
			}
		case "OR":
			if val == "OR" {
				return token{typ: tOR, val: val}
			}
		}
		return token{typ: tIDENT, val: val}
	}

	// unrecognised character — advance and return as ident to surface in parser error
	l.pos++
	return token{typ: tIDENT, val: string(ch)}
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}
