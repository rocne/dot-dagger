package linker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
)

// --- confRelPath ---

func TestConfRelPath(t *testing.T) {
	tests := []struct {
		absPath string
		want    string
	}{
		{"/dotfiles/conf/dot-zshrc", ".zshrc"},
		{"/dotfiles/conf/dot-config/tmux/tmux.conf", ".config/tmux/tmux.conf"},
		{"/dotfiles/conf/dot-config/dot-tmux/tmux.conf", ".config/.tmux/tmux.conf"},
		{"/dotfiles/nosync-work/conf/dot-gitconfig", ".gitconfig"},
	}
	for _, tt := range tests {
		got, err := confRelPath(tt.absPath)
		if err != nil {
			t.Errorf("confRelPath(%q) error = %v", tt.absPath, err)
			continue
		}
		if filepath.ToSlash(got) != tt.want {
			t.Errorf("confRelPath(%q) = %q, want %q", tt.absPath, got, tt.want)
		}
	}
}

func TestConfRelPathNoConf(t *testing.T) {
	_, err := confRelPath("/dotfiles/shellrc/base.sh")
	if err == nil {
		t.Error("expected error for path without conf/ ancestor")
	}
}

// --- resolveSymlinkDest ---

func TestResolveSymlinkDestAbsolute(t *testing.T) {
	got := resolveSymlinkDest("/etc/foo", "/link_root")
	if got != "/etc/foo" {
		t.Errorf("got %q, want /etc/foo", got)
	}
}

func TestResolveSymlinkDestHomeTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := resolveSymlinkDest("~/.foo", "/ignored")
	want := filepath.Join(home, ".foo")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSymlinkDestRelative(t *testing.T) {
	got := resolveSymlinkDest("foo/bar", "/link_root")
	want := "/link_root/foo/bar"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- Plan ---

func TestPlanConfNode(t *testing.T) {
	nodes := []fileset.Node{
		{
			Path: "/repo/conf/dot-zshrc",
			Kind: fileset.KindConf,
		},
	}
	opts := Options{LinkRoot: "/home/user", BinDir: "/home/user/.local/bin"}
	links, err := Plan(nodes, opts)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	want := "/home/user/.zshrc"
	if links[0].Dst != want {
		t.Errorf("Dst = %q, want %q", links[0].Dst, want)
	}
	if links[0].Src != "/repo/conf/dot-zshrc" {
		t.Errorf("Src = %q, want /repo/conf/dot-zshrc", links[0].Src)
	}
}

func TestPlanBinNode(t *testing.T) {
	nodes := []fileset.Node{
		{
			Path: "/repo/bin/tmux-sessionizer",
			Kind: fileset.KindBin,
		},
	}
	opts := Options{LinkRoot: "/home/user", BinDir: "/home/user/.local/bin/dot-dagger"}
	links, err := Plan(nodes, opts)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	want := "/home/user/.local/bin/dot-dagger/tmux-sessionizer"
	if links[0].Dst != want {
		t.Errorf("Dst = %q, want %q", links[0].Dst, want)
	}
}

func TestPlanConfNodePerNodeLinkRoot(t *testing.T) {
	nodes := []fileset.Node{
		{
			Path:     "/repo/nvim/conf/dot-init.lua",
			Kind:     fileset.KindConf,
			LinkRoot: "/custom/nvim",
		},
	}
	opts := Options{LinkRoot: "/home/user", BinDir: "/home/user/.local/bin"}
	links, err := Plan(nodes, opts)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	// confRelPath strips the conf/ prefix component, so the dest is /custom/nvim/.init.lua.
	want := "/custom/nvim/.init.lua"
	if links[0].Dst != want {
		t.Errorf("Dst = %q, want %q", links[0].Dst, want)
	}
}

func TestPlanScriptNodeSkipped(t *testing.T) {
	nodes := []fileset.Node{
		{Path: "/repo/shellrc/base.sh", Kind: fileset.KindScript},
	}
	links, err := Plan(nodes, Options{LinkRoot: "/home/user", BinDir: "/home/user/.local/bin"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected no links for KindScript, got %d", len(links))
	}
}

func TestPlanOtherNodeSkipped(t *testing.T) {
	nodes := []fileset.Node{
		{Path: "/repo/readme.txt", Kind: fileset.KindOther},
	}
	links, err := Plan(nodes, Options{LinkRoot: "/home/user", BinDir: "/home/user/.local/bin"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected no links for KindOther, got %d", len(links))
	}
}

func TestPlanSymlinkAnnotationOverrides(t *testing.T) {
	nodes := []fileset.Node{
		{
			Path: "/repo/conf/dot-zshrc",
			Kind: fileset.KindConf,
			Annotations: []annotation.Annotation{
				{Key: annotation.KeySymlink, Value: "/custom/dest"},
			},
		},
	}
	links, err := Plan(nodes, Options{LinkRoot: "/home/user", BinDir: "/home/user/.local/bin"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if links[0].Dst != "/custom/dest" {
		t.Errorf("Dst = %q, want /custom/dest", links[0].Dst)
	}
}

// --- Check and Apply (filesystem tests) ---

func TestCheckMissing(t *testing.T) {
	dir := t.TempDir()
	links := []Link{
		{Src: "/src/file.sh", Dst: filepath.Join(dir, "nonexistent")},
	}
	got := Check(links, "/repo")
	if got[0].State != StateMissing {
		t.Errorf("State = %v, want Missing", got[0].State)
	}
}

func TestCheckOK(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	dst := filepath.Join(dir, "dst.sh")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: src, Dst: dst}}
	got := Check(links, dir)
	if got[0].State != StateOK {
		t.Errorf("State = %v, want OK", got[0].State)
	}
	if !got[0].Owned {
		t.Error("Owned should be true when target is under repoRoot")
	}
}

func TestCheckWrongTarget(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	other := filepath.Join(dir, "other.sh")
	dst := filepath.Join(dir, "dst.sh")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, dst); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: src, Dst: dst}}
	got := Check(links, dir)
	if got[0].State != StateWrongTarget {
		t.Errorf("State = %v, want WrongTarget", got[0].State)
	}
}

func TestCheckConflict(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "real-file")
	if err := os.WriteFile(dst, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: "/src", Dst: dst}}
	got := Check(links, dir)
	if got[0].State != StateConflict {
		t.Errorf("State = %v, want Conflict", got[0].State)
	}
}

func TestApplyCreatesMissing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	dst := filepath.Join(dir, "sub", "dst.sh")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: src, Dst: dst, State: StateMissing}}
	if err := Apply(links, false); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if target != src {
		t.Errorf("target = %q, want %q", target, src)
	}
}

func TestApplyConflictRequiresForce(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "file")
	if err := os.WriteFile(dst, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: "/src", Dst: dst, State: StateConflict}}
	if err := Apply(links, false); err == nil {
		t.Error("Apply() error = nil, want error for conflict without force")
	}
}

func TestApplyConflictWithForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	dst := filepath.Join(dir, "file")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: src, Dst: dst, State: StateConflict}}
	if err := Apply(links, true); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if target != src {
		t.Errorf("target = %q, want %q", target, src)
	}
}

func TestRemoveOwned(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	dst := filepath.Join(dir, "dst.sh")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: src, Dst: dst, State: StateOK, Owned: true}}
	if err := Remove(links); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Error("symlink should be removed")
	}
}

func TestRemoveNotOwned(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	dst := filepath.Join(dir, "dst.sh")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}
	links := []Link{{Src: src, Dst: dst, State: StateOK, Owned: false}}
	if err := Remove(links); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	// Not owned — should NOT be removed.
	if _, err := os.Lstat(dst); err != nil {
		t.Error("non-owned symlink should not be removed")
	}
}
