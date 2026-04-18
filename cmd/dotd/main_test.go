package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run executes the root command with the given args, capturing stdout.
// Returns (stdout, error).
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// emptyDotfiles creates a minimal temp dotfiles directory (no files).
func emptyDotfiles(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// emptyEnvFile creates a temp env.yaml that is empty (no overrides).
func emptyEnvFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte("env: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- dotd env list ---

func TestEnvListShowsBuiltins(t *testing.T) {
	out, err := run(t, "env", "show",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env show error = %v", err)
	}
	// Built-in detectors always populate os and shell.
	if !strings.Contains(out, "os=") {
		t.Errorf("output missing os=: %q", out)
	}
	if !strings.Contains(out, "shell=") {
		t.Errorf("output missing shell=: %q", out)
	}
}

func TestEnvListShowsFileKeys(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("env:\n  context: work\n"), 0o644); err != nil {
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

func TestEnvListEnvFlagOverride(t *testing.T) {
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
	out, err := run(t, "env", "get", "os",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env get error = %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("expected non-empty value for os")
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
	if err := os.WriteFile(envFile, []byte("env: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "env", "set", "context=work",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env set error = %v", err)
	}

	// Verify key is readable back.
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

func TestEnvSetDryRun(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("env: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "env", "set", "context=work",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("env set --dry-run error = %v", err)
	}
	if !strings.Contains(out, "would set") {
		t.Errorf("expected 'would set' in dry-run output: %q", out)
	}

	// File must not be modified.
	data, _ := os.ReadFile(envFile)
	if strings.Contains(string(data), "context") {
		t.Error("dry-run must not write to env file")
	}
}

func TestEnvSetInvalidArg(t *testing.T) {
	_, err := run(t, "env", "set", "no-equals",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err == nil {
		t.Error("expected error for missing = in set arg")
	}
}

// --- dotd env diff ---

func TestEnvDiffNoOverrides(t *testing.T) {
	out, err := run(t, "env", "diff",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "no overrides") {
		t.Errorf("expected 'no overrides' with empty env file: %q", out)
	}
}

func TestEnvDiffShowsOverrides(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	// Override "os" to a value that differs from detected.
	if err := os.WriteFile(envFile, []byte("env:\n  os: testplatform\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "env", "diff",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "os") || !strings.Contains(out, "testplatform") {
		t.Errorf("expected os override in diff output: %q", out)
	}
}

func TestEnvDiffMatchingOverrideNotShown(t *testing.T) {
	// If env.yaml sets a value that happens to match what's detected, no diff.
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")

	// Detect real OS value first.
	osOut, err := run(t, "env", "get", "os",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env get os: %v", err)
	}
	detectedOS := strings.TrimSpace(osOut)

	// Write env.yaml with the same OS value as detected.
	if err := os.WriteFile(envFile, []byte("env:\n  os: "+detectedOS+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "env", "diff",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "no overrides") {
		t.Errorf("matching override should not appear in diff: %q", out)
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
	out, err := run(t, "check",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !strings.Contains(out, "scripts: 0 active") {
		t.Errorf("expected 'scripts: 0 active': %q", out)
	}
	if !strings.Contains(out, "symlinks:") {
		t.Errorf("expected 'symlinks:' summary: %q", out)
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

func TestApplyDryRunWithScript(t *testing.T) {
	dotfiles := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotfiles, "shellrc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dotfiles, "shellrc", "base.sh"), []byte("# @when os=linux\nexport X=1\n"), 0o644); err != nil {
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
	if !strings.Contains(out, "base.sh") {
		t.Errorf("expected base.sh in dry-run output: %q", out)
	}
}
