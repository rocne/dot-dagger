package predicate

import "testing"

// FuzzParse asserts the predicate parser never panics on arbitrary input and
// that any expression it accepts can be evaluated without panicking. Invalid
// input must be rejected with an error, never a crash.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"os=linux",
		"os=linux,macos",
		"os=linux & shell=bash",
		"os=linux | os=macos",
		"(os=linux | os=macos) & context=work",
		"exists(git)",
		"a=b & (c=d | e=f)",
		"(((a=b)))",
		"!os=linux",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	env := map[string]string{"os": "linux", "shell": "bash", "context": "work"}
	f.Fuzz(func(t *testing.T, input string) {
		expr, err := Parse(input)
		if err != nil {
			return // invalid input rejected cleanly — that's correct behaviour
		}
		ev := NewEvaluator(env)
		// Stub PATH lookups so exists()/installed() never shell out during fuzzing.
		ev.LookPath = func(string) (string, error) { return "", nil }
		_, _ = ev.Eval(expr) // must not panic
	})
}
