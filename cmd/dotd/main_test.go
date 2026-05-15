package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run executes the root command with the given args, capturing combined stdout+stderr.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// emptyDotfiles creates a minimal temp dotfiles directory (no files).
func emptyDotfiles(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// emptyEnvFile creates a temp env.yaml that is empty (flat v2 format).
func emptyEnvFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- dotd env show ---

func TestEnvShowEmpty(t *testing.T) {
	_, err := run(t, "env", "show",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env show error = %v", err)
	}
}

func TestEnvShowFileKeys(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	// v2 env.yaml is a flat map[string]string
	if err := os.WriteFile(envFile, []byte("context: work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "env", "show",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env show error = %v", err)
	}
	if !strings.Contains(out, "context=work") {
		t.Errorf("output missing context=work: %q", out)
	}
}

func TestEnvShowEnvFlagOverride(t *testing.T) {
	out, err := run(t, "env", "show",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
		"--env", "os=testvalue",
	)
	if err != nil {
		t.Fatalf("env show error = %v", err)
	}
	if !strings.Contains(out, "os=testvalue") {
		t.Errorf("output missing overridden os=testvalue: %q", out)
	}
}

// --- dotd env get ---

func TestEnvGetExistingKey(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("mykey: myval\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "env", "get", "mykey",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env get error = %v", err)
	}
	if strings.TrimSpace(out) != "myval" {
		t.Errorf("env get mykey = %q, want %q", strings.TrimSpace(out), "myval")
	}
}

func TestEnvGetMissingKey(t *testing.T) {
	_, err := run(t, "env", "get", "no-such-key",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

// --- dotd env set ---

func TestEnvSetWritesKey(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "env", "set", "context", "work",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env set error = %v", err)
	}

	out, err := run(t, "env", "get", "context",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env get after set error = %v", err)
	}
	if strings.TrimSpace(out) != "work" {
		t.Errorf("context = %q, want work", strings.TrimSpace(out))
	}
}

func TestEnvSetInvalidArgs(t *testing.T) {
	_, err := run(t, "env", "set",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err == nil {
		t.Error("expected error for wrong number of args")
	}
}

// --- dotd completion ---

func TestCompletionBash(t *testing.T) {
	out, err := run(t, "completion", "bash")
	if err != nil {
		t.Fatalf("completion bash error = %v", err)
	}
	if !strings.Contains(out, "bash") {
		t.Errorf("expected bash completion output: %q", out[:min(len(out), 200)])
	}
}

func TestCompletionZsh(t *testing.T) {
	out, err := run(t, "completion", "zsh")
	if err != nil {
		t.Fatalf("completion zsh error = %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionInvalidShell(t *testing.T) {
	_, err := run(t, "completion", "nosuchshell")
	if err == nil {
		t.Error("expected error for unsupported shell")
	}
}

// --- dotd check ---

func TestCheckEmptyRepo(t *testing.T) {
	// check on a fresh repo always reports init.sh missing — exit code non-zero is expected.
	// What we care about: it runs without panicking and produces symlink/init.sh summary.
	out, _ := run(t, "check",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if !strings.Contains(out, "symlinks:") {
		t.Errorf("expected symlinks summary in output: %q", out)
	}
}

func TestCheckAfterApply_Unit(t *testing.T) {
	dotfiles := t.TempDir()
	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	// Apply first so init.sh exists.
	_, err := run(t, "apply",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply error = %v", err)
	}

	// check should succeed since init.sh was created.
	_, err = run(t, "check",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("check after apply error = %v", err)
	}
}

// --- dotd apply --dry-run ---

func TestApplyDryRunEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
	if !strings.Contains(out, "would write") {
		t.Errorf("expected 'would write' in dry-run output: %q", out)
	}
	// init.sh must not be created.
	if _, err := os.Stat(initFile); !os.IsNotExist(err) {
		t.Error("dry-run must not create init.sh")
	}
}

func TestApplyDryRunWithActiveNode(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// v2 annotation syntax: @when(os=linux), .dagger defaults source action
	daggerFile := filepath.Join(shellrcDir, ".dagger")
	if err := os.WriteFile(daggerFile, []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("# @when(os=linux)\nexport X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
		"--env", "os=linux",
	)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
	// Dry-run reports the number of sourced nodes including the file name.
	if !strings.Contains(out, "would write") {
		t.Errorf("expected 'would write' in dry-run output: %q", out)
	}
	// init.sh must not be created.
	if _, err := os.Stat(initFile); !os.IsNotExist(err) {
		t.Error("dry-run must not create init.sh")
	}
}

func TestApplyWritesInitSh(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	daggerFile := filepath.Join(shellrcDir, ".dagger")
	if err := os.WriteFile(daggerFile, []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	_, err := run(t, "apply",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply error = %v", err)
	}
	content, err := os.ReadFile(initFile)
	if err != nil {
		t.Fatalf("init.sh not created: %v", err)
	}
	if !strings.Contains(string(content), "base.sh") {
		t.Errorf("init.sh missing base.sh: %s", string(content))
	}
}

// --- dotd list ---

func TestListEmpty(t *testing.T) {
	_, err := run(t, "list",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
}

func TestListActiveNodes(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ".dagger"), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "macos.sh"), []byte("# @when(os=macos)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--env", "os=linux",
	)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "shellrc.base") {
		t.Errorf("expected shellrc.base in list output: %q", out)
	}
	if strings.Contains(out, "shellrc.macos") {
		t.Errorf("shellrc.macos should be inactive and not shown: %q", out)
	}
}

func TestListInactiveFlag(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ".dagger"), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "macos.sh"), []byte("# @when(os=macos)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list", "--inactive",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--env", "os=linux",
	)
	if err != nil {
		t.Fatalf("list --inactive error = %v", err)
	}
	if !strings.Contains(out, "shellrc.macos") {
		t.Errorf("expected shellrc.macos in --inactive output: %q", out)
	}
	if !strings.Contains(out, "inactive") {
		t.Errorf("expected 'inactive' tag in output: %q", out)
	}
}

// --- dotd bundle ---

func TestBundleNoFile(t *testing.T) {
	_, err := run(t, "bundle",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
		"nonexistent.sh",
	)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestBundleSimple(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ".dagger"), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export BASE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "bundle",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"shellrc/base.sh",
	)
	if err != nil {
		t.Fatalf("bundle error = %v", err)
	}
	if !strings.Contains(out, "export BASE=1") {
		t.Errorf("expected file content in bundle output: %q", out)
	}
}

// --- dotd env diff ---

func TestEnvDiffShowsOverride(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("context: work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTD_CONTEXT", "")

	out, err := run(t, "env", "diff",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "context") {
		t.Errorf("expected 'context' in diff output, got %q", out)
	}
}

func TestEnvDiffEmptyFile(t *testing.T) {
	out, err := run(t, "env", "diff",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "no overrides") {
		t.Errorf("expected 'no overrides', got %q", out)
	}
}

func TestEnvDiffShellVarMatches(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("context: work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTD_CONTEXT", "work")

	out, err := run(t, "env", "diff",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "no overrides") {
		t.Errorf("expected 'no overrides' when values match, got %q", out)
	}
}
