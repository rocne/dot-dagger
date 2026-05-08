package dag

import (
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
)

func node(name string, afters ...string) fileset.Node {
	var anns []annotation.Annotation
	for _, a := range afters {
		anns = append(anns, annotation.Annotation{Key: "after", Args:a})
	}
	return fileset.Node{LogicalName: name, Annotations: anns}
}

func names(nodes []fileset.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.LogicalName
	}
	return out
}

func TestBuildEmpty(t *testing.T) {
	got, err := Build(nil)
	if err != nil {
		t.Fatalf("Build(nil) error = %v", err)
	}
	if got != nil {
		t.Errorf("Build(nil) = %v, want nil", got)
	}
}

func TestBuildSingle(t *testing.T) {
	got, err := Build([]fileset.Node{node("a")})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(got) != 1 || got[0].LogicalName != "a" {
		t.Errorf("got %v, want [a]", names(got))
	}
}

func TestBuildAlphabeticalTieBreak(t *testing.T) {
	nodes := []fileset.Node{node("c"), node("a"), node("b")}
	got, err := Build(nodes)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if got[i].LogicalName != w {
			t.Errorf("pos %d = %q, want %q", i, got[i].LogicalName, w)
		}
	}
}

func TestBuildLinearChain(t *testing.T) {
	// c after b, b after a — must emit a, b, c.
	nodes := []fileset.Node{
		node("c", "b"),
		node("b", "a"),
		node("a"),
	}
	got, err := Build(nodes)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if got[i].LogicalName != w {
			t.Errorf("pos %d = %q, want %q", i, got[i].LogicalName, w)
		}
	}
}

func TestBuildDiamondOrdering(t *testing.T) {
	// a → b, a → c → d; b → d
	nodes := []fileset.Node{
		node("d", "b", "c"),
		node("c", "a"),
		node("b", "a"),
		node("a"),
	}
	got, err := Build(nodes)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	// a must be first, d must be last.
	if got[0].LogicalName != "a" {
		t.Errorf("first = %q, want a", got[0].LogicalName)
	}
	if got[len(got)-1].LogicalName != "d" {
		t.Errorf("last = %q, want d", got[len(got)-1].LogicalName)
	}
}

func TestBuildCycleDetected(t *testing.T) {
	nodes := []fileset.Node{
		node("a", "b"),
		node("b", "a"),
	}
	_, err := Build(nodes)
	if err == nil {
		t.Error("Build() error = nil, want cycle error")
	}
}

func TestBuildDuplicateLogicalName(t *testing.T) {
	nodes := []fileset.Node{
		{LogicalName: "dup", Path: "/a.sh"},
		{LogicalName: "dup", Path: "/b.sh"},
	}
	_, err := Build(nodes)
	if err == nil {
		t.Error("Build() error = nil, want conflict error")
	}
}

func TestBuildMissingAfterTargetIgnored(t *testing.T) {
	// @after referencing nonexistent name: treated as no-op.
	nodes := []fileset.Node{
		node("b", "missing"),
		node("a"),
	}
	got, err := Build(nodes)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestBuildSelfReferenceIgnored(t *testing.T) {
	nodes := []fileset.Node{node("a", "a")}
	got, err := Build(nodes)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestBuildPathPrefixAfter(t *testing.T) {
	// "tmux/" prefix should match all nodes with logical names starting "tmux."
	nodes := []fileset.Node{
		{LogicalName: "base", Annotations: []annotation.Annotation{
			{Key: "after", Args: "tmux/"},
		}},
		{LogicalName: "tmux.a"},
		{LogicalName: "tmux.b"},
	}
	got, err := Build(nodes)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	// base must come after tmux.a and tmux.b.
	basePos := -1
	for i, n := range got {
		if n.LogicalName == "base" {
			basePos = i
		}
	}
	if basePos < 2 {
		t.Errorf("base at pos %d, want >= 2", basePos)
	}
}

func TestResolveAfterPathPrefix(t *testing.T) {
	nodes := []fileset.Node{
		{LogicalName: "tmux.shellrc.helpers"},
		{LogicalName: "tmux.shellrc.base"},
		{LogicalName: "other.shellrc.foo"},
	}
	got := resolveAfter("tmux/shellrc/", nodes)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %v", len(got), got)
	}
}

func TestResolveAfterExact(t *testing.T) {
	nodes := []fileset.Node{{LogicalName: "foo"}, {LogicalName: "bar"}}
	got := resolveAfter("foo", nodes)
	if len(got) != 1 || got[0] != "foo" {
		t.Errorf("got %v, want [foo]", got)
	}
}
