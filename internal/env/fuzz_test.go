package env

import (
	"strings"
	"testing"
)

// FuzzLoad asserts env.yaml decoding never panics on arbitrary content. (The
// reader-based unexported loader is used directly so no disk or shell is
// touched — Expand, which runs sh -c, is deliberately not fuzzed.)
func FuzzLoad(f *testing.F) {
	seeds := []string{
		"",
		"os: linux\nshell: bash\n",
		"context: work\n",
		"key: $(echo hi)\n",
		"nested:\n  a: b\n", // wrong shape for map[string]string
		"- a\n- b\n",        // sequence, not a map
		":\n",
		"key: [1, 2]\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		raw, err := load(strings.NewReader(input))
		if err != nil {
			return
		}
		// shellExpr is part of the raw-value contract; exercise it crash-free.
		for _, v := range raw {
			_, _ = shellExpr(v)
		}
	})
}

// FuzzShellVars asserts DOTD_* environ parsing never panics — it slices each
// entry by hand around the first '='.
func FuzzShellVars(f *testing.F) {
	for _, s := range []string{"DOTD_CONTEXT=work", "DOTD_=x", "DOTD_OS", "NOPREFIX=y", "DOTD_A=B=C", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, entry string) {
		_ = ShellVars([]string{entry}) // must not panic on any single environ entry
	})
}
