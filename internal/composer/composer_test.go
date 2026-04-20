package composer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkFragments builds a slice of KindCompose nodes pointing at real temp files.
func mkFragments(t *testing.T, targetDir string, targetKind fileset.Kind, files map[string]string) []fileset.Node {
	t.Helper()
	var nodes []fileset.Node
	for name, content := range files {
		path := filepath.Join(targetDir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		nodes = append(nodes, fileset.Node{
			Path:              path,
			Kind:              fileset.KindCompose,
			LogicalName:       "compose." + name,
			ComposeTarget:     targetDir,
			ComposeTargetKind: targetKind,
		})
	}
	return nodes
}

func TestDeriveOutputName_FromBasename(t *testing.T) {
	cases := []struct {
		dir  string
		want string
	}{
		{"/repo/shellrc/dot-aliases.sh.d", "aliases.sh"},
		{"/repo/conf/dot-tmux.conf.d", "tmux.conf"},
		{"/repo/bin/my-tool.d", "my-tool"},
		{"/repo/shellrc/nosync-dot-work.sh.d", "work.sh"},
	}
	for _, c := range cases {
		got, err := deriveOutputName(c.dir, "")
		require.NoError(t, err, "dir=%s", c.dir)
		assert.Equal(t, c.want, got, "dir=%s", c.dir)
	}
}

func TestDeriveOutputName_Override(t *testing.T) {
	got, err := deriveOutputName("/repo/conf/dot-tmux.conf.d", "my-override.conf")
	require.NoError(t, err)
	assert.Equal(t, "my-override.conf", got)
}

func TestApply_ConcatenatesFragments(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "dot-aliases.sh.d")
	require.NoError(t, os.Mkdir(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".dotd.yaml"), []byte("dotd:\n  compose: true\n"), 0o644))

	frags := mkFragments(t, targetDir, fileset.KindScript, map[string]string{
		"a.sh": "# fragment a\n",
		"b.sh": "# fragment b\n",
	})

	outDir := t.TempDir()
	synthetic, err := Apply(frags, Options{GeneratedDir: outDir})
	require.NoError(t, err)
	require.Len(t, synthetic, 1)

	assert.Equal(t, "aliases.sh", synthetic[0].LogicalName)
	assert.Equal(t, fileset.KindScript, synthetic[0].Kind)

	content, err := os.ReadFile(synthetic[0].Path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "fragment a")
	assert.Contains(t, string(content), "fragment b")
}

func TestApply_DryRun(t *testing.T) {
	targetDotD := filepath.Join(t.TempDir(), "dot-aliases.sh.d")
	require.NoError(t, os.Mkdir(targetDotD, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDotD, ".dotd.yaml"), []byte("dotd:\n  compose: true\n"), 0o644))

	fragPath := filepath.Join(targetDotD, "base.sh")
	require.NoError(t, os.WriteFile(fragPath, []byte("echo hello\n"), 0o644))

	frags := []fileset.Node{{
		Path:              fragPath,
		Kind:              fileset.KindCompose,
		LogicalName:       "shellrc.aliases.sh.d.base",
		ComposeTarget:     targetDotD,
		ComposeTargetKind: fileset.KindScript,
	}}

	outDir := t.TempDir()
	synthetic, err := Apply(frags, Options{GeneratedDir: outDir, DryRun: true})
	require.NoError(t, err)
	require.Len(t, synthetic, 1)

	// DryRun: generated file should NOT exist on disk.
	_, statErr := os.Stat(synthetic[0].Path)
	assert.True(t, os.IsNotExist(statErr), "dry run should not write file")
}

func TestApply_InvalidAnnotation(t *testing.T) {
	targetDotD := filepath.Join(t.TempDir(), "dot-aliases.sh.d")
	require.NoError(t, os.Mkdir(targetDotD, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDotD, ".dotd.yaml"), []byte("dotd:\n  compose: true\n"), 0o644))

	fragPath := filepath.Join(targetDotD, "bad.sh")
	require.NoError(t, os.WriteFile(fragPath, []byte("# @symlink ~/.foo\necho hi\n"), 0o644))

	// Inject a @symlink annotation directly to simulate an invalid fragment.
	frags := []fileset.Node{{
		Path:              fragPath,
		Kind:              fileset.KindCompose,
		LogicalName:       "shellrc.aliases.sh.d.bad",
		ComposeTarget:     targetDotD,
		ComposeTargetKind: fileset.KindScript,
		Annotations: []annotation.Annotation{{Key: "symlink", Value: "~/.foo"}},
	}}

	_, err := Apply(frags, Options{GeneratedDir: t.TempDir()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "@symlink")
}

func TestCheck_Missing(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "dot-aliases.sh.d")
	require.NoError(t, os.Mkdir(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".dotd.yaml"), []byte("dotd:\n  compose: true\n"), 0o644))

	fragPath := filepath.Join(targetDir, "base.sh")
	require.NoError(t, os.WriteFile(fragPath, []byte("echo hi\n"), 0o644))

	frags := []fileset.Node{{
		Path:              fragPath,
		Kind:              fileset.KindCompose,
		LogicalName:       "shellrc.aliases.sh.d.base",
		ComposeTarget:     targetDir,
		ComposeTargetKind: fileset.KindScript,
	}}

	outDir := t.TempDir()
	statuses, err := Check(frags, Options{GeneratedDir: outDir})
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, StateMissing, statuses[0].State)
}

func TestCheck_Stale(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "dot-aliases.sh.d")
	require.NoError(t, os.Mkdir(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".dotd.yaml"), []byte("dotd:\n  compose: true\n"), 0o644))

	fragPath := filepath.Join(targetDir, "base.sh")
	require.NoError(t, os.WriteFile(fragPath, []byte("echo hi\n"), 0o644))

	frags := []fileset.Node{{
		Path:              fragPath,
		Kind:              fileset.KindCompose,
		LogicalName:       "shellrc.aliases.sh.d.base",
		ComposeTarget:     targetDir,
		ComposeTargetKind: fileset.KindScript,
	}}

	outDir := t.TempDir()
	// Write stale content.
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "aliases.sh"), []byte("old content\n"), 0o644))

	statuses, err := Check(frags, Options{GeneratedDir: outDir})
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, StateStale, statuses[0].State)
}

func TestCheck_OK(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "dot-aliases.sh.d")
	require.NoError(t, os.Mkdir(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".dotd.yaml"), []byte("dotd:\n  compose: true\n"), 0o644))

	fragPath := filepath.Join(targetDir, "base.sh")
	require.NoError(t, os.WriteFile(fragPath, []byte("echo hi\n"), 0o644))

	frags := []fileset.Node{{
		Path:              fragPath,
		Kind:              fileset.KindCompose,
		LogicalName:       "shellrc.aliases.sh.d.base",
		ComposeTarget:     targetDir,
		ComposeTargetKind: fileset.KindScript,
	}}

	outDir := t.TempDir()
	// Write the correct content.
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "aliases.sh"), []byte("echo hi\n"), 0o644))

	statuses, err := Check(frags, Options{GeneratedDir: outDir})
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, StateOK, statuses[0].State)
}

func TestConfDest_Basic(t *testing.T) {
	cases := []struct {
		targetDir string
		linkRoot  string
		dotdName  string
		want      string
	}{
		{
			targetDir: "/repo/conf/dot-tmux.conf.d",
			linkRoot:  "/home/user",
			want:      "/home/user/.tmux.conf",
		},
		{
			targetDir: "/repo/conf/dot-gitconfig.d",
			linkRoot:  "/home/user",
			want:      "/home/user/.gitconfig",
		},
		{
			targetDir: "/repo/conf/subdir/dot-tmux.conf.d",
			linkRoot:  "/home/user",
			want:      "/home/user/subdir/.tmux.conf",
		},
		{
			// dotd.name override: derived from dotdName, not basename
			targetDir: "/repo/conf/dot-tmux.conf.d",
			linkRoot:  "/home/user",
			dotdName:  "dot-gitconfig",
			want:      "/home/user/.gitconfig",
		},
		{
			// dotd.name without dot- prefix
			targetDir: "/repo/conf/dot-tmux.conf.d",
			linkRoot:  "/home/user",
			dotdName:  "gitconfig",
			want:      "/home/user/gitconfig",
		},
	}
	for _, c := range cases {
		got := confDest(c.targetDir, c.linkRoot, c.dotdName)
		assert.Equal(t, c.want, got, "targetDir=%s dotdName=%q", c.targetDir, c.dotdName)
	}
}

