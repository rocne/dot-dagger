package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
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

// --- shellQuote unit tests ---

func TestShellQuote_PlainString(t *testing.T) {
	if got := shellQuote("hello"); got != "hello" {
		t.Errorf("shellQuote('hello') = %q, want 'hello'", got)
	}
}

func TestShellQuote_StringWithSpaces(t *testing.T) {
	got := shellQuote("hello world")
	if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, "'") {
		t.Errorf("expected single-quoted string for value with spaces, got %q", got)
	}
	// Must contain the original content safely.
	if !strings.Contains(got, "hello world") {
		t.Errorf("quoted value must preserve content, got %q", got)
	}
}

func TestShellQuote_StringWithSingleQuote(t *testing.T) {
	// "it's" should be escaped safely: 'it'"'"'s'
	got := shellQuote("it's a test")
	// Should not contain an unescaped single quote that would break shell parsing.
	// The standard POSIX escape pattern: 'it'"'"'s a test'
	if !strings.Contains(got, `'"'"'`) {
		t.Errorf("shellQuote did not properly escape single quote, got %q", got)
	}
}

func TestShellQuote_StringWithDollarSign(t *testing.T) {
	// Dollar sign must be quoted to avoid variable expansion.
	got := shellQuote("$HOME")
	if !strings.HasPrefix(got, "'") {
		t.Errorf("expected quoting for string with $, got %q", got)
	}
}

// --- bundle command integration tests ---

// bundleShellrcDir creates a shellrc dir with the given files.
// files maps filename → content.
func bundleShellrcDir(t *testing.T, dotfiles string, files map[string]string) {
	t.Helper()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile),
		[]byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(shellrcDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBundle_TransitiveAfterChain verifies that bundling a node with a transitive
// @after chain (A → B → C) includes A and B before C in the output.
func TestBundle_TransitiveAfterChain(t *testing.T) {
	dotfiles := t.TempDir()
	bundleShellrcDir(t, dotfiles, map[string]string{
		"a.sh": "# a\nexport A=1\n",
		"b.sh": "# @after(shellrc.a)\nexport B=2\n",
		"c.sh": "# @after(shellrc.b)\nexport C=3\n",
	})

	out, err := run(t, "bundle",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"shellrc/c.sh",
	)
	if err != nil {
		t.Fatalf("bundle error = %v", err)
	}

	// All three files' content must appear.
	if !strings.Contains(out, "export A=1") {
		t.Error("expected content from a.sh (dep of b which is dep of c)")
	}
	if !strings.Contains(out, "export B=2") {
		t.Error("expected content from b.sh (direct dep of c)")
	}
	if !strings.Contains(out, "export C=3") {
		t.Error("expected content from c.sh (the target)")
	}

	// DAG order: A must appear before B, B before C.
	posA := strings.Index(out, "export A=1")
	posB := strings.Index(out, "export B=2")
	posC := strings.Index(out, "export C=3")
	if posA >= posB {
		t.Errorf("a.sh content should appear before b.sh content (A=%d, B=%d)", posA, posB)
	}
	if posB >= posC {
		t.Errorf("b.sh content should appear before c.sh content (B=%d, C=%d)", posB, posC)
	}
}

// TestBundle_IncludeEnv_ShellQuoting verifies that --include-env escapes
// values containing spaces and that the export line is present in the output.
func TestBundle_IncludeEnv_ShellQuoting(t *testing.T) {
	dotfiles := t.TempDir()
	bundleShellrcDir(t, dotfiles, map[string]string{
		"base.sh": "export BASE=1\n",
	})

	// Supply an env value with spaces via --env; it should be shell-quoted in output.
	out, err := run(t, "bundle", "--include-env",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--env", "myvar=hello world",
		"shellrc/base.sh",
	)
	if err != nil {
		t.Fatalf("bundle --include-env error = %v", err)
	}

	// The output must have an export line for myvar.
	if !strings.Contains(out, "export myvar=") {
		t.Errorf("expected 'export myvar=' in bundle output: %q", out)
	}
	// The value must be shell-quoted (wrapped in single quotes) because it has a space.
	if !strings.Contains(out, "export myvar='hello world'") {
		t.Errorf("expected single-quoted value in export, got output:\n%s", out)
	}
}

// TestBundle_OutputFile verifies that --output writes to the given file
// and that the file has the expected shebang and mode.
func TestBundle_OutputFile(t *testing.T) {
	dotfiles := t.TempDir()
	bundleShellrcDir(t, dotfiles, map[string]string{
		"base.sh": "export BASE=1\n",
	})

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "bundled.sh")

	_, err := run(t, "bundle",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--output", outFile,
		"shellrc/base.sh",
	)
	if err != nil {
		t.Fatalf("bundle --output error = %v", err)
	}

	// File must exist.
	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	// Mode should be 0o644 (ModeFile).
	if info.Mode() != 0o644 {
		t.Errorf("output file mode = %04o, want 0644", info.Mode())
	}

	// Content must have the POSIX shebang.
	content, _ := os.ReadFile(outFile)
	if !strings.HasPrefix(string(content), "#!/bin/sh\n") {
		t.Errorf("expected POSIX shebang at start of bundle output, got:\n%s", string(content)[:min(len(content), 80)])
	}
}

