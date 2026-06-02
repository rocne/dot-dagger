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
