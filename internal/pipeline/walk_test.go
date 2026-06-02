package pipeline

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
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
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected nodes, got none")
	}
}

func TestWalk_NoDaggerFiles(t *testing.T) {
	root := fixtureRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if filepath.Base(n.Path) == ecosystem.ConfigFile {
			t.Errorf(".dagger file should not be in output: %s", n.Path)
		}
	}
}

func TestWalk_DefaultsActions_Inherited(t *testing.T) {
	root := fixtureRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// shellrc/base.sh should inherit source action from shellrc/.dagger defaults
	n := nodeByLogicalName(nodes, "shellrc.base")
	if n == nil {
		t.Fatal("expected node shellrc.base")
	}
	if !hasAction(n.Actions, ActionSource) {
		t.Errorf("shellrc.base should have source action, got %v", n.Actions)
	}
}

func TestWalk_FileAnnotation_When(t *testing.T) {
	root := fixtureRoot(t)
	nodes, _, err := Walk(root)
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
	nodes, _, err := Walk(root)
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
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	// conf/dot-gitconfig has @link(~/.gitconfig) annotation
	n := nodeByLogicalName(nodes, "conf.gitconfig")
	if n == nil {
		t.Fatal("expected node conf.gitconfig")
	}
	if !hasActionWithDest(n.Actions, ActionLink, "~/.gitconfig") {
		t.Errorf("conf.gitconfig should have link(~/.gitconfig) action, got %v", n.Actions)
	}
}

func TestWalk_ConfdirLinkRoot(t *testing.T) {
	root := fixtureRoot(t)
	nodes, _, err := Walk(root)
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
	nodes, _, err := Walk(root)
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

func TestWalk_SkipsGitDir(t *testing.T) {
	root := t.TempDir()
	// Create a real file dotd should see.
	shellrc := filepath.Join(root, "shellrc")
	if err := os.MkdirAll(shellrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrc, "base.sh"), []byte("# @action source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create .git with a file that would conflict if walked.
	gitObjects := filepath.Join(root, ".git", "objects")
	if err := os.MkdirAll(gitObjects, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitObjects, "something"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if filepath.Base(filepath.Dir(n.Path)) == ".git" || filepath.Base(n.Path) == ".git" {
			t.Errorf("Walk returned node inside .git: %s", n.Path)
		}
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

// TestParseDaggerActions exercises parseDaggerActions directly, covering the
// zero-coverage path identified in AUDIT-037.
func TestParseDaggerActions(t *testing.T) {
	cases := []struct {
		name   string
		input  []string
		checks func(t *testing.T, actions []Action)
	}{
		{
			name:  "empty input returns nil",
			input: []string{},
			checks: func(t *testing.T, actions []Action) {
				if len(actions) != 0 {
					t.Errorf("expected empty, got %v", actions)
				}
			},
		},
		{
			name:  "source action",
			input: []string{"source"},
			checks: func(t *testing.T, actions []Action) {
				if !hasAction(actions, ActionSource) {
					t.Errorf("expected source action, got %v", actions)
				}
			},
		},
		{
			name:  "compose action string",
			input: []string{"compose"},
			checks: func(t *testing.T, actions []Action) {
				if !hasAction(actions, ActionCompose) {
					t.Errorf("expected compose action, got %v", actions)
				}
			},
		},
		{
			name:  "no-source action",
			input: []string{"no-source"},
			checks: func(t *testing.T, actions []Action) {
				if !hasAction(actions, ActionNoSource) {
					t.Errorf("expected no-source action, got %v", actions)
				}
			},
		},
		{
			name:  "link with destination",
			input: []string{"link(~/.tmux.conf)"},
			checks: func(t *testing.T, actions []Action) {
				if !hasActionWithDest(actions, ActionLink, "~/.tmux.conf") {
					t.Errorf("expected link(~/.tmux.conf), got %v", actions)
				}
			},
		},
		{
			name:  "unknown action is ignored",
			input: []string{"bogus"},
			checks: func(t *testing.T, actions []Action) {
				if len(actions) != 0 {
					t.Errorf("expected unknown action to be ignored, got %v", actions)
				}
			},
		},
		{
			name:  "blank string is ignored",
			input: []string{""},
			checks: func(t *testing.T, actions []Action) {
				if len(actions) != 0 {
					t.Errorf("expected blank to be ignored, got %v", actions)
				}
			},
		},
		{
			name:  "mixed actions",
			input: []string{"source", "link(~/bin/tool)", "no-source"},
			checks: func(t *testing.T, actions []Action) {
				if !hasAction(actions, ActionSource) {
					t.Errorf("missing source action in %v", actions)
				}
				if !hasActionWithDest(actions, ActionLink, "~/bin/tool") {
					t.Errorf("missing link action in %v", actions)
				}
				if !hasAction(actions, ActionNoSource) {
					t.Errorf("missing no-source action in %v", actions)
				}
				if len(actions) != 3 {
					t.Errorf("expected 3 actions, got %d: %v", len(actions), actions)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actions := parseDaggerActions(c.input)
			c.checks(t, actions)
		})
	}
}

// TestWalk_ComposeTarget_EmitsDirectoryNode verifies that Walk emits a compose-target
// directory node (IsCompose=false, ActionCompose present) for dirs with
// composition.enabled: true, and emits fragment file nodes with IsCompose=true.
func TestWalk_ComposeTarget_EmitsDirectoryNode(t *testing.T) {
	root := fixtureRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	// Find the compose-target directory node for dot-tmux.conf.d.
	var targetNode *RawNode
	var fragNodes []RawNode
	for i := range nodes {
		n := &nodes[i]
		if filepath.Base(n.Path) == "dot-tmux.conf.d" {
			targetNode = n
		}
		if n.IsCompose && n.ComposeTarget != "" && filepath.Base(n.ComposeTarget) == "dot-tmux.conf.d" {
			fragNodes = append(fragNodes, *n)
		}
	}

	if targetNode == nil {
		names := make([]string, len(nodes))
		for i, n := range nodes {
			names[i] = n.LogicalName
		}
		t.Fatalf("expected compose-target node for dot-tmux.conf.d, got nodes: %v", names)
	}
	if targetNode.IsCompose {
		t.Error("compose-target directory node should have IsCompose=false")
	}
	if !hasAction(targetNode.Actions, ActionCompose) {
		t.Errorf("compose-target node should have ActionCompose, got %v", targetNode.Actions)
	}
	if !hasActionWithDest(targetNode.Actions, ActionLink, "~/.tmux.conf") {
		t.Errorf("compose-target node should have link(~/.tmux.conf), got %v", targetNode.Actions)
	}

	if len(fragNodes) == 0 {
		t.Error("expected at least one compose fragment node for dot-tmux.conf.d")
	}
	for _, f := range fragNodes {
		if f.ComposeTarget == "" {
			t.Errorf("fragment %s has empty ComposeTarget", f.Path)
		}
	}
}
