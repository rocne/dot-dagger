package node

import "testing"

// FuzzDeriveName asserts the logical-name derivation never panics on arbitrary
// relative paths (it splits and slices path components by hand) and never
// returns an empty name for a non-empty input — an empty name collides with
// other empty names and aborts apply with a spurious duplicate-name error
// (regression direction for the B-2 ".gitignore" → "" bug).
func FuzzDeriveName(f *testing.F) {
	seeds := []string{
		"dot-gitconfig",
		"config/dot-tmux.conf",
		"nosync-dot-secret",
		".gitignore",
		"a/b/c.sh",
		"dot-.x",
		"...",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, relPath string) {
		got := DeriveName(relPath) // must not panic
		if relPath != "" && got == "" {
			t.Errorf("DeriveName(%q) = %q: non-empty input must not derive an empty name", relPath, got)
		}
	})
}
