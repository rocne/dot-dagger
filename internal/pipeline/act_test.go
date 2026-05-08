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
	n := actNode(t, dir, "base", []Action{{Type: "source"}})
	res, err := Act([]RawNode{n}, ActOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sourced) != 1 || res.Sourced[0].LogicalName != "base" {
		t.Errorf("expected base in Sourced, got %v", res.Sourced)
	}
}

func TestAct_NoSource_SuppressesSource(t *testing.T) {
	dir := t.TempDir()
	n := actNode(t, dir, "base", []Action{{Type: "source"}, {Type: "no-source"}})
	res, err := Act([]RawNode{n}, ActOptions{})
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
	n := actNode(t, dir, "tmux", []Action{{Type: "link", Dest: dest}})
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
	n1 := actNode(t, dir, "a", []Action{{Type: "link", Dest: dest}})
	n2 := actNode(t, dir, "b", []Action{{Type: "link", Dest: dest}})

	_, err := Act([]RawNode{n1, n2}, ActOptions{HomeDir: destDir})
	if err == nil {
		t.Error("expected conflict error, got nil")
	}
}

func TestAct_Link_TildeExpansion(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	n := actNode(t, dir, "tmux", []Action{{Type: "link", Dest: "~/.tmux.conf"}})
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
	n := actNode(t, dir, "tmux", []Action{{Type: "link", Dest: dest}})
	n.LinkRoot = destDir

	_, err := Act([]RawNode{n}, ActOptions{HomeDir: destDir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("dry run should not create symlink")
	}
}
