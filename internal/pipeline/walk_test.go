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

// filesDictRoot returns the absolute path to the files-dict fixture directory.
func filesDictRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/files-dict")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// nodeByPath returns the first RawNode whose Path base matches the given filename, or nil.
func nodeByPath(nodes []RawNode, filename string) *RawNode {
	for i := range nodes {
		if filepath.Base(nodes[i].Path) == filename {
			return &nodes[i]
		}
	}
	return nil
}

// TestWalk_FilesDict_Disable checks that a file with disable: true in the files: dict
// appears in the disabled slice and NOT in nodes.
func TestWalk_FilesDict_Disable(t *testing.T) {
	root := filesDictRoot(t)
	nodes, disabled, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	// Must NOT appear in nodes.
	if n := nodeByPath(nodes, "disabled-file.json"); n != nil {
		t.Errorf("disabled-file.json should not appear in nodes, but got node: %+v", n)
	}

	// Must appear in disabled.
	found := false
	for _, p := range disabled {
		if filepath.Base(p) == "disabled-file.json" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("disabled-file.json should appear in disabled slice, got: %v", disabled)
	}
}

// TestWalk_AtDisable_AnnotationPath checks that a file carrying a # @disable annotation
// (walk.go:188-191) is absent from nodes AND present in the disabled return slice.
// This exercises the inline annotation code path, distinct from the files-dict
// disable: true path tested by TestWalk_FilesDict_Disable.
func TestWalk_AtDisable_AnnotationPath(t *testing.T) {
	root := fixtureRoot(t)
	nodes, disabled, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	// shellrc/disabled.sh carries # @disable — must NOT appear in nodes.
	if n := nodeByPath(nodes, "disabled.sh"); n != nil {
		t.Errorf("disabled.sh should not appear in nodes (has @disable annotation), got node: %+v", n)
	}

	// Must appear in the disabled return slice.
	found := false
	for _, p := range disabled {
		if filepath.Base(p) == "disabled.sh" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("disabled.sh should appear in disabled slice (has @disable annotation), got: %v", disabled)
	}
}

// TestWalk_FilesDict_NameOverride checks that name: in the files: dict overrides the
// derived logical name.
func TestWalk_FilesDict_NameOverride(t *testing.T) {
	root := filesDictRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	n := nodeByPath(nodes, "named-file.json")
	if n == nil {
		names := make([]string, len(nodes))
		for i, nd := range nodes {
			names[i] = nd.LogicalName
		}
		t.Fatalf("named-file.json not found in nodes; got: %v", names)
	}
	if n.LogicalName != "my.custom.name" {
		t.Errorf("LogicalName = %q, want %q", n.LogicalName, "my.custom.name")
	}
}

// TestWalk_FilesDict_WhenCascade checks that per-file when: is combined with the
// directory-level defaults.when via AND.
func TestWalk_FilesDict_WhenCascade(t *testing.T) {
	root := filesDictRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	n := nodeByPath(nodes, "when-file.json")
	if n == nil {
		t.Fatal("when-file.json not found in nodes")
	}
	// defaults.when = "os=linux" → state.when = "(os=linux)"
	// file when = "shell=bash"  → fileWhen = "(shell=bash)"
	// combined → "(os=linux) AND (shell=bash)"
	want := "(os=linux) AND (shell=bash)"
	if n.EffectiveWhen != want {
		t.Errorf("EffectiveWhen = %q, want %q", n.EffectiveWhen, want)
	}
}

// TestWalk_FilesDict_Actions checks that actions listed in the files: dict entry
// are emitted as the node's actions.
func TestWalk_FilesDict_Actions(t *testing.T) {
	root := filesDictRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	n := nodeByPath(nodes, "actions-file.json")
	if n == nil {
		t.Fatal("actions-file.json not found in nodes")
	}
	if !hasAction(n.Actions, ActionSource) {
		t.Errorf("actions-file.json should have source action, got %v", n.Actions)
	}
}

// TestWalk_FilesDict_DedupByType checks that duplicate action types in the files: dict
// entry are collapsed so only one action per type survives.
func TestWalk_FilesDict_DedupByType(t *testing.T) {
	root := filesDictRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	n := nodeByPath(nodes, "dedup-file.json")
	if n == nil {
		t.Fatal("dedup-file.json not found in nodes")
	}
	// actions: [source, source] → only one source action should survive.
	count := 0
	for _, a := range n.Actions {
		if a.Type == ActionSource {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 source action after dedup, got %d: %v", count, n.Actions)
	}
}

// TestWalk_FilesDict_After checks that after: in the files: dict entry is propagated
// to the node's After slice.
func TestWalk_FilesDict_After(t *testing.T) {
	root := filesDictRoot(t)
	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	n := nodeByPath(nodes, "after-file.json")
	if n == nil {
		t.Fatal("after-file.json not found in nodes")
	}
	found := false
	for _, dep := range n.After {
		if dep == "some.other.file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("after-file.json After should contain %q, got: %v", "some.other.file", n.After)
	}
}

// TestWalk_FilesDict_MissingFileSkip checks that a file listed in the files: dict
// but absent on disk is silently skipped — it appears neither in nodes nor in disabled.
func TestWalk_FilesDict_MissingFileSkip(t *testing.T) {
	root := filesDictRoot(t)
	nodes, disabled, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}

	// Confirm the file genuinely does not exist on disk.
	missingPath := filepath.Join(root, "missing-file.json")
	if _, statErr := os.Stat(missingPath); statErr == nil {
		t.Fatal("test setup error: missing-file.json exists on disk; it should be absent")
	}

	// Must NOT appear in nodes.
	if n := nodeByPath(nodes, "missing-file.json"); n != nil {
		t.Errorf("missing-file.json should not appear in nodes, but got: %+v", n)
	}

	// Must NOT appear in disabled.
	for _, p := range disabled {
		if filepath.Base(p) == "missing-file.json" {
			t.Errorf("missing-file.json should not appear in disabled, but found: %s", p)
		}
	}
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
			// After unification with parseActionString, bare unknown types pass through
			// rather than being silently dropped. Downstream (mergeActions/Act) ignores
			// types it doesn't recognise, so this is harmless.
			name:  "unknown bare action passes through as Action{Type}",
			input: []string{"bogus"},
			checks: func(t *testing.T, actions []Action) {
				if len(actions) != 1 || actions[0].Type != "bogus" {
					t.Errorf("expected Action{Type:\"bogus\"}, got %v", actions)
				}
			},
		},
		{
			// Arbitrary type(dest) syntax previously silently dropped at dir level;
			// after unification it is parsed and emitted (AUDIT-013).
			name:  "arbitrary type(dest) is parsed and emitted",
			input: []string{"custom(arg)"},
			checks: func(t *testing.T, actions []Action) {
				if len(actions) != 1 || actions[0].Type != "custom" || actions[0].Dest != "arg" {
					t.Errorf("expected Action{Type:\"custom\", Dest:\"arg\"}, got %v", actions)
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

	// dot-tmux.conf.d contains exactly 2 fragment files: base.conf and nosync-work.conf.
	if len(fragNodes) != 2 {
		t.Errorf("expected 2 fragment nodes for dot-tmux.conf.d, got %d", len(fragNodes))
	}
	for _, f := range fragNodes {
		if f.ComposeTarget == "" {
			t.Errorf("fragment %s has empty ComposeTarget", f.Path)
		}
	}
}

// --- 2026-06-13 audit regressions (B5, B8) ---

// writeTestFile writes content to root/rel, creating parent dirs.
func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWalk_ComposeDirWhen_ParenWrapped: the compose dir's own when: must be
// paren-wrapped before AND-joining with the cascade when — AND binds tighter
// than OR, so an unwrapped "a=1 OR b=2" would regroup as
// (cascade AND a=1) OR b=2.
func TestWalk_ComposeDirWhen_ParenWrapped(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".dagger", "defaults:\n  when: \"c=3\"\n")
	writeTestFile(t, root, "target.d/.dagger",
		"composition:\n  enabled: true\nwhen: \"a=1 OR b=2\"\nactions:\n  - source\n")
	writeTestFile(t, root, "target.d/frag.sh", "x=1\n")

	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	n := nodeByPath(nodes, "target.d")
	if n == nil {
		t.Fatal("compose-target node not found")
	}
	want := "(c=3) AND (a=1 OR b=2)"
	if n.EffectiveWhen != want {
		t.Errorf("EffectiveWhen = %q, want %q", n.EffectiveWhen, want)
	}
}

// TestWalk_FilesDict_LinkRootDir: files:-dict nodes must carry LinkRootDir
// alongside LinkRoot — deriveLinkDest returns "" without it, so an entry
// relying on an inherited link_root silently produced no link.
func TestWalk_FilesDict_LinkRootDir(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".dagger",
		"link_root: \"~/cfg\"\nfiles:\n  blob.bin:\n    actions:\n      - link\n")
	writeTestFile(t, root, "blob.bin", "\x00")

	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	n := nodeByPath(nodes, "blob.bin")
	if n == nil {
		t.Fatal("blob.bin not found in nodes")
	}
	if n.LinkRoot != "~/cfg" {
		t.Errorf("LinkRoot = %q, want %q", n.LinkRoot, "~/cfg")
	}
	if n.LinkRootDir != root {
		t.Errorf("LinkRootDir = %q, want %q", n.LinkRootDir, root)
	}
	if got, want := deriveLinkDest(*n), filepath.Join("~/cfg", "blob.bin"); got != want {
		t.Errorf("deriveLinkDest = %q, want %q", got, want)
	}
}

// TestWalk_FilesDict_ComposeFragment: a files:-dict entry inside a compose
// dir is a fragment, same as an annotatable file in that dir.
func TestWalk_FilesDict_ComposeFragment(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "gen.sh.d/.dagger",
		"composition:\n  enabled: true\nactions:\n  - source\nfiles:\n  data.bin:\n    name: data\n")
	writeTestFile(t, root, "gen.sh.d/data.bin", "\x00")

	nodes, _, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	n := nodeByPath(nodes, "data.bin")
	if n == nil {
		t.Fatal("data.bin not found in nodes")
	}
	if !n.IsCompose {
		t.Error("files:-dict entry inside compose dir must have IsCompose=true")
	}
	if filepath.Base(n.ComposeTarget) != "gen.sh.d" {
		t.Errorf("ComposeTarget = %q, want .../gen.sh.d", n.ComposeTarget)
	}
}
