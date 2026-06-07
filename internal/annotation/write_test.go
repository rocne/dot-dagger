package annotation

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Plain file: no shebang. Annotation goes at top, blank line after.
func TestWrite_PlainFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "test.sh", "export FOO=bar\n")

	if err := Write(path, nil, []string{"# @when(os=macos)"}); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	want := "# @when(os=macos)\n\nexport FOO=bar\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Shebang on line 0: annotations go after shebang with blank line before and after.
// Preceding line (shebang) is non-empty → blank inserted before annotation block.
func TestWrite_ShebangPreserved(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "test.sh", "#!/bin/sh\nexport FOO=bar\n")

	if err := Write(path, nil, []string{"# @when(os=macos)"}); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	want := "#!/bin/sh\n\n# @when(os=macos)\n\nexport FOO=bar\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Shebang + copyright block: annotations go after block, blank line before and after.
func TestWrite_CopyrightBlock(t *testing.T) {
	dir := t.TempDir()
	input := "#!/bin/sh\n# Copyright 2026\n#\nexport FOO=bar\n"
	path := writeFile(t, dir, "test.sh", input)

	if err := Write(path, nil, []string{"# @when(os=macos)", "# @after(shellrc.base)"}); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	want := "#!/bin/sh\n# Copyright 2026\n#\n\n# @when(os=macos)\n# @after(shellrc.base)\n\nexport FOO=bar\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Existing annotations are stripped and replaced.
func TestWrite_ExistingAnnotationsStripped(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "test.sh", "# @when(os=linux)\nexport FOO=bar\n")

	if err := Write(path, nil, []string{"# @when(os=macos)"}); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	want := "# @when(os=macos)\n\nexport FOO=bar\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Both preserved and lines empty: annotations are stripped, file content remains.
func TestWrite_EmptyBlock(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "test.sh", "# @when(os=linux)\nexport FOO=bar\n")

	if err := Write(path, nil, nil); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	want := "export FOO=bar\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Preserved lines are written before lines.
func TestWrite_PreservedFirst(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "test.sh", "export FOO=bar\n")

	preserved := []string{"# @link(~/.tmux.conf)"}
	lines := []string{"# @when(os=macos)"}
	if err := Write(path, preserved, lines); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	want := "# @link(~/.tmux.conf)\n# @when(os=macos)\n\nexport FOO=bar\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// No .tmp file left behind on success.
func TestWrite_NoTmpFileOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "test.sh", "export FOO=bar\n")

	if err := Write(path, nil, []string{"# @when(os=macos)"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error(".tmp file should not exist after successful write")
	}
}

// Following line already blank: no extra blank inserted after annotation block.
func TestWrite_NoExtraBlankAfterWhenFollowingIsBlank(t *testing.T) {
	dir := t.TempDir()
	// The preceding line is already blank — don't add another.
	path := writeFile(t, dir, "test.sh", "#!/bin/sh\n\nexport FOO=bar\n")

	if err := Write(path, nil, []string{"# @when(os=macos)"}); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	// insertAt stops at 1 (blank line "" is not a comment).
	// Preceding = "#!/bin/sh" (non-empty) → blank inserted before.
	// Following = "" (already blank) → no extra blank inserted after.
	// The original blank line in `after` provides the visual separation.
	want := "#!/bin/sh\n\n# @when(os=macos)\n\nexport FOO=bar\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// An annotation-looking comment in the file BODY (after the first code line) must
// not be stripped — Scan never treats it as a header annotation, so Write must
// leave it alone. Guards the header-bounded strip scope.
func TestWrite_BodyAnnotationLikeCommentPreserved(t *testing.T) {
	dir := t.TempDir()
	input := "export FOO=bar\n# @param x is documentation, not an annotation\n"
	path := writeFile(t, dir, "test.sh", input)

	if err := Write(path, nil, []string{"# @when(os=macos)"}); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	want := "# @when(os=macos)\n\nexport FOO=bar\n# @param x is documentation, not an annotation\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}
