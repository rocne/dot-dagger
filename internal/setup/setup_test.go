package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/setup"
)

func TestScaffoldCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir: dir,
		EnvFilePath: filepath.Join(dir, "env.yaml"),
	}

	res, err := setup.Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	for _, sub := range []string{"shellrc", "conf", "bin"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("expected dir %s to exist: %v", sub, err)
		}
	}

	created := countState(res, setup.KindDir, setup.StateCreated)
	if created != 3 {
		t.Errorf("expected 3 created dirs, got %d", created)
	}
}

func TestScaffoldCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir: dir,
		EnvFilePath: filepath.Join(dir, "env.yaml"),
	}

	res, err := setup.Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	for _, name := range []string{"env.yaml", ecosystem.ConfigFile, "packages.yaml"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
		}
	}

	created := countState(res, setup.KindFile, setup.StateCreated)
	if created != 3 {
		t.Errorf("expected 3 created files, got %d", created)
	}
}

func TestScaffoldIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir: dir,
		EnvFilePath: filepath.Join(dir, "env.yaml"),
	}

	if _, err := setup.Scaffold(opts); err != nil {
		t.Fatalf("first Scaffold error = %v", err)
	}
	res, err := setup.Scaffold(opts)
	if err != nil {
		t.Fatalf("second Scaffold error = %v", err)
	}

	skipped := countState(res, setup.KindDir, setup.StateSkipped) +
		countState(res, setup.KindFile, setup.StateSkipped)
	if skipped != 6 {
		t.Errorf("expected 6 skipped actions on second run, got %d", skipped)
	}
}

func TestScaffoldWithSelectedManagers(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir:      dir,
		EnvFilePath:      filepath.Join(dir, "env.yaml"),
		SelectedManagers: []string{"dnf"},
	}

	if _, err := setup.Scaffold(opts); err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "packages.yaml"))
	if err != nil {
		t.Fatalf("read packages.yaml: %v", err)
	}
	if !strings.Contains(string(content), "dnf") {
		t.Errorf("packages.yaml should contain 'dnf', got: %s", content)
	}
}

func TestScaffoldWithDetectedValues(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir:    dir,
		EnvFilePath:    filepath.Join(dir, "env.yaml"),
		DetectedOS:     "linux",
		DetectedDistro: "fedora",
		DetectedShell:  "zsh",
	}

	if _, err := setup.Scaffold(opts); err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "env.yaml"))
	if err != nil {
		t.Fatalf("read env.yaml: %v", err)
	}
	body := string(content)
	for _, want := range []string{"linux", "fedora", "zsh"} {
		if !strings.Contains(body, want) {
			t.Errorf("env.yaml should mention %q, got:\n%s", want, body)
		}
	}
}

func countState(res *setup.Result, kind setup.ActionKind, state setup.ActionState) int {
	n := 0
	for _, a := range res.Actions {
		if a.Kind == kind && a.State == state {
			n++
		}
	}
	return n
}
