package main

import (
	"testing"

	"github.com/rocne/dot-dagger/internal/pipeline"
)

// --- collectDeps unit tests ---

// TestCollectDeps_TransitiveChain builds three nodes with A→B→C @after chain
// and verifies that collectDeps(C) returns [A, B] in DAG order.
func TestCollectDeps_TransitiveChain(t *testing.T) {
	// ordered slice as Kahn would produce: A, B, C
	ordered := []pipeline.RawNode{
		{Path: "/dots/a.sh", LogicalName: "shellrc.a", After: nil},
		{Path: "/dots/b.sh", LogicalName: "shellrc.b", After: []string{"shellrc.a"}},
		{Path: "/dots/c.sh", LogicalName: "shellrc.c", After: []string{"shellrc.b"}},
	}

	// Target is C (index 2).
	deps := collectDeps(ordered, 2)

	if len(deps) != 2 {
		t.Fatalf("expected 2 deps (A, B), got %d: %v", len(deps), deps)
	}
	if deps[0].LogicalName != "shellrc.a" {
		t.Errorf("deps[0] = %q, want 'shellrc.a'", deps[0].LogicalName)
	}
	if deps[1].LogicalName != "shellrc.b" {
		t.Errorf("deps[1] = %q, want 'shellrc.b'", deps[1].LogicalName)
	}
}

// TestCollectDeps_DirectOnly verifies that a node with a direct @after dep
// includes only that dep (no extras).
func TestCollectDeps_DirectOnly(t *testing.T) {
	ordered := []pipeline.RawNode{
		{Path: "/dots/base.sh", LogicalName: "shellrc.base", After: nil},
		{Path: "/dots/path.sh", LogicalName: "shellrc.path", After: []string{"shellrc.base"}},
		{Path: "/dots/unrelated.sh", LogicalName: "shellrc.unrelated", After: nil},
		{Path: "/dots/work.sh", LogicalName: "shellrc.work", After: []string{"shellrc.path"}},
	}

	// Bundling "work" (index 3) should only include base and path, not unrelated.
	deps := collectDeps(ordered, 3)

	names := make(map[string]bool, len(deps))
	for _, d := range deps {
		names[d.LogicalName] = true
	}
	if !names["shellrc.base"] {
		t.Error("expected 'shellrc.base' in deps")
	}
	if !names["shellrc.path"] {
		t.Error("expected 'shellrc.path' in deps")
	}
	if names["shellrc.unrelated"] {
		t.Error("'shellrc.unrelated' should NOT be a dep of shellrc.work")
	}
}

// TestCollectDeps_NoDeps verifies that a node with no @after edges returns nil.
func TestCollectDeps_NoDeps(t *testing.T) {
	ordered := []pipeline.RawNode{
		{Path: "/dots/base.sh", LogicalName: "shellrc.base", After: nil},
		{Path: "/dots/other.sh", LogicalName: "shellrc.other", After: nil},
	}
	deps := collectDeps(ordered, 1)
	if len(deps) != 0 {
		t.Errorf("expected 0 deps for node with no @after, got %d", len(deps))
	}
}

// TestCollectDeps_FirstNode verifies that targetIdx=0 returns nil immediately.
func TestCollectDeps_FirstNode(t *testing.T) {
	ordered := []pipeline.RawNode{
		{Path: "/dots/base.sh", LogicalName: "shellrc.base"},
	}
	deps := collectDeps(ordered, 0)
	if deps != nil {
		t.Errorf("expected nil deps for first node, got %v", deps)
	}
}
