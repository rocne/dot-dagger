package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// actNode builds a RawNode backed by a real temp file.
func actNode(t *testing.T, dir, name string, actions []Action) RawNode {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return RawNode{Path: path, LogicalName: name, Actions: actions}
}

func TestAct_Source(t *testing.T) {
	dir := t.TempDir()
	n := actNode(t, dir, "base", []Action{{Type: ActionSource}})
	res, err := Act([]RawNode{n}, ActOptions{HomeDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sourced) != 1 || res.Sourced[0].LogicalName != "base" {
		t.Errorf("expected base in Sourced, got %v", res.Sourced)
	}
}

func TestAct_NoSource_SuppressesSource(t *testing.T) {
	dir := t.TempDir()
	n := actNode(t, dir, "base", []Action{{Type: ActionSource}, {Type: ActionNoSource}})
	res, err := Act([]RawNode{n}, ActOptions{HomeDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sourced) != 0 {
		t.Errorf("no-source should suppress source, got %v", res.Sourced)
	}
}

func TestAct_Link_CreatesSymlink(t *testing.T) {
	dir := t.TempDir()
	destDir := t.TempDir()
	dest := filepath.Join(destDir, ".tmux.conf")
	n := actNode(t, dir, "tmux", []Action{{Type: ActionLink, Dest: dest}})
	n.LinkRoot = destDir

	opts := ActOptions{HomeDir: destDir}
	res, err := Act([]RawNode{n}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Links) != 1 {
		t.Fatalf("expected 1 link, got %v", res.Links)
	}
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if target != n.Path {
		t.Errorf("symlink target = %q, want %q", target, n.Path)
	}
}

func TestAct_Link_Conflict_Error(t *testing.T) {
	dir := t.TempDir()
	destDir := t.TempDir()
	dest := filepath.Join(destDir, "same")
	n1 := actNode(t, dir, "a", []Action{{Type: ActionLink, Dest: dest}})
	n2 := actNode(t, dir, "b", []Action{{Type: ActionLink, Dest: dest}})

	_, err := Act([]RawNode{n1, n2}, ActOptions{HomeDir: destDir})
	if err == nil {
		t.Error("expected conflict error, got nil")
	}
}

func TestAct_Link_TildeExpansion(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	n := actNode(t, dir, "tmux", []Action{{Type: ActionLink, Dest: "~/.tmux.conf"}})
	n.LinkRoot = home

	res, err := Act([]RawNode{n}, ActOptions{HomeDir: home})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Links) != 1 {
		t.Fatalf("expected 1 link, got %v", res.Links)
	}
	want := filepath.Join(home, ".tmux.conf")
	if res.Links[0].Dest != want {
		t.Errorf("link dest = %q, want %q", res.Links[0].Dest, want)
	}
}

func TestAct_DryRun_NoWrite(t *testing.T) {
	dir := t.TempDir()
	destDir := t.TempDir()
	dest := filepath.Join(destDir, ".tmux.conf")
	n := actNode(t, dir, "tmux", []Action{{Type: ActionLink, Dest: dest}})
	n.LinkRoot = destDir

	_, err := Act([]RawNode{n}, ActOptions{HomeDir: destDir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("dry run should not create symlink")
	}
}

// fragment pairs a filename with its content for use in composeDir.
// Using a slice (not a map) makes iteration order explicit and self-documenting.
type fragment struct {
	name    string
	content string
}

// composeDir creates a compose-target directory with .dagger fragments and
// returns the dir path and a compose-target RawNode for use in Act tests.
// Fragments are written to disk in the order they appear in frags; that same
// order must be reflected in the composeFragNode calls so Act receives them in
// the expected sequence.
func composeDir(t *testing.T, root, dirName string, frags []fragment, actions []Action) (string, RawNode) {
	t.Helper()
	compDir := filepath.Join(root, dirName)
	if err := os.MkdirAll(compDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range frags {
		if err := os.WriteFile(filepath.Join(compDir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	targetNode := RawNode{
		Path:          compDir,
		LogicalName:   dirName,
		Actions:       append([]Action{{Type: ActionCompose}}, actions...),
		ComposeTarget: compDir,
	}
	return compDir, targetNode
}

// composeFragNode builds a RawNode representing a compose fragment backed by a real file.
func composeFragNode(t *testing.T, fragDir, name string) RawNode {
	t.Helper()
	path := filepath.Join(fragDir, name)
	return RawNode{
		Path:          path,
		LogicalName:   name,
		IsCompose:     true,
		ComposeTarget: fragDir,
	}
}

// TestAct_Compose_Assembled verifies that Act assembles compose fragments into
// a Generated result with the correct content, filename, and no symlink action.
// Fragment ordering follows the nodes slice passed to Act; composeDir writes
// files in declaration order so it is self-evidently controlled here.
func TestAct_Compose_Assembled(t *testing.T) {
	root := t.TempDir()
	genDir := t.TempDir()
	home := t.TempDir()

	frags := []fragment{
		{"base.conf", "line1\n"},
		{"nosync-work.conf", "line2\n"},
	}
	compDir, targetNode := composeDir(t, root, "dot-tmux.conf.d", frags, nil)

	frag1 := composeFragNode(t, compDir, "base.conf")
	frag2 := composeFragNode(t, compDir, "nosync-work.conf")

	nodes := []RawNode{targetNode, frag1, frag2}
	res, err := Act(nodes, ActOptions{HomeDir: home, GeneratedDir: genDir})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Generated) != 1 {
		t.Fatalf("expected 1 Generated, got %d", len(res.Generated))
	}
	gen := res.Generated[0]

	// ComposeFileName: "dot-tmux.conf.d" → "tmux.conf"
	wantName := "tmux.conf"
	if filepath.Base(gen.Path) != wantName {
		t.Errorf("Generated filename = %q, want %q", filepath.Base(gen.Path), wantName)
	}

	// Content is concatenation of both fragments.
	wantContent := "line1\nline2\n"
	if string(gen.Content) != wantContent {
		t.Errorf("Generated content = %q, want %q", string(gen.Content), wantContent)
	}

	// File should have been written to disk.
	data, err := os.ReadFile(gen.Path)
	if err != nil {
		t.Fatalf("generated file not written: %v", err)
	}
	if string(data) != wantContent {
		t.Errorf("on-disk content = %q, want %q", string(data), wantContent)
	}
}

// TestAct_Compose_Link verifies that a compose target with an ActionLink creates
// a symlink from the link destination to the generated file.
func TestAct_Compose_Link(t *testing.T) {
	root := t.TempDir()
	genDir := t.TempDir()
	home := t.TempDir()

	frags := []fragment{
		{"base.conf", "config-line\n"},
	}
	dest := filepath.Join(home, ".tmux.conf")
	compDir, targetNode := composeDir(t, root, "dot-tmux.conf.d", frags, []Action{{Type: ActionLink, Dest: dest}})

	frag := composeFragNode(t, compDir, "base.conf")

	res, err := Act([]RawNode{targetNode, frag}, ActOptions{HomeDir: home, GeneratedDir: genDir})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(res.Links))
	}
	lnk := res.Links[0]

	// Symlink should point to the generated file.
	wantSrc := filepath.Join(genDir, "tmux.conf")
	if lnk.Src != wantSrc {
		t.Errorf("link.Src = %q, want %q", lnk.Src, wantSrc)
	}
	if lnk.Dest != dest {
		t.Errorf("link.Dest = %q, want %q", lnk.Dest, dest)
	}

	// Symlink on disk: dest → generated file.
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("symlink not created at %s: %v", dest, err)
	}
	if target != wantSrc {
		t.Errorf("symlink target = %q, want %q", target, wantSrc)
	}
}

// TestAct_Compose_Source verifies that a compose target with ActionSource adds a
// synthetic node (pointing to the generated file) to res.Sourced.
func TestAct_Compose_Source(t *testing.T) {
	root := t.TempDir()
	genDir := t.TempDir()
	home := t.TempDir()

	frags := []fragment{
		{"base.sh", "export FOO=1\n"},
	}
	compDir, targetNode := composeDir(t, root, "dot-shellrc-extras.sh.d", frags, []Action{{Type: ActionSource}})

	frag := composeFragNode(t, compDir, "base.sh")

	res, err := Act([]RawNode{targetNode, frag}, ActOptions{HomeDir: home, GeneratedDir: genDir})
	if err != nil {
		t.Fatal(err)
	}

	// "dot-shellrc-extras.sh.d" → "shellrc-extras.sh"
	wantGenName := "shellrc-extras.sh"
	if len(res.Generated) != 1 || filepath.Base(res.Generated[0].Path) != wantGenName {
		t.Errorf("Generated = %v, want one entry named %q", res.Generated, wantGenName)
	}

	// A synthetic sourced node with the generated file path.
	if len(res.Sourced) != 1 {
		t.Fatalf("expected 1 Sourced node, got %d", len(res.Sourced))
	}
	if res.Sourced[0].Path != res.Generated[0].Path {
		t.Errorf("sourced.Path = %q, want %q", res.Sourced[0].Path, res.Generated[0].Path)
	}
}

// TestAct_Compose_DryRun verifies that DryRun mode produces the correct Generated
// metadata without writing files to disk.
func TestAct_Compose_DryRun(t *testing.T) {
	root := t.TempDir()
	genDir := t.TempDir()
	home := t.TempDir()

	// In DryRun mode, os.ReadFile is skipped so fragment content will be nil/empty.
	frags := []fragment{
		{"base.conf", "irrelevant\n"},
	}
	compDir, targetNode := composeDir(t, root, "dot-tmux.conf.d", frags, nil)
	frag := composeFragNode(t, compDir, "base.conf")

	res, err := Act([]RawNode{targetNode, frag}, ActOptions{HomeDir: home, GeneratedDir: genDir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Generated) != 1 {
		t.Fatalf("expected 1 Generated, got %d", len(res.Generated))
	}
	// No file should have been written in dry-run mode.
	genPath := filepath.Join(genDir, "tmux.conf")
	if _, err := os.Lstat(genPath); !os.IsNotExist(err) {
		t.Error("dry run should not write the generated file")
	}
	// Content must be empty — dry-run skips ReadFile entirely.
	if len(res.Generated[0].Content) != 0 {
		t.Error("dry run should not populate Content")
	}
}

// TestAct_Compose_NoSourceSuppressesSource verifies that ActionNoSource on a
// compose target prevents the generated file from being added to res.Sourced,
// even when ActionSource is also present. This covers the compose-path noSource
// branch in Act (lines 108-113 of act.go).
func TestAct_Compose_NoSourceSuppressesSource(t *testing.T) {
	root := t.TempDir()
	genDir := t.TempDir()
	home := t.TempDir()

	frags := []fragment{
		{"base.sh", "export BAR=1\n"},
	}
	compDir, targetNode := composeDir(t, root, "dot-shellrc-extras.sh.d", frags,
		[]Action{{Type: ActionSource}, {Type: ActionNoSource}},
	)
	frag := composeFragNode(t, compDir, "base.sh")

	res, err := Act([]RawNode{targetNode, frag}, ActOptions{HomeDir: home, GeneratedDir: genDir})
	if err != nil {
		t.Fatal(err)
	}
	// The file is still generated …
	if len(res.Generated) != 1 {
		t.Fatalf("expected 1 Generated, got %d", len(res.Generated))
	}
	// … but no-source must suppress the source entry.
	if len(res.Sourced) != 0 {
		t.Errorf("no-source should suppress source on compose target, got Sourced=%v", res.Sourced)
	}
}

// TestComposeFileName_Variants covers the naming rules for ComposeFileName.
func TestComposeFileName_Variants(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/a/b/dot-tmux.conf.d", "tmux.conf"},
		{"/a/b/nosync-dot-shellrc.d", "shellrc"},
		{"/a/b/dot-shellrc-extras.sh.d", "shellrc-extras.sh"},
		{"/a/b/nosync-work.d", "work"},
	}
	for _, c := range cases {
		got := ComposeFileName(c.input)
		if got != c.want {
			t.Errorf("ComposeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
