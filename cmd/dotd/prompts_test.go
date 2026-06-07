package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestPromptYN_EOF_ReturnsFalse is the regression guard for AUDIT-035.
// Before the fix, promptYN returned true on EOF — silently auto-accepting
// every filesystem-mutating prompt when stdin is closed (e.g. CI pipelines).
func TestPromptYN_EOF_ReturnsFalse(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader(nil)) // empty reader → immediate EOF
	got, err := promptYN(io.Discard, r, "Create this directory?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("promptYN with EOF input returned true; want false (safe default)")
	}
}

// TestPromptConfirm_EOF_ReturnsFalse confirms promptConfirm also returns false on EOF.
func TestPromptConfirm_EOF_ReturnsFalse(t *testing.T) {
	r := bytes.NewReader(nil) // empty reader → immediate EOF
	got := promptConfirm(io.Discard, r)
	if got {
		t.Error("promptConfirm with EOF input returned true; want false (safe default)")
	}
}

// TestPromptYN_Interactive verifies that interactive usage is not regressed.
func TestPromptYN_Interactive(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"\n", true},       // Enter → yes (default)
		{"y\n", true},      // explicit yes
		{"Y\n", true},      // case-insensitive yes
		{"yes\n", true},    // full word yes
		{"YES\n", true},    // case-insensitive full word
		{"n\n", false},     // explicit no
		{"N\n", false},     // case-insensitive no
		{"no\n", false},    // full word no
		{"NO\n", false},    // case-insensitive full word
		{"maybe\n", false}, // anything else → no
	}

	for _, tc := range cases {
		tc := tc
		t.Run(strings.TrimRight(tc.input, "\n"), func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tc.input))
			got, err := promptYN(io.Discard, r, "test prompt")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("input=%q: got %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestPromptConfirm_Interactive verifies promptConfirm interactive cases are not regressed.
func TestPromptConfirm_Interactive(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"\n", false},    // Enter → no (safe default for [y/N])
		{"n\n", false},
		{"no\n", false},
		{"maybe\n", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(strings.TrimRight(tc.input, "\n"), func(t *testing.T) {
			r := strings.NewReader(tc.input)
			got := promptConfirm(io.Discard, r)
			if got != tc.want {
				t.Errorf("input=%q: got %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsTTY_StringsReader_ReturnsFalse(t *testing.T) {
	if isTTY(strings.NewReader("")) {
		t.Error("strings.Reader is not a TTY; isTTY should return false")
	}
}

func TestIsTTY_DevNull_ReturnsFalse(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTTY(f) {
		t.Error("/dev/null is not a TTY; isTTY should return false")
	}
}
