package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
)

// makeShellScript creates a .sh file in dir with the given content and returns its path.
func makeShellScript(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("# shell script\nexport FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// makeExecutable creates a file with the executable bit set and returns its path.
func makeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// makeConfigFile creates a plain config file and returns its path.
func makeConfigFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("[section]\nkey = value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// adoptDotfiles returns a temp dotfiles dir with a root-level .dagger (empty conventions → defaults).
func adoptDotfiles(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Root .dagger with no conventions set → conventionsFrom returns defaults.
	if err := os.WriteFile(filepath.Join(dir, ecosystem.ConfigFile), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// --- adopt: shellrc/ inference ---

func TestAdopt_ShellScript_InfersShellrc(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfiles := adoptDotfiles(t)
	src := makeShellScript(t, home, "myscript.sh")

	_, err := run(t, "adopt", "--yes",
		"--files", dotfiles,
		"--dotd-env", emptyEnvFile(t),
		src,
	)
	if err != nil {
		t.Fatalf("adopt .sh file error = %v", err)
	}

	// File must be moved into shellrc/.
	dest := filepath.Join(dotfiles, "shellrc", "myscript.sh")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected file at %s (shellrc/ destination): %v", dest, err)
	}
	// Original src must be gone (it was a shellrc source, no symlink).
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("expected original src %s to be removed", src)
	}
}

// --- adopt: bin/ inference ---

func TestAdopt_Executable_InfersBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfiles := adoptDotfiles(t)
	// $bin resolves to ~/.local/bin/dot-dagger by default; it must exist for Act.
	binDir := filepath.Join(home, ".local", "bin", "dot-dagger")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	src := makeExecutable(t, home, "my-tool")

	_, err := run(t, "adopt", "--yes",
		"--files", dotfiles,
		"--dotd-env", emptyEnvFile(t),
		src,
	)
	if err != nil {
		t.Fatalf("adopt executable error = %v", err)
	}

	// File must be moved into bin/.
	dest := filepath.Join(dotfiles, "bin", "my-tool")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected file at %s (bin/ destination): %v", dest, err)
	}
}

// --- adopt: --to flag overrides inference ---

func TestAdopt_ToFlag_OverridesInference(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfiles := adoptDotfiles(t)
	src := makeConfigFile(t, home, "app.conf")

	// --to config/foo.conf: should land at dotfiles/config/foo.conf
	// Note: no symlink destination inference needed since we provide explicit --to.
	_, err := run(t, "adopt", "--yes",
		"--files", dotfiles,
		"--dotd-env", emptyEnvFile(t),
		"--to", "config/foo.conf",
		src,
	)
	if err != nil {
		t.Fatalf("adopt --to error = %v", err)
	}

	dest := filepath.Join(dotfiles, "config", "foo.conf")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected file at %s (--to destination): %v", dest, err)
	}
}

// --- adopt: unknown file type without --to returns error ---

func TestAdopt_UnknownType_ErrorWithoutTo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfiles := adoptDotfiles(t)

	// A file with no extension, no dot prefix, and no executable bit — unknown type.
	src := filepath.Join(home, "SOME_FILE")
	if err := os.WriteFile(src, []byte("data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "adopt", "--yes",
		"--files", dotfiles,
		"--dotd-env", emptyEnvFile(t),
		src,
	)
	if err == nil {
		t.Fatal("expected error for unknown file type without --to, got nil")
	}
	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("expected '--to' in error message, got: %v", err)
	}
}

// --- adopt: --dry-run does not move file ---

func TestAdopt_DryRun_DoesNotMoveFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfiles := adoptDotfiles(t)
	src := makeShellScript(t, home, "dry.sh")

	out, err := run(t, "adopt", "--yes", "--dry-run",
		"--files", dotfiles,
		"--dotd-env", emptyEnvFile(t),
		src,
	)
	if err != nil {
		t.Fatalf("adopt --dry-run error = %v", err)
	}

	// Source must still exist.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("dry-run must not remove source file: %v", err)
	}
	// Destination must NOT have been created.
	dest := filepath.Join(dotfiles, "shellrc", "dry.sh")
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dry-run must not move file to %s", dest)
	}
	// Output should describe the planned action.
	if !strings.Contains(out, "adopt") {
		t.Errorf("expected 'adopt' in dry-run output: %q", out)
	}
}

// --- adopt: non-TTY stdin without --yes is refused (2026-06-13 audit, B2) ---

func TestAdopt_NonTTYWithoutYesRefuses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dotfiles := adoptDotfiles(t)
	src := makeShellScript(t, home, "keepme.sh")

	// Piped stdin (strings.Reader) is not a TTY; without --yes adopt must
	// refuse rather than silently move the file.
	out, err := runWithStdin(t, strings.NewReader("y\n"), "adopt",
		"--files", dotfiles,
		"--dotd-env", emptyEnvFile(t),
		src,
	)
	if err == nil {
		t.Fatalf("expected refusal on non-TTY stdin without --yes\noutput: %s", out)
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Errorf("error = %q, want confirmation-required message", err)
	}
	var h Hinter
	if !errors.As(err, &h) || !strings.Contains(h.Hint(), "--yes") {
		t.Errorf("expected --yes hint, got %v", err)
	}
	// Nothing moved.
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("src must be untouched on refusal: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dotfiles, "shellrc", "keepme.sh")); !os.IsNotExist(statErr) {
		t.Error("file must not be adopted on refusal")
	}
}

// --- adopt round-trip: issue #191 ---

// scaffoldInitConfigDir writes the same config/.dagger content that
// `dotd init` scaffolds (link_root "$config", link default) into dotfiles.
func scaffoldInitConfigDir(t *testing.T, dotfiles string) string {
	t.Helper()
	dir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ecosystem.ConfigFile)
	content := "link_root: \"$config\"\ndefaults:\n  actions:\n    - link\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAdopt_RoundTripsThroughApply reproduces issue #191: under the default
// init scaffold (config/.dagger with link_root "$config"), adopting a
// dot-prefixed $HOME file must record the original location so a following
// apply re-creates adopt's symlink instead of a second, wrong one at
// ~/.config/.<name>.
func TestAdopt_RoundTripsThroughApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

	dotfiles := t.TempDir()
	daggerPath := scaffoldInitConfigDir(t, dotfiles)
	env := emptyEnvFile(t)

	src := filepath.Join(home, ".dotd-repro-rc")
	if err := os.WriteFile(src, []byte("export FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "adopt", "--yes", "--files", dotfiles, "--dotd-env", env, src)
	if err != nil {
		t.Fatalf("adopt error = %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "recorded") || !strings.Contains(out, "link destination ~/.dotd-repro-rc in") {
		t.Errorf("adopt output must state what was recorded, got:\n%s", out)
	}
	if !strings.Contains(out, daggerPath) {
		t.Errorf("adopt output must name the .dagger written, got:\n%s", out)
	}

	// adopt's own symlink: src → repo file.
	destAbs := filepath.Join(dotfiles, "config", "dot-dotd-repro-rc")
	if target, err := os.Readlink(src); err != nil || target != destAbs {
		t.Fatalf("adopt symlink = %q (%v), want %q", target, err, destAbs)
	}

	// apply --dry-run must plan exactly one link, at the ORIGINAL path — not
	// a second symlink at ~/.config/.dotd-repro-rc.
	out, err = run(t, "apply", "--dry-run", "--files", dotfiles, "--dotd-env", env)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v\noutput: %s", err, out)
	}
	if got := strings.Count(out, "dry-run: link "); got != 1 {
		t.Errorf("apply --dry-run planned %d links, want 1:\n%s", got, out)
	}
	if !strings.Contains(out, "→ "+src+"\n") {
		t.Errorf("apply --dry-run must link to the original path %s:\n%s", src, out)
	}
	wrong := filepath.Join(home, ".config", ".dotd-repro-rc")
	if strings.Contains(out, wrong) {
		t.Errorf("apply --dry-run still derives the wrong dest %s:\n%s", wrong, out)
	}

	// Real apply: no second symlink appears and adopt's symlink is untouched.
	if out, err = run(t, "apply", "--files", dotfiles, "--dotd-env", env); err != nil {
		t.Fatalf("apply error = %v\noutput: %s", err, out)
	}
	if _, err := os.Lstat(wrong); !os.IsNotExist(err) {
		t.Errorf("apply created a second symlink at %s", wrong)
	}
	if target, err := os.Readlink(src); err != nil || target != destAbs {
		t.Errorf("original symlink damaged: %q (%v)", target, err)
	}
}

// TestAdopt_NoRecordWhenDerivedMatches: a file already under $config derives
// the same destination the adopt symlink uses — nothing must be persisted.
func TestAdopt_NoRecordWhenDerivedMatches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config")
	t.Setenv("XDG_CONFIG_HOME", configDir)

	dotfiles := t.TempDir()
	daggerPath := scaffoldInitConfigDir(t, dotfiles)
	before, err := os.ReadFile(daggerPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := makeConfigFile(t, configDir, "app.toml")

	out, err := run(t, "adopt", "--yes", "--files", dotfiles, "--dotd-env", emptyEnvFile(t), src)
	if err != nil {
		t.Fatalf("adopt error = %v\noutput: %s", err, out)
	}
	if strings.Contains(out, "link destination") {
		t.Errorf("nothing should be recorded when derivation matches:\n%s", out)
	}
	after, err := os.ReadFile(daggerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf(".dagger modified although derivation matched:\n%s", after)
	}
}

// TestAdopt_DryRunReportsPersistPlan: --dry-run must state what would be
// recorded and why, without touching anything.
func TestAdopt_DryRunReportsPersistPlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	dotfiles := t.TempDir()
	daggerPath := scaffoldInitConfigDir(t, dotfiles)
	before, _ := os.ReadFile(daggerPath)

	src := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(src, []byte("# rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "adopt", "--yes", "--dry-run", "--files", dotfiles, "--dotd-env", emptyEnvFile(t), src)
	if err != nil {
		t.Fatalf("adopt --dry-run error = %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "dry-run: record link destination ~/.zshrc") {
		t.Errorf("dry-run must state the would-be record, got:\n%s", out)
	}
	if !strings.Contains(out, "apply would otherwise link it to "+filepath.Join(home, ".config", ".zshrc")) {
		t.Errorf("dry-run must explain why, got:\n%s", out)
	}
	// Nothing moved, nothing written.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("dry-run must not move the source: %v", err)
	}
	after, _ := os.ReadFile(daggerPath)
	if string(after) != string(before) {
		t.Error("dry-run must not modify .dagger")
	}
}

// TestAdopt_ManualFallbackWhenEntryExists: a pre-existing files: entry for the
// same name cannot be mutated safely — adopt must succeed anyway and print the
// exact snippet for manual addition.
func TestAdopt_ManualFallbackWhenEntryExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	dotfiles := t.TempDir()
	dir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	daggerPath := filepath.Join(dir, ecosystem.ConfigFile)
	orig := "link_root: \"$config\"\ndefaults:\n  actions:\n    - link\nfiles:\n  dot-zshrc:\n    actions:\n      - link(~/.elsewhere)\n"
	if err := os.WriteFile(daggerPath, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(src, []byte("# rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "adopt", "--yes", "--files", dotfiles, "--dotd-env", emptyEnvFile(t), src)
	if err != nil {
		t.Fatalf("adopt must succeed even when recording fails: %v\noutput: %s", err, out)
	}
	// The adopt itself happened.
	destAbs := filepath.Join(dir, "dot-zshrc")
	if target, rlErr := os.Readlink(src); rlErr != nil || target != destAbs {
		t.Fatalf("adopt symlink = %q (%v), want %q", target, rlErr, destAbs)
	}
	// Fallback output: warning + the exact snippet.
	if !strings.Contains(out, "could not record") {
		t.Errorf("expected could-not-record warning, got:\n%s", out)
	}
	if !strings.Contains(out, "link(~/.zshrc)") {
		t.Errorf("expected manual snippet with link(~/.zshrc), got:\n%s", out)
	}
	// .dagger untouched.
	after, _ := os.ReadFile(daggerPath)
	if string(after) != orig {
		t.Errorf(".dagger must be untouched on fallback:\n%s", after)
	}
}
