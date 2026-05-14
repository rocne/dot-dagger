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
