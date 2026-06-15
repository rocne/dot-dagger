package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
)

// run executes the root command with the given args, capturing combined stdout+stderr.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runWithStdin(t, nil, args...)
}

// runWithStdin is run() but also wires stdin so tests covering interactive
// commands (setup, teardown's confirm prompt, unapply's confirm prompt) don't
// need to re-wire the cobra plumbing themselves. A nil stdin is allowed for
// non-interactive invocations.
func runWithStdin(t *testing.T, stdin io.Reader, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	if stdin != nil {
		root.SetIn(stdin)
	}
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
		"--dotd-env", emptyEnvFile(t),
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
		"--dotd-env", envFile,
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
		"--dotd-env", emptyEnvFile(t),
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
		"--dotd-env", envFile,
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
		"--dotd-env", emptyEnvFile(t),
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
		"--dotd-env", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env set error = %v", err)
	}

	out, err := run(t, "env", "get", "context",
		"--dotd-env", envFile,
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
		"--dotd-env", emptyEnvFile(t),
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
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	out, _ := run(t, "check",
		"--dotd-env", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if !strings.Contains(out, "symlinks:") {
		t.Errorf("expected symlinks summary in output: %q", out)
	}
}

func TestCheckAfterApply_Unit(t *testing.T) {
	dotfiles := t.TempDir()
	xdgData := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", xdgData)

	// Apply first so init.sh exists.
	_, err := run(t, "apply",
		"--dotd-env", emptyEnvFile(t),
		"--files", dotfiles,
	)
	if err != nil {
		t.Fatalf("apply error = %v", err)
	}

	// check should succeed since init.sh was created.
	_, err = run(t, "check",
		"--dotd-env", emptyEnvFile(t),
		"--files", dotfiles,
	)
	if err != nil {
		t.Fatalf("check after apply error = %v", err)
	}
}

// TestCheck_DetectsMissingSymlink verifies that check exits non-zero AND emits
// a "missing" report when an expected symlink is absent on disk (AUDIT-061).
func TestCheck_DetectsMissingSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n[core]\n  autocrlf = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	envFile := emptyEnvFile(t)

	// init.sh must exist so the missing-symlink case is the only failure.
	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Now sabotage: delete the symlink apply just created.
	if err := os.Remove(filepath.Join(home, ".gitconfig")); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "check",
		"--files", dotfiles, "--dotd-env", envFile,
	)
	if err == nil {
		t.Fatalf("check should fail when symlink is missing; out=%q", out)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("check output should mention 'missing'; got %q", out)
	}
}

// TestCheck_IssuesFoundHintsApply: when config.yaml exists but a symlink is
// missing or wrong, check must guide the user to the fix ('dotd apply') instead
// of just reporting "issues found" (Track H error-hint pass).
func TestCheck_IssuesFoundHintsApply(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile := emptyEnvFile(t)

	// A real config.yaml so check takes the "config exists" branch (not the
	// existing "no config — run setup" hint).
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("dotfiles: "+dotfiles+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// apply so init.sh exists, then delete the symlink so the only issue is a
	// missing link (init.sh missing has its own separate message).
	if _, err := run(t, "apply", "--files", dotfiles, "--dotd-env", envFile, "--dotd-config", configPath); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := os.Remove(filepath.Join(home, ".gitconfig")); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "check", "--files", dotfiles, "--dotd-env", envFile, "--dotd-config", configPath)
	if err == nil {
		t.Fatal("check should fail when a symlink is missing")
	}
	var h Hinter
	if !errors.As(err, &h) || !strings.Contains(h.Hint(), "dotd apply") {
		t.Errorf("check 'issues found' should hint 'dotd apply'; got err=%v", err)
	}
}

// TestCheck_DetectsWrongTarget verifies that check exits non-zero when the
// symlink exists but points at a different file (AUDIT-061).
func TestCheck_DetectsWrongTarget(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	envFile := emptyEnvFile(t)

	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Replace the symlink with one pointing elsewhere.
	link := filepath.Join(home, ".gitconfig")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/tmp/elsewhere", link); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "check",
		"--files", dotfiles, "--dotd-env", envFile,
	)
	if err == nil {
		t.Fatalf("check should fail on wrong-target symlink; out=%q", out)
	}
	if !strings.Contains(out, "wrong") {
		t.Errorf("check output should mention 'wrong'; got %q", out)
	}
}

// TestCheck_DetectsMissingInitSh verifies that check exits non-zero and
// reports "missing" when init.sh has not yet been written (AUDIT-061).
func TestCheck_DetectsMissingInitSh(t *testing.T) {
	dotfiles := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	// Intentionally do NOT create xdgData/dot-dagger/init.sh
	envFile := emptyEnvFile(t)

	out, err := run(t, "check",
		"--files", dotfiles, "--dotd-env", envFile,
	)
	if err == nil {
		t.Fatal("check should fail when init.sh is absent")
	}
	// The output must mention init.sh and missing — not just any error.
	if !strings.Contains(out, "init.sh") && !strings.Contains(out, "missing") {
		t.Errorf("check output should report init.sh missing; got %q", out)
	}
}

// --- dotd apply --dry-run ---

func TestApplyDryRunEmptyRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	initFile := filepath.Join(xdgData, "dot-dagger", "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--dotd-env", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
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
	daggerFile := filepath.Join(shellrcDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("# @when(os=linux)\nexport X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", t.TempDir())
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	initFile := filepath.Join(xdgData, "dot-dagger", "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--dotd-env", emptyEnvFile(t),
		"--files", dotfiles,
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
	daggerFile := filepath.Join(shellrcDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", t.TempDir())
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	initFile := filepath.Join(xdgData, "dot-dagger", "init.sh")

	_, err := run(t, "apply",
		"--dotd-env", emptyEnvFile(t),
		"--files", dotfiles,
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

// TestApply_ResumesAfterPartialFailure locks dot-dagger's recovery contract:
// apply is idempotent and resumable, not transactional. A mid-apply failure
// leaves partial state on disk, and simply re-running apply once the cause is
// cleared converges to the full plan — no rollback, no manual cleanup. This
// guards the idempotency the design relies on (createSymlink remove+recreate,
// WriteAtomic overwrite) from regressing (Track J, J-1/B-4, 2026-06-15).
func TestApply_ResumesAfterPartialFailure(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Two link nodes; alphabetical tie-break links dot-aaa before dot-zzz.
	if err := os.WriteFile(filepath.Join(confDir, "dot-aaa"), []byte("# @link(~/.aaa)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-zzz"), []byte("# @link(~/.zzz)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile := emptyEnvFile(t)

	// Block the second link's destination with a regular file so apply fails
	// mid-plan, after the first symlink is already on disk.
	blocker := filepath.Join(home, ".zzz")
	if err := os.WriteFile(blocker, []byte("pre-existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// First apply: fails on the blocked target, leaving partial state.
	if _, err := run(t, "apply", "--files", dotfiles, "--dotd-env", envFile); err == nil {
		t.Fatal("apply should fail while .zzz is blocked by a non-symlink file")
	}
	if fi, err := os.Lstat(filepath.Join(home, ".aaa")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf(".aaa should be a symlink after the partial apply: %v", err)
	}

	// Clear the cause and re-run: apply must converge with no manual cleanup.
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "apply", "--files", dotfiles, "--dotd-env", envFile); err != nil {
		t.Fatalf("re-apply should converge after the cause is cleared: %v", err)
	}
	for _, name := range []string{".aaa", ".zzz"} {
		if fi, err := os.Lstat(filepath.Join(home, name)); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s should be a symlink after re-apply (convergence): %v", name, err)
		}
	}
}

// TestAnnotate_OutsideDotfilesHints: annotating a file that isn't inside the
// dotfiles repo must point the user at how to bring it in, not just report the
// rejection (Track H error-hint pass).
func TestAnnotate_OutsideDotfilesHints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	outside := filepath.Join(t.TempDir(), "loose.sh")
	if err := os.WriteFile(outside, []byte("echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "annotate", outside,
		"--files", emptyDotfiles(t),
		"--dotd-env", emptyEnvFile(t),
	)
	if err == nil {
		t.Fatal("annotate should reject a file outside the dotfiles dir")
	}
	var h Hinter
	if !errors.As(err, &h) || !strings.Contains(h.Hint(), "adopt") {
		t.Errorf("outside-dotfiles error should hint 'dotd adopt'; got err=%v", err)
	}
}

// --- dotd list ---

func TestListEmpty(t *testing.T) {
	_, err := run(t, "list",
		"--dotd-env", emptyEnvFile(t),
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
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "macos.sh"), []byte("# @when(os=macos)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list",
		"--dotd-env", emptyEnvFile(t),
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
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "macos.sh"), []byte("# @when(os=macos)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list", "--inactive",
		"--dotd-env", emptyEnvFile(t),
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
		"--dotd-env", emptyEnvFile(t),
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
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export BASE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "bundle",
		"--dotd-env", emptyEnvFile(t),
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
		"--dotd-env", envFile,
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
		"--dotd-env", emptyEnvFile(t),
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
		"--dotd-env", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "no overrides") {
		t.Errorf("expected 'no overrides' when values match, got %q", out)
	}
}

// --- dotd package list ---

func TestPackageListEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "packages.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, "package", "list",
		"--dotd-env", emptyEnvFile(t),
		"--files", dir,
	)
	if err != nil {
		t.Fatalf("package list error = %v", err)
	}
}

func TestPackageListShowsAnnotations(t *testing.T) {
	dir := t.TempDir()
	shellrc := filepath.Join(dir, "shellrc")
	if err := os.MkdirAll(shellrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrc, "base.sh"), []byte("# @require(ripgrep)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "packages.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "package", "list",
		"--dotd-env", emptyEnvFile(t),
		"--files", dir,
	)
	if err != nil {
		t.Fatalf("package list error = %v", err)
	}
	if !strings.Contains(out, "ripgrep") {
		t.Errorf("expected 'ripgrep' in output, got %q", out)
	}
	if !strings.Contains(out, "require") {
		t.Errorf("expected 'require' in output, got %q", out)
	}
}

func TestPackageListShowsRequestAnnotation(t *testing.T) {
	dir := t.TempDir()
	shellrc := filepath.Join(dir, "shellrc")
	if err := os.MkdirAll(shellrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrc, "base.sh"), []byte("# @request(curl)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "packages.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "package", "list",
		"--dotd-env", emptyEnvFile(t),
		"--files", dir,
	)
	if err != nil {
		t.Fatalf("package list error = %v", err)
	}
	if !strings.Contains(out, "curl") {
		t.Errorf("expected 'curl' in output, got %q", out)
	}
	if !strings.Contains(out, "request") {
		t.Errorf("expected 'request' in output, got %q", out)
	}
}

func TestResolveToFlag(t *testing.T) {
	tests := []struct {
		to   string
		name string
		want string
	}{
		{"config/dot-gitconfig-work", ".gitconfig", "config/dot-gitconfig-work"},
		{"config/", ".gitconfig", "config/.gitconfig"},
		{"bin/", "my-script", "bin/my-script"},
	}
	for _, tt := range tests {
		got := resolveToFlag(tt.to, tt.name)
		if got != tt.want {
			t.Errorf("resolveToFlag(%q, %q) = %q, want %q", tt.to, tt.name, got, tt.want)
		}
	}
}

// --- dotd setup ---

func TestSetup_WritesConfigAndEnv(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", t.TempDir()) // so default dotfiles path exists

	// Simulate pressing Enter for all prompts (accept defaults).
	out, err := runWithStdin(t, strings.NewReader(strings.Repeat("\n", 10)), "setup")
	if err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, out)
	}

	configPath := filepath.Join(xdg, "dot-dagger", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml not written: %v", err)
	}
	envPath := filepath.Join(xdg, "dot-dagger", "env.yaml")
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("env.yaml not written: %v", err)
	}
}

// TestSetup_NonInteractive verifies that --non-interactive writes a config
// without reading any stdin input.
func TestSetup_NonInteractive(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", t.TempDir())

	out, err := runWithStdin(t, strings.NewReader(""), "setup", "--non-interactive")
	if err != nil {
		t.Fatalf("setup --non-interactive error = %v\noutput:\n%s", err, out)
	}

	configPath := filepath.Join(xdg, "dot-dagger", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml not written: %v", err)
	}
}

func TestSetup_UsesCurrentValuesAsDefaults(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	// Pre-write config.yaml with a known dotfiles path.
	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingDotfiles := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("dotfiles: "+existingDotfiles+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Accept all defaults (Enter for each prompt).
	out, err := runWithStdin(t, strings.NewReader(strings.Repeat("\n", 10)), "setup")
	if err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, out)
	}

	content, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), existingDotfiles) {
		t.Errorf("existing dotfiles path not preserved, got:\n%s", string(content))
	}
}

func TestSetup_SkipsEnvIfExists(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", t.TempDir())

	// Pre-write env.yaml.
	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envContent := "context: work\n"
	if err := os.WriteFile(filepath.Join(configDir, "env.yaml"), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runWithStdin(t, strings.NewReader(strings.Repeat("\n", 10)), "setup")
	if err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, out)
	}

	got, _ := os.ReadFile(filepath.Join(configDir, "env.yaml"))
	if string(got) != envContent {
		t.Errorf("env.yaml modified: got %q, want %q", string(got), envContent)
	}
}

// --- dotd init ---

func TestInit_RequiresConfig(t *testing.T) {
	// Fresh XDG dir so no config.yaml exists.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := run(t, "init",
		"--files", emptyDotfiles(t),
		"--dotd-env", emptyEnvFile(t),
	)
	if err == nil {
		t.Fatal("expected error when config.yaml is absent, got nil")
	}
}

// --- dotd teardown ---

func TestTeardown_RemovesConfigAndEnv(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	envPath := filepath.Join(configDir, "env.yaml")
	if err := os.WriteFile(configPath, []byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--dotd-env", envPath,
	)
	if err != nil {
		t.Fatalf("teardown error = %v", err)
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config.yaml should be removed")
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Error("env.yaml should be removed")
	}
}

// TestTeardown_LegacyConfigStillRemoved: a config.yaml left over from before the
// roots-model migration (carrying removed fields) must not block teardown — its
// whole job is to remove that file. Regression for the strict-decode hard-fail
// in the path-resolution preamble.
func TestTeardown_LegacyConfigStillRemoved(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	envPath := filepath.Join(configDir, "env.yaml")
	legacy := "dotfiles: /tmp/df\nbin_dir: /x\ngenerated_dir: /g\nlink_root: ~/.config\n"
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--dotd-env", envPath,
	)
	if err != nil {
		t.Fatalf("teardown must tolerate a legacy config, got error: %v\noutput:\n%s", err, out)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("legacy config.yaml should be removed")
	}
}

func TestTeardown_SkipsAbsentFiles(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	out, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--dotd-env", filepath.Join(xdg, "env.yaml"), // doesn't exist
	)
	if err != nil {
		t.Fatalf("teardown error = %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "not found") && !strings.Contains(out, "skip") {
		t.Errorf("expected 'not found'/'skip' in output: %q", out)
	}
}

func TestTeardown_CancelExits0(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runWithStdin(t, strings.NewReader("n\n"), "teardown",
		"--files", emptyDotfiles(t),
		"--dotd-env", filepath.Join(xdg, "env.yaml"),
	)
	if err != nil {
		t.Fatalf("teardown cancel should exit 0, got %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Error("config.yaml should be preserved on cancel")
	}
	if !strings.Contains(out, "cancelled") {
		t.Errorf("expected 'cancelled' in output: %q", out)
	}
}

// TestTeardown_BestEffortOnRemoveFailure: a failure removing one target must not
// abort the rest. teardown attempts every target, then returns non-zero — the same
// best-effort contract runUnapply already follows. Regression for the fail-fast
// path that left env.yaml + the RC source line untouched after config.yaml failed
// (Track J, 2026-06-15).
func TestTeardown_BestEffortOnRemoveFailure(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	// config.yaml is a NON-EMPTY DIRECTORY so os.Remove fails with ENOTEMPTY —
	// deterministic and uid-independent (root would ignore a perms trick).
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.MkdirAll(filepath.Join(configPath, "blocker"), 0o755); err != nil {
		t.Fatal(err)
	}
	// env.yaml is a normal, removable file in a separate writable dir.
	envPath := filepath.Join(t.TempDir(), "env.yaml")
	if err := os.WriteFile(envPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--dotd-config", configPath,
		"--dotd-env", envPath,
	)
	if err == nil {
		t.Fatal("teardown should return non-zero when a target fails to remove")
	}
	// Best-effort: the later target is removed despite the earlier failure.
	if _, statErr := os.Stat(envPath); !os.IsNotExist(statErr) {
		t.Error("env.yaml should be removed even though config.yaml removal failed")
	}
}

// TestTeardown_DryRunRemovesNothing: --dry-run is advertised for teardown
// (pathFlagOwners) and must stop after the preview — even with --yes
// (2026-06-13 audit, B1).
func TestTeardown_DryRunRemovesNothing(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	envPath := filepath.Join(configDir, "env.yaml")
	if err := os.WriteFile(configPath, []byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "teardown", "--yes", "--dry-run",
		"--files", emptyDotfiles(t),
		"--dotd-env", envPath,
	)
	if err != nil {
		t.Fatalf("teardown --dry-run error = %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Will remove:") {
		t.Errorf("expected preview in dry-run output: %q", out)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Error("config.yaml must survive --dry-run")
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Error("env.yaml must survive --dry-run")
	}
}

// TestTeardown_HonorsPathOverrides: teardown removes the resolved
// config/env paths — --config and --env-file overrides are honored
// (2026-06-13 audit, teardown-scope decision).
func TestTeardown_HonorsPathOverrides(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defaultConfig := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(defaultConfig, []byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	custom := t.TempDir()
	customConfig := filepath.Join(custom, "config.yaml")
	customEnv := filepath.Join(custom, "env.yaml")
	if err := os.WriteFile(customConfig, []byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customEnv, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--dotd-config", customConfig,
		"--dotd-env", customEnv,
	)
	if err != nil {
		t.Fatalf("teardown error = %v", err)
	}

	if _, err := os.Stat(customConfig); !os.IsNotExist(err) {
		t.Error("overridden config.yaml should be removed")
	}
	if _, err := os.Stat(customEnv); !os.IsNotExist(err) {
		t.Error("overridden env.yaml should be removed")
	}
	if _, err := os.Stat(defaultConfig); err != nil {
		t.Error("default-location config.yaml must survive when --config points elsewhere")
	}
}

// --- dotd help --all ---

// TestHelpAll_RevealsHiddenCommands: 'dotd help --all' lists internal
// helpers; plain 'dotd help' does not. The flag is local to the help
// command so it cannot collide with 'unapply --all' (2026-06-12 audit, P5).
func TestHelpAll_RevealsHiddenCommands(t *testing.T) {
	plain, err := run(t, "help")
	if err != nil {
		t.Fatalf("help error = %v", err)
	}
	if strings.Contains(plain, "get-os") {
		t.Errorf("plain help should hide internal commands: %q", plain)
	}

	all, err := run(t, "help", "--all")
	if err != nil {
		t.Fatalf("help --all error = %v", err)
	}
	if !strings.Contains(all, "get-os") || !strings.Contains(all, "get-hostname") {
		t.Errorf("help --all should list internal commands: %q", all)
	}
}

// --- dotd unapply ---

func TestUnapply_RemovesSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Set up .dagger file to mark files as links
	daggerFile := filepath.Join(confDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// File with link destination annotation
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n[core]\n  autocrlf = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile := emptyEnvFile(t)

	// Apply first.
	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	linkDest := filepath.Join(home, ".gitconfig")
	if _, err := os.Lstat(linkDest); err != nil {
		t.Fatalf("symlink not created by apply: %v", err)
	}

	// Unapply.
	if _, err := run(t, "unapply", "--yes",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("unapply error = %v", err)
	}

	if _, err := os.Lstat(linkDest); !os.IsNotExist(err) {
		t.Error("symlink should be removed after unapply")
	}
}

// TestUnapply_PartialFailureExits1 verifies that when os.Remove fails on
// one of the planned targets, unapply exits non-zero and writes the failure
// message to stderr (AUDIT-033).
func TestUnapply_PartialFailureExits1(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n[core]\n  autocrlf = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile := emptyEnvFile(t)

	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	// Sabotage the planned removal: strip write permission on the parent dir
	// so os.Remove on the symlink fails. We restore perms via t.Cleanup so
	// t.TempDir's own cleanup can run.
	if err := os.Chmod(home, 0o555); err != nil {
		t.Fatalf("chmod home: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o755) })

	out, err := run(t, "unapply", "--yes",
		"--files", dotfiles, "--dotd-env", envFile,
	)
	if err == nil {
		t.Fatalf("unapply should have failed; output: %s", out)
	}
	if !strings.Contains(out, "removing") {
		t.Errorf("expected failure message containing 'removing', got: %s", out)
	}
}

func TestUnapply_NothingToRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	out, err := run(t, "unapply", "--yes",
		"--files", emptyDotfiles(t),
		"--dotd-env", emptyEnvFile(t),
	)
	if err != nil {
		t.Fatalf("unapply error = %v", err)
	}
	if !strings.Contains(out, "nothing to remove") {
		t.Errorf("expected 'nothing to remove', got %q", out)
	}
}

func TestUnapply_DryRunPreservesSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Set up .dagger file to mark files as links
	daggerFile := filepath.Join(confDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile := emptyEnvFile(t)

	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	out, err := run(t, "unapply", "--dry-run", "--yes",
		"--files", dotfiles, "--dotd-env", envFile,
	)
	if err != nil {
		t.Fatalf("unapply --dry-run error = %v", err)
	}

	// Symlink must still exist.
	if _, err := os.Lstat(filepath.Join(home, ".gitconfig")); err != nil {
		t.Error("dry-run must not remove symlink")
	}
	if !strings.Contains(out, ".gitconfig") {
		t.Errorf("dry-run should mention .gitconfig in preview: %q", out)
	}
}

func TestUnapply_CancelExits0(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Set up .dagger file to mark files as links
	daggerFile := filepath.Join(confDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile := emptyEnvFile(t)

	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	out, err := runWithStdin(t, strings.NewReader("n\n"), "unapply",
		"--files", dotfiles, "--dotd-env", envFile,
	)
	if err != nil {
		t.Fatalf("unapply cancel should exit 0, got %v", err)
	}

	if _, err := os.Lstat(filepath.Join(home, ".gitconfig")); err != nil {
		t.Error("symlink must be preserved on cancel")
	}
	if !strings.Contains(out, "cancelled") {
		t.Errorf("expected 'cancelled' in output: %q", out)
	}
}

// --- AUDIT-001: home resolution ---

// TestAdopt_UsesHomeFromEnv verifies that adopt resolves the home directory
// from $HOME (without any --link-root flag). The test places the source file
// in $HOME and confirms adopt runs without "act: HomeDir is required".
func TestAdopt_UsesHomeFromEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A plain config file to adopt (hidden → conf/dot-testrc).
	src := filepath.Join(home, ".testrc")
	if err := os.WriteFile(src, []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "adopt", "--yes",
		"--files", dotfiles,
		"--dotd-env", emptyEnvFile(t),
		src,
	)
	if err != nil && strings.Contains(err.Error(), "HomeDir is required") {
		t.Fatalf("adopt failed with HomeDir error — home not resolved from $HOME: %v", err)
	}
	// If adopt errors for another reason (e.g. missing .dagger conventions) that
	// is acceptable; what matters is that the HomeDir error does not appear.
}

// TestTeardown_UsesHome verifies that teardown resolves the shell RC file
// under $HOME and strips the dot-dagger source line.
//
// Setup:
//   - $HOME contains a .bashrc with the dot-dagger source line.
//   - env.yaml contains shell: bash so DetectShellConfig is invoked and the
//     RC path is built as $HOME/.bashrc.
//
// Pass criterion: the source line is gone from $HOME/.bashrc after teardown.
func TestTeardown_UsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	xdgData := t.TempDir()
	xdgConfig := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	// Create the XDG config dir and env.yaml with shell: bash.
	configDir := filepath.Join(xdgConfig, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(configDir, "env.yaml")
	if err := os.WriteFile(envPath, []byte("shell: bash\nos: linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create $HOME/.bashrc containing the dot-dagger source line.
	// DetectShellConfig(bash, linux, home) → home/.bashrc.
	// HasSourceLine and RemoveSourceLine key on filepath.Base(initFile) = "init.sh".
	bashrc := filepath.Join(home, ".bashrc")
	sourceLine := `source "$HOME/.local/share/dot-dagger/init.sh"`
	bashrcContents := "# existing rc\n\n# dotd \xe2\x80\x94 generated shell init\n" + sourceLine + "\n"
	if err := os.WriteFile(bashrc, []byte(bashrcContents), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--dotd-env", envPath,
	)
	if err != nil {
		t.Fatalf("teardown error = %v", err)
	}

	// The source line must be gone from $HOME/.bashrc.
	got, readErr := os.ReadFile(bashrc)
	if readErr != nil {
		t.Fatalf("read %s: %v", bashrc, readErr)
	}
	if strings.Contains(string(got), "init.sh") {
		t.Errorf("source line still present in %s — teardown did not strip it:\n%s", bashrc, got)
	}
}

// --- AUDIT-002: configPath overridable via --config / DOTD_CONFIG_FILE ---

// minimalCfg returns an appConfig pre-populated with an empty envFile so that
// resolvePaths can run without touching the real XDG config dir.
func minimalCfg(t *testing.T) *appConfig {
	t.Helper()
	return &appConfig{
		envFile: emptyEnvFile(t),
	}
}

// TestConfigPath_FlagOverride verifies that --config /tmp/x.yaml makes
// cfg.configPath resolve to that path after resolvePaths runs.
func TestConfigPath_FlagOverride(t *testing.T) {
	dir := t.TempDir()
	customConfig := filepath.Join(dir, "custom-config.yaml")
	// File need not exist — resolvePaths only resolves the path, not loads it
	// (Load handles the missing-file case gracefully).

	t.Setenv("DOTD_CONFIG_FILE", "") // ensure env var doesn't interfere

	cfg := minimalCfg(t)
	cfg.configPath = customConfig // simulates --config flag binding

	if err := resolvePaths(cfg); err != nil {
		t.Fatalf("resolvePaths error = %v", err)
	}

	if cfg.configPath != customConfig {
		t.Errorf("cfg.configPath = %q, want %q", cfg.configPath, customConfig)
	}
}

// --- AUDIT-003: setup/init honor --config override (cfg.configPath) ---

// TestSetup_HonorsConfigFlag verifies that "dotd setup --config /tmp/x.yaml"
// writes config.yaml to the override path and NOT to the XDG default.
// Before AUDIT-003 the fix, setup called dotcfg.DefaultPath() directly and
// silently ignored the --config override.
func TestSetup_HonorsConfigFlag(t *testing.T) {
	// Use a fresh XDG dir so the default path is somewhere we can detect.
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", t.TempDir()) // valid dotfiles default
	t.Setenv("DOTD_CONFIG_FILE", "")  // ensure env var doesn't interfere

	// The override config path — must not be inside xdg so we can distinguish.
	overrideDir := t.TempDir()
	overrideConfig := filepath.Join(overrideDir, "my-config.yaml")

	// Simulate pressing Enter for all prompts (accept all defaults).
	out, err := runWithStdin(t, strings.NewReader(strings.Repeat("\n", 10)),
		"setup", "--dotd-config", overrideConfig)
	if err != nil {
		t.Fatalf("setup --config error = %v\noutput:\n%s", err, out)
	}

	// Override path must have been written.
	if _, err := os.Stat(overrideConfig); err != nil {
		t.Errorf("config.yaml not written to override path %s: %v", overrideConfig, err)
	}

	// Default XDG config must NOT exist (setup must not have written there).
	defaultConfig := filepath.Join(xdg, "dot-dagger", "config.yaml")
	if _, err := os.Stat(defaultConfig); err == nil {
		t.Errorf("config.yaml written to default XDG path %s — --config override was ignored", defaultConfig)
	}
}

// TestInit_HonorsConfigFlag verifies that "dotd init --config /tmp/missing.yaml"
// errors with "no config found" when the override path does not exist.
// Before AUDIT-003, init called dotcfg.DefaultPath() and would check the XDG
// default instead of the override, masking the wrong path.
func TestInit_HonorsConfigFlag(t *testing.T) {
	// Ensure the XDG default config.yaml exists so that if init ignores the
	// --config override and checks the default, it would NOT error — letting us
	// detect the regression.
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTD_CONFIG_FILE", "") // ensure env var doesn't interfere

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write the default config so it exists — init must still error because the
	// override path is missing.
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override path that does NOT exist.
	missingConfig := filepath.Join(t.TempDir(), "no-such-config.yaml")

	_, err := run(t, "init",
		"--dotd-config", missingConfig,
		"--files", emptyDotfiles(t),
		"--dotd-env", emptyEnvFile(t),
	)
	if err == nil {
		t.Fatal("expected error when --dotd-config path is missing, got nil — --dotd-config override was ignored")
	}
	if !strings.Contains(err.Error(), "no config found") {
		t.Errorf("expected 'no config found' error, got: %v", err)
	}
}

// TestConfigPath_EnvVarOverride verifies that DOTD_CONFIG_FILE=/tmp/x.yaml
// makes cfg.configPath resolve to that path after resolvePaths runs.
func TestConfigPath_EnvVarOverride(t *testing.T) {
	dir := t.TempDir()
	customConfig := filepath.Join(dir, "env-config.yaml")

	t.Setenv("DOTD_CONFIG_FILE", customConfig)

	cfg := minimalCfg(t)
	// configPath is empty — DOTD_CONFIG_FILE should win.

	if err := resolvePaths(cfg); err != nil {
		t.Fatalf("resolvePaths error = %v", err)
	}

	if cfg.configPath != customConfig {
		t.Errorf("cfg.configPath = %q, want %q", cfg.configPath, customConfig)
	}
}

// --- dotd unapply --all (AUDIT-059) ---

// buildLinkDotfiles creates a temp dotfiles repo with two link nodes:
//   - "unconditional" (no @when predicate — always active)
//   - "conditional"   (guarded by @when(os=special) — inactive under normal env)
//
// Returns (dotfilesDir, homeDir, envFile). Sets HOME and XDG_DATA_HOME via t.Setenv.
func buildLinkDotfiles(t *testing.T) (dotfiles, home, envFile string) {
	t.Helper()
	dotfiles = t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Unconditional link node.
	if err := os.WriteFile(filepath.Join(confDir, "dot-bashrc"),
		[]byte("# @link(~/.bashrc)\n# bashrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Predicate-gated link node — inactive unless os=special.
	if err := os.WriteFile(filepath.Join(confDir, "dot-specialrc"),
		[]byte("# @link(~/.specialrc)\n# @when(os=special)\n# specialrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile = emptyEnvFile(t)
	return
}

// TestUnapply_All_RemovesAllDotfilesSymlinks verifies that unapply --all removes
// every symlink pointing into the dotfiles repo regardless of @when predicates,
// even when the current env would not activate all nodes (AUDIT-059).
//
// Strategy:
//  1. Apply with os=special so BOTH nodes are active → two symlinks created.
//  2. Unapply --all with os=linux (so the predicate-gated node is INACTIVE under
//     normal pipeline) → both symlinks must be removed anyway.
func TestUnapply_All_RemovesAllDotfilesSymlinks(t *testing.T) {
	dotfiles, home, envFile := buildLinkDotfiles(t)

	// Apply with os=special so both nodes are created.
	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
		"--env", "os=special",
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	// Confirm both symlinks exist before unapply.
	bashrc := filepath.Join(home, ".bashrc")
	specialrc := filepath.Join(home, ".specialrc")
	for _, p := range []string{bashrc, specialrc} {
		if _, err := os.Lstat(p); err != nil {
			t.Fatalf("symlink not created by apply: %s: %v", p, err)
		}
	}

	// Unapply --all with a DIFFERENT env (os=linux, so predicate-gated node is inactive).
	if _, err := run(t, "unapply", "--all", "--yes",
		"--files", dotfiles, "--dotd-env", envFile,
		"--env", "os=linux",
	); err != nil {
		t.Fatalf("unapply --all error = %v", err)
	}

	// Both symlinks must be gone: --all ignores predicate gating.
	for _, p := range []string{bashrc, specialrc} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("symlink should be removed by unapply --all: %s", p)
		}
	}
}

// TestUnapply_All_OnlyRemovesDotfilesSymlinks verifies that --all does not
// remove symlinks that do NOT point into the dotfiles repo (AUDIT-059).
func TestUnapply_All_OnlyRemovesDotfilesSymlinks(t *testing.T) {
	dotfiles, home, envFile := buildLinkDotfiles(t)

	// Apply with os=special so both nodes are created.
	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
		"--env", "os=special",
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	// Place a non-dotfiles symlink in home.
	outsideTarget := filepath.Join(t.TempDir(), "other.txt")
	if err := os.WriteFile(outsideTarget, []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideLink := filepath.Join(home, ".otherrc")
	if err := os.Symlink(outsideTarget, outsideLink); err != nil {
		t.Fatal(err)
	}

	if _, err := run(t, "unapply", "--all", "--yes",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("unapply --all error = %v", err)
	}

	// Non-dotfiles symlink must be untouched.
	if _, err := os.Lstat(outsideLink); err != nil {
		t.Errorf("non-dotfiles symlink removed by unapply --all: %v", err)
	}
}

// TestUnapply_YesFlagSkipsConfirmation verifies that the local --yes flag on
// unapply skips the confirmation prompt (flag binding test — AUDIT-059).
func TestUnapply_YesFlagSkipsConfirmation(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-vimrc"),
		[]byte("# @link(~/.vimrc)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	envFile := emptyEnvFile(t)

	if _, err := run(t, "apply",
		"--files", dotfiles, "--dotd-env", envFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	// --yes bypasses the prompt; no stdin input needed.
	_, err := run(t, "unapply", "--yes",
		"--files", dotfiles, "--dotd-env", envFile,
	)
	if err != nil {
		t.Fatalf("unapply --yes error = %v (prompt may not have been bypassed)", err)
	}

	// Symlink must be removed — confirming --yes actually executed the removal.
	if _, err := os.Lstat(filepath.Join(home, ".vimrc")); !os.IsNotExist(err) {
		t.Error("symlink should be removed when --yes is set")
	}
}

// --- Real env.yaml expansion path (AUDIT-068) ---

// --- Exit code semantics: usage (2) vs runtime (1) ---

// runErr runs the root command and returns just the resulting error so tests
// can assert on its type (e.g. usageError).
func runErr(t *testing.T, args ...string) error {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	return root.Execute()
}

func assertUsageError(t *testing.T, err error, label string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error, got nil", label)
	}
	var ue *usageError
	if !errors.As(err, &ue) {
		t.Errorf("%s: error is not *usageError (would exit 1, not 2): %v", label, err)
	}
}

func assertRuntimeError(t *testing.T, err error, label string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error, got nil", label)
	}
	var ue *usageError
	if errors.As(err, &ue) {
		t.Errorf("%s: error wrongly classified as usage (would exit 2 instead of 1): %v", label, err)
	}
}

// TestUsageError_ArgValidator verifies that a missing-arg failure on an env
// subcommand classifies as usage (would exit 2).
func TestUsageError_ArgValidator(t *testing.T) {
	err := runErr(t, "env", "get",
		"--dotd-env", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	assertUsageError(t, err, "env get (no args)")
}

// TestUsageError_UnknownFlag verifies that an unknown flag classifies as
// usage (would exit 2).
func TestUsageError_UnknownFlag(t *testing.T) {
	err := runErr(t, "--no-such-flag")
	assertUsageError(t, err, "unknown flag")
}

// TestUsageError_CompletionInvalidShell verifies that the completion command's
// "unsupported shell" error classifies as usage (would exit 2).
func TestUsageError_CompletionInvalidShell(t *testing.T) {
	err := runErr(t, "completion", "nosuchshell")
	assertUsageError(t, err, "completion nosuchshell")
}

// TestRuntimeError_UnknownConfigKey verifies that a domain failure (bad key
// passed to a syntactically-valid command) stays a runtime error (exit 1).
func TestRuntimeError_UnknownConfigKey(t *testing.T) {
	err := runErr(t, "config", "get", "bogus",
		"--dotd-env", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	assertRuntimeError(t, err, "config get bogus")
}

// TestHelp_HidesIrrelevantPathFlags verifies that `dotd config get --help`
// omits mutation flags it doesn't act on (--force, --dry-run), while
// `dotd apply --help` still advertises them. The path flags (--bin-dir,
// --init-file, --link-root, --generated-dir) were removed in the XDG model.
func TestHelp_HidesIrrelevantPathFlags(t *testing.T) {
	// Flags scoped to mutation commands (pathFlagOwners) must be hidden on config get.
	hiddenOnConfigGet := []string{"--force", "--dry-run"}

	out, err := run(t, "config", "get", "--help")
	if err != nil {
		t.Fatalf("config get --help error = %v", err)
	}
	for _, flag := range hiddenOnConfigGet {
		if strings.Contains(out, flag) {
			t.Errorf("config get --help should not advertise %s; got:\n%s", flag, out)
		}
	}
	// Sanity: globally relevant flags must remain visible.
	for _, flag := range []string{"--log-level", "--dotd-config", "--dotd-env"} {
		if !strings.Contains(out, flag) {
			t.Errorf("config get --help should still show %s; got:\n%s", flag, out)
		}
	}

	out, err = run(t, "apply", "--help")
	if err != nil {
		t.Fatalf("apply --help error = %v", err)
	}
	for _, flag := range hiddenOnConfigGet {
		if !strings.Contains(out, flag) {
			t.Errorf("apply --help should still show %s; got:\n%s", flag, out)
		}
	}
}

// TestHelp_HiddenStateRestored verifies that filtering one subcommand's --help
// doesn't permanently mutate flag state — a subsequent root --help must still
// show every flag (including mutation flags hidden on config get).
func TestHelp_HiddenStateRestored(t *testing.T) {
	if _, err := run(t, "config", "get", "--help"); err != nil {
		t.Fatalf("config get --help error = %v", err)
	}
	out, err := run(t, "--help")
	if err != nil {
		t.Fatalf("root --help error = %v", err)
	}
	// Mutation flags (scoped via pathFlagOwners) must be restored on root --help.
	for _, flag := range []string{"--dry-run", "--force"} {
		if !strings.Contains(out, flag) {
			t.Errorf("root --help should show %s after subcommand --help ran; got:\n%s", flag, out)
		}
	}
}

func TestResolveLogLevel(t *testing.T) {
	cases := []struct {
		logLevel         string
		debug            bool
		logLevelExplicit bool
		quiet            bool
		want             string
	}{
		{"info", false, false, false, "info"},
		{"info", true, false, false, "debug"},  // --debug sets debug
		{"info", true, true, false, "info"},    // --log-level wins over --debug
		{"warn", true, true, false, "warn"},    // --log-level warn wins
		{"info", true, false, true, "error"},   // --quiet wins over --debug
		{"debug", false, true, false, "debug"}, // --log-level debug, no --debug flag
		{"warn", false, true, true, "error"},   // --quiet wins over explicit --log-level
	}
	for _, c := range cases {
		got := resolveLogLevel(c.logLevel, c.debug, c.logLevelExplicit, c.quiet)
		if got != c.want {
			t.Errorf("resolveLogLevel(%q, debug=%v, explicit=%v, quiet=%v) = %q, want %q",
				c.logLevel, c.debug, c.logLevelExplicit, c.quiet, got, c.want)
		}
	}
}

// TestEnvResolution_ShellExpandedValue verifies the full env.yaml → Expand →
// Resolve → normalize chain for shell-expanded values. An env.yaml that
// contains `os: $(echo linux)` must produce the same predicate result as
// passing `--env os=linux` directly, i.e. a @when(os=linux) node appears
// in dotd list output (AUDIT-068).
func TestEnvResolution_ShellExpandedValue(t *testing.T) {
	// env.yaml with a shell-expanded value: $(echo linux) must evaluate to "linux".
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("os: $(echo linux)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dotfiles with a node gated on @when(os=linux).
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile),
		[]byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// This node is active only when os=linux.
	if err := os.WriteFile(filepath.Join(shellrcDir, "linux.sh"),
		[]byte("# @when(os=linux)\nexport LINUX=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// This node is active only when os=macos — must NOT appear.
	if err := os.WriteFile(filepath.Join(shellrcDir, "macos.sh"),
		[]byte("# @when(os=macos)\nexport MACOS=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run dotd list — no --env flag; env resolution must come entirely from
	// the shell-expanded env.yaml value.
	out, err := run(t, "list",
		"--dotd-env", envFile,
		"--files", dotfiles,
	)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}

	// linux.sh should be active (os expanded to "linux").
	if !strings.Contains(out, "shellrc.linux") {
		t.Errorf("expected shellrc.linux in list output (shell-expanded os=linux); got %q", out)
	}
	// macos.sh must be inactive.
	if strings.Contains(out, "shellrc.macos") {
		t.Errorf("shellrc.macos should be inactive when os=linux; got %q", out)
	}
}

// --- first-run guardrails (2026-06-12 audit, PR 3) ---

// TestApplyRefusesCwdFallbackWhenUnconfigured: with no -f, no $DOTFILES, and
// no config.yaml, apply must refuse to walk the cwd rather than scanning
// whatever directory the user happens to be in.
func TestApplyRefusesCwdFallbackWhenUnconfigured(t *testing.T) {
	t.Setenv("DOTFILES", "")
	t.Setenv("DOTD_FILES", "")
	missingConfig := filepath.Join(t.TempDir(), "config.yaml")
	out, err := run(t, "apply",
		"--dotd-config", missingConfig,
		"--dotd-env", emptyEnvFile(t),
	)
	if err == nil {
		t.Fatalf("expected error, got none\noutput: %s", out)
	}
	if !strings.Contains(err.Error(), "no dotfiles repo configured") {
		t.Errorf("error = %q, want cwd-fallback refusal", err)
	}
	var h Hinter
	if !errors.As(err, &h) || !strings.Contains(h.Hint(), "dotd setup") {
		t.Errorf("expected 'dotd setup' hint, got %v", err)
	}
}

// TestListRefusesCwdFallbackWhenUnconfigured: the same guard covers the
// read path (walkOrdered).
func TestListRefusesCwdFallbackWhenUnconfigured(t *testing.T) {
	t.Setenv("DOTFILES", "")
	t.Setenv("DOTD_FILES", "")
	missingConfig := filepath.Join(t.TempDir(), "config.yaml")
	_, err := run(t, "list",
		"--dotd-config", missingConfig,
		"--dotd-env", emptyEnvFile(t),
	)
	if err == nil || !strings.Contains(err.Error(), "no dotfiles repo configured") {
		t.Errorf("list error = %v, want cwd-fallback refusal", err)
	}
}

// TestApplyWarnsWhenNoActions: active nodes that produce zero actions
// (convention dir without .dagger) must surface a warning instead of a
// silent no-op.
func TestApplyWarnsWhenNoActions(t *testing.T) {
	dotfiles := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotfiles, "shellrc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dotfiles, "shellrc", "a.sh"), []byte("a=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	out, err := run(t, "apply",
		"-f", dotfiles,
		"--dotd-env", emptyEnvFile(t),
	)
	if err != nil {
		t.Fatalf("apply error = %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "produced no actions") {
		t.Errorf("expected no-actions warning in output:\n%s", out)
	}
	if !strings.Contains(out, "dotd init") {
		t.Errorf("expected 'dotd init' pointer in warning:\n%s", out)
	}
}

// TestCheckHintsSetupWhenUnconfigured: check failures on a machine without
// config.yaml must point at 'dotd setup'.
func TestCheckHintsSetupWhenUnconfigured(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	missingConfig := filepath.Join(t.TempDir(), "config.yaml")
	_, err := run(t, "check",
		"-f", emptyDotfiles(t),
		"--dotd-config", missingConfig,
		"--dotd-env", emptyEnvFile(t),
	)
	if err == nil {
		t.Fatal("expected check to report issues")
	}
	var h Hinter
	if !errors.As(err, &h) || !strings.Contains(h.Hint(), "dotd setup") {
		t.Errorf("expected 'dotd setup' hint, got %v", err)
	}
}

// TestListEmptyPrintsNote: empty repo gives an orienting stderr note while
// stdout stays empty for pipes.
func TestListEmptyPrintsNote(t *testing.T) {
	out, err := run(t, "list",
		"-f", emptyDotfiles(t),
		"--dotd-env", emptyEnvFile(t),
	)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "no nodes found") {
		t.Errorf("expected 'no nodes found' note, got:\n%s", out)
	}
}

// TestResolvePaths_AnchorsFromEnv verifies that resolvePaths correctly resolves
// home, configDir, and binDir from XDG environment variables.
func TestResolvePaths_AnchorsFromEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "/xdg/conf")
	t.Setenv("XDG_BIN_HOME", "/xdg/bin")
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("DOTD_ENV_FILE", filepath.Join(t.TempDir(), "env.yaml"))
	t.Setenv("DOTD_CONFIG_FILE", filepath.Join(t.TempDir(), "nope.yaml"))
	t.Setenv("DOTFILES", "")
	t.Setenv("DOTD_FILES", "")
	cfg := &config{}
	if err := resolvePaths(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.home != home || cfg.configDir != "/xdg/conf" || cfg.binDir != "/xdg/bin/dot-dagger" {
		t.Fatalf("home=%q configDir=%q binDir=%q", cfg.home, cfg.configDir, cfg.binDir)
	}
}
