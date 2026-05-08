package annotation

import (
	"strings"
	"testing"
)

func TestScan_HashComment(t *testing.T) {
	anns, err := Scan(strings.NewReader("# @when(os=macos)"))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "when" || anns[0].Args != "os=macos" || anns[0].Line != 1 {
		t.Errorf("got %+v", anns)
	}
}

func TestScan_SlashComment(t *testing.T) {
	anns, err := Scan(strings.NewReader("// @name(my.name)"))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "name" || anns[0].Args != "my.name" {
		t.Errorf("got %+v", anns)
	}
}

func TestScan_ZeroArgAnnotations(t *testing.T) {
	input := "# @source\n# @no-source\n# @disable"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 3 {
		t.Fatalf("want 3 annotations, got %d: %+v", len(anns), anns)
	}
	if anns[0].Key != "source" || anns[0].Args != "" {
		t.Errorf("anns[0] = %+v", anns[0])
	}
	if anns[1].Key != "no-source" || anns[1].Args != "" {
		t.Errorf("anns[1] = %+v", anns[1])
	}
	if anns[2].Key != "disable" || anns[2].Args != "" {
		t.Errorf("anns[2] = %+v", anns[2])
	}
}

func TestScan_ShebangSkipped(t *testing.T) {
	input := "#!/bin/bash\n# @when(os=macos)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "when" || anns[0].Line != 2 {
		t.Errorf("got %+v", anns)
	}
}

func TestScan_BlankLinePermitted(t *testing.T) {
	input := "# @when(os=macos)\n\n# @after(shellrc.base)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 2 {
		t.Fatalf("blank line should not stop scan, want 2 annotations, got %d: %+v", len(anns), anns)
	}
}

func TestScan_NonCommentStops(t *testing.T) {
	input := "# @when(os=macos)\nexport FOO=bar\n# @after(shellrc.base)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 {
		t.Errorf("non-comment line should stop scan, want 1 annotation, got %d: %+v", len(anns), anns)
	}
}

func TestScan_NonAtCommentIgnored(t *testing.T) {
	input := "# just a comment\n# @when(os=macos)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "when" {
		t.Errorf("non-@ comment should be ignored, got %+v", anns)
	}
}

func TestScan_MultipleAnnotations(t *testing.T) {
	input := "# @when(os=macos)\n# @after(shellrc.base)\n# @link(~/.tmux.conf)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []Annotation{
		{Key: "when", Args: "os=macos", Line: 1},
		{Key: "after", Args: "shellrc.base", Line: 2},
		{Key: "link", Args: "~/.tmux.conf", Line: 3},
	}
	if len(anns) != len(want) {
		t.Fatalf("want %d, got %d: %+v", len(want), len(anns), anns)
	}
	for i := range want {
		if anns[i] != want[i] {
			t.Errorf("anns[%d] = %+v, want %+v", i, anns[i], want[i])
		}
	}
}

func TestScan_EmptyInput(t *testing.T) {
	anns, err := Scan(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 0 {
		t.Errorf("want 0 annotations, got %+v", anns)
	}
}

func TestScan_PathPrefix(t *testing.T) {
	input := "# @after(shellrc/)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "after" || anns[0].Args != "shellrc/" {
		t.Errorf("got %+v", anns)
	}
}

func TestGet(t *testing.T) {
	anns := []Annotation{
		{Key: "when", Args: "os=macos"},
		{Key: "name", Args: "foo"},
		{Key: "when", Args: "context=work"},
	}
	got := Get(anns, "when")
	if len(got) != 2 {
		t.Fatalf("Get() len = %d, want 2", len(got))
	}
	if got[0].Args != "os=macos" || got[1].Args != "context=work" {
		t.Errorf("Get() = %+v, unexpected values", got)
	}
}

func TestFirst(t *testing.T) {
	anns := []Annotation{
		{Key: "when", Args: "os=macos"},
		{Key: "name", Args: "foo"},
	}
	got, ok := First(anns, "name")
	if !ok {
		t.Fatal("First() ok = false, want true")
	}
	if got.Args != "foo" {
		t.Errorf("First().Args = %q, want %q", got.Args, "foo")
	}
	_, ok = First(anns, "after")
	if ok {
		t.Error("First() ok = true for missing key, want false")
	}
}

func TestCombineWhen(t *testing.T) {
	tests := []struct {
		name string
		anns []Annotation
		want string
	}{
		{
			name: "single when",
			anns: []Annotation{{Key: "when", Args: "os=macos"}},
			want: "(os=macos)",
		},
		{
			name: "multiple when",
			anns: []Annotation{
				{Key: "when", Args: "os=macos OR os=linux"},
				{Key: "when", Args: "context=work"},
			},
			want: "(os=macos OR os=linux) AND (context=work)",
		},
		{
			name: "no when",
			anns: []Annotation{{Key: "name", Args: "foo"}},
			want: "",
		},
		{
			name: "empty when args ignored",
			anns: []Annotation{{Key: "when", Args: ""}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CombineWhen(tt.anns)
			if got != tt.want {
				t.Errorf("CombineWhen() = %q, want %q", got, tt.want)
			}
		})
	}
}
