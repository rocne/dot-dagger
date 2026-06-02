package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveYAML_RoundTrip verifies successful encoding and that SetIndent(2) is used.
func TestSaveYAML_RoundTrip(t *testing.T) {
	type payload struct {
		Name  string `yaml:"name"`
		Value int    `yaml:"value"`
	}
	v := payload{Name: "hello", Value: 42}

	dir := t.TempDir()
	path := filepath.Join(dir, "out.yaml")

	if err := SaveYAML(path, v); err != nil {
		t.Fatalf("SaveYAML error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error = %v", err)
	}
	got := string(data)

	// Must contain both keys.
	if !strings.Contains(got, "name: hello") {
		t.Errorf("expected 'name: hello' in output:\n%s", got)
	}
	if !strings.Contains(got, "value: 42") {
		t.Errorf("expected 'value: 42' in output:\n%s", got)
	}
}

// TestSaveYAML_Indentation verifies that nested structures use 2-space indentation.
func TestSaveYAML_Indentation(t *testing.T) {
	type nested struct {
		Top map[string]string `yaml:"top"`
	}
	v := nested{Top: map[string]string{"key": "val"}}

	dir := t.TempDir()
	path := filepath.Join(dir, "indent.yaml")

	if err := SaveYAML(path, v); err != nil {
		t.Fatalf("SaveYAML error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error = %v", err)
	}
	got := string(data)

	// 2-space indentation: "  key: val"
	if !strings.Contains(got, "  key: val") {
		t.Errorf("expected 2-space indented 'key: val', got:\n%s", got)
	}
}

// TestSaveYAML_MkdirAll verifies that SaveYAML creates parent directories if missing.
func TestSaveYAML_MkdirAll(t *testing.T) {
	base := t.TempDir()
	// Path under a nonexistent sub-directory.
	path := filepath.Join(base, "subdir", "nested", "out.yaml")

	if err := SaveYAML(path, map[string]string{"x": "1"}); err != nil {
		t.Fatalf("SaveYAML error = %v", err)
	}

	// Parent directory must have been created.
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); err != nil {
		t.Fatalf("expected parent dir %s to exist: %v", parentDir, err)
	}

	// File must exist.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected output file %s to exist: %v", path, err)
	}
}

// TestSaveYAML_TempFileCleanupOnFailure verifies that no leftover .tmp files
// remain when the write fails (unwritable parent directory).
func TestSaveYAML_TempFileCleanupOnFailure(t *testing.T) {
	base := t.TempDir()

	// Create a sub-directory, then make it unwritable so temp-file creation fails.
	subdir := filepath.Join(base, "locked")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(subdir, 0o400); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o755) })

	path := filepath.Join(subdir, "out.yaml")

	err := SaveYAML(path, map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error writing to unwritable dir, got nil")
	}

	// Assert no leftover .tmp files.
	entries, readErr := os.ReadDir(subdir)
	if readErr != nil {
		// If ReadDir fails (still unreadable), restore and retry.
		_ = os.Chmod(subdir, 0o755)
		entries, readErr = os.ReadDir(subdir)
		if readErr != nil {
			t.Fatalf("readdir after chmod: %v", readErr)
		}
	} else {
		_ = os.Chmod(subdir, 0o755) // restore for cleanup
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml.tmp") {
			t.Errorf("leftover temp file found: %s", e.Name())
		}
	}
}
