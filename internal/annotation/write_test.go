package annotation

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
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

// TestWriteScan_RoundTrip is a property test over the mutation path: a set of
// annotations written into a file's header must Scan back as the identical
// (key, args) sequence. Guards Write↔Scan from drifting — including args that
// carry nested parens (e.g. @when(exists(git))), which the first-"("/last-")"
// split must preserve.
func TestWriteScan_RoundTrip(t *testing.T) {
	// (key, args) pairs the wizard could plausibly produce. Empty args models a
	// bool annotation ("# @disable"); the rest exercise predicate/after content.
	type pair struct{ key, args string }
	pool := []pair{
		{KeyWhen, "os=linux"},
		{KeyWhen, "os=linux,macos"},
		{KeyWhen, "exists(git)"},
		{KeyWhen, "context=work & os=macos"},
		{KeyAfter, "shellrc/"},
		{KeyAfter, "core.base"},
		{KeyRequire, "git"},
		{KeyRequest, "ripgrep"},
		{KeyName, "my-thing"},
		{KeyDisable, ""},
	}

	rng := rand.New(rand.NewSource(7))
	const iterations = 300
	for it := 0; it < iterations; it++ {
		// Pick a random ordered subset (with possible repeats) of the pool.
		count := rng.Intn(6)
		chosen := make([]pair, count)
		for i := range chosen {
			chosen[i] = pool[rng.Intn(len(pool))]
		}

		// Format like the wizard: "# @key(args)" or "# @key" for empty args.
		lines := make([]string, len(chosen))
		for i, p := range chosen {
			if p.args == "" {
				lines[i] = "# @" + p.key
			} else {
				lines[i] = "# @" + p.key + "(" + p.args + ")"
			}
		}

		dir := t.TempDir()
		path := writeFile(t, dir, "f.sh", "#!/bin/sh\necho hi\n")
		if err := Write(path, nil, lines); err != nil {
			t.Fatalf("iter %d: Write: %v", it, err)
		}

		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("iter %d: open: %v", it, err)
		}
		scanned, err := Scan(f)
		_ = f.Close()
		if err != nil {
			t.Fatalf("iter %d: Scan: %v", it, err)
		}

		if len(scanned) != len(chosen) {
			t.Fatalf("iter %d: scanned %d annotations, wrote %d\nlines:\n%s\nfile:\n%s",
				it, len(scanned), len(chosen), strings.Join(lines, "\n"), readFile(t, path))
		}
		for i, p := range chosen {
			if scanned[i].Key != p.key || scanned[i].Args != p.args {
				t.Fatalf("iter %d pos %d: round-trip mismatch: wrote (%q,%q), scanned (%q,%q)",
					it, i, p.key, p.args, scanned[i].Key, scanned[i].Args)
			}
		}
	}
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

// Write must strip every header annotation Scan recognizes — including forms the
// strip regex missed (a space after "@", or a non-word first char). Otherwise a
// stale annotation Scan still reads survives the rewrite, duplicating the key.
func TestWrite_StripsScanRecognizedAnnotations(t *testing.T) {
	cases := map[string]string{
		"space after at": "# @ when(os=linux)\nexport FOO=bar\n",
		"dot key":        "# @.when(os=linux)\nexport FOO=bar\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, "test.sh", input)

			// Sanity: Scan recognizes the existing line as an annotation.
			if anns, err := Scan(strings.NewReader(input)); err != nil || len(anns) == 0 {
				t.Fatalf("precondition: Scan should recognize %q (err=%v anns=%+v)", input, err, anns)
			}

			if err := Write(path, nil, []string{"# @when(os=macos)"}); err != nil {
				t.Fatal(err)
			}

			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			anns, err := Scan(f)
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if len(anns) != 1 {
				t.Fatalf("stale annotation survived rewrite: want 1, got %d: %+v\nfile:\n%s",
					len(anns), anns, readFile(t, path))
			}
			if anns[0].Key != KeyWhen || anns[0].Args != "os=macos" {
				t.Errorf("got (%q,%q), want (%q,%q)", anns[0].Key, anns[0].Args, KeyWhen, "os=macos")
			}
		})
	}
}
