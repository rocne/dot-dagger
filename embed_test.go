package dotdagger

import (
	"io/fs"
	"testing"
)

func TestDocsFS_ContainsCoreDocs(t *testing.T) {
	for _, want := range []string{
		"docs/index.md",
		"docs/concepts/conditions.md",
		"docs/reference/dotd.md",
		"docs/getting-started/index.md",
	} {
		if _, err := fs.Stat(DocsFS, want); err != nil {
			t.Errorf("DocsFS missing %s: %v", want, err)
		}
	}
}

func TestDocsFS_ExcludesSuperpowers(t *testing.T) {
	if _, err := fs.Stat(DocsFS, "docs/superpowers"); err == nil {
		t.Error("DocsFS must not embed docs/superpowers (internal specs/plans)")
	}
}
