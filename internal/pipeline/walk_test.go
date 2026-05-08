package pipeline

import (
	"path/filepath"
	"sort"
	"testing"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/dotfiles")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// nodeByLogicalName returns the first RawNode with the given logical name, or nil.
func nodeByLogicalName(nodes []RawNode, name string) *RawNode {
	for i := range nodes {
		if nodes[i].LogicalName == name {
			return &nodes[i]
		}
	}
	return nil
}

func TestWalk_ProducesNodes(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected nodes, got none")
	}
}

func TestWalk_NoDaggerFiles(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if filepath.Base(n.Path) == ".dagger" {
			t.Errorf(".dagger file should not be in output: %s", n.Path)
		}
	}
}

func TestWalk_DefaultsActions_Inherited(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// shellrc/base.sh should inherit source action from shellrc/.dagger defaults
	n := nodeByLogicalName(nodes, "shellrc.base")
	if n == nil {
		t.Fatal("expected node shellrc.base")
	}
	if !hasAction(n.Actions, "source") {
		t.Errorf("shellrc.base should have source action, got %v", n.Actions)
	}
}

func TestWalk_FileAnnotation_When(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// shellrc/macos.sh has @when(os=macos)
	n := nodeByLogicalName(nodes, "shellrc.macos")
	if n == nil {
		t.Fatal("expected node shellrc.macos")
	}
	if n.EffectiveWhen != "(os=macos)" {
		t.Errorf("shellrc.macos EffectiveWhen = %q, want %q", n.EffectiveWhen, "(os=macos)")
	}
}

func TestWalk_DefaultsWhen_AndFileWhen(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// nosync-work/shellrc/aliases.sh:
	// - dir .dagger has defaults.when = "context=work"
	// - file has no @when
	// → EffectiveWhen = "(context=work)"
	n := nodeByLogicalName(nodes, "work.shellrc.aliases")
	if n == nil {
		// Print all node names for debugging
		names := make([]string, len(nodes))
		for i, nd := range nodes {
			names[i] = nd.LogicalName
		}
		sort.Strings(names)
		t.Fatalf("expected node work.shellrc.aliases, got: %v", names)
	}
	if n.EffectiveWhen != "(context=work)" {
		t.Errorf("work.shellrc.aliases EffectiveWhen = %q, want %q", n.EffectiveWhen, "(context=work)")
	}
}

func TestWalk_FileAnnotation_Link(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// conf/dot-gitconfig has @link(~/.gitconfig) annotation
	n := nodeByLogicalName(nodes, "conf.gitconfig")
	if n == nil {
		t.Fatal("expected node conf.gitconfig")
	}
	if !hasActionWithDest(n.Actions, "link", "~/.gitconfig") {
		t.Errorf("conf.gitconfig should have link(~/.gitconfig) action, got %v", n.Actions)
	}
}

func TestWalk_ConfdirLinkRoot(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// conf/ has link_root: "~" — all nodes under conf/ should inherit it
	n := nodeByLogicalName(nodes, "conf.tmux")
	if n == nil {
		t.Fatal("expected node conf.tmux")
	}
	if n.LinkRoot != "~" {
		t.Errorf("conf.tmux LinkRoot = %q, want %q", n.LinkRoot, "~")
	}
}

func TestWalk_LogicalName_DotPrefix(t *testing.T) {
	root := fixtureRoot(t)
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// conf/dot-tmux.conf → "conf.tmux" (dot- stripped, .conf extension stripped)
	n := nodeByLogicalName(nodes, "conf.tmux")
	if n == nil {
		names := make([]string, len(nodes))
		for i, nd := range nodes {
			names[i] = nd.LogicalName
		}
		sort.Strings(names)
		t.Fatalf("expected node conf.tmux, got: %v", names)
	}
}

func hasAction(actions []Action, typ string) bool {
	for _, a := range actions {
		if a.Type == typ {
			return true
		}
	}
	return false
}

func hasActionWithDest(actions []Action, typ, dest string) bool {
	for _, a := range actions {
		if a.Type == typ && a.Dest == dest {
			return true
		}
	}
	return false
}
