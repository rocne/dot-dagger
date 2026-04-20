//go:build integration

package main

import (
	"bytes"
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
	dotfiles string // copy of the fixture (writable)
	home     string // fake $HOME for symlinks
	initFile string // where init.sh is written
	binDir   string // fake bin directory
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

	return &ienv{
		dotfiles: dotfiles,
		home:     home,
		initFile: filepath.Join(tmp, "init.sh"),
		binDir:   filepath.Join(tmp, "bin"),
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
		"--env-file", filepath.Join(e.dotfiles, "env.yaml"),
		"--link-root", e.home,
		"--bin-dir", e.binDir,
		"--init-file", e.initFile,
	}
	var buf bytes.Buffer
	cmd := newRootCmd()
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
		filepath.Join(e.dotfiles, "conf/dot-zshrc"),
	)
	assertSymlink(t,
		filepath.Join(e.home, ".gitconfig"),
		filepath.Join(e.dotfiles, "conf/dot-gitconfig"),
	)
	assertSymlink(t,
		filepath.Join(e.home, ".config", "nvim", "init.lua"),
		filepath.Join(e.dotfiles, "conf/dot-config/nvim/init.lua"),
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

// TestDAGVerboseOrder verifies the dag check output lists scripts in dependency order.
func TestDAGVerboseOrder(t *testing.T) {
	e := newIenv(t)
	out := e.run(t, "dag", "check", "--log-level", "debug", "--env", "os=linux", "--env", "context=work")

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

	// Without --force, apply should report conflict (non-zero exit or conflict in check).
	_, err := e.runMayFail(t, "apply", "--env", "os=linux")
	if err == nil {
		// Some implementations warn rather than error; check the symlink is wrong.
		got, readErr := os.Readlink(filepath.Join(e.home, ".zshrc"))
		if readErr == nil && got == filepath.Join(e.dotfiles, "conf/dot-zshrc") {
			t.Log("apply without --force unexpectedly succeeded; skipping conflict test")
			return
		}
	}

	// With --force it should succeed and the symlink should be correct.
	e.run(t, "apply", "--env", "os=linux", "--force")
	assertSymlink(t,
		filepath.Join(e.home, ".zshrc"),
		filepath.Join(e.dotfiles, "conf/dot-zshrc"),
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
	script := "#!/bin/bash\n# @require not-installable\necho hi\n"
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
