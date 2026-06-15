package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/pipeline"
)

// bundleDotfiles builds a minimal hermetic dotfiles repo with a single sourced
// shell script and returns its path. Paths resolve from HOME/XDG, not the real
// machine.
func bundleDotfiles(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))

	dotfiles := t.TempDir()
	shellrc := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrc, ecosystem.ConfigFile),
		[]byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrc, "aliases.sh"),
		[]byte("#!/bin/bash\nalias ll='ls -la'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dotfiles
}

// TestBundleIncludeEnvSortedOrder verifies that `bundle --include-env` emits its
// export lines in sorted key order. Map iteration is nondeterministic, so without
// an explicit sort the "single portable script" would differ run-to-run and
// defeat reproducible output/diffing.
func TestBundleIncludeEnvSortedOrder(t *testing.T) {
	dotfiles := bundleDotfiles(t)

	out, err := run(t, "bundle", "shellrc/aliases.sh", "--include-env",
		"-f", dotfiles, "--dotd-env", emptyEnvFile(t),
		"--env", "os=linux", "--env", "context=personal",
		"--env", "zeta=1", "--env", "alpha=2", "--env", "mu=3")
	if err != nil {
		t.Fatalf("bundle error = %v\noutput: %s", err, out)
	}

	var keys []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "export ") {
			continue
		}
		rest := strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			continue
		}
		keys = append(keys, rest[:eq])
	}

	if len(keys) < 2 {
		t.Fatalf("expected multiple export lines, got %d:\n%s", len(keys), out)
	}
	if !sort.StringsAreSorted(keys) {
		t.Errorf("export lines are not in sorted key order: %v\noutput:\n%s", keys, out)
	}
}

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
