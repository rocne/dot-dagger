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
	t.Setenv("DOTD_LINK_ROOT", "")

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
	t.Setenv("DOTD_LINK_ROOT", "")

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
	t.Setenv("DOTD_LINK_ROOT", "")

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
	t.Setenv("DOTD_LINK_ROOT", "")

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
	t.Setenv("DOTD_LINK_ROOT", "")

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
