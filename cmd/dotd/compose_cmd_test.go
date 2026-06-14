package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
)

// --- dotd compose check exit codes (2026-06-13 audit, PR 1) ---

// composeDotfiles creates a minimal dotfiles repo containing one compose
// target (gen.sh.d → gen.sh) and returns the repo plus a fresh generated dir.
func composeDotfiles(t *testing.T) (dotfiles, generatedDir string) {
	t.Helper()
	// Hermetic env: paths resolve from HOME/XDG, not the real machine.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dataHome := filepath.Join(tmp, "data")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_BIN_HOME", filepath.Join(tmp, "bin"))

	dotfiles = t.TempDir()
	target := filepath.Join(dotfiles, "gen.sh.d")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	dagger := "composition:\n  enabled: true\nactions:\n  - source\n"
	if err := os.WriteFile(filepath.Join(target, ".dagger"), []byte(dagger), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "a.sh"), []byte("a=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// generated files resolve to $XDG_DATA_HOME/dot-dagger/generated
	return dotfiles, filepath.Join(dataHome, ecosystem.Name, "generated")
}

// composeFlags returns the common flag set for compose tests. Paths now resolve
// from the environment (set in composeDotfiles), so only the repo + env file
// remain as flags. generatedDir is unused but kept for call-site symmetry.
func composeFlags(t *testing.T, dotfiles, generatedDir string) []string {
	t.Helper()
	_ = generatedDir
	return []string{
		"-f", dotfiles,
		"--dotd-env", emptyEnvFile(t),
	}
}

// TestComposeCheckExitsNonZeroWhenMissing: a compose target whose generated
// file has never been written must fail the check (the help text promises a
// non-zero exit).
func TestComposeCheckExitsNonZeroWhenMissing(t *testing.T) {
	dotfiles, generatedDir := composeDotfiles(t)

	out, err := run(t, append([]string{"compose", "check"}, composeFlags(t, dotfiles, generatedDir)...)...)
	if err == nil {
		t.Fatalf("expected non-zero exit for missing generated file\noutput: %s", out)
	}
	if !strings.Contains(err.Error(), "stale or missing") {
		t.Errorf("error = %q, want stale-or-missing message", err)
	}
	var h Hinter
	if !errors.As(err, &h) || !strings.Contains(h.Hint(), "dotd apply") {
		t.Errorf("expected 'dotd apply' hint, got %v", err)
	}
}

// TestComposeCheckCleanAfterApply: apply writes the generated file; check
// must then exit zero.
func TestComposeCheckCleanAfterApply(t *testing.T) {
	dotfiles, generatedDir := composeDotfiles(t)
	flags := composeFlags(t, dotfiles, generatedDir)

	if out, err := run(t, append([]string{"apply"}, flags...)...); err != nil {
		t.Fatalf("apply error = %v\noutput: %s", err, out)
	}
	if out, err := run(t, append([]string{"compose", "check"}, flags...)...); err != nil {
		t.Fatalf("compose check after apply should exit 0, got %v\noutput: %s", err, out)
	}
}

// TestComposeCheckExitsNonZeroWhenStale: modifying the generated file after
// apply must fail the check.
func TestComposeCheckExitsNonZeroWhenStale(t *testing.T) {
	dotfiles, generatedDir := composeDotfiles(t)
	flags := composeFlags(t, dotfiles, generatedDir)

	if out, err := run(t, append([]string{"apply"}, flags...)...); err != nil {
		t.Fatalf("apply error = %v\noutput: %s", err, out)
	}
	gen := filepath.Join(generatedDir, "gen.sh")
	if err := os.WriteFile(gen, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, append([]string{"compose", "check"}, flags...)...)
	if err == nil {
		t.Fatalf("expected non-zero exit for stale generated file\noutput: %s", out)
	}
}

// TestComposeCheckJSONExitsNonZero: --json must report status AND still exit
// non-zero so scripts don't need to parse the array to learn the verdict.
func TestComposeCheckJSONExitsNonZero(t *testing.T) {
	dotfiles, generatedDir := composeDotfiles(t)

	out, err := run(t, append([]string{"compose", "check", "--json"}, composeFlags(t, dotfiles, generatedDir)...)...)
	if err == nil {
		t.Fatalf("expected non-zero exit for --json with missing file\noutput: %s", out)
	}
	if !strings.Contains(out, `"status": "missing"`) {
		t.Errorf("expected JSON status entry in output: %q", out)
	}
}
