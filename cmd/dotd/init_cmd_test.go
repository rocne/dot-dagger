package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
)

// --- scaffoldDagger tests (AUDIT-056) ---

// TestScaffoldDagger_CreatesDirectoryAndFile verifies that scaffoldDagger creates
// both the directory and the .dagger file when both are absent.
func TestScaffoldDagger_CreatesDirectoryAndFile(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "shellrc")
	content := "defaults:\n  actions:\n    - source\n"

	if err := scaffoldDagger(dir, content); err != nil {
		t.Fatalf("scaffoldDagger error: %v", err)
	}

	// Directory must exist.
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("scaffoldDagger: directory %q not created", dir)
	}

	// .dagger file must exist with the given content.
	daggerPath := filepath.Join(dir, ecosystem.ConfigFile)
	got, err := os.ReadFile(daggerPath)
	if err != nil {
		t.Fatalf("scaffoldDagger: .dagger not created: %v", err)
	}
	if string(got) != content {
		t.Errorf("scaffoldDagger: .dagger content = %q, want %q", got, content)
	}
}

// TestScaffoldDagger_SkipsExisting verifies the skip-existing idempotency: when
// a .dagger file already exists, scaffoldDagger is a no-op (file unchanged).
func TestScaffoldDagger_SkipsExisting(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "shellrc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	daggerPath := filepath.Join(dir, ecosystem.ConfigFile)
	original := "original content\n"
	if err := os.WriteFile(daggerPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	// Call with different content — must be ignored because .dagger exists.
	if err := scaffoldDagger(dir, "new content\n"); err != nil {
		t.Fatalf("scaffoldDagger error: %v", err)
	}

	got, err := os.ReadFile(daggerPath)
	if err != nil {
		t.Fatalf("cannot read .dagger: %v", err)
	}
	if string(got) != original {
		t.Errorf("scaffoldDagger overwrote existing file: got %q, want %q", got, original)
	}
}

// TestScaffoldDagger_Idempotent verifies that calling scaffoldDagger twice on a
// non-existent dir produces the file exactly once (second call is a no-op).
func TestScaffoldDagger_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "config")
	content := "defaults:\n  actions:\n    - link\n"

	if err := scaffoldDagger(dir, content); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if err := scaffoldDagger(dir, content); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	daggerPath := filepath.Join(dir, ecosystem.ConfigFile)
	got, err := os.ReadFile(daggerPath)
	if err != nil {
		t.Fatalf("cannot read .dagger after second call: %v", err)
	}
	if string(got) != content {
		t.Errorf("idempotent: content changed on second call: got %q", got)
	}
}

// TestScaffoldDagger_ParentDirCreated verifies that a deeply nested target
// directory path is created via MkdirAll.
func TestScaffoldDagger_ParentDirCreated(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "a", "b", "c")

	if err := scaffoldDagger(dir, "content\n"); err != nil {
		t.Fatalf("scaffoldDagger nested: %v", err)
	}

	daggerPath := filepath.Join(dir, ecosystem.ConfigFile)
	if _, err := os.Stat(daggerPath); err != nil {
		t.Errorf("nested .dagger not created: %v", err)
	}
}

// --- scaffoldDaggerInteractive tests (AUDIT-056) ---

// makeBufioReader wraps the given response string in a *bufio.Reader so it can
// be passed directly to scaffoldDaggerInteractive / promptYN / promptDefault.
func makeBufioReader(responses string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(responses))
}

// TestScaffoldDaggerInteractive_AcceptAll simulates the user accepting all three
// convention roles with their default directory names (y\n then \n per role).
// All three .dagger files must be written.
func TestScaffoldDaggerInteractive_AcceptAll(t *testing.T) {
	tmp := t.TempDir()
	var out bytes.Buffer

	// 3 roles × (y\n for promptYN + \n for promptDefault)
	reader := makeBufioReader("y\n\ny\n\ny\n\n")

	written, err := scaffoldDaggerInteractive(&out, reader, tmp, false)
	if err != nil {
		t.Fatalf("scaffoldDaggerInteractive error: %v", err)
	}

	if len(written) != 3 {
		t.Errorf("accept-all: wrote %d files, want 3; written = %v", len(written), written)
	}

	for _, p := range written {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("written file %q does not exist: %v", p, err)
		}
	}
}

// TestScaffoldDaggerInteractive_DeclineAll simulates the user declining (n) for
// all three convention roles. No files should be written.
func TestScaffoldDaggerInteractive_DeclineAll(t *testing.T) {
	tmp := t.TempDir()
	var out bytes.Buffer

	reader := makeBufioReader("n\nn\nn\n")

	written, err := scaffoldDaggerInteractive(&out, reader, tmp, false)
	if err != nil {
		t.Fatalf("scaffoldDaggerInteractive error: %v", err)
	}

	if len(written) != 0 {
		t.Errorf("decline-all: wrote %d files, want 0; written = %v", len(written), written)
	}
}

// TestScaffoldDaggerInteractive_IdempotentSecondRun verifies idempotency at the
// file-content level: running scaffoldDaggerInteractive twice (accepting all
// both times) does NOT alter the content of already-created .dagger files.
// scaffoldDagger itself enforces the skip-existing contract; the interactive
// wrapper still returns the paths the user chose on each run. The key
// invariant is that file content is preserved, not that the returned slice is
// empty. AUDIT-056 idempotency requirement.
func TestScaffoldDaggerInteractive_IdempotentSecondRun(t *testing.T) {
	tmp := t.TempDir()
	var out bytes.Buffer

	// First run — accept all with defaults.
	firstReader := makeBufioReader("y\n\ny\n\ny\n\n")
	written1, err := scaffoldDaggerInteractive(&out, firstReader, tmp, false)
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}
	if len(written1) != 3 {
		t.Fatalf("first run: expected 3 files written, got %d", len(written1))
	}

	// Capture content of all three files before the second run.
	beforeContent := map[string][]byte{}
	for _, p := range written1 {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("cannot read %q after first run: %v", p, err)
		}
		beforeContent[p] = data
	}

	// Second run — accept all again.
	out.Reset()
	secondReader := makeBufioReader("y\n\ny\n\ny\n\n")
	_, err = scaffoldDaggerInteractive(&out, secondReader, tmp, false)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// The core idempotency guarantee: file contents must be identical to the
	// first run's output. scaffoldDagger's skip-existing logic ensures that
	// already-present .dagger files are never overwritten.
	for _, p := range written1 {
		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("cannot re-read %q after second run: %v", p, err)
		}
		if !bytes.Equal(beforeContent[p], after) {
			t.Errorf("idempotent: file %q content changed on second run\n  before: %q\n  after:  %q",
				p, beforeContent[p], after)
		}
	}
}

// TestScaffoldDaggerInteractive_DefaultDirsUnderDotfilesPath verifies that the
// written .dagger files land inside dotfilesPath (not cwd or some other root).
func TestScaffoldDaggerInteractive_DefaultDirsUnderDotfilesPath(t *testing.T) {
	tmp := t.TempDir()
	var out bytes.Buffer

	reader := makeBufioReader("y\n\ny\n\ny\n\n")
	written, err := scaffoldDaggerInteractive(&out, reader, tmp, false)
	if err != nil {
		t.Fatalf("scaffoldDaggerInteractive error: %v", err)
	}

	for _, p := range written {
		if !strings.HasPrefix(p, tmp) {
			t.Errorf("written file %q is not under dotfilesPath %q", p, tmp)
		}
	}
}
