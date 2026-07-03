package pipeline

import (
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
)

func TestNormalizeActionAnnotations(t *testing.T) {
	tests := []struct {
		name  string
		input []annotation.Annotation
		want  []annotation.Annotation
	}{
		{
			name:  "source becomes action source",
			input: []annotation.Annotation{{Key: "source", Line: 2}},
			want:  []annotation.Annotation{{Key: "action", Args: "source", Line: 2}},
		},
		{
			name:  "no-source becomes action no-source",
			input: []annotation.Annotation{{Key: "no-source", Line: 3}},
			want:  []annotation.Annotation{{Key: "action", Args: "no-source", Line: 3}},
		},
		{
			name:  "link(dest) becomes action link(dest)",
			input: []annotation.Annotation{{Key: "link", Args: "~/.gitconfig", Line: 4}},
			want:  []annotation.Annotation{{Key: "action", Args: "link(~/.gitconfig)", Line: 4}},
		},
		{
			name:  "symlink(dest) becomes action link(dest)",
			input: []annotation.Annotation{{Key: "symlink", Args: "~/.gitconfig", Line: 5}},
			want:  []annotation.Annotation{{Key: "action", Args: "link(~/.gitconfig)", Line: 5}},
		},
		{
			name:  "action passes through unchanged",
			input: []annotation.Annotation{{Key: "action", Args: "source", Line: 6}},
			want:  []annotation.Annotation{{Key: "action", Args: "source", Line: 6}},
		},
		{
			name:  "action link passes through unchanged",
			input: []annotation.Annotation{{Key: "action", Args: "link(~/dest)", Line: 7}},
			want:  []annotation.Annotation{{Key: "action", Args: "link(~/dest)", Line: 7}},
		},
		{
			name:  "when annotation passes through unchanged",
			input: []annotation.Annotation{{Key: "when", Args: "os=darwin", Line: 8}},
			want:  []annotation.Annotation{{Key: "when", Args: "os=darwin", Line: 8}},
		},
		{
			name:  "after annotation passes through unchanged",
			input: []annotation.Annotation{{Key: "after", Args: "foo", Line: 9}},
			want:  []annotation.Annotation{{Key: "after", Args: "foo", Line: 9}},
		},
		{
			name: "mixed annotations: action and non-action",
			input: []annotation.Annotation{
				{Key: "when", Args: "os=darwin", Line: 1},
				{Key: "source", Line: 2},
				{Key: "after", Args: "base", Line: 3},
			},
			want: []annotation.Annotation{
				{Key: "when", Args: "os=darwin", Line: 1},
				{Key: "action", Args: "source", Line: 2},
				{Key: "after", Args: "base", Line: 3},
			},
		},
		{
			name:  "empty slice returns empty slice",
			input: []annotation.Annotation{},
			want:  []annotation.Annotation{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeActionAnnotations(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d, len(want)=%d", len(got), len(tc.want))
			}
			for i, a := range got {
				w := tc.want[i]
				if a.Key != w.Key || a.Args != w.Args || a.Line != w.Line {
					t.Errorf("ann[%d]: got {Key:%q Args:%q Line:%d}, want {Key:%q Args:%q Line:%d}",
						i, a.Key, a.Args, a.Line, w.Key, w.Args, w.Line)
				}
			}
		})
	}
}

func TestMergeActions_CanonicalAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		defaults []string
		anns     []annotation.Annotation // must be pre-normalized
		want     []Action
	}{
		{
			name:     "inherited source default, no annotation",
			defaults: []string{ActionSource},
			anns:     nil,
			want:     []Action{{Type: ActionSource}},
		},
		{
			name:     "action source annotation adds source",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "source"}},
			want:     []Action{{Type: ActionSource}},
		},
		{
			name:     "action link annotation adds link",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "link(~/.gitconfig)"}},
			want:     []Action{{Type: ActionLink, Dest: "~/.gitconfig"}},
		},
		{
			name:     "inherited source suppressed by action no-source",
			defaults: []string{ActionSource},
			anns:     []annotation.Annotation{{Key: "action", Args: "no-source"}},
			want:     []Action{{Type: ActionNoSource}},
		},
		{
			name:     "action no-source without inherited source still adds no-source",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "no-source"}},
			want:     []Action{{Type: ActionNoSource}},
		},
		{
			name:     "action link replaces inherited link dest",
			defaults: []string{"link(~/old-dest)"},
			anns:     []annotation.Annotation{{Key: "action", Args: "link(~/new-dest)"}},
			want:     []Action{{Type: ActionLink, Dest: "~/new-dest"}},
		},
		{
			name:     "non-action annotation ignored by mergeActions",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "when", Args: "os=darwin"}},
			want:     nil,
		},
		{
			name:     "action source and action link both applied",
			defaults: nil,
			anns: []annotation.Annotation{
				{Key: "action", Args: "source"},
				{Key: "action", Args: "link(~/.gitconfig)"},
			},
			want: []Action{{Type: ActionSource}, {Type: ActionLink, Dest: "~/.gitconfig"}},
		},
		// AUDIT-055: two-explicit-link merge handoff
		{
			name:     "two explicit link annotations with different dests — both kept for validateNode",
			defaults: nil,
			anns: []annotation.Annotation{
				{Key: "action", Args: "link(~/foo)"},
				{Key: "action", Args: "link(~/bar)"},
			},
			// Both must be retained so validateNode can flag the conflict.
			want: []Action{
				{Type: ActionLink, Dest: "~/foo"},
				{Type: ActionLink, Dest: "~/bar"},
			},
		},
		{
			name:     "two explicit link annotations with same dest — deduped to one",
			defaults: nil,
			anns: []annotation.Annotation{
				{Key: "action", Args: "link(~/foo)"},
				{Key: "action", Args: "link(~/foo)"},
			},
			want: []Action{{Type: ActionLink, Dest: "~/foo"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeActions(tc.defaults, tc.anns)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d want=%d; got=%v want=%v", len(got), len(tc.want), got, tc.want)
			}
			for i, a := range got {
				w := tc.want[i]
				if a.Type != w.Type || a.Dest != w.Dest {
					t.Errorf("action[%d]: got {Type:%q Dest:%q}, want {Type:%q Dest:%q}",
						i, a.Type, a.Dest, w.Type, w.Dest)
				}
			}
		})
	}
}

func TestValidateAnchor(t *testing.T) {
	ok := []string{"", "~", "~/.zshrc", "$bin", "$bin/fmt", "$config", "$config/nvim", "/abs", "rel/path"}
	for _, v := range ok {
		if err := validateAnchor("link_root", v); err != nil {
			t.Errorf("validateAnchor(%q) = %v, want nil", v, err)
		}
	}
	bad := []string{"~bin", "~config", "$conifg", "$HOME", "$binary", "~root/x"}
	for _, v := range bad {
		if err := validateAnchor("link_root", v); err == nil {
			t.Errorf("validateAnchor(%q) = nil, want error", v)
		}
	}
}

func TestValidateNodes(t *testing.T) {
	// dirNode returns a RawNode that looks like a compose-target directory (no actions yet).
	dirNode := func(name string) RawNode {
		return RawNode{
			Path:          "/dotfiles/" + name,
			LogicalName:   name,
			ComposeTarget: "/dotfiles/" + name, // ComposeTarget == Path → directory
		}
	}
	// withActions clones n and sets its Actions slice.
	withActions := func(n RawNode, actions []Action) RawNode {
		n.Actions = actions
		return n
	}
	// fileNode returns a RawNode that looks like a regular file.
	fileNode := func(name string, actions []Action) RawNode {
		return RawNode{
			Path:        "/dotfiles/" + name,
			LogicalName: name,
			Actions:     actions,
			// ComposeTarget is empty → not a directory
		}
	}

	tests := []struct {
		name    string
		nodes   []RawNode
		opts    ActOptions
		wantErr bool
		errMsg  string
	}{
		// --- error cases ---
		{
			name:    "compose on file is an error",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: ActionCompose}})},
			wantErr: true,
			errMsg:  "compose is only valid on directories",
		},
		{
			name:    "link without dest and no link_root is an error",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: ActionLink, Dest: ""}})},
			wantErr: true,
			errMsg:  "link requires a destination",
		},
		{
			name: "link before compose on dir is an error",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: ActionLink, Dest: "~/.zshrc"},
					{Type: ActionCompose},
				}),
			},
			wantErr: true,
			errMsg:  "link/source must follow compose",
		},
		{
			name: "source before compose on dir is an error",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: ActionSource},
					{Type: ActionCompose},
				}),
			},
			wantErr: true,
			errMsg:  "link/source must follow compose",
		},
		{
			name: "conflicting link destinations is an error",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: ActionCompose},
					{Type: ActionLink, Dest: "~/.zshrc"},
					{Type: ActionLink, Dest: "~/.bashrc"},
				}),
			},
			wantErr: true,
			errMsg:  "conflicting link destinations",
		},
		// anchor validation through ValidateNodes
		{
			name: "node link_root with invalid anchor is an error",
			nodes: []RawNode{{
				Path:        "/dotfiles/conf/dot-gitconfig",
				LogicalName: "conf.dot-gitconfig",
				LinkRoot:    "~bin",
				LinkRootDir: "/dotfiles/conf",
				Actions:     []Action{{Type: ActionLink, Dest: ""}},
			}},
			wantErr: true,
			errMsg:  "unknown anchor token",
		},
		{
			name:    "link destination with invalid anchor is an error",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: ActionLink, Dest: "$typo"}})},
			wantErr: true,
			errMsg:  "unknown anchor token",
		},
		// --- valid cases ---
		{
			name:    "empty nodes slice is valid",
			nodes:   nil,
			wantErr: false,
		},
		{
			name:    "file with source only is valid",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: ActionSource}})},
			wantErr: false,
		},
		{
			name:    "file with link is valid",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: ActionLink, Dest: "~/.foo"}})},
			wantErr: false,
		},
		{
			name:    "file with no actions is valid",
			nodes:   []RawNode{fileNode("foo.sh", nil)},
			wantErr: false,
		},
		{
			name: "link without dest but with link_root is valid",
			nodes: []RawNode{{
				Path:        "/dotfiles/conf/dot-gitconfig",
				LogicalName: "conf.dot-gitconfig",
				LinkRoot:    "~",
				LinkRootDir: "/dotfiles/conf",
				Actions:     []Action{{Type: ActionLink, Dest: ""}},
			}},
			wantErr: false,
		},
		{
			name: "compose fragment with inherited link is valid",
			nodes: []RawNode{{
				Path:          "/dotfiles/conf/dot-tmux.conf.d/base",
				LogicalName:   "conf.dot-tmux.conf.d.base",
				IsCompose:     true,
				ComposeTarget: "/dotfiles/conf/dot-tmux.conf.d",
				LinkRoot:      "~",
				Actions:       []Action{{Type: ActionLink, Dest: ""}},
			}},
			wantErr: false,
		},
		{
			name: "dir with compose then link is valid",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: ActionCompose},
					{Type: ActionLink, Dest: "~/.zshrc"},
				}),
			},
			wantErr: false,
		},
		{
			name: "dir with compose then source then link is valid",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: ActionCompose},
					{Type: ActionSource},
					{Type: ActionLink, Dest: "~/.zshrc"},
				}),
			},
			wantErr: false,
		},
		{
			name: "multiple errors are all reported",
			nodes: []RawNode{
				fileNode("a.sh", []Action{{Type: ActionCompose}}),
				fileNode("b.sh", []Action{{Type: ActionLink, Dest: ""}}),
			},
			wantErr: true,
			errMsg:  "compose is only valid on directories",
		},
		// Cross-node link conflict: ~ vs absolute path resolve to the same dest.
		{
			name: "cross-node link conflict via ~ vs absolute path",
			nodes: []RawNode{
				fileNode("a", []Action{{Type: ActionLink, Dest: "~/.x"}}),
				fileNode("b", []Action{{Type: ActionLink, Dest: "/home/user/.x"}}),
			},
			opts:    ActOptions{HomeDir: "/home/user"},
			wantErr: true,
			errMsg:  "link conflict",
		},
		// Same cross-node conflict without HomeDir: raw strings differ, so no error.
		{
			name: "cross-node ~ vs absolute — no HomeDir, raw strings differ, no conflict",
			nodes: []RawNode{
				fileNode("a", []Action{{Type: ActionLink, Dest: "~/.x"}}),
				fileNode("b", []Action{{Type: ActionLink, Dest: "/home/user/.x"}}),
			},
			opts:    ActOptions{},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNodes(tc.nodes, tc.opts)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr && tc.errMsg != "" {
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
			}
		})
	}
}

// TestCheckLinkConflicts_EmptyDestNoFalseConflict verifies that two nodes whose
// link destinations resolve to empty (link action with no dest and no link_root)
// do NOT trigger a bogus "both link to the empty string" conflict. Such nodes fail validateNode
// separately ("link requires a destination"); CheckLinkConflicts must not pile a
// false conflict on top by mapping them all to the empty string.
func TestCheckLinkConflicts_EmptyDestNoFalseConflict(t *testing.T) {
	n1 := RawNode{LogicalName: "a", Actions: []Action{{Type: ActionLink, Dest: ""}}}
	n2 := RawNode{LogicalName: "b", Actions: []Action{{Type: ActionLink, Dest: ""}}}
	if err := CheckLinkConflicts([]RawNode{n1, n2}, ActOptions{HomeDir: "/home/u"}); err != nil {
		t.Errorf("empty-dest nodes should not conflict, got: %v", err)
	}
}
