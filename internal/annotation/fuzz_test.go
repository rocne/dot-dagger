package annotation

import (
	"strings"
	"testing"
)

// FuzzScan asserts the annotation scanner never panics on arbitrary file
// content, and that the downstream consumers fed by its output (CombineWhen,
// SplitParen) are equally crash-proof.
func FuzzScan(f *testing.F) {
	seeds := []string{
		"# @when(os=linux)\n# @after(foo)\ncontent\n",
		"#!/bin/sh\n# @action(link)\n",
		"// @name(thing)\n// @require(bar)\n",
		"# @when(os=linux) and more\n",
		"# @when(\n",      // unterminated paren
		"# @(empty key)\n", // empty key after @
		"",
		"no annotations here\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		anns, err := Scan(strings.NewReader(input))
		if err != nil {
			return
		}
		_ = CombineWhen(anns)
		for _, a := range anns {
			_, _, _ = SplitParen(a.Args)
		}
	})
}

// FuzzSplitParen asserts SplitParen never panics and respects its byte-slicing
// contract on arbitrary input (it indexes into the string by hand).
func FuzzSplitParen(f *testing.F) {
	for _, s := range []string{"head(body)", "head", "head(", "(body)", "()", "a(b(c)d)e", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _, _ = SplitParen(input) // must not panic on any string
	})
}
