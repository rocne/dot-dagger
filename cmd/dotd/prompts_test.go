package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
		{"\n", false}, // Enter → no (safe default for [y/N])
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

// newPromptTestCmd returns a cobra command wired to the given stdin content
// with discarded output, for driving the huh-backed prompt helpers.
func newPromptTestCmd(stdin string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

// TestPromptMenu_EOF_Aborts guards against the wizard's infinite loop on
// exhausted stdin. In accessible mode, huh fills the selection with its
// default and returns nil on EOF instead of erroring — so a menu presented
// in a loop (dotd annotate) re-selects the first option forever. EOF with no
// input consumed must surface as errUserAborted.
func TestPromptMenu_EOF_Aborts(t *testing.T) {
	cmd := newPromptTestCmd("") // stdin already at EOF
	_, err := promptMenu(cmd, "Select annotation", []string{"When", "After", "Done"})
	if !errors.Is(err, errUserAborted) {
		t.Fatalf("promptMenu with EOF stdin: err=%v, want errUserAborted", err)
	}
}

// TestPromptMenu_SelectsOption verifies a normal piped selection still works,
// including without a trailing newline (input that ends exactly at the answer
// must not be mistaken for an EOF abort).
func TestPromptMenu_SelectsOption(t *testing.T) {
	for name, input := range map[string]string{"newline": "2\n", "no-trailing-newline": "2"} {
		t.Run(name, func(t *testing.T) {
			cmd := newPromptTestCmd(input)
			idx, err := promptMenu(cmd, "Select annotation", []string{"When", "After", "Done"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if idx != 1 {
				t.Errorf("idx = %d, want 1", idx)
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
	if isTTY(f) {
		t.Error("/dev/null is not a TTY; isTTY should return false")
	}
	_ = f.Close()
}
