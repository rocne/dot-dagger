package main

// Prompt conventions:
//   - All prompts default to a SAFE choice when stdin is EOF (no, cancel, skip).
//   - When the user is interactive, [Y/n] vs [y/N] indicates the Enter-default.
//   - [Y/n]: Enter → yes.  [y/N]: Enter → no.
//   - Never auto-accept a destructive or filesystem-mutating action on EOF.

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/rocne/dot-dagger/internal/ui"
)

// promptConfirm prints "Proceed? [y/N]: ", reads a line, and returns true only
// on "y" or "yes". Any other input (including empty / Enter or EOF) prints
// "cancelled" and returns false — callers should return nil when false.
func promptConfirm(out io.Writer, r io.Reader) bool {
	fmt.Fprint(out, "\nProceed? [y/N]: ")
	ans, _ := bufio.NewReader(r).ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))
	if ans != "y" && ans != "yes" {
		ui.Skipf(out, "cancelled")
		return false
	}
	return true
}

// promptDefault prints "msg [default]: " and reads input.
// Returns defaultVal if input is empty.
func promptDefault(w io.Writer, reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", msg, ui.Skip(defaultVal))
	} else {
		fmt.Fprintf(w, "%s: ", msg)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil // EOF — use default
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

// promptYN prints "msg [Y/n]: " and returns true unless user types n/no.
// Interactive: empty input (Enter) defaults to yes.
// Non-interactive: EOF returns false (safe default — never auto-accept on closed stdin).
func promptYN(w io.Writer, reader *bufio.Reader, msg string) (bool, error) {
	fmt.Fprintf(w, "  %s [Y/n]: ", msg)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, nil // EOF → safe default: no
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "" || line == "y" || line == "yes", nil
}

// printField prints a bold field label and a faint description, then a blank line.
func printField(w io.Writer, label, desc string) {
	fmt.Fprintf(w, "\n  %s\n", ui.Key(label))
	fmt.Fprintf(w, "  %s\n", ui.Skip(desc))
}

// fieldPrompt returns the prompt text used after a printField call.
func fieldPrompt() string {
	return "  " + ui.Arrow("›")
}

// promptMenu presents a huh.Select menu and returns the index of the chosen option.
// options must be non-empty. The last option is treated as "Done" by callers.
// Uses huh.Select — renders directly to terminal (accepted exception to cobra I/O routing).
func promptMenu(options []string) (int, error) {
	var chosen string
	if err := huh.NewSelect[string]().
		Title("Select annotation").
		Options(huh.NewOptions(options...)...).
		Value(&chosen).
		Run(); err != nil {
		return 0, err
	}
	for i, o := range options {
		if o == chosen {
			return i, nil
		}
	}
	return 0, fmt.Errorf("promptMenu: unknown selection %q", chosen)
}
