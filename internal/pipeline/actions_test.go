package pipeline

import (
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
			defaults: []string{"source"},
			anns:     nil,
			want:     []Action{{Type: "source"}},
		},
		{
			name:     "action source annotation adds source",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "source"}},
			want:     []Action{{Type: "source"}},
		},
		{
			name:     "action link annotation adds link",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "link(~/.gitconfig)"}},
			want:     []Action{{Type: "link", Dest: "~/.gitconfig"}},
		},
		{
			name:     "inherited source suppressed by action no-source",
			defaults: []string{"source"},
			anns:     []annotation.Annotation{{Key: "action", Args: "no-source"}},
			want:     []Action{{Type: "no-source"}},
		},
		{
			name:     "action no-source without inherited source still adds no-source",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "no-source"}},
			want:     []Action{{Type: "no-source"}},
		},
		{
			name:     "action link replaces inherited link dest",
			defaults: []string{"link(~/old-dest)"},
			anns:     []annotation.Annotation{{Key: "action", Args: "link(~/new-dest)"}},
			want:     []Action{{Type: "link", Dest: "~/new-dest"}},
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
			want: []Action{{Type: "source"}, {Type: "link", Dest: "~/.gitconfig"}},
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
