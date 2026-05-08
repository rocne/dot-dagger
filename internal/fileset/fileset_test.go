package fileset

import (
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/walk"
)

func TestBuildFiltersInactive(t *testing.T) {
	nodes := []walk.Node{
		{Path: "/a.sh", Kind: KindScript, LogicalName: "a", EffectiveWhen: "os=linux"},
		{Path: "/b.sh", Kind: KindScript, LogicalName: "b", EffectiveWhen: "os=macos"},
		{Path: "/c.sh", Kind: KindScript, LogicalName: "c", EffectiveWhen: ""},
	}
	env := map[string]string{"os": "linux"}

	s, err := Build(nodes, env, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(s.Nodes) != 2 {
		t.Fatalf("len(Nodes) = %d, want 2", len(s.Nodes))
	}
	if s.Nodes[0].LogicalName != "a" {
		t.Errorf("Nodes[0].LogicalName = %q, want a", s.Nodes[0].LogicalName)
	}
	if s.Nodes[1].LogicalName != "c" {
		t.Errorf("Nodes[1].LogicalName = %q, want c", s.Nodes[1].LogicalName)
	}
}

func TestBuildPartitionsByKind(t *testing.T) {
	nodes := []walk.Node{
		{Path: "/s.sh", Kind: KindScript, LogicalName: "s"},
		{Path: "/c.cfg", Kind: KindConf, LogicalName: "c"},
		{Path: "/b", Kind: KindBin, LogicalName: "b"},
		{Path: "/o.txt", Kind: KindOther, LogicalName: "o"},
	}

	s, err := Build(nodes, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(s.Scripts()) != 1 || s.Scripts()[0].LogicalName != "s" {
		t.Errorf("Scripts() = %v", s.Scripts())
	}
	if len(s.Conf()) != 1 || s.Conf()[0].LogicalName != "c" {
		t.Errorf("Conf() = %v", s.Conf())
	}
	if len(s.Bin()) != 1 || s.Bin()[0].LogicalName != "b" {
		t.Errorf("Bin() = %v", s.Bin())
	}
}

func TestBuildPreservesAnnotations(t *testing.T) {
	anns := []annotation.Annotation{{Key: "after", Args: "base"}}
	nodes := []walk.Node{
		{Path: "/a.sh", Kind: KindScript, LogicalName: "a", Annotations: anns},
	}

	s, err := Build(nodes, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(s.Nodes[0].Annotations) != 1 {
		t.Error("annotations not preserved")
	}
}

func TestBuildEnvStored(t *testing.T) {
	env := map[string]string{"os": "linux"}
	s, err := Build(nil, env, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if s.Env["os"] != "linux" {
		t.Errorf("Env[os] = %q, want linux", s.Env["os"])
	}
}

func TestBuildUnfilteredIncludesAll(t *testing.T) {
	nodes := []walk.Node{
		{Path: "/a.sh", Kind: KindScript, LogicalName: "a", EffectiveWhen: "os=never-true"},
		{Path: "/b.sh", Kind: KindScript, LogicalName: "b", EffectiveWhen: "os=also-never"},
		{Path: "/c.sh", Kind: KindScript, LogicalName: "c", EffectiveWhen: ""},
	}

	s := BuildUnfiltered(nodes)

	if len(s.Nodes) != 3 {
		t.Fatalf("len(Nodes) = %d, want 3 (all nodes included regardless of @when)", len(s.Nodes))
	}
}

func TestBuildMissingEnvKeyError(t *testing.T) {
	nodes := []walk.Node{
		{Path: "/a.sh", Kind: KindScript, LogicalName: "a", EffectiveWhen: "context=work"},
	}
	// context key absent from env.
	_, err := Build(nodes, map[string]string{"os": "linux"}, nil)
	if err == nil {
		t.Error("Build() error = nil, want error for missing env key")
	}
}
