package ecosystem_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
)

func TestResolvePathCliArgWins(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "from-shell")
	got, err := ecosystem.ResolvePath("from-cli", "DOTD_TEST_VAR", "from-file", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-cli" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-cli")
	}
}

func TestResolvePathShellVarBeatsFile(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "from-shell")
	got, err := ecosystem.ResolvePath("", "DOTD_TEST_VAR", "from-file", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-shell" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-shell")
	}
}

func TestResolvePathFileBeatsDefault(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "")
	got, err := ecosystem.ResolvePath("", "DOTD_TEST_VAR", "from-file", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-file" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-file")
	}
}

func TestResolvePathUsesDefault(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "")
	got, err := ecosystem.ResolvePath("", "DOTD_TEST_VAR", "", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-default" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-default")
	}
}

func TestDefaultInitFileUsesXDGDataHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	got, err := ecosystem.DefaultInitFile()
	if err != nil {
		t.Fatalf("DefaultInitFile error = %v", err)
	}
	want := filepath.Join(tmp, "dot-dagger", "init.sh")
	if got != want {
		t.Errorf("DefaultInitFile = %q, want %q", got, want)
	}
}

func TestDefaultEnvFileUsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := ecosystem.DefaultEnvFile()
	if err != nil {
		t.Fatalf("DefaultEnvFile error = %v", err)
	}
	want := filepath.Join(tmp, "dot-dagger", "env.yaml")
	if got != want {
		t.Errorf("DefaultEnvFile = %q, want %q", got, want)
	}
}

func TestDefaultLinkRootUsesHOME(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := ecosystem.DefaultLinkRoot()
	if err != nil {
		t.Fatalf("DefaultLinkRoot error = %v", err)
	}
	if got != tmp {
		t.Errorf("DefaultLinkRoot = %q, want %q", got, tmp)
	}
}

func TestDefaultDotfilesFromEnvVar(t *testing.T) {
	t.Setenv("DOTFILES", "/my/dotfiles")
	got := ecosystem.DefaultDotfiles()
	if got != "/my/dotfiles" {
		t.Errorf("DefaultDotfiles = %q, want %q", got, "/my/dotfiles")
	}
}

// TestPackagesFileNameConsistency guards that the packages.yaml path written by
// setup and the path read by the package command both derive from the same constant.
func TestPackagesFileNameConsistency(t *testing.T) {
	dir := t.TempDir()
	// Path written during scaffold (internal/setup logic).
	setupPath := filepath.Join(dir, ecosystem.PackagesFileName)
	// Path read by the package command (cmd/dotd/package.go logic).
	readPath := filepath.Join(dir, ecosystem.PackagesFileName)
	if setupPath != readPath {
		t.Errorf("setup path %q != read path %q — paths diverged", setupPath, readPath)
	}
	if ecosystem.PackagesFileName != "packages.yaml" {
		t.Errorf("PackagesFileName = %q, want %q", ecosystem.PackagesFileName, "packages.yaml")
	}
}

func TestDefaultDotfilesFallsToCwd(t *testing.T) {
	origDotfiles, hadDotfiles := os.LookupEnv("DOTFILES")
	if err := os.Unsetenv("DOTFILES"); err != nil {
		t.Fatalf("unsetenv DOTFILES: %v", err)
	}
	t.Cleanup(func() {
		if hadDotfiles {
			if err := os.Setenv("DOTFILES", origDotfiles); err != nil {
				t.Errorf("restore DOTFILES: %v", err)
			}
		}
	})

	wd, _ := os.Getwd()
	got := ecosystem.DefaultDotfiles()
	if got != wd {
		t.Errorf("DefaultDotfiles = %q, want cwd %q", got, wd)
	}
}

// --- Additional path tests (AUDIT-046) ---

// TestDefaultBinDir verifies that DefaultBinDir returns $HOME/.local/bin/dot-dagger
// and is NOT rooted at XDG_DATA_HOME (FHS user-local binary convention).
func TestDefaultBinDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Explicitly set XDG_DATA_HOME to a different dir to prove it is not used.
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "xdg-data"))

	got, err := ecosystem.DefaultBinDir()
	if err != nil {
		t.Fatalf("DefaultBinDir error: %v", err)
	}
	want := filepath.Join(tmp, ".local", "bin", "dot-dagger")
	if got != want {
		t.Errorf("DefaultBinDir = %q, want %q", got, want)
	}
	// Confirm it is NOT under XDG_DATA_HOME.
	xdgHome := filepath.Join(tmp, "xdg-data")
	if len(got) > len(xdgHome) && got[:len(xdgHome)] == xdgHome {
		t.Errorf("DefaultBinDir %q must NOT be rooted at XDG_DATA_HOME %q", got, xdgHome)
	}
}

// TestDefaultGeneratedDir verifies DefaultGeneratedDir = $XDG_DATA_HOME/dot-dagger/generated.
func TestDefaultGeneratedDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	got, err := ecosystem.DefaultGeneratedDir()
	if err != nil {
		t.Fatalf("DefaultGeneratedDir error: %v", err)
	}
	want := filepath.Join(tmp, "dot-dagger", "generated")
	if got != want {
		t.Errorf("DefaultGeneratedDir = %q, want %q", got, want)
	}
}

// TestDefaultConfigFile verifies DefaultConfigFile = $XDG_CONFIG_HOME/dot-dagger/config.yaml.
func TestDefaultConfigFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := ecosystem.DefaultConfigFile()
	if err != nil {
		t.Fatalf("DefaultConfigFile error: %v", err)
	}
	want := filepath.Join(tmp, "dot-dagger", "config.yaml")
	if got != want {
		t.Errorf("DefaultConfigFile = %q, want %q", got, want)
	}
}

// TestXdgConfigHome verifies XdgConfigHome respects XDG_CONFIG_HOME when absolute,
// and falls back to ~/.config otherwise.
func TestXdgConfigHome(t *testing.T) {
	t.Run("uses XDG_CONFIG_HOME when set to absolute path", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmp)
		got, err := ecosystem.XdgConfigHome()
		if err != nil {
			t.Fatalf("XdgConfigHome error: %v", err)
		}
		if got != tmp {
			t.Errorf("XdgConfigHome = %q, want %q", got, tmp)
		}
	})

	t.Run("falls back to ~/.config when XDG_CONFIG_HOME unset", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("HOME", tmp)
		t.Setenv("XDG_CONFIG_HOME", "") // unset / empty
		got, err := ecosystem.XdgConfigHome()
		if err != nil {
			t.Fatalf("XdgConfigHome error: %v", err)
		}
		want := filepath.Join(tmp, ".config")
		if got != want {
			t.Errorf("XdgConfigHome = %q, want %q", got, want)
		}
	})

	t.Run("ignores relative XDG_CONFIG_HOME (falls back to ~/.config)", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("HOME", tmp)
		t.Setenv("XDG_CONFIG_HOME", "relative/path") // not absolute — must be ignored
		got, err := ecosystem.XdgConfigHome()
		if err != nil {
			t.Fatalf("XdgConfigHome error: %v", err)
		}
		want := filepath.Join(tmp, ".config")
		if got != want {
			t.Errorf("XdgConfigHome with relative XDG_CONFIG_HOME = %q, want %q", got, want)
		}
	})
}

// TestDefaultPaths_HomeUnavailable exercises the error branches when $HOME is
// unset and no XDG override is provided. Each Default* function that relies on
// os.UserHomeDir must propagate a non-nil error.
func TestDefaultPaths_HomeUnavailable(t *testing.T) {
	// Unset all env vars that could provide a home directory.
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	t.Run("DefaultBinDir errors without HOME", func(t *testing.T) {
		_, err := ecosystem.DefaultBinDir()
		if err == nil {
			t.Error("DefaultBinDir: want error when HOME unavailable, got nil")
		}
	})

	t.Run("DefaultGeneratedDir errors without XDG_DATA_HOME and HOME", func(t *testing.T) {
		_, err := ecosystem.DefaultGeneratedDir()
		if err == nil {
			t.Error("DefaultGeneratedDir: want error when HOME unavailable, got nil")
		}
	})

	t.Run("DefaultConfigFile errors without XDG_CONFIG_HOME and HOME", func(t *testing.T) {
		_, err := ecosystem.DefaultConfigFile()
		if err == nil {
			t.Error("DefaultConfigFile: want error when HOME unavailable, got nil")
		}
	})

	t.Run("DefaultInitFile errors without XDG_DATA_HOME and HOME", func(t *testing.T) {
		_, err := ecosystem.DefaultInitFile()
		if err == nil {
			t.Error("DefaultInitFile: want error when HOME unavailable, got nil")
		}
	})

	t.Run("DefaultLinkRoot errors without HOME", func(t *testing.T) {
		_, err := ecosystem.DefaultLinkRoot()
		if err == nil {
			t.Error("DefaultLinkRoot: want error when HOME unavailable, got nil")
		}
	})
}

// --- ResolvePath contract tests (AUDIT-064) ---

// TestResolvePath_EnvVarNameIsolation verifies that ResolvePath reads env via the
// *passed* envVar name, not a hard-coded one. Each sub-test uses a distinct env
// var so cross-test pollution is impossible.
func TestResolvePath_EnvVarNameIsolation(t *testing.T) {
	t.Run("DOTD_AUDIT_TEST_A reads from DOTD_AUDIT_TEST_A", func(t *testing.T) {
		t.Setenv("DOTD_AUDIT_TEST_A", "value-from-A")
		got, err := ecosystem.ResolvePath("", "DOTD_AUDIT_TEST_A", "", func() (string, error) {
			return "default-A", nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "value-from-A" {
			t.Errorf("ResolvePath read %q, want value-from-A", got)
		}
	})

	t.Run("DOTD_AUDIT_TEST_B reads from DOTD_AUDIT_TEST_B not A", func(t *testing.T) {
		t.Setenv("DOTD_AUDIT_TEST_A", "value-from-A-should-not-leak")
		t.Setenv("DOTD_AUDIT_TEST_B", "value-from-B")
		got, err := ecosystem.ResolvePath("", "DOTD_AUDIT_TEST_B", "", func() (string, error) {
			return "default-B", nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "value-from-B" {
			t.Errorf("ResolvePath read %q, want value-from-B (not A's value)", got)
		}
	})
}

// TestResolvePath_TildeNotExpanded verifies that ResolvePath does NOT expand
// tilde in the returned path — tilde expansion is the caller's responsibility.
func TestResolvePath_TildeNotExpanded(t *testing.T) {
	// Set env var to a tilde path.
	t.Setenv("DOTD_AUDIT_TILDE_TEST", "~/mydir")
	got, err := ecosystem.ResolvePath("", "DOTD_AUDIT_TILDE_TEST", "", func() (string, error) {
		return "/default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ResolvePath must return the literal tilde string, not an expanded path.
	if got != "~/mydir" {
		t.Errorf("ResolvePath = %q, want literal ~/mydir (no tilde expansion)", got)
	}
}
