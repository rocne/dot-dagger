package manifest

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	yaml := `
- packages:
    - ripgrep
    - fd

- when: os=macos
  packages:
    - aerospace
`
	blocks, err := Parse(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, blocks, 2)

	assert.Equal(t, "", blocks[0].When)
	assert.Equal(t, []string{"ripgrep", "fd"}, blocks[0].Packages)

	assert.Equal(t, "os=macos", blocks[1].When)
	assert.Equal(t, []string{"aerospace"}, blocks[1].Packages)
}

func TestCollectFromPaths_unconditional(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- packages:
    - ripgrep
    - fd
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "linux"})
	require.NoError(t, err)
	require.Len(t, reqs, 2)
	assert.Equal(t, "ripgrep", reqs[0].Package)
	assert.Equal(t, "fd", reqs[1].Package)
	assert.False(t, reqs[0].Hard)
}

func TestCollectFromPaths_predicate_match(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- when: os=macos
  packages:
    - aerospace

- when: os=linux
  packages:
    - i3
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "macos"})
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, "aerospace", reqs[0].Package)
}

func TestCollectFromPaths_predicate_no_match(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- when: os=macos
  packages:
    - aerospace
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "linux"})
	require.NoError(t, err)
	assert.Empty(t, reqs)
}

func TestCollectFromPaths_multiple_files(t *testing.T) {
	dir := t.TempDir()
	p1 := writeManifest(t, dir, "dotd-packages.yaml", `
- packages:
    - ripgrep
`)
	p2 := writeManifest(t, dir, "mac.dotd-packages.yaml", `
- when: os=macos
  packages:
    - aerospace
`)
	reqs, err := CollectFromPaths([]string{p1, p2}, map[string]string{"os": "macos"})
	require.NoError(t, err)
	require.Len(t, reqs, 2)
}

func TestCollectFromPaths_complex_predicate(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- when: os=macos AND context=work
  packages:
    - some-work-tool
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "macos", "context": "work"})
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, "some-work-tool", reqs[0].Package)

	reqs, err = CollectFromPaths([]string{path}, map[string]string{"os": "macos", "context": "personal"})
	require.NoError(t, err)
	assert.Empty(t, reqs)
}

func writeManifest(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
