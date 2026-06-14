//go:build integration

package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----------------------------------------------------------------------------
// Test environment helpers
// ----------------------------------------------------------------------------

// ienv holds isolated paths for one integration test run.
type ienv struct {
	dotfiles     string // copy of the fixture (writable)
	home         string // fake $HOME for symlinks
	initFile     string // where init.sh is written
	binDir       string // fake bin directory
	generatedDir string // where compose-generated files are written
}

// newIenv copies the fixture into a temp dir and returns isolated paths.
func newIenv(t *testing.T) *ienv {
	t.Helper()
	tmp := t.TempDir()

	dotfiles := filepath.Join(tmp, "dotfiles")
	copyFixture(t, "testdata/dotfiles", dotfiles)

	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}

	// Paths resolve from the environment (pure-XDG model): "~" = $HOME, and
	// init.sh + generated live under $XDG_DATA_HOME. The bin fixture uses an
	// explicit "~/bin/hello" dest, so binDir is $HOME/bin. $config defaults to
	// $HOME/.config (XDG_CONFIG_HOME left unset).
	t.Setenv("HOME", home)
	xdgData := filepath.Join(tmp, "data")
	t.Setenv("XDG_DATA_HOME", xdgData)

	return &ienv{
		dotfiles:     dotfiles,
		home:         home,
		initFile:     filepath.Join(xdgData, "dot-dagger", "init.sh"),
		binDir:       filepath.Join(home, "bin"),
		generatedDir: filepath.Join(xdgData, "dot-dagger", "generated"),
	}
}

// run executes a dotd subcommand with the test environment wired up.
// It fails the test immediately on non-zero exit.
func (e *ienv) run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := e.runMayFail(t, args...)
	if err != nil {
		t.Fatalf("dotd %s failed: %v\noutput:\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// runMayFail executes a dotd subcommand and returns output + error without
// failing the test. Use this to test expected failure cases.
func (e *ienv) runMayFail(t *testing.T, args ...string) (string, error) {
	t.Helper()
	base := []string{
		"--files", e.dotfiles,
		"--dotd-env", filepath.Join(e.dotfiles, "env.yaml"),
	}
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append(base, args...))
	err := cmd.Execute()
	return buf.String(), err
}

func (e *ienv) runWithStdin(t *testing.T, stdin io.Reader, args ...string) (string, error) {
	t.Helper()
	base := []string{
		"--files", e.dotfiles,
		"--dotd-env", filepath.Join(e.dotfiles, "env.yaml"),
	}
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetIn(stdin)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append(base, args...))
	err := cmd.Execute()
	return buf.String(), err
}

// copyFixture recursively copies src into dst, preserving file modes.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatalf("copy fixture %s → %s: %v", src, dst, err)
	}
}

// ----------------------------------------------------------------------------
// Assertion helpers
// ----------------------------------------------------------------------------

// assertSymlink checks that link exists and points to target.
func assertSymlink(t *testing.T, link, target string) {
	t.Helper()
	got, err := os.Readlink(link)
	if err != nil {
		t.Errorf("symlink %s: %v", link, err)
		return
	}
	if got != target {
		t.Errorf("symlink %s\n  got  → %s\n  want → %s", link, got, target)
	}
}

// assertNoPath checks that path does not exist at all.
func assertNoPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("expected %s to not exist", path)
	}
}

// assertInInit checks that the init.sh source line for scriptName is present.
func assertInInit(t *testing.T, initContent, scriptName string) {
	t.Helper()
	if !strings.Contains(initContent, scriptName) {
		t.Errorf("init.sh: expected to contain %q", scriptName)
	}
}

// assertNotInInit checks that scriptName is absent from init.sh.
func assertNotInInit(t *testing.T, initContent, scriptName string) {
	t.Helper()
	if strings.Contains(initContent, scriptName) {
		t.Errorf("init.sh: expected NOT to contain %q", scriptName)
	}
}

// assertOrder checks that scripts appear in init.sh in the given order.
// All scripts must be present; fails if any are missing.
func assertOrder(t *testing.T, initContent string, scripts ...string) {
	t.Helper()
	pos := make([]int, len(scripts))
	for i, s := range scripts {
		p := strings.Index(initContent, s)
		if p < 0 {
			t.Fatalf("init.sh: %q not found (needed for order check)", s)
		}
		pos[i] = p
	}
	for i := 1; i < len(pos); i++ {
		if pos[i] <= pos[i-1] {
			t.Errorf("init.sh: expected %q before %q", scripts[i-1], scripts[i])
		}
	}
}

// readFile reads a file and returns its contents, failing the test on error.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

// TestApplyLinuxPersonal verifies the full pipeline on a Linux/personal machine.
// Expected active: base, path, aliases, linux — NOT macos, work.
// Expected symlinks: .zshrc, .gitconfig (personal), .config/nvim/init.lua, bin/hello.
func TestApplyLinuxPersonal(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	// --- symlinks ---
	assertSymlink(t,
		filepath.Join(e.home, ".zshrc"),
		filepath.Join(e.dotfiles, "config/dot-zshrc"),
	)
	assertSymlink(t,
		filepath.Join(e.home, ".gitconfig"),
		filepath.Join(e.dotfiles, "config/dot-gitconfig"),
	)
	assertSymlink(t,
		filepath.Join(e.home, ".config", "nvim", "init.lua"),
		filepath.Join(e.dotfiles, "config/dot-config/nvim/init.lua"),
	)
	assertSymlink(t,
		filepath.Join(e.binDir, "hello"),
		filepath.Join(e.dotfiles, "bin/hello"),
	)

	// --- init.sh contents ---
	init := readFile(t, e.initFile)
	assertInInit(t, init, "base.sh")
	assertInInit(t, init, "path.sh")
	assertInInit(t, init, "aliases.sh")
	assertInInit(t, init, "linux.sh")
	assertNotInInit(t, init, "macos.sh")
	assertNotInInit(t, init, "work.sh")

	// --- load order: base → path → aliases, base → linux ---
	assertOrder(t, init, "base.sh", "path.sh", "aliases.sh")
	assertOrder(t, init, "base.sh", "linux.sh")
}

// TestApplyMacOSPersonal verifies the pipeline on a macOS/personal machine.
// Expected active: base, path, aliases, macos — NOT linux, work.
func TestApplyMacOSPersonal(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=macos", "--env", "context=personal")

	init := readFile(t, e.initFile)
	assertInInit(t, init, "base.sh")
	assertInInit(t, init, "path.sh")
	assertInInit(t, init, "aliases.sh")
	assertInInit(t, init, "macos.sh")
	assertNotInInit(t, init, "linux.sh")
	assertNotInInit(t, init, "work.sh")

	// macos.sh depends on base; base must come first
	assertOrder(t, init, "base.sh", "macos.sh")
	assertOrder(t, init, "base.sh", "path.sh", "aliases.sh")
}

// TestApplyLinuxWork verifies the pipeline on a Linux/work machine.
// Expected active: base, path, aliases, linux, work — NOT macos.
// gitconfig should NOT be linked (context=work, annotation says context=personal).
func TestApplyLinuxWork(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=work")

	init := readFile(t, e.initFile)
	assertInInit(t, init, "base.sh")
	assertInInit(t, init, "aliases.sh")
	assertInInit(t, init, "linux.sh")
	assertInInit(t, init, "work.sh")
	assertNotInInit(t, init, "macos.sh")

	// work.sh depends on aliases
	assertOrder(t, init, "aliases.sh", "work.sh")

	// .gitconfig should NOT be created — it's gated on context=personal
	assertNoPath(t, filepath.Join(e.home, ".gitconfig"))
}

// TestDryRunNoChanges verifies that --dry-run writes nothing to disk.
func TestDryRunNoChanges(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal", "--dry-run")

	assertNoPath(t, filepath.Join(e.home, ".zshrc"))
	assertNoPath(t, e.initFile)
	assertNoPath(t, filepath.Join(e.binDir, "hello"))
}

// TestCheckAfterApply verifies that `dotd check` exits cleanly after a successful apply.
func TestCheckAfterApply(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")
	e.run(t, "check", "--env", "os=linux", "--env", "context=personal")
}

// TestMultipleAppliesIdempotent verifies that running apply twice produces the same result.
func TestMultipleAppliesIdempotent(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")
	init1 := readFile(t, e.initFile)

	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")
	init2 := readFile(t, e.initFile)

	if init1 != init2 {
		t.Error("init.sh changed between two identical apply runs")
	}
}

// TestDAGVerboseOrder verifies the dag order output lists scripts in dependency order.
func TestDAGVerboseOrder(t *testing.T) {
	e := newIenv(t)
	out := e.run(t, "dag", "order", "--log-level", "debug", "--env", "os=linux", "--env", "context=work")

	// Verbose output lists scripts numbered; verify ordering by finding line numbers.
	lines := strings.Split(out, "\n")
	pos := map[string]int{}
	for i, line := range lines {
		for _, name := range []string{"base.sh", "path.sh", "aliases.sh", "linux.sh", "work.sh"} {
			if strings.Contains(line, name) {
				pos[name] = i
			}
		}
	}

	checkBefore := func(a, b string) {
		t.Helper()
		if pos[a] == 0 && pos[b] == 0 {
			return // not found — other assertions will catch missing scripts
		}
		if pos[a] >= pos[b] {
			t.Errorf("dag order: expected %q before %q in verbose output", a, b)
		}
	}
	checkBefore("base.sh", "path.sh")
	checkBefore("path.sh", "aliases.sh")
	checkBefore("aliases.sh", "work.sh")
}

// TestSymlinkConflictWithForce verifies that --force overwrites a conflicting plain file.
func TestSymlinkConflictWithForce(t *testing.T) {
	e := newIenv(t)

	// Create a plain file where the symlink would go.
	if err := os.WriteFile(filepath.Join(e.home, ".zshrc"), []byte("old content"), 0o644); err != nil {
		t.Fatalf("setup conflict: %v", err)
	}

	// Without --force, apply MUST error AND leave the plain file intact.
	_, err := e.runMayFail(t, "apply", "--env", "os=linux")
	if err == nil {
		t.Fatal("apply without --force must error when a plain file blocks the symlink target")
	}
	// The plain file at ~/.zshrc must NOT have been replaced with a symlink.
	if fi, statErr := os.Lstat(filepath.Join(e.home, ".zshrc")); statErr != nil {
		t.Fatalf("zshrc disappeared after failed apply: %v", statErr)
	} else if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("zshrc was replaced with a symlink despite --force absence")
	}

	// With --force it should succeed and the symlink should be correct.
	e.run(t, "apply", "--env", "os=linux", "--force")
	assertSymlink(t,
		filepath.Join(e.home, ".zshrc"),
		filepath.Join(e.dotfiles, "config/dot-zshrc"),
	)
}

// ----------------------------------------------------------------------------
// Package management tests
// ----------------------------------------------------------------------------
//
// Strategy: avoid real package managers entirely.
//
//   fake-installed  — binary: sh, which is always on PATH.
//                     @require fake-installed always passes without installing anything.
//
//   not-installable — no managers defined, so it can never be installed.
//                     @require not-installable always hard-fails.
//                     @request not-installable always silently skips.
//
// This makes package tests deterministic on any machine.

// TestPackageRequireMet verifies that apply succeeds when a @require package is installed.
// needs-fake.sh has @require fake-installed (binary=sh, always on PATH).
func TestPackageRequireMet(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	// needs-fake.sh should be in init.sh — its @require was satisfied
	init := readFile(t, e.initFile)
	assertInInit(t, init, "needs-fake.sh")
}

// TestPackageRequestSoftSkip verifies that apply succeeds even when a @request
// package is not installable. optional-tool.sh has @request not-installable.
func TestPackageRequestSoftSkip(t *testing.T) {
	e := newIenv(t)
	// Should succeed — @request is soft, missing package is silently skipped.
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	// The script itself is still active (condition passes); only the package install is skipped.
	init := readFile(t, e.initFile)
	assertInInit(t, init, "optional-tool.sh")
}

// TestPackageRequireHardFail verifies that `dotd package generate` fails when a
// @require package has no installable manager. apply itself no longer validates
// packages — that is the responsibility of `dotd package generate`.
func TestPackageRequireHardFail(t *testing.T) {
	e := newIenv(t)

	// Write a script with a hard requirement that can never be met.
	script := "#!/bin/bash\n# @require(not-installable)\necho hi\n"
	path := filepath.Join(e.dotfiles, "shellrc", "hard-fail.sh")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatalf("write hard-fail.sh: %v", err)
	}

	_, err := e.runMayFail(t, "package", "generate", "--env", "os=linux")
	if err == nil {
		t.Error("expected package generate to fail when @require package is not installable")
	}
}

// TestPackageCheckOutput verifies that `dotd package check` reports the right
// status for each package type.
func TestPackageCheckOutput(t *testing.T) {
	e := newIenv(t)
	out := e.run(t, "package", "check", "--env", "os=linux", "--env", "context=personal")

	// fake-installed should show as installed (binary=sh is on PATH)
	if !strings.Contains(out, "fake-installed") || !strings.Contains(out, "installed") {
		t.Errorf("expected fake-installed to show as installed\noutput: %s", out)
	}

	// not-installable should show as not available
	if !strings.Contains(out, "not-installable") {
		t.Errorf("expected not-installable to appear in output\noutput: %s", out)
	}
}

// TestPackageDryRun verifies that --dry-run skips actual installation.
// We use a script with @require fake-installed — in non-dry-run this is a no-op
// (already installed), but dry-run should also exit cleanly.
func TestPackageDryRun(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal", "--dry-run")
	// If we reached here without error, dry-run handled packages correctly.
}

// ----------------------------------------------------------------------------
// Compose tests
// ----------------------------------------------------------------------------

// TestComposeApply verifies that apply assembles compose targets:
//   - shellrc compose target generates a file that is sourced in init.sh
//   - config compose target generates a file and creates a symlink
func TestComposeApply(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	// shellrc compose target: generated file sourced in init.sh
	init := readFile(t, e.initFile)
	assertInInit(t, init, "shellrc-extras.sh")

	// config compose target: generated file exists
	generatedTmux := filepath.Join(e.generatedDir, "tmux.conf")
	if _, err := os.Stat(generatedTmux); err != nil {
		t.Errorf("compose: generated tmux.conf missing: %v", err)
	}
	// conf compose target: symlink ~/.tmux.conf → generatedDir/tmux.conf
	assertSymlink(t, filepath.Join(e.home, ".tmux.conf"), generatedTmux)
}

// TestComposePredicateGating verifies that inactive fragments are excluded from
// the generated file and active ones are included.
func TestComposePredicateGating(t *testing.T) {
	// context=personal: nosync-work.sh inactive
	ep := newIenv(t)
	ep.run(t, "apply", "--env", "os=linux", "--env", "context=personal")
	personal := readFile(t, filepath.Join(ep.generatedDir, "shellrc-extras.sh"))
	if strings.Contains(personal, "EXTRAS_WORK") {
		t.Error("work fragment should not appear in personal context")
	}
	if !strings.Contains(personal, "EXTRAS_BASE") {
		t.Error("base fragment should appear in personal context")
	}

	// context=work: nosync-work.sh active
	ew := newIenv(t)
	ew.run(t, "apply", "--env", "os=linux", "--env", "context=work")
	work := readFile(t, filepath.Join(ew.generatedDir, "shellrc-extras.sh"))
	if !strings.Contains(work, "EXTRAS_WORK") {
		t.Error("work fragment should appear in work context")
	}
}

// TestComposeList verifies that dotd compose list reports active compose targets.
func TestComposeList(t *testing.T) {
	e := newIenv(t)
	out := e.run(t, "compose", "list", "--env", "os=linux", "--env", "context=personal")

	if !strings.Contains(out, "shellrc-extras.sh") {
		t.Errorf("compose list: missing shellrc-extras.sh target\noutput: %s", out)
	}
	if !strings.Contains(out, "tmux.conf") {
		t.Errorf("compose list: missing tmux.conf target\noutput: %s", out)
	}
}

// TestComposeCheck_AfterApply verifies that compose check exits cleanly after apply.
func TestComposeCheck_AfterApply(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")
	e.run(t, "compose", "check", "--env", "os=linux", "--env", "context=personal")
}

// TestComposeCheck_Stale verifies that compose check detects a stale generated file.
func TestComposeCheck_Stale(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	// Overwrite the generated file with stale content.
	stalePath := filepath.Join(e.generatedDir, "shellrc-extras.sh")
	if err := os.WriteFile(stalePath, []byte("stale content\n"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	out, err := e.runMayFail(t, "compose", "check", "--env", "os=linux", "--env", "context=personal")
	if err == nil {
		t.Errorf("compose check: expected non-zero exit for stale target\noutput: %s", out)
	}
	if !strings.Contains(out, "stale") {
		t.Errorf("compose check: expected stale report\noutput: %s", out)
	}
}

func TestUnapplyAfterApply(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	assertSymlink(t,
		filepath.Join(e.home, ".zshrc"),
		filepath.Join(e.dotfiles, "config/dot-zshrc"),
	)
	if _, err := os.Stat(e.initFile); err != nil {
		t.Fatalf("init.sh not created by apply: %v", err)
	}

	e.run(t, "unapply", "--yes", "--env", "os=linux", "--env", "context=personal")

	assertNoPath(t, filepath.Join(e.home, ".zshrc"))
	assertNoPath(t, filepath.Join(e.home, ".gitconfig"))
	assertNoPath(t, filepath.Join(e.binDir, "hello"))
	assertNoPath(t, e.initFile)
}

func TestUnapplyCancel(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	zshrc := filepath.Join(e.home, ".zshrc")
	assertSymlink(t, zshrc, filepath.Join(e.dotfiles, "config/dot-zshrc"))

	out, err := e.runWithStdin(t, strings.NewReader("n\n"),
		"unapply", "--env", "os=linux", "--env", "context=personal")
	if err != nil {
		t.Fatalf("unapply cancel: %v\noutput: %s", err, out)
	}

	assertSymlink(t, zshrc, filepath.Join(e.dotfiles, "config/dot-zshrc"))
	if !strings.Contains(out, "cancelled") {
		t.Errorf("expected 'cancelled' in output: %q", out)
	}
}

func TestSetupThenTeardown(t *testing.T) {
	xdg := t.TempDir()
	dotfilesDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", dotfilesDir)

	setupCmd := newRootCmd()
	setupCmd.SetIn(strings.NewReader(strings.Repeat("\n", 10)))
	var setupBuf bytes.Buffer
	setupCmd.SetOut(&setupBuf)
	setupCmd.SetErr(&setupBuf)
	setupCmd.SetArgs([]string{"setup"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, setupBuf.String())
	}

	configPath := filepath.Join(xdg, "dot-dagger", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml not written by setup: %v", err)
	}

	teardownCmd := newRootCmd()
	teardownCmd.SetIn(strings.NewReader("y\n"))
	var teardownBuf bytes.Buffer
	teardownCmd.SetOut(&teardownBuf)
	teardownCmd.SetErr(&teardownBuf)
	teardownCmd.SetArgs([]string{"teardown",
		"--files", dotfilesDir,
	})
	if err := teardownCmd.Execute(); err != nil {
		t.Fatalf("teardown error = %v\noutput:\n%s", err, teardownBuf.String())
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config.yaml should be removed by teardown")
	}
	if _, err := os.Stat(filepath.Join(xdg, "dot-dagger", "env.yaml")); !os.IsNotExist(err) {
		t.Error("env.yaml should be removed by teardown")
	}
}

func TestInitAfterSetup(t *testing.T) {
	xdg := t.TempDir()
	dotfilesDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", dotfilesDir)

	setupCmd := newRootCmd()
	setupCmd.SetIn(strings.NewReader(strings.Repeat("\n", 10)))
	var setupBuf bytes.Buffer
	setupCmd.SetOut(&setupBuf)
	setupCmd.SetErr(&setupBuf)
	setupCmd.SetArgs([]string{"setup"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, setupBuf.String())
	}

	// y\n = create dir, \n = accept default name; 3 dirs; then n\n = skip source-line prompt
	initCmd := newRootCmd()
	initCmd.SetIn(strings.NewReader("y\n\ny\n\ny\n\nn\n"))
	var initBuf bytes.Buffer
	initCmd.SetOut(&initBuf)
	initCmd.SetErr(&initBuf)
	initCmd.SetArgs([]string{"init",
		"--files", dotfilesDir,
		"--dotd-env", filepath.Join(dotfilesDir, "env.yaml"),
	})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init error = %v\noutput:\n%s", err, initBuf.String())
	}

	for _, dir := range []string{"shellrc", "config", "bin"} {
		daggerPath := filepath.Join(dotfilesDir, dir, ".dagger")
		if _, err := os.Stat(daggerPath); err != nil {
			t.Errorf(".dagger not scaffolded in %s/: %v", dir, err)
		}
	}
}

// TestInitNonInteractive verifies that 'dotd init -n' scaffolds all three
// convention directories under their default names without reading stdin.
func TestInitNonInteractive(t *testing.T) {
	xdg := t.TempDir()
	dotfilesDir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // keeps the RC source line away from the real home dir
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", dotfilesDir)

	setupCmd := newRootCmd()
	setupCmd.SetIn(strings.NewReader(""))
	var setupBuf bytes.Buffer
	setupCmd.SetOut(&setupBuf)
	setupCmd.SetErr(&setupBuf)
	setupCmd.SetArgs([]string{"setup", "-n"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("setup -n error = %v\noutput:\n%s", err, setupBuf.String())
	}

	initCmd := newRootCmd()
	initCmd.SetIn(strings.NewReader("")) // EOF immediately — -n must not need input
	var initBuf bytes.Buffer
	initCmd.SetOut(&initBuf)
	initCmd.SetErr(&initBuf)
	initCmd.SetArgs([]string{"init", "-n",
		"--files", dotfilesDir,
		"--dotd-env", filepath.Join(dotfilesDir, "env.yaml"),
	})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init -n error = %v\noutput:\n%s", err, initBuf.String())
	}

	for _, dir := range []string{"shellrc", "config", "bin"} {
		daggerPath := filepath.Join(dotfilesDir, dir, ".dagger")
		if _, err := os.Stat(daggerPath); err != nil {
			t.Errorf(".dagger not scaffolded in %s/: %v", dir, err)
		}
	}
	if strings.Contains(initBuf.String(), "skipping") {
		t.Errorf("init -n skipped a directory:\n%s", initBuf.String())
	}
}

// TestSetupNonInteractivePrintsValues verifies that 'dotd setup -n' shows the
// values it accepted, not just the section labels.
func TestSetupNonInteractivePrintsValues(t *testing.T) {
	xdg := t.TempDir()
	dotfilesDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", dotfilesDir)

	setupCmd := newRootCmd()
	setupCmd.SetIn(strings.NewReader(""))
	var buf bytes.Buffer
	setupCmd.SetOut(&buf)
	setupCmd.SetErr(&buf)
	setupCmd.SetArgs([]string{"setup", "-n"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("setup -n error = %v\noutput:\n%s", err, buf.String())
	}

	if !strings.Contains(buf.String(), dotfilesDir) {
		t.Errorf("setup -n output does not show the accepted dotfiles path %q:\n%s", dotfilesDir, buf.String())
	}
}

// TestConfigCmdsLifecycle exercises config show / set / get as a lifecycle.
func TestConfigCmdsLifecycle(t *testing.T) {
	// config show on missing file exits 0 with dotfiles= in output
	missingConfig := filepath.Join(t.TempDir(), "config.yaml")
	out, err := run(t, "config", "show",
		"--dotd-config", missingConfig,
		"--dotd-env", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config show on missing file: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "dotfiles=") {
		t.Errorf("config show: expected dotfiles= in output: %q", out)
	}

	// config set dotfiles /tmp/x → config get dotfiles returns /tmp/x
	configPath := writeConfigYAML(t, "{}\n")
	if _, err = run(t, "config", "set", "dotfiles", "/tmp/x",
		"--dotd-config", configPath,
		"--dotd-env", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	); err != nil {
		t.Fatalf("config set error = %v", err)
	}

	out, err = run(t, "config", "get", "dotfiles",
		"--dotd-config", configPath,
		"--dotd-env", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config get error = %v", err)
	}
	if strings.TrimSpace(out) != "/tmp/x" {
		t.Errorf("config get dotfiles = %q, want /tmp/x", strings.TrimSpace(out))
	}
}

// TestAdoptShellScript_Integration verifies that adopt moves a file into the
// dotfiles repo and removes the original source.
func TestAdoptShellScript_Integration(t *testing.T) {
	e := newIenv(t)

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "newscript.sh")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatalf("write srcPath: %v", err)
	}

	e.run(t, "adopt", "--yes", srcPath, "--to", "shellrc/")

	assertNoPath(t, srcPath)

	dest := filepath.Join(e.dotfiles, "shellrc", "newscript.sh")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("adopted file not found at %s: %v", dest, err)
	}
}

// TestBundleOutput_Integration verifies that bundling aliases.sh includes its
// transitive dependencies (path.sh and base.sh), producing DOT_BASE_LOADED in output.
func TestBundleOutput_Integration(t *testing.T) {
	e := newIenv(t)

	out := e.run(t, "bundle", "shellrc/aliases.sh",
		"--env", "os=linux", "--env", "context=personal")

	if !strings.Contains(out, "DOT_BASE_LOADED") {
		t.Errorf("bundle: expected DOT_BASE_LOADED (from base.sh) in output: %q", out)
	}
}

// TestEnvCmdsLifecycle exercises env show / set / get / diff as a lifecycle.
func TestEnvCmdsLifecycle(t *testing.T) {
	e := newIenv(t)

	// env show: testdata env.yaml has context=personal
	out := e.run(t, "env", "show")
	if !strings.Contains(out, "context=personal") {
		t.Errorf("env show: expected context=personal: %q", out)
	}

	// env set context staging
	e.run(t, "env", "set", "context", "staging")

	// env get context → staging
	out = e.run(t, "env", "get", "context")
	if strings.TrimSpace(out) != "staging" {
		t.Errorf("env get context = %q, want staging", strings.TrimSpace(out))
	}

	// env diff: file has context=staging; DOTD_CONTEXT is not set in test env
	// so diff reports context as an override (unset in shell → "staging" in file).
	// --env os=macos is a CLI override, not a file value — env diff ignores it.
	t.Setenv("DOTD_CONTEXT", "") // ensure shell var is absent for determinism
	out = e.run(t, "env", "diff", "--env", "os=macos")
	if !strings.Contains(out, "context") {
		t.Errorf("env diff: expected 'context' in output: %q", out)
	}
}

// TestAnnotate_AddWhen drives the wizard to add a @when(os=macos) annotation.
// Accessible-mode input: select When (1), enter value, select Done (8), confirm Yes.
func TestAnnotate_AddWhen(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	// aliases.sh already has @after(shellrc.path). We're adding @when on top.
	// Menu: 8 options (7 types + Done). Select 1=When, input value, 8=Done, y=Yes.
	stdin := strings.NewReader("1\nos=macos\n8\ny\n")
	out, err := e.runWithStdin(t, stdin, "annotate", target)
	if err != nil {
		t.Fatalf("annotate wizard failed: %v\noutput:\n%s", err, out)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	got := string(content)
	if !strings.Contains(got, "# @when(os=macos)") {
		t.Errorf("expected @when(os=macos) in file, got:\n%s", got)
	}
	if !strings.Contains(got, "# @after(shellrc.path)") {
		t.Errorf("expected original @after(shellrc.path) preserved, got:\n%s", got)
	}
}

// TestAnnotate_SetAction drives the wizard to set @action(source).
// Accessible-mode input: select Action (5), select source (1 of [source,no-source,link,none]), Done (8), Yes.
func TestAnnotate_SetAction(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	stdin := strings.NewReader("5\n1\n8\ny\n")
	out, err := e.runWithStdin(t, stdin, "annotate", target)
	if err != nil {
		t.Fatalf("annotate wizard failed: %v\noutput:\n%s", err, out)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(content), "# @action(source)") {
		t.Errorf("expected @action(source) in file, got:\n%s", string(content))
	}
}

// TestAnnotate_SetDisable drives the wizard to set @disable.
// Accessible-mode input: select Disable (7), confirm Set (y), Done (8), confirm write (y).
func TestAnnotate_SetDisable(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	stdin := strings.NewReader("7\ny\n8\ny\n")
	out, err := e.runWithStdin(t, stdin, "annotate", target)
	if err != nil {
		t.Fatalf("annotate wizard failed: %v\noutput:\n%s", err, out)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(content), "# @disable") {
		t.Errorf("expected @disable in file, got:\n%s", string(content))
	}
}

// TestAnnotate_CancelAtConfirm drives the wizard to Done with no changes,
// then cancels at the final confirm. The file must be unmodified.
func TestAnnotate_CancelAtConfirm(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	// Done immediately (8), then No at confirm (n).
	stdin := strings.NewReader("8\nn\n")
	out, runErr := e.runWithStdin(t, stdin, "annotate", target)
	if runErr != nil {
		t.Fatalf("annotate wizard failed: %v\noutput:\n%s", runErr, out)
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("file changed after cancel; before:\n%s\nafter:\n%s", before, after)
	}
}
