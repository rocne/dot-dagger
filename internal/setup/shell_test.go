package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/setup"
)

// --- DetectShellConfig ---

func TestDetectShellConfig_BashLinux(t *testing.T) {
	home := t.TempDir()
	sc, ok := setup.DetectShellConfig("bash", "linux", home, "")
	if !ok {
		t.Fatal("expected ok=true for bash+linux")
	}
	if sc.Shell != "bash" {
		t.Errorf("Shell = %q, want 'bash'", sc.Shell)
	}
	want := filepath.Join(home, ".bashrc")
	if sc.RCFile != want {
		t.Errorf("RCFile = %q, want %q", sc.RCFile, want)
	}
}

func TestDetectShellConfig_BashMacOS(t *testing.T) {
	home := t.TempDir()
	sc, ok := setup.DetectShellConfig("bash", "macos", home, "")
	if !ok {
		t.Fatal("expected ok=true for bash+macos")
	}
	want := filepath.Join(home, ".bash_profile")
	if sc.RCFile != want {
		t.Errorf("RCFile = %q, want %q (macos bash uses .bash_profile)", sc.RCFile, want)
	}
}

func TestDetectShellConfig_ZshLinux(t *testing.T) {
	home := t.TempDir()
	sc, ok := setup.DetectShellConfig("zsh", "linux", home, "")
	if !ok {
		t.Fatal("expected ok=true for zsh+linux")
	}
	want := filepath.Join(home, ".zshrc")
	if sc.RCFile != want {
		t.Errorf("RCFile = %q, want %q", sc.RCFile, want)
	}
}

func TestDetectShellConfig_ZshMacOS(t *testing.T) {
	home := t.TempDir()
	sc, ok := setup.DetectShellConfig("zsh", "macos", home, "")
	if !ok {
		t.Fatal("expected ok=true for zsh+macos")
	}
	want := filepath.Join(home, ".zshrc")
	if sc.RCFile != want {
		t.Errorf("RCFile = %q, want %q (zsh uses .zshrc on macos too)", sc.RCFile, want)
	}
}

// fish RC lives under the XDG config home, not $HOME. Verify configDir is
// honored (and that the path is built from the passed-in value, not the env).
func TestDetectShellConfig_Fish(t *testing.T) {
	home := t.TempDir()
	configDir := t.TempDir()
	sc, ok := setup.DetectShellConfig("fish", "linux", home, configDir)
	if !ok {
		t.Fatal("expected ok=true for fish")
	}
	if sc.Shell != "fish" {
		t.Errorf("Shell = %q, want 'fish'", sc.Shell)
	}
	want := filepath.Join(configDir, "fish", "config.fish")
	if sc.RCFile != want {
		t.Errorf("RCFile = %q, want %q", sc.RCFile, want)
	}
}

func TestDetectShellConfig_UnknownShell(t *testing.T) {
	home := t.TempDir()
	_, ok := setup.DetectShellConfig("tcsh", "linux", home, "")
	if ok {
		t.Error("expected ok=false for unknown shell 'tcsh'")
	}
}

// Verify that the RCFile path is rooted at the provided home (home convention).
func TestDetectShellConfig_RCFileRootedAtHome(t *testing.T) {
	home := t.TempDir()
	for _, shell := range []string{"bash", "zsh"} {
		sc, ok := setup.DetectShellConfig(shell, "linux", home, "")
		if !ok {
			t.Fatalf("DetectShellConfig(%s): expected ok=true", shell)
		}
		if !strings.HasPrefix(sc.RCFile, home) {
			t.Errorf("DetectShellConfig(%s).RCFile = %q — not rooted at home %q", shell, sc.RCFile, home)
		}
	}
}

func writeTempRC(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "rc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

func TestRemoveSourceLine_RemovesHeaderAndLine(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	content := "# existing content\n\n# dotd \xe2\x80\x94 generated shell init\nsource \"$HOME/.local/share/dot-dagger/init.sh\"\n"
	rc := writeTempRC(t, content)

	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(rc)
	if strings.Contains(string(got), "dotd") {
		t.Errorf("expected dotd lines removed, got:\n%s", string(got))
	}
	if !strings.Contains(string(got), "# existing content") {
		t.Errorf("expected existing content preserved, got:\n%s", string(got))
	}
}

func TestRemoveSourceLine_NoopWhenAbsent(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	content := "# just some rc content\nexport FOO=bar\n"
	rc := writeTempRC(t, content)

	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(rc)
	if string(got) != content {
		t.Errorf("expected unchanged content, got:\n%s", string(got))
	}
}

func TestRemoveSourceLine_NoopWhenFileMissing(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	if err := setup.RemoveSourceLine("/nonexistent/path/.bashrc", initFile); err != nil {
		t.Fatalf("expected nil error for missing RC file, got %v", err)
	}
}

func TestRemoveSourceLine_MultipleCallsIdempotent(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	content := "# header\n\n# dotd \xe2\x80\x94 generated shell init\nsource \"$HOME/.local/share/dot-dagger/init.sh\"\n# footer\n"
	rc := writeTempRC(t, content)

	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(rc)
	if strings.Contains(string(got), "dotd") {
		t.Errorf("second call: dotd lines not removed:\n%s", string(got))
	}
	if !strings.Contains(string(got), "# footer") {
		t.Errorf("second call: footer removed:\n%s", string(got))
	}
}

func TestSourceLine_RoundTrip(t *testing.T) {
	home := t.TempDir()
	initFile := filepath.Join(home, ".local", "share", "dot-dagger", "init.sh")

	// Start with a pre-existing RC file so we can verify surrounding content survives.
	preContent := "# pre-existing content\nexport FOO=bar\n"
	rc := writeTempRC(t, preContent)

	// Precondition: source line not yet present.
	has, err := setup.HasSourceLine(rc, initFile)
	if err != nil {
		t.Fatalf("HasSourceLine before append: %v", err)
	}
	if has {
		t.Fatal("expected HasSourceLine=false before AppendSourceLine")
	}

	// Append the source line.
	if err := setup.AppendSourceLine(rc, initFile, home); err != nil {
		t.Fatalf("AppendSourceLine: %v", err)
	}

	// Source line should now be detected.
	has, err = setup.HasSourceLine(rc, initFile)
	if err != nil {
		t.Fatalf("HasSourceLine after append: %v", err)
	}
	if !has {
		got, _ := os.ReadFile(rc)
		t.Fatalf("expected HasSourceLine=true after AppendSourceLine; RC content:\n%s", got)
	}

	// Remove the source line.
	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatalf("RemoveSourceLine: %v", err)
	}

	// Source line should no longer be present.
	has, err = setup.HasSourceLine(rc, initFile)
	if err != nil {
		t.Fatalf("HasSourceLine after remove: %v", err)
	}
	if has {
		got, _ := os.ReadFile(rc)
		t.Fatalf("expected HasSourceLine=false after RemoveSourceLine; RC content:\n%s", got)
	}

	// Pre-existing content must survive the full round trip.
	got, _ := os.ReadFile(rc)
	if !strings.Contains(string(got), "# pre-existing content") {
		t.Errorf("pre-existing content lost after round trip; RC content:\n%s", got)
	}
	if !strings.Contains(string(got), "export FOO=bar") {
		t.Errorf("pre-existing export lost after round trip; RC content:\n%s", got)
	}
}
